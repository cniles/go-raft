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

		for _, endpoint := range config.Endpoints {
			state.NextIndex = append(state.NextIndex, state.CommitIndex+1)
			state.Peers = append(state.Peers, peer.MakePeer(endpoint, requestVoteReplyCh, appendEntriesReplyCh))
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

		timeout := time.After(randomTimeout())
		finished := false
		currentState := int64(-1)
		nextState := int64(0)

		for !finished {
			if nextState != currentState {
				currentState = nextState
				config.Behaviors[currentState].Entered(&state)
			}

			select {
			case r := <-serviceAgent.RequestVoteCh:
				_, nextState = config.Behaviors[currentState].RequestVote(r.Args)
				r.ReplyCh <- &service.RequestVoteReply{}
			case r := <-serviceAgent.AppendEntriesCh:
				_ = config.Behaviors[currentState].AppendEntries(r.Args)
				r.ReplyCh <- &service.AppendEntriesReply{}
			case r := <-requestVoteReplyCh:
				nextState = config.Behaviors[currentState].RequestVoteReply(r.VoteGranted)
			case r := <-appendEntriesReplyCh:
				nextState = config.Behaviors[currentState].AppendEntriesReply(r.Success)
			case <-timeout:
				timeout = time.After(randomTimeout())
				nextState = config.Behaviors[currentState].Timeout()
			case <-done:
				finished = true
			}
		}
		serviceAgent.Stop()
	}()
	return done
}
