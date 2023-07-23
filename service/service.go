package service

import (
	"net"
	"net/http"
	"net/rpc"
	"strconv"
)

type RequestVoteArgs struct {
	Term         int64
	CandidateId  string
	LastLogTerm  int64
	LastLogIndex int64
}

func (a RequestVoteArgs) GetTerm() int64 {
	return a.Term
}

type AppendEntriesArgs struct {
	Term         int64
	LeaderId     string
	PrevLogIndex int64
	PrevLogTerm  int64
	Entries      []Entry
	LeaderCommit int64
}

func (a AppendEntriesArgs) GetTerm() int64 {
	return a.Term
}

type RequestVoteReply struct {
	Term        int64
	VoteGranted bool
}

func (r RequestVoteReply) GetTerm() int64 {
	return r.Term
}

type AppendEntriesReply struct {
	Term    int64
	Success bool
}

func (r AppendEntriesReply) GetTerm() int64 {
	return r.Term
}

type RequestVoteMessage struct {
	Args    *RequestVoteArgs
	ReplyCh chan *RequestVoteReply
}

type AppendEntriesMessage struct {
	Args    *AppendEntriesArgs
	ReplyCh chan *AppendEntriesReply
}

type Agent struct {
	RequestVoteCh   chan RequestVoteMessage
	AppendEntriesCh chan AppendEntriesMessage

	l net.Listener
}

type Entry struct {
	Term    int64
	Command string
}

func (t *Agent) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) error {
	replyCh := make(chan *AppendEntriesReply)
	t.AppendEntriesCh <- AppendEntriesMessage{args, replyCh}
	*reply = *(<-replyCh)
	return nil
}

func (t *Agent) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
	replyCh := make(chan *RequestVoteReply)
	t.RequestVoteCh <- RequestVoteMessage{args, replyCh}
	*reply = *(<-replyCh)
	return nil
}

func (t *Agent) Stop() {
	t.l.Close()
}

func RunAgent(port int64) (*Agent, error) {
	agent := new(Agent)
	agent.RequestVoteCh = make(chan RequestVoteMessage)
	agent.AppendEntriesCh = make(chan AppendEntriesMessage)

	serveMux := http.NewServeMux()
	server := rpc.NewServer()

	server.Register(agent)
	serveMux.Handle(rpc.DefaultRPCPath, server)
	serveMux.Handle(rpc.DefaultDebugPath, struct{ *rpc.Server }{})

	l, err := net.Listen("tcp", ":"+strconv.FormatInt(port, 10))

	if err != nil {
		return nil, err
	}

	go http.Serve(l, serveMux)

	agent.l = l

	return agent, nil
}
