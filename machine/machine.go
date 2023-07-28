package machine

import (
	"log"
	"raft/peer"
	"raft/service"
	"raft/state"
	"raft/util"
	"strings"
	"time"
)

type MachineConfig struct {
	Port       int64
	Endpoints  []string
	MinTimeout int64
	MaxTimeout int64
	Behaviors  []state.StateBehavior
	AgentId    string
}

type RpcRequestResponse interface {
	GetTerm() int64
}

type clientRequest struct {
	replyCh  chan *service.ClientCommandReply
	logIndex int64
}

func requestVoteHandler(args *service.RequestVoteArgs, state *state.State) *service.RequestVoteReply {

	reply := &service.RequestVoteReply{
		Term:        state.CurrentTerm,
		VoteGranted: false,
	}

	if args.Term < state.CurrentTerm {
		return reply
	}

	if state.VotedFor == "" || state.VotedFor == args.CandidateId {
		commitTerm := state.Log[state.CommitIndex].Term
		if commitTerm <= args.LastLogTerm {
			if commitTerm == args.LastLogTerm {
				reply.VoteGranted = args.LastLogIndex >= state.CommitIndex
			}
			if commitTerm < args.LastLogTerm {
				reply.VoteGranted = true
			}
		}
	}

	if reply.VoteGranted {
		state.VotedFor = args.CandidateId
	}

	return reply
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func appendEntriesHandler(args *service.AppendEntriesArgs, state *state.State) *service.AppendEntriesReply {
	reply := &service.AppendEntriesReply{
		Term:      state.CurrentTerm,
		Success:   false,
		LogLength: int64(len(state.Log) - 1),
	}

	if args.Term < state.CurrentTerm {
		log.Println("I am a greater term than this request")
		return reply
	}

	state.Leader = args.LeaderId

	if int64(len(state.Log)-1) < args.PrevLogIndex {
		log.Printf("The previous log (%d) does not exist for me (log length %d)", args.PrevLogIndex, len(state.Log)-1)
		return reply
	}

	if state.Log[args.PrevLogIndex].Term != args.PrevLogTerm {
		log.Println("The previous log's term does not match mine")
		return reply
	}

	reply.Success = true

	index := args.PrevLogIndex + 1
	entries := args.Entries

	for {
		if len(entries) == 0 {
			break
		}
		if index >= int64(len(state.Log)) {
			// log.Println("Appending logs")
			state.Log = append(state.Log, entries...)
			break
		} else if state.Log[index].Term != entries[0].Term {
			// log.Println("Truncating logs!")
			state.Log = state.Log[:index]
		} else {
			// log.Println("Replacing log at ", index)
			state.Log[index] = entries[0]
			entries = entries[1:]
			index++
		}
	}

	reply.LogLength = int64(len(state.Log) - 1)

	if args.LeaderCommit > state.CommitIndex {
		newCommitIndex := min(args.LeaderCommit, reply.LogLength)
		// log.Printf("Updating leader commit from %d to %d\n", state.CommitIndex, newCommitIndex)
		state.CommitIndex = newCommitIndex
	}

	return reply
}

func processLogs(input chan service.Entry, output chan int64, index int64) {
	count := 0
	logs := []string{}
	for {
		e := <-input

		logs = append(logs, e.Command)

		count++
		index++

		// log.Printf("Processed log #%d: %s\n", index, e.Command)

		if strings.HasPrefix(e.Command, "print") {
			log.Println("Logs", logs)
		}

		if strings.HasPrefix(e.Command, "count") {
			log.Println("Count", count)
		}
		output <- index
	}
}

func purgePendingVotes(queue []*service.RequestVoteMessage, currentTerm int64) {
	for _, r := range queue {
		log.Println("Purging vote from", r.Args.CandidateId)
		r.ReplyCh <- &service.RequestVoteReply{
			Term:        currentTerm,
			VoteGranted: false,
		}
	}
}

func Run(config MachineConfig) chan struct{} {
	done := make(chan struct{})
	go func() {
		applyLogCh := make(chan service.Entry)
		logAppliedCh := make(chan int64)
		requestVoteReplyCh := make(chan peer.RequestVoteReplyMessage)
		appendEntriesReplyCh := make(chan peer.AppendEntriesReplyMessage)

		pendingVoteRequests := make([]*service.RequestVoteMessage, 0)

		pendingClientRequests := make([]clientRequest, 0)
		pendingServerChangeRequests := make([]state.ServerChangeRequest, 0)
		var uncomittedAddServerRequest *state.ServerChangeRequest

		s := state.State{
			CurrentTerm:    0,
			VotedFor:       "",
			Log:            []service.Entry{{Term: -1, Command: ""}},
			CommitIndex:    0,
			LastApplied:    0,
			NextIndex:      make(map[string]int64),
			MatchIndex:     make(map[string]int64),
			Peers:          make(map[string]peer.Peer),
			ServerChangeCh: make(chan state.ServerChangeRequest),
			LogRequestCh:   make(chan state.LogRequest),
		}

		s.MakePeer = func(endpoint string) {
			s.Peers[endpoint] = peer.MakePeer(endpoint, requestVoteReplyCh, appendEntriesReplyCh)
		}

		go processLogs(applyLogCh, logAppliedCh, s.LastApplied)

		for _, endpoint := range config.Endpoints {
			s.NextIndex[endpoint] = int64(len(s.Log))
			s.MatchIndex[endpoint] = 0
			s.MakePeer(endpoint)
		}

		serviceAgent, err := service.RunAgent(config.Port)

		if err != nil {
			log.Fatal("Could not start agent")
		}

		finished := false
		currentState := int64(-1)
		nextState := int64(0)

		var changeState = func() {
			log.Println("Switching to state: ", nextState)
			currentState = nextState
			config.Behaviors[currentState].Entered(&s)
		}

		var checkTerm = func(term int64) {
			if s.CurrentTerm < term {
				log.Printf("Greater term than ours: leader %d follower %d\n", term, s.CurrentTerm)
				s.CurrentTerm = term
				nextState = 0
				changeState()
			}
		}

		configIndex := 0
		lastChecked := 0

		for !finished {
			if nextState != currentState {
				changeState()
			}

			if s.TimeoutCh == nil {
				s.TimeoutCh = time.After(util.RandomTimeout(config.MinTimeout, config.MaxTimeout))
			}

			// log.Printf("Muxing on state %d term %d leader %s log length %d commit index %d\n", currentState, state.CurrentTerm, state.VotedFor, len(state.Log)-1, state.CommitIndex)
			// log.Println("pending client requests", pendingClientRequests)
			select {
			case r := <-serviceAgent.RequestVoteCh:
				if r.Args.Term < s.CurrentTerm || currentState == 2 || s.VotedFor != "" {
					r.ReplyCh <- &service.RequestVoteReply{
						Term:        s.CurrentTerm,
						VoteGranted: false,
					}
				} else {
					pendingVoteRequests = append(pendingVoteRequests, &r)
				}
			case r := <-serviceAgent.AppendEntriesCh:
				// log.Println("Responding to AppendEntries from ", r.Args.LeaderId)
				reply := appendEntriesHandler(r.Args, &s)
				checkTerm(r.Args.Term)

				if r.Args.Term >= s.CurrentTerm {
					go purgePendingVotes(pendingVoteRequests, s.CurrentTerm)
					pendingVoteRequests = make([]*service.RequestVoteMessage, 0)
					nextState = config.Behaviors[currentState].AppendEntries()
				}
				r.ReplyCh <- reply
			case r := <-serviceAgent.ClientCommandCh:
				// log.Println("Received client command: ", r.Args.Command)
				index := config.Behaviors[currentState].ClientCommand(r.Args.Command)
				if index == -1 {
					r.ReplyCh <- &service.ClientCommandReply{
						Leader:    s.Leader,
						LastIndex: index,
					}
				} else {
					pendingClientRequests = append(pendingClientRequests, clientRequest{
						replyCh:  r.ReplyCh,
						logIndex: index,
					})
				}
			case r := <-serviceAgent.AddServerCh:
				log.Println("Received add server request: ", r.Args.NewServer)
				config.Behaviors[currentState].AddServer(r)
			case r := <-serviceAgent.RemoveServerCh:
				log.Println("Received remove server request: ", r.Args.NewServer)
				config.Behaviors[currentState].RemoveServer(r)
			case r := <-requestVoteReplyCh:
				checkTerm(r.Reply.Term)
				nextState = config.Behaviors[currentState].RequestVoteReply(r)
			case r := <-appendEntriesReplyCh:
				checkTerm(r.Reply.Term)
				nextState = config.Behaviors[currentState].AppendEntriesReply(r)
			case <-s.TimeoutCh:
				// log.Println("Timeout")
				if currentState == 2 {
					purgePendingVotes(pendingVoteRequests, s.CurrentTerm)
					pendingVoteRequests = make([]*service.RequestVoteMessage, 0)
				}
				if len(pendingVoteRequests) > 0 {
					log.Println("Responding to votes")
					r := pendingVoteRequests[0]
					pendingVoteRequests = pendingVoteRequests[1:]
					checkTerm(r.Args.Term)
					reply := requestVoteHandler(r.Args, &s)
					if reply.VoteGranted {
						log.Println("Vote granted for term to: ", s.CurrentTerm, s.VotedFor)
						nextState = config.Behaviors[currentState].GrantedVote()
					}
					r.ReplyCh <- reply
				}
				s.TimeoutCh = nil
				nextState = config.Behaviors[currentState].Timeout()
			case index := <-logAppliedCh:
				// log.Printf("Log applied for index %d\n", index)
				for ; len(pendingClientRequests) > 0 && pendingClientRequests[0].logIndex <= index; pendingClientRequests = pendingClientRequests[1:] {
					// log.Println("Notifying client")
					pendingClientRequests[0].replyCh <- &service.ClientCommandReply{
						Leader:    "",
						LastIndex: pendingClientRequests[0].logIndex,
					}
				}
			case r := <-s.LogRequestCh:
				index := min(r.NextIndex, int64(len(s.Log)))

				args := &service.AppendEntriesArgs{
					Term:         s.CurrentTerm,
					LeaderId:     s.Leader,
					PrevLogIndex: index - 1,
					PrevLogTerm:  s.Log[index-1].Term,
					Entries:      s.Log[index:],
					LeaderCommit: s.CommitIndex,
				}
				r.ReplyCh <- args
			case r := <-s.ServerChangeCh:
				log.Println("Queueing server change")
				pendingServerChangeRequests = append(pendingServerChangeRequests, r)
			case <-done:
				finished = true
			}

			for lastChecked < len(s.Log) {
				if strings.HasPrefix(s.Log[lastChecked].Command, "config") {
					configIndex = lastChecked
				}

				if configIndex > 0 && configIndex == lastChecked {
					endpoints := strings.Split(strings.Split(s.Log[configIndex].Command, " ")[1], ",")
					log.Println("Applying new configuration", endpoints)
					em := make(map[string]bool)
					for _, e := range endpoints {
						em[e] = true
						if e == config.AgentId {
							continue
						}
						if _, ok := s.Peers[e]; !ok {
							log.Println("Added new peer", e)
							s.MakePeer(e)
							s.NextIndex[e] = int64(len(s.Log))
							s.MatchIndex[e] = 0
						}
					}

					for e := range s.Peers {
						if !em[e] {
							log.Println("Removing peer", e)
							delete(s.Peers, e)
							delete(s.NextIndex, e)
							delete(s.MatchIndex, e)
						}
					}
				}
				lastChecked++
			}

			if currentState == 2 {
				if int64(configIndex) <= s.CommitIndex {
					if uncomittedAddServerRequest != nil {
						// log.Println("Replying to server request")
						if uncomittedAddServerRequest.Action == "REMOVE" && uncomittedAddServerRequest.Endpoint == config.AgentId {
							log.Println("Removed from cluster")
							nextState = 0
						}
						uncomittedAddServerRequest.ReplyCh <- struct{}{}
						uncomittedAddServerRequest = nil
					}
				}
				if len(pendingServerChangeRequests) > 0 && uncomittedAddServerRequest == nil {
					log.Println("Changing server config")
					uncomittedAddServerRequest = &pendingServerChangeRequests[0]
					pendingServerChangeRequests = pendingServerChangeRequests[1:]
					endpoint := uncomittedAddServerRequest.Endpoint

					adding := uncomittedAddServerRequest.Action == "ADD"

					if adding {
						s.MakePeer(endpoint)
						s.NextIndex[endpoint] = int64(len(s.Log))
						s.MatchIndex[endpoint] = 0
					} else {
						delete(s.Peers, endpoint)
						delete(s.NextIndex, endpoint)
						delete(s.MatchIndex, endpoint)
					}
					newConfig := []string{}

					if adding || config.AgentId != endpoint {
						newConfig = append(newConfig, config.AgentId)
					}

					for pe := range s.Peers {
						if adding || pe != endpoint {
							newConfig = append(newConfig, pe)
						}
					}

					s.Log = append(s.Log, service.Entry{
						Command: "config " + strings.Join(newConfig, ","),
						Term:    s.CurrentTerm,
					})

					log.Println("New cluster configuration added", newConfig)

					l, ok := config.Behaviors[currentState].(*Leader)

					if !ok {
						log.Fatal("We should have this behavior here.")
					}

					l.updateFollowers()
				}
			}

			if s.CommitIndex > s.LastApplied {
				select {
				case applyLogCh <- s.Log[s.LastApplied+1]:
					s.LastApplied++
				default:
					// currently applying a log so don't wait. we'll be notified when its done
				}
			}

		}
		serviceAgent.Stop()
	}()
	return done
}
