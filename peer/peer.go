package peer

import (
	"net/rpc"
	"raft/service"
)

type RequestVoteReplyMessage struct {
	peer  int64
	args  *service.RequestVoteArgs
	reply *service.RequestVoteReply
}

type AppendEntriesReplyMessage struct {
	peer  int64
	args  *service.AppendEntriesArgs
	reply *service.AppendEntriesReply
}

type Peer struct {
	num                  int64
	endpoint             string
	requestVoteReplyCh   chan RequestVoteReplyMessage
	appendEntriesReplyCh chan AppendEntriesReplyMessage
}

func MakePeer(num int64, endpoint string, requestVoteReplyCh chan RequestVoteReplyMessage, appendEntriesReplyCh chan AppendEntriesReplyMessage) Peer {
	return Peer{num, endpoint, requestVoteReplyCh, appendEntriesReplyCh}
}

func (p *Peer) RequestVote(args *service.RequestVoteArgs) {
	reply := &service.RequestVoteReply{}

	for {
		client, err := rpc.DialHTTP("tcp", p.endpoint)
		defer client.Close()
		if err != nil {
			continue
		}
		client.Call("Agent.RequestVote", args, reply)
		if err != nil {
			client.Close()
			continue
		}
		break
	}

	p.requestVoteReplyCh <- RequestVoteReplyMessage{p.num, args, reply}
}

func (p *Peer) AppendEntries(args *service.AppendEntriesArgs) {
	reply := &service.AppendEntriesReply{}

	for {
		client, err := rpc.DialHTTP("tcp", p.endpoint)
		defer client.Close()
		if err != nil {
			continue
		}
		client.Call("Agent.AppendEntries", args, reply)
		if err != nil {
			client.Close()
			continue
		}
		break
	}

	p.appendEntriesReplyCh <- AppendEntriesReplyMessage{p.num, args, reply}
}
