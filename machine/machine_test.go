package machine

import (
	"fmt"
	"raft/peer"
	"raft/service"
	"raft/state"
	"testing"
)

type TestBehavior struct {
	timeoutCh            chan struct{}
	enteredCh            chan struct{}
	grantedVoteCh        chan struct{}
	requestVoteReplyCh   chan struct{}
	appendEntriesCh      chan struct{}
	appendEntriesReplyCh chan struct{}
	state                *state.State
}

func (b *TestBehavior) Entered(state *state.State) {
	b.enteredCh <- struct{}{}
	b.state = state
	go b.state.Peers[0].RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})
	go b.state.Peers[0].AppendEntries(&service.AppendEntriesArgs{})
}

func (b *TestBehavior) Timeout() int64 {
	b.timeoutCh <- struct{}{}
	return 0
}

func (b *TestBehavior) GrantedVote() int64 {
	b.grantedVoteCh <- struct{}{}
	return 0
}

func (b *TestBehavior) AppendEntries() int64 {
	b.appendEntriesCh <- struct{}{}
	return 0
}

func (b *TestBehavior) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	b.requestVoteReplyCh <- struct{}{}
	return 0
}
func (b *TestBehavior) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	b.appendEntriesReplyCh <- struct{}{}
	return 0
}

func TestMachine(t *testing.T) {
	agent, err := service.RunAgent(9990)

	if err != nil {
		t.Fatal("Could not start agent: ", err)
	}

	replyCh1 := make(chan peer.RequestVoteReplyMessage)
	replyCh2 := make(chan peer.AppendEntriesReplyMessage)

	peer := peer.MakePeer("localhost:9999", replyCh1, replyCh2)

	timeoutCh := make(chan struct{})
	enteredCh := make(chan struct{})
	grantedVoteCh := make(chan struct{})
	requestVoteReplyCh := make(chan struct{})
	appendEntriesCh := make(chan struct{})
	appendEntriesReplyCh := make(chan struct{})

	done := Run(MachineConfig{
		Port:       9999,
		Endpoints:  []string{"localhost:9990"},
		MinTimeout: 10,
		MaxTimeout: 15,
		Behaviors:  []state.StateBehavior{&TestBehavior{timeoutCh, enteredCh, grantedVoteCh, requestVoteReplyCh, appendEntriesCh, appendEntriesReplyCh, nil}},
	})

	go peer.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})
	go peer.AppendEntries(&service.AppendEntriesArgs{Term: 1, LeaderId: "", PrevLogIndex: 1, PrevLogTerm: 1, Entries: []service.Entry{}, LeaderCommit: 1})

	go func() {
		message := <-agent.RequestVoteCh
		message.ReplyCh <- &service.RequestVoteReply{
			Term:        0,
			VoteGranted: false,
		}
	}()
	go func() {
		message := <-agent.AppendEntriesCh
		message.ReplyCh <- &service.AppendEntriesReply{
			Term:    0,
			Success: false,
		}
	}()

	<-enteredCh
	result := 1
	for i := 0; i < 4; i++ {
		select {
		case <-grantedVoteCh:
			fmt.Println("Request vote")
			result *= 2
		case <-appendEntriesCh:
			fmt.Println("Append entries")
			result *= 3
		case <-requestVoteReplyCh:
			fmt.Println("Request vote reply")
			result *= 5
		case <-appendEntriesReplyCh:
			fmt.Println("Append entries reply")
			result *= 7
		}

	}
	<-timeoutCh

	if result != 210 {
		t.Fatal("Wrong interaction")
	}

	done <- struct{}{}
}
