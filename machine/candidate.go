package machine

import (
	"log"
	"raft/peer"
	"raft/state"
)

type Candidate struct {
	state          *state.State
	term           int64
	voterResponded []bool
}

func (c *Candidate) Entered(state *state.State) {
	c.voterResponded = make([]bool, 12)

}

func (c *Candidate) GrantedVote() int64 {
	log.Fatal("Should not occur.")
	return 1
}

func (c *Candidate) AppendEntries() int64 {
	return 0
}

func (c *Candidate) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	return 1
}

func (c *Candidate) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	return 1
}

func (c *Candidate) Timeout() int64 {
	// TODO increment currentState.  Start new election
	return 1
}
