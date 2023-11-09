package state

import (
	"log"
	"raft/peer"
	"raft/persistence"
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

	ClientCommand(command []string) int64

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
	ConfigIndex int64

	// leader only

	NextIndex  map[string]int64
	MatchIndex map[string]int64

	Servers map[string]bool

	Peers map[string]peer.Peer

	Leader string

	AgentId string
	// The timeout channel to be read from
	// when muxing in events.
	TimeoutCh <-chan time.Time

	StateDir string

	LogRequestCh chan LogRequest

	ServerChangeCh chan ServerChangeRequest

	MakePeer func(endpoint string)

	LastTime time.Time

	Persistence persistence.Persistence
}

func (s *State) LogLen() int64 {
	return int64(len(s.Log)) - 1
}

func (s *State) AddLogs(entries ...service.Entry) {
	s.Persistence.AddLogs(entries...)
	s.Log = append(s.Log, entries...)
}

func (s *State) SaveState() {
	s.Persistence.SaveState(persistence.SavedState{
		CurrentTerm: s.CurrentTerm,
		VotedFor:    s.VotedFor,
		CommitIndex: s.CommitIndex,
	})
}

func (s *State) LoadState() {
	s.Log = append(s.Log, s.Persistence.ReadLogs()...)
	ls := s.Persistence.ReadState()
	log.Println("Loaded logs ", s.LogLen())
	s.CommitIndex = ls.CommitIndex
	s.CurrentTerm = ls.CurrentTerm
	s.VotedFor = ls.VotedFor
	log.Println("Loaded state ", ls)
}

func (s *State) TruncateLogs(index int64) {
	s.Log = s.Log[:index]
	s.Persistence.Truncate(index)
}
