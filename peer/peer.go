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

func (p *Peer) RequestVote(args *service.RequestVoteArgs) {
	reply := &service.RequestVoteReply{}
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

		client.Call("Agent.RequestVote", args, reply)
		client.Close()
		if err != nil {
			time.Sleep(10 * f * time.Millisecond)
			continue
		}
		break
	}

	log.Print("Received RequestVote reply: ", reply.Term, reply.VoteGranted)
	p.requestVoteReplyCh <- RequestVoteReplyMessage{p.Num, args, reply}
}

func (p *Peer) AppendEntries(args *service.AppendEntriesArgs) {
	reply := &service.AppendEntriesReply{}

	f := time.Duration(0)
	for {
		f++
		client, err := rpc.DialHTTP("tcp", p.endpoint)
		defer client.Close()
		if err != nil {
			time.Sleep(100 * f * time.Millisecond)
			continue
		}
		client.Call("Agent.AppendEntries", args, reply)
		if err != nil {
			client.Close()
			time.Sleep(10 * f * time.Millisecond)
			continue
		}
		break
	}

	log.Print("Received AppendEntries reply: ", reply.Term, reply.Success)
	p.appendEntriesReplyCh <- AppendEntriesReplyMessage{p.Num, args, reply}
}
