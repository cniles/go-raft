package machine

import (
	"fmt"
	"raft/service"
	"raft/state"
)

type Follower struct {
	state       *state.State
	notTimedOut bool
}

func (f *Follower) Entered(state *state.State) {
	f.notTimedOut = false
	f.state = state
	state.VotedFor = ""
}

func (f *Follower) RequestVote(args *service.RequestVoteArgs) (bool, int64) {
	f.notTimedOut = true

	granted := false

	if f.state.VotedFor == "" || f.state.VotedFor == args.CandidateId {
		commitTerm := f.state.Log[f.state.CommitIndex].Term
		if commitTerm <= args.LastLogTerm {
			fmt.Println("Checking term")
			if commitTerm == args.LastLogTerm {
				granted = args.LastLogIndex >= f.state.CommitIndex
			}
			if commitTerm < args.LastLogTerm {
				granted = true
			}
		}
	}

	return granted, 0
}

func (f *Follower) RequestVoteReply(voteGranted bool) int64 {
	return 0
}

func (f *Follower) AppendEntries(args *service.AppendEntriesArgs) bool {
	f.notTimedOut = true
	return false
}

func (f *Follower) AppendEntriesReply(accepted bool) int64 {
	return 0
}

func (f *Follower) Timeout() int64 {
	if f.notTimedOut {
		f.notTimedOut = false
		return 0
	}
	return 1
}
