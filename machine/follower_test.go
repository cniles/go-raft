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
	f.GrantedVote()

	if f.notTimedOut != true {
		t.Fatal("GrantedVote should reset timeout")
	}
}

func TestAppendEntriesResetsTimeout(t *testing.T) {
	f := Follower{}
	s := state.State{}
	f.Entered(&s)

	f.AppendEntries()

	if f.notTimedOut != true {
		t.Fatal("AppendEntries should reset timeout")
	}
}
