package machine

import (
	"log"
	"raft/peer"
	"raft/service"
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
	log.Println("Reverting to follower for term: ", state.CurrentTerm)
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

	if !f.state.Servers[f.state.AgentId] {
		log.Println("Not in the configuration")
		return 0
	}

	return 1
}

func (f *Follower) ClientCommand(command string) int64 {
	return -1
}

func (f *Follower) AddServer(message service.AddServerMessage) {
	message.ReplyCh <- &service.AddServerReply{
		Status:     "NOT_LEADER",
		LeaderHint: f.state.Leader,
	}
}

func (f *Follower) RemoveServer(message service.RemoveServerMessage) {
	message.ReplyCh <- &service.RemoveServerReply{
		Status:     "NOT_LEADER",
		LeaderHint: f.state.Leader,
	}
}
