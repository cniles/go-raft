package machine

import (
	"log"
	"raft/peer"
	"raft/service"
	"raft/state"
	"time"
)

type Leader struct {
	state      *state.State
	LeaderId   string
	MinTimeout int64
}

func (l *Leader) handshake() {
	log.Println("Sending heartbeat")
	for _, p := range l.state.Peers {

		go func(p peer.Peer) {
			p.AppendEntries(&service.AppendEntriesArgs{
				Term:         l.state.CurrentTerm,
				LeaderId:     l.LeaderId,
				PrevLogIndex: l.state.CommitIndex,
				PrevLogTerm:  l.state.Log[l.state.CommitIndex].Term,
				Entries:      []service.Entry{},
				LeaderCommit: l.state.CommitIndex,
			})
		}(p)
	}
	duration := time.Duration(l.MinTimeout) * time.Millisecond
	l.state.TimeoutCh = time.After(duration)
}

func (l *Leader) Entered(state *state.State) {
	l.state = state
	log.Println("Elected leader for term: ", state.CurrentTerm)
	l.handshake()
}

func (l *Leader) GrantedVote() int64 {
	log.Fatal("Should not occur.")
	return 2
}

func (l *Leader) AppendEntries() int64 {
	return 2
}

func (l *Leader) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	return 2
}

func (l *Leader) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	return 2
}

func (l *Leader) Timeout() int64 {
	l.handshake()
	return 2
}
