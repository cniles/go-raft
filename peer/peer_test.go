package peer

import (
	"raft/service"
	"testing"
	"time"
)

func TestPeerRequestVote(t *testing.T) {
	agent, err := service.RunAgent(9999)
	defer agent.Stop()

	if err != nil {
		t.Fatal("listen error: ", err)
	}

	replyCh := make(chan *service.RequestVoteReply)
	peer := MakePeer("localhost:9999", replyCh, nil)

	go peer.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})

	message := <-agent.RequestVoteCh

	message.ReplyCh <- &service.RequestVoteReply{}

	<-replyCh
}

func TestPeerAppendEntries(t *testing.T) {
	agent, err := service.RunAgent(9999)
	defer agent.Stop()

	if err != nil {
		t.Fatal("listen error: ", err)
	}

	replyCh := make(chan *service.AppendEntriesReply)
	peer := MakePeer("localhost:9999", nil, replyCh)

	go peer.AppendEntries(&service.AppendEntriesArgs{})

	message := <-agent.AppendEntriesCh

	message.ReplyCh <- &service.AppendEntriesReply{}

	<-replyCh
}

func TestPeerRetriesRequestVote(t *testing.T) {
	replyCh := make(chan *service.RequestVoteReply)
	// simulate no connection
	peer := MakePeer("localhost:9999", replyCh, nil)

	go peer.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})

	time.Sleep(50 * time.Millisecond)

	agent, err := service.RunAgent(9999)
	defer agent.Stop()
	if err != nil {
		t.Fatal("listen error: ", err)
	}

	message := <-agent.RequestVoteCh

	message.ReplyCh <- &service.RequestVoteReply{}

	<-replyCh
}

func TestPeerRetriesRequestVote2(t *testing.T) {
	// simulate server restart after connect
	replyCh := make(chan *service.RequestVoteReply)
	peer := MakePeer("localhost:9999", replyCh, nil)
	agent, err := service.RunAgent(9999)

	go peer.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})

	if err != nil {
		t.Fatal("listen error: ", err)
	}

	agent.Stop()

	agent, err = service.RunAgent(9999)
	defer agent.Stop()
	if err != nil {
		t.Fatal("listen error: ", err)
	}

	message := <-agent.RequestVoteCh
	message.ReplyCh <- &service.RequestVoteReply{}

	<-replyCh
}

func TestMultiplePeers(t *testing.T) {
	agent, err := service.RunAgent(9999)
	defer agent.Stop()

	replyCh := make(chan *service.RequestVoteReply)
	peer1 := MakePeer("localhost:9999", replyCh, nil)
	peer2 := MakePeer("localhost:9999", replyCh, nil)

	if err != nil {
		t.Fatal("listen error: ", err)
	}

	go peer1.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})
	go peer1.RequestVote(&service.RequestVoteArgs{Term: 2, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})
	go peer2.RequestVote(&service.RequestVoteArgs{Term: 3, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})
	go peer2.RequestVote(&service.RequestVoteArgs{Term: 4, CandidateId: "1", LastLogTerm: 1, LastLogIndex: 1})

	for i := 0; i < 4; i++ {
		message := <-agent.RequestVoteCh
		message.ReplyCh <- &service.RequestVoteReply{Term: message.Args.Term}
	}

	for i := 0; i < 4; i++ {
		<-replyCh
	}
}
