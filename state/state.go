package state

import (
	"raft/peer"
	"raft/service"
	"time"
)

type LogRequest struct {
	NextIndex int64
	ReplyCh   chan<- *service.AppendEntriesArgs
}

type ServerChangeRequest struct {
	Action   string
	Endpoint string
	ReplyCh  chan struct{}
}

type StateBehavior interface {
	Entered(state *State)

	GrantedVote() int64
	AppendEntries() int64

	RequestVoteReply(message peer.RequestVoteReplyMessage) int64
	AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64

	ClientCommand(command string) int64

	AddServer(message service.AddServerMessage)
	RemoveServer(message service.RemoveServerMessage)

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

	NextIndex  map[string]int64
	MatchIndex map[string]int64

	Peers map[string]peer.Peer

	Leader string
	// The timeout channel to be read from
	// when muxing in events.
	TimeoutCh <-chan time.Time

	LogRequestCh chan LogRequest

	ServerChangeCh chan ServerChangeRequest

	MakePeer func(endpoint string)
}
