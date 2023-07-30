package machine

import (
	"log"
	"math"
	"raft/peer"
	"raft/service"
	"raft/state"
	"raft/util"
	"sort"
	"time"
)

type Leader struct {
	LeaderId      string
	MinTimeout    int64
	MaxTimeout    int64
	TimeoutFactor float64

	appending map[string]bool

	state *state.State
}

type ILeader interface {
	updateFollowers()
}

type ByMatch []int64

func (m ByMatch) Len() int           { return len(m) }
func (m ByMatch) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m ByMatch) Less(i, j int) bool { return m[i] > m[j] }

type AppendEntriesArgCh chan *service.AppendEntriesArgs

func (l *Leader) updateFollower(endpoint string, nextIndex int64) {
	lastLogIndex := int64(len(l.state.Log)) - 1
	peer := l.state.Peers[endpoint]
	args := &service.AppendEntriesArgs{
		Term:         l.state.CurrentTerm,
		LeaderId:     l.LeaderId,
		PrevLogIndex: nextIndex - 1,
		PrevLogTerm:  l.state.Log[nextIndex-1].Term,
		Entries:      l.state.Log[nextIndex : lastLogIndex+1],
		LeaderCommit: l.state.CommitIndex,
	}
	// log.Printf("Updating follower with %d logs %d next index (appending %t)", len(args.Entries), nextIndex, l.appending[endpoint])
	if nextIndex <= lastLogIndex && !l.appending[endpoint] {
		if !l.appending[endpoint] {
			l.appending[endpoint] = true
			peer.AppendEntries(args)
		}
	}
}

func (l *Leader) updateFollowers() {
	for endpoint, nextIndex := range l.state.NextIndex {
		l.updateFollower(endpoint, nextIndex)
	}
}

func (l *Leader) handshake() {
	for endpoint, p := range l.state.Peers {
		args := &service.AppendEntriesArgs{
			Term:         l.state.CurrentTerm,
			LeaderId:     l.LeaderId,
			PrevLogIndex: l.state.NextIndex[endpoint] - 1,
			PrevLogTerm:  l.state.Log[l.state.NextIndex[endpoint]-1].Term,
			Entries:      []service.Entry{},
			LeaderCommit: l.state.CommitIndex,
		}
		peer := p

		if !l.appending[endpoint] {
			l.appending[endpoint] = true
			peer.AppendEntries(args)
			// log.Println("Sending handshake")
		} else {
			// log.Println("Skipping handshake")
		}
	}

	min := int64(float64(l.MinTimeout) * l.TimeoutFactor)
	max := int64(float64(l.MaxTimeout) * l.TimeoutFactor)

	// log.Println("Resetting timeout", min, max)
	l.state.TimeoutCh = time.After(util.RandomTimeout(min, max))
}

func (l *Leader) Entered(state *state.State) {
	l.state = state
	log.Println("Elected leader for term: ", state.CurrentTerm)

	l.state.MatchIndex = make(map[string]int64)
	l.state.NextIndex = make(map[string]int64)
	l.appending = make(map[string]bool)
	lastLogIndex := int64(len(l.state.Log) - 1)
	for endpoint := range l.state.Peers {
		l.state.NextIndex[endpoint] = lastLogIndex + 1
		l.state.MatchIndex[endpoint] = 0
	}

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
	lastLogIndex := int64(len(l.state.Log) - 1)
	l.appending[message.Endpoint] = false

	l.state.NextIndex[message.Endpoint] = message.Reply.LogLength + 1
	if message.Reply.Success {
		l.state.MatchIndex[message.Endpoint] = message.Reply.LogLength

		matchIndex := make([]int64, 0, len(l.state.MatchIndex))

		for _, m := range l.state.MatchIndex {
			matchIndex = append(matchIndex, m)
		}
		matchIndex = append(matchIndex, lastLogIndex)

		sort.Sort(ByMatch(matchIndex))
		majority := int64(math.Floor(float64(len(l.state.Peers)+1)/2.0) + 1)
		N := matchIndex[majority-1]
		if N > l.state.CommitIndex && l.state.Log[N].Term == l.state.CurrentTerm {
			// log.Println("Leader has committed up to ", N)
			l.state.CommitIndex = N
		}
	}

	_, ok := l.state.Peers[message.Endpoint]
	if ok {
		l.updateFollower(message.Endpoint, l.state.NextIndex[message.Endpoint])
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

type roundConfig struct {
	endpoint   string
	nextIndex  int64
	rounds     int64
	minTimeout int64
	maxTimeout int64
	logsCh     chan<- state.LogRequest
}

func doRounds(c roundConfig) bool {
	log.Println("Doing rounds for new server!")
	appendEntriesReplyCh := make(chan peer.AppendEntriesReplyMessage)
	newPeer := peer.MakePeer(c.endpoint, nil, appendEntriesReplyCh)
	timeoutCh := time.After(util.RandomTimeout(c.minTimeout, c.maxTimeout))

	for c.rounds > 0 {
		log.Println("Starting round", c.rounds)
		timedOut := false
		progressed := false
		getLogs := func() *service.AppendEntriesArgs {
			ch := make(chan *service.AppendEntriesArgs)
			c.logsCh <- state.LogRequest{
				NextIndex: c.nextIndex,
				ReplyCh:   ch,
			}
			return <-ch
		}
		args := getLogs()
		done := newPeer.AppendEntries(args)
		for !progressed {
			select {
			case replyMessage := <-appendEntriesReplyCh:
				p := c.nextIndex
				c.nextIndex = replyMessage.Reply.LogLength + 1
				if replyMessage.Reply.Success {
					// log.Println("Received success")
					timeoutCh = time.After(util.RandomTimeout(c.minTimeout, c.maxTimeout))
					progressed = true
					args = getLogs()
				} else {
					// log.Println("Round reply failure")
					l := (p - c.nextIndex) + int64(len(args.Entries))
					args = getLogs()
					args.Entries = args.Entries[:l]
					done = newPeer.AppendEntries(args)
				}
			case <-timeoutCh:
				log.Println("round timeout")
				timedOut = true
				timeoutCh = time.After(util.RandomTimeout(c.minTimeout, c.maxTimeout))
			}
			if timedOut && !progressed {
				done <- struct{}{}
				return false
			}
		}

		c.rounds--
	}

	return true
}

func (l *Leader) RemoveServer(message service.RemoveServerMessage) {
	reply := &service.RemoveServerReply{
		Status:     "OK",
		LeaderHint: "",
	}

	_, ok := l.state.Peers[message.Args.NewServer]
	if message.Args.NewServer != l.LeaderId && !ok {
		log.Println("Server not in config")
		message.ReplyCh <- reply
		return
	}

	log.Println("Sending peer request")
	replyCh := make(chan struct{})
	go func() {
		l.state.ServerChangeCh <- state.ServerChangeRequest{
			Action:   "REMOVE",
			Endpoint: message.Args.NewServer,
			ReplyCh:  replyCh,
		}
	}()

	go func() {
		<-replyCh // change committed
		message.ReplyCh <- reply
	}()
}

func (l *Leader) AddServer(message service.AddServerMessage) {
	reply := &service.AddServerReply{
		Status:     "TIMEOUT",
		LeaderHint: l.LeaderId,
	}

	_, ok := l.state.Peers[message.Args.NewServer]
	if message.Args.NewServer == l.LeaderId || ok {
		log.Println("Server already in configuration")
		reply.Status = "OK"
		message.ReplyCh <- reply
	} else {
		c := roundConfig{
			endpoint:   message.Args.NewServer,
			nextIndex:  l.state.CommitIndex + 1,
			rounds:     10,
			minTimeout: l.MinTimeout,
			maxTimeout: l.MaxTimeout,
			logsCh:     l.state.LogRequestCh,
		}
		go func() {
			if doRounds(c) {
				reply.Status = "OK"
				replyCh := make(chan struct{})
				go func() {
					l.state.ServerChangeCh <- state.ServerChangeRequest{
						Action:   "ADD",
						Endpoint: message.Args.NewServer,
						ReplyCh:  replyCh,
					}
				}()
				<-replyCh // change committed
			}
			message.ReplyCh <- reply
		}()
	}
}
