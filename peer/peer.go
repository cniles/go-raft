package peer

import (
	"log"
	"net/rpc"
	"raft/service"
	"time"
)

type RequestVoteReplyMessage struct {
	Peer  int64
	Args  *service.RequestVoteArgs
	Reply *service.RequestVoteReply
}

type AppendEntriesReplyMessage struct {
	Peer  int64
	Args  *service.AppendEntriesArgs
	Reply *service.AppendEntriesReply
}

type Peer struct {
	Num                  int64
	endpoint             string
	requestVoteReplyCh   chan RequestVoteReplyMessage
	appendEntriesReplyCh chan AppendEntriesReplyMessage
}

func MakePeer(num int64, endpoint string, requestVoteReplyCh chan RequestVoteReplyMessage, appendEntriesReplyCh chan AppendEntriesReplyMessage) Peer {
	return Peer{num, endpoint, requestVoteReplyCh, appendEntriesReplyCh}
}

func (p *Peer) retryCall(name string, args any, reply any) {
	f := time.Duration(1)

	for {
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

	log.Printf("Received %s reply\n", name)
}

func (p *Peer) RequestVote(args *service.RequestVoteArgs) {
	reply := &service.RequestVoteReply{}
	p.retryCall("Agent.RequestVote", args, reply)
	p.requestVoteReplyCh <- RequestVoteReplyMessage{p.Num, args, reply}
}

func (p *Peer) AppendEntries(args *service.AppendEntriesArgs) {
	reply := &service.AppendEntriesReply{}
	p.retryCall("Agent.AppendEntries", args, reply)
	p.appendEntriesReplyCh <- AppendEntriesReplyMessage{p.Num, args, reply}
}
