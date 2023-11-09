package machine

import (
	"log"
	"math"
	"raft/peer"
	"raft/service"
	"raft/state"
	"time"
)

type Candidate struct {
	state        *state.State
	awaitingVote map[string]bool
	CandidateId  string
	tally        int64
}

func (c *Candidate) startElection() {
	c.tally = 1

	// Increment current term
	c.state.CurrentTerm++

	// Vote for self
	c.state.VotedFor = c.CandidateId

	c.state.SaveState()

	// Cause machine to reset timeout
	c.state.TimeoutCh = nil
	log.Println("Starting election as ", c.CandidateId, c.state.CurrentTerm, time.Now().UnixMilli())
	// Request votes
	for endpoint, p := range c.state.Peers {
		args :=
			service.RequestVoteArgs{
				Term:         c.state.CurrentTerm,
				CandidateId:  c.CandidateId,
				LastLogTerm:  c.state.Log[c.state.CommitIndex].Term,
				LastLogIndex: c.state.CommitIndex,
			}
		if !c.awaitingVote[endpoint] {
			c.awaitingVote[endpoint] = true
			go func(endpoint string, p peer.Peer) {
				p.RequestVote(&args)
			}(endpoint, p)
		}
	}
}

func (c *Candidate) Entered(state *state.State) {
	c.awaitingVote = make(map[string]bool)
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
	c.awaitingVote[message.Endpoint] = false
	if message.Args.Term == c.state.CurrentTerm {
		if message.Reply.VoteGranted {
			log.Println("-------> Vote granted from ", message.Endpoint)
			c.tally++
		}
	} else {
		log.Println("Didn't count vote", message.Reply.VoteGranted, message.Reply.Term)
	}

	majority := int64(math.Floor(float64(len(c.state.Peers)+1)/2.0) + 1)

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

func (c *Candidate) ClientCommand(command []string) int64 {
	return -1
}

func (c *Candidate) AddServer(message service.AddServerMessage) {
	message.ReplyCh <- &service.AddServerReply{
		Status:     "NOT_LEADER",
		LeaderHint: c.state.Leader,
	}
}

func (c *Candidate) RemoveServer(message service.RemoveServerMessage) {
	message.ReplyCh <- &service.RemoveServerReply{
		Status:     "NOT_LEADER",
		LeaderHint: c.state.Leader,
	}
}
