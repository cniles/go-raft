package machine

import (
	"log"
	"raft/peer"
	"raft/service"
	"raft/state"
)

type Candidate struct {
	state          *state.State
	voterResponded []bool
	CandidateId    string
	tally          int64
}

func (c *Candidate) startElection() {
	c.voterResponded = make([]bool, len(c.state.Peers))
	c.tally = 1

	// Increment current term
	c.state.CurrentTerm++

	// Vote for self
	c.state.VotedFor = c.CandidateId

	// Cause machine to reset timeout
	c.state.TimeoutCh = nil
	log.Println("Starting election as ", c.CandidateId)

	// Request votes
	for _, p := range c.state.Peers {
		args :=
			service.RequestVoteArgs{
				Term:         c.state.CurrentTerm,
				CandidateId:  c.CandidateId,
				LastLogTerm:  c.state.Log[c.state.CommitIndex].Term,
				LastLogIndex: c.state.CommitIndex,
			}
		go func(p peer.Peer) {
			p.RequestVote(&args)
		}(p)
	}
}

func (c *Candidate) Entered(state *state.State) {
	c.state = state
	c.startElection()
}

func (c *Candidate) GrantedVote() int64 {
	log.Fatal("GrantedVote but I'm a candidate. Should not occur.")
	return 1
}

func (c *Candidate) AppendEntries() int64 {
	return 0
}

func (c *Candidate) RequestVoteReply(message peer.RequestVoteReplyMessage) int64 {
	if message.Args.Term == c.state.CurrentTerm && !c.voterResponded[message.Peer] {
		if message.Reply.VoteGranted {
			log.Println("-------> Vote granted from ", message.Peer)
			c.tally++
		}
		c.voterResponded[message.Peer] = true
	}

	majority := len(c.state.Peers) + 1

	log.Println("Tally at: ", c.tally, majority)

	if c.tally >= int64(majority) {
		return 2
	}

	return 1
}

func (c *Candidate) AppendEntriesReply(message peer.AppendEntriesReplyMessage) int64 {
	return 1
}

func (c *Candidate) Timeout() int64 {
	c.startElection()
	return 1
}
