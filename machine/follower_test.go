package machine

import (
	"raft/service"
	"raft/state"
	"testing"
)

func TestFollowerTimeout(t *testing.T) {
	f := Follower{}

	result := f.Timeout()

	if result != 1 {
		t.Fatal("Should have timed out")
	}
}

func TestFollowerEntered(t *testing.T) {
	f := Follower{notTimedOut: true}
	s := state.State{}

	s.VotedFor = "foo"
	f.Entered(&s)

	if s.VotedFor != "" {
		t.Fatal("VotedFor should be reset")
	}

	if f.notTimedOut {
		t.Fatal("notTimedOut should be reset")
	}
}

func TestFollowerNoTimeout(t *testing.T) {
	f := Follower{}
	s := state.State{}
	f.Entered(&s)

	f.notTimedOut = true

	result := f.Timeout()

	if result != 0 {
		t.Fatal("Should not have timed out")
	}

	if f.notTimedOut {
		t.Fatal("Should have reset notTimedOut")
	}
}

func TestRequestVoteResetsTimeout(t *testing.T) {
	f := Follower{}
	s := state.State{}
	s.Log = []service.Entry{{Term: -1}}

	f.Entered(&s)
	f.RequestVote(&service.RequestVoteArgs{})

	if f.notTimedOut != true {
		t.Fatal("RequestVote should reset timeout")
	}
}

func TestAppendEntriesResetsTimeout(t *testing.T) {
	f := Follower{}
	s := state.State{}
	f.Entered(&s)

	f.AppendEntries(&service.AppendEntriesArgs{})

	if f.notTimedOut != true {
		t.Fatal("AppendEntries should reset timeout")
	}
}

func TestRequestVoteGrant(t *testing.T) {
	f := Follower{}
	s := state.State{}
	s.Log = []service.Entry{{Term: 1}}

	f.Entered(&s)
	granted, nextState := f.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "foo", LastLogTerm: 1, LastLogIndex: 1})

	if granted != true {
		t.Fatal("Should have granted")
	}

	if nextState != 0 {
		t.Fatal("Should have remained candidate")
	}

	granted, nextState = f.RequestVote(&service.RequestVoteArgs{Term: 1, CandidateId: "foo", LastLogTerm: 2, LastLogIndex: 1})

	if granted != true {
		t.Fatal("Should have granted")
	}

	if nextState != 0 {
		t.Fatal("Should have remained candidate")
	}
}

func TestRequestVoteDenied(t *testing.T) {
	f := Follower{}
	s := state.State{}

	f.Entered(&s)
}
