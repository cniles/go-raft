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

func requestVoteHandler(args *service.RequestVoteArgs, state *state.State) *service.RequestVoteReply {
	granted := false

	if state.VotedFor == "" || state.VotedFor == args.CandidateId {
		commitTerm := state.Log[state.CommitIndex].Term
		if commitTerm <= args.LastLogTerm {
			if commitTerm == args.LastLogTerm {
				granted = args.LastLogIndex >= state.CommitIndex
			}
			if commitTerm < args.LastLogTerm {
				granted = true
			}
		}
	}

	if granted {
		state.VotedFor = args.CandidateId
	}

	return &service.RequestVoteReply{
		Term:        state.CurrentTerm,
		VoteGranted: granted,
	}
}

func appendEntriesHandler(args *service.AppendEntriesArgs, state *state.State) *service.AppendEntriesReply {
	return &service.AppendEntriesReply{
		Term:    state.CurrentTerm,
		Success: false,
	}
}

func Run(config MachineConfig) chan struct{} {
	done := make(chan struct{})
	go func() {
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

		var checkTerm = func(r RpcRequestResponse) {
			if r.GetTerm() > state.CurrentTerm {
				log.Println("Greater term than ours")
				state.CurrentTerm = r.GetTerm()
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

			log.Printf("Muxing on state %d term %d leader %s\n", currentState, state.CurrentTerm, state.VotedFor)

			select {
			case r := <-serviceAgent.RequestVoteCh:
				log.Println("Responding to request for vote from ", r.Args.CandidateId)
				checkTerm(r.Args)
				reply := requestVoteHandler(r.Args, &state)
				if reply.VoteGranted {
					log.Println("Vote granted for term to: ", state.CurrentTerm, state.VotedFor)
					nextState = config.Behaviors[currentState].GrantedVote()
				}
				r.ReplyCh <- reply
			case r := <-serviceAgent.AppendEntriesCh:
				log.Println("Responding to AppendEntries from ", r.Args.LeaderId)
				checkTerm(r.Args)
				reply := appendEntriesHandler(r.Args, &state)
				if r.Args.Term == state.CurrentTerm {
					nextState = config.Behaviors[currentState].AppendEntries()
				}
				r.ReplyCh <- reply
			case r := <-requestVoteReplyCh:
				checkTerm(r.Args)
				nextState = config.Behaviors[currentState].RequestVoteReply(r)
			case r := <-appendEntriesReplyCh:
				checkTerm(r.Args)
				nextState = config.Behaviors[currentState].AppendEntriesReply(r)
			case <-state.TimeoutCh:
				state.TimeoutCh = nil
				nextState = config.Behaviors[currentState].Timeout()
			case <-done:
				finished = true
			}
		}
		serviceAgent.Stop()
	}()
	return done
}
