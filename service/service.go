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
	Term      int64
	Success   bool
	LogLength int64
}

func (r AppendEntriesReply) GetTerm() int64 {
	return r.Term
}

type ClientCommandArgs struct {
	Command string
}

type ClientCommandReply struct {
	Leader    string
	LastIndex int64
}

type AddServerArgs struct {
	NewServer string
}

type AddServerReply struct {
	Status     string
	LeaderHint string
}

type AddServerMessage struct {
	Args    *AddServerArgs
	ReplyCh chan *AddServerReply
}

type RemoveServerArgs struct {
	NewServer string
}

type RemoveServerReply struct {
	Status     string
	LeaderHint string
}

type RemoveServerMessage struct {
	Args    *RemoveServerArgs
	ReplyCh chan *RemoveServerReply
}

type RequestVoteMessage struct {
	Args    *RequestVoteArgs
	ReplyCh chan *RequestVoteReply
}

type AppendEntriesMessage struct {
	Args    *AppendEntriesArgs
	ReplyCh chan *AppendEntriesReply
}

type ClientCommandMessage struct {
	Args    *ClientCommandArgs
	ReplyCh chan *ClientCommandReply
}

type Agent struct {
	RequestVoteCh   chan RequestVoteMessage
	AppendEntriesCh chan AppendEntriesMessage
	ClientCommandCh chan ClientCommandMessage
	AddServerCh     chan AddServerMessage
	RemoveServerCh  chan RemoveServerMessage

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

func (t *Agent) ClientCommand(args *ClientCommandArgs, reply *ClientCommandReply) error {
	replyCh := make(chan *ClientCommandReply)
	t.ClientCommandCh <- ClientCommandMessage{args, replyCh}
	*reply = *(<-replyCh)
	return nil
}

func (t *Agent) AddServer(args *AddServerArgs, reply *AddServerReply) error {
	replyCh := make(chan *AddServerReply)
	t.AddServerCh <- AddServerMessage{args, replyCh}
	*reply = *(<-replyCh)
	return nil
}

func (t *Agent) RemoveServer(args *RemoveServerArgs, reply *RemoveServerReply) error {
	replyCh := make(chan *RemoveServerReply)
	t.RemoveServerCh <- RemoveServerMessage{args, replyCh}
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
	agent.ClientCommandCh = make(chan ClientCommandMessage)
	agent.AddServerCh = make(chan AddServerMessage)
	agent.RemoveServerCh = make(chan RemoveServerMessage)

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
