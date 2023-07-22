package machine

import (
	"log"
	"raft/peer"
	"raft/state"
)

type Leader struct {
	state *state.State
}

func (l *Leader) Entered(state *state.State) {
	// TODO run election
}

func (l *Leader) GrantedVote() int64 {
	log.Fatal("Should not occur.")
	return 1
}

func (l *Leader) AppendEntries() int64 {
	return 0
}

func (l *Leader) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	return 1
}

func (l *Leader) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	return 1
}

func (l *Leader) Timeout() int64 {
	// TODO increment currentState.  Start new election
	return 1
}
