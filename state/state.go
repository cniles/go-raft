package state

import (
	"raft/peer"
	"raft/service"
	"time"
)

type StateBehavior interface {
	Entered(state *State)

	GrantedVote() int64
	AppendEntries() int64

	RequestVoteReply(message peer.RequestVoteReplyMessage) int64
	AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64

	Timeout() int64
}

// protected state
type State struct {
	CurrentTerm int64
	VotedFor    string
	Log         []service.Entry

	CommitIndex int64
	LastApplied int64

	// leader only
	NextIndex  []int64
	MatchIndex []int64

	Peers []peer.Peer

	// The timeout channel to be read from
	// when muxing in events.
	TimeoutCh <-chan time.Time
}
