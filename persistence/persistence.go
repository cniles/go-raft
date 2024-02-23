package persistence

import "raft/service"

type SavedState struct {
	CurrentTerm int64
	VotedFor    string
	CommitIndex int64
}

type Persistence interface {
	AddLogs(entry ...service.Entry)
	SaveState(state SavedState)
	Truncate(index int64)
	ReadLogs() []service.Entry
	ReadState() SavedState
}
