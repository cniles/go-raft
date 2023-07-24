package machine

import (
	"log"
	"math"
	"raft/peer"
	"raft/service"
	"raft/state"
	"sort"
	"time"
)

type Leader struct {
	state      *state.State
	LeaderId   string
	MinTimeout int64
}

type ByMatch []int64

func (m ByMatch) Len() int           { return len(m) }
func (m ByMatch) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m ByMatch) Less(i, j int) bool { return m[i] > m[j] }

func (l *Leader) updateFollowers() {
	lastLogIndex := int64(len(l.state.Log) - 1)
	log.Println("Updating followers: ", l.state.NextIndex)
	log.Println("Updating followers to at most ", lastLogIndex)
	for i, nextIndex := range l.state.NextIndex {
		if nextIndex <= lastLogIndex {
			log.Printf("Updating client: %d <= %d.", nextIndex, lastLogIndex)
			peer := l.state.Peers[i]
			args := &service.AppendEntriesArgs{
				Term:         l.state.CurrentTerm,
				LeaderId:     l.LeaderId,
				PrevLogIndex: nextIndex - 1,
				PrevLogTerm:  l.state.Log[nextIndex-1].Term,
				Entries:      l.state.Log[nextIndex:],
				LeaderCommit: l.state.CommitIndex,
			}
			go peer.AppendEntries(args)
		}
	}
}

func (l *Leader) handshake() {
	log.Println("Sending heartbeat")
	for i, p := range l.state.Peers {
		args := &service.AppendEntriesArgs{
			Term:         l.state.CurrentTerm,
			LeaderId:     l.LeaderId,
			PrevLogIndex: l.state.NextIndex[i] - 1,
			PrevLogTerm:  l.state.Log[l.state.NextIndex[i]-1].Term,
			Entries:      []service.Entry{},
			LeaderCommit: l.state.CommitIndex,
		}
		peer := p
		go peer.AppendEntries(args)
	}
	duration := time.Duration(l.MinTimeout) * time.Millisecond
	l.state.TimeoutCh = time.After(duration)
}

func (l *Leader) Entered(state *state.State) {
	l.state = state
	log.Println("Elected leader for term: ", state.CurrentTerm)

	peerCount := len(l.state.Peers)

	l.state.MatchIndex = make([]int64, peerCount)
	l.state.NextIndex = make([]int64, peerCount)
	lastLogIndex := int64(len(l.state.Log) - 1)
	for i := range l.state.NextIndex {
		l.state.NextIndex[i] = lastLogIndex + 1
	}

	log.Println("Initialized NextIndex: ", l.state.NextIndex)

	l.handshake()
}

func (l *Leader) GrantedVote() int64 {
	log.Fatal("Leader should not have granted a vote.")
	return -1
}

func (l *Leader) AppendEntries() int64 {
	log.Fatal("Leader should not receive request to append entries.")
	return -1
}

func (l *Leader) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	return 2
}

func (l *Leader) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	clientLastLogIndex := message.Args.PrevLogIndex + int64(len(message.Args.Entries))
	lastLogIndex := int64(len(l.state.Log) - 1)

	if message.Reply.Success {
		l.state.MatchIndex[message.Peer] = clientLastLogIndex
		l.state.NextIndex[message.Peer] = clientLastLogIndex + 1

		matchIndex := l.state.MatchIndex[:]
		matchIndex = append(matchIndex, lastLogIndex)

		sort.Sort(ByMatch(matchIndex))
		majority := int64(math.Floor(float64(len(l.state.Peers)+1)/2.0) + 1)

		N := matchIndex[majority-1]
		log.Println("Match indices: ", matchIndex)
		log.Println("Calculated N to be: ", N)

		if N > l.state.CommitIndex && l.state.Log[N].Term == l.state.CurrentTerm {
			log.Println("Leader has committed up to ", N)
			l.state.CommitIndex = N
		}
	} else {
		// will have reverted to follower if term < currentTerm so logs are out of date
		log.Println("Decrementing peer to", l.state.NextIndex[message.Peer]-1)
		l.state.NextIndex[message.Peer]--
		l.updateFollowers()
	}

	return 2
}

func (l *Leader) Timeout() int64 {
	l.handshake()
	return 2
}

func (l *Leader) ClientCommand(command string) int64 {
	l.state.Log = append(l.state.Log, service.Entry{
		Term:    l.state.CurrentTerm,
		Command: command,
	})

	l.updateFollowers()

	return int64(len(l.state.Log) - 1)
}
