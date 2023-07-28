package peer

import (
	"net/rpc"
	"raft/service"
	"time"
)

type RequestVoteReplyMessage struct {
	Endpoint   string
	Args       *service.RequestVoteArgs
	Reply      *service.RequestVoteReply
	Terminated bool
}

type AppendEntriesReplyMessage struct {
	Endpoint   string
	Args       *service.AppendEntriesArgs
	Reply      *service.AppendEntriesReply
	Terminated bool
}

type Peer struct {
	endpoint             string
	requestVoteReplyCh   chan RequestVoteReplyMessage
	appendEntriesReplyCh chan AppendEntriesReplyMessage
}

func MakePeer(endpoint string, requestVoteReplyCh chan RequestVoteReplyMessage, appendEntriesReplyCh chan AppendEntriesReplyMessage) Peer {
	return Peer{endpoint, requestVoteReplyCh, appendEntriesReplyCh}
}

func (p *Peer) retryCall(name string, args any, reply any, terminate <-chan struct{}) bool {
	f := time.Duration(1)

	for {
		select {
		case <-terminate:
			return true
		default:
		}
		if f > 16 {
			f = 20
		}
		client, err := rpc.DialHTTP("tcp", p.endpoint)
		if err != nil {
			time.Sleep(100 * f * time.Millisecond)
			f = f * 2
			continue
		}

		client.Call(name, args, reply)
		client.Close()
		if err != nil {
			time.Sleep(10 * f * time.Millisecond)
			continue
		}
		break
	}

	// log.Printf("Received %s reply\n", name)
	return false

}

func (p *Peer) RequestVote(args *service.RequestVoteArgs) chan<- struct{} {
	terminate := make(chan struct{})
	reply := &service.RequestVoteReply{}
	terminated := p.retryCall("Agent.RequestVote", args, reply, terminate)
	p.requestVoteReplyCh <- RequestVoteReplyMessage{p.endpoint, args, reply, terminated}
	return terminate
}

func (p *Peer) AppendEntries(args *service.AppendEntriesArgs) chan<- struct{} {
	terminate := make(chan struct{})
	reply := &service.AppendEntriesReply{}
	go func() {
		terminated := p.retryCall("Agent.AppendEntries", args, reply, terminate)
		p.appendEntriesReplyCh <- AppendEntriesReplyMessage{p.endpoint, args, reply, terminated}
	}()
	return terminate
}
