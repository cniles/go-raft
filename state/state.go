package state

import (
	"raft/peer"
	"raft/service"
	"time"
)

type StateBehavior interface {
	Entered(state *State)

	RequestVote(args *service.RequestVoteArgs) (bool, int64)
	RequestVoteReply(voteGranted bool) int64

	AppendEntries(args *service.AppendEntriesArgs) bool
	AppendEntriesReply(accepted bool) int64

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
	TimeoutCh chan time.Time
}
