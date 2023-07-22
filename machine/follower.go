package machine

import (
	"raft/peer"
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

func (f *Follower) GrantedVote() int64 {
	f.notTimedOut = true
	return 0
}

func (f *Follower) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	return 0
}

func (f *Follower) AppendEntries() int64 {
	f.notTimedOut = true
	return 0
}

func (f *Follower) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	return 0
}

func (f *Follower) Timeout() int64 {
	if f.notTimedOut {
		f.notTimedOut = false
		return 0
	}
	return 1
}
