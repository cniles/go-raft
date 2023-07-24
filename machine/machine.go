package machine

import (
	"log"
	"math/rand"
	"raft/peer"
	"raft/service"
	"raft/state"
	"time"
)

type MachineConfig struct {
	Port       int64
	Endpoints  []string
	MinTimeout int64
	MaxTimeout int64
	Behaviors  []state.StateBehavior
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
		log.Println("Checking if commited log at least as up to date as candidate", state.CommitIndex)
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
		Term:    state.CurrentTerm,
		Success: false,
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

	log.Printf("AppendEntries success, writing %d log(s)\n", len(args.Entries))

	index := args.PrevLogIndex + 1
	entries := args.Entries

	for {
		if len(entries) == 0 {
			break
		}
		if index >= int64(len(state.Log)) {
			log.Println("Appending all logs")
			state.Log = append(state.Log, entries...)
			break
		} else if state.Log[index].Term != entries[0].Term {
			log.Println("Truncating logs!!!")
			state.Log = state.Log[:index]
		} else {
			log.Println("replacing a log", index)
			state.Log[index] = entries[0]
			entries = entries[1:]
			index++
		}

	}

	log.Println("After", state.Log)

	lastNewEntryIndex := int64(len(state.Log) - 1)

	if args.LeaderCommit > state.CommitIndex {
		newCommitIndex := min(args.LeaderCommit, lastNewEntryIndex)
		log.Printf("Updating leader commit from %d to %d\n", state.CommitIndex, newCommitIndex)
		state.CommitIndex = newCommitIndex
	}

	return reply
}

func processLogs(input chan service.Entry, output chan int64, index int64) {
	for {
		entry := <-input

		// simulate some work
		time.Sleep(10)
		index++
		log.Printf("Processed log #%d: %s\n", index, entry.Command)

		output <- index
	}
}

func Run(config MachineConfig) chan struct{} {
	done := make(chan struct{})

	go func() {
		applyLogCh := make(chan service.Entry)
		logAppliedCh := make(chan int64)

		pendingClientRequests := make([]clientRequest, 0)
		peerCount := len(config.Endpoints)

		state := state.State{
			CurrentTerm: 0,
			VotedFor:    "",
			Log:         []service.Entry{{Term: -1, Command: ""}},
			CommitIndex: 0,
			LastApplied: 0,
			NextIndex:   make([]int64, 0, peerCount),
			MatchIndex:  make([]int64, peerCount),
			Peers:       make([]peer.Peer, 0, peerCount),
		}

		go processLogs(applyLogCh, logAppliedCh, state.LastApplied)

		requestVoteReplyCh := make(chan peer.RequestVoteReplyMessage)
		appendEntriesReplyCh := make(chan peer.AppendEntriesReplyMessage)

		for idx, endpoint := range config.Endpoints {
			state.NextIndex = append(state.NextIndex, state.CommitIndex+1)
			state.Peers = append(state.Peers, peer.MakePeer(int64(idx), endpoint, requestVoteReplyCh, appendEntriesReplyCh))
		}

		serviceAgent, err := service.RunAgent(config.Port)

		if err != nil {
			log.Fatal("Could not start agent")
		}

		randomTimeout := func() time.Duration {
			d := config.MaxTimeout - config.MinTimeout
			rnd := rand.Int63n(d)
			timeout := rnd + config.MinTimeout
			return time.Duration(timeout) * time.Millisecond
		}

		finished := false
		currentState := int64(-1)
		nextState := int64(0)

		var changeState = func() {
			log.Println("Switching to state: ", nextState)
			currentState = nextState
			config.Behaviors[currentState].Entered(&state)
		}

		var checkTerm = func(term int64) {
			log.Println("Checking term: ", term, state.CurrentTerm)
			if state.CurrentTerm < term {
				log.Printf("Greater term than ours: leader %d follower %d\n", term, state.CurrentTerm)
				state.CurrentTerm = term
				nextState = 0
				changeState()
			}
		}

		for !finished {
			if nextState != currentState {
				changeState()
			}

			if state.TimeoutCh == nil {
				log.Print("Timeout reset")
				state.TimeoutCh = time.After(randomTimeout())
			}

			log.Printf("Muxing on state %d term %d leader %s log length %d\n", currentState, state.CurrentTerm, state.VotedFor, len(state.Log)-1)
			log.Println(state.Log)
			select {
			case r := <-serviceAgent.RequestVoteCh:
				log.Println("Responding to request for vote from ", r.Args.CandidateId)
				checkTerm(r.Args.Term)
				reply := requestVoteHandler(r.Args, &state)
				if reply.VoteGranted {
					log.Println("Vote granted for term to: ", state.CurrentTerm, state.VotedFor)
					nextState = config.Behaviors[currentState].GrantedVote()
				}
				r.ReplyCh <- reply
			case r := <-serviceAgent.AppendEntriesCh:
				log.Println("Responding to AppendEntries from ", r.Args.LeaderId)
				reply := appendEntriesHandler(r.Args, &state)
				checkTerm(r.Args.Term)
				if r.Args.Term >= state.CurrentTerm {
					log.Println("Leader sent append entries")
					nextState = config.Behaviors[currentState].AppendEntries()
				}
				r.ReplyCh <- reply
			case r := <-serviceAgent.ClientCommandCh:
				log.Println("Received client command: ", r.Args.Command)
				index := config.Behaviors[currentState].ClientCommand(r.Args.Command)
				if index == -1 {
					r.ReplyCh <- &service.ClientCommandReply{
						Leader:    state.VotedFor,
						LastIndex: index,
					}
				} else {
					pendingClientRequests = append(pendingClientRequests, clientRequest{
						replyCh:  r.ReplyCh,
						logIndex: index,
					})
				}
			case r := <-requestVoteReplyCh:
				checkTerm(r.Reply.Term)
				nextState = config.Behaviors[currentState].RequestVoteReply(r)
			case r := <-appendEntriesReplyCh:
				checkTerm(r.Reply.Term)
				nextState = config.Behaviors[currentState].AppendEntriesReply(r)
			case <-state.TimeoutCh:
				state.TimeoutCh = nil
				nextState = config.Behaviors[currentState].Timeout()
			case index := <-logAppliedCh:
				log.Printf("Log applied for index %d\n", index)
				log.Println("Pending requests: ", pendingClientRequests)
				for ; len(pendingClientRequests) > 0 && pendingClientRequests[0].logIndex <= index; pendingClientRequests = pendingClientRequests[1:] {
					pendingClientRequests[0].replyCh <- &service.ClientCommandReply{
						Leader:    "",
						LastIndex: pendingClientRequests[0].logIndex,
					}
				}
			case <-done:
				finished = true
			}

			log.Printf("Checking if we need to apply new logs: %d > %d\n", state.CommitIndex, state.LastApplied)
			if state.CommitIndex > state.LastApplied {
				select {
				case applyLogCh <- state.Log[state.LastApplied+1]:
					state.LastApplied++
				default:
					// currently applying a log so don't wait. we'll be notified when its done
				}
			}

		}
		serviceAgent.Stop()
	}()
	return done
}
