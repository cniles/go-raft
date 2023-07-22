package peer

import (
	"net/rpc"
	"raft/service"
)

type RequestVoteReplyMessage struct {
	args  *service.RequestVoteArgs
	reply *service.RequestVoteReply
}

type AppendEntriesReplyMessage struct {
	args  *service.AppendEntriesArgs
	reply *service.AppendEntriesReply
}

type Peer struct {
	endpoint             string
	requestVoteReplyCh   chan RequestVoteReplyMessage
	appendEntriesReplyCh chan AppendEntriesReplyMessage
}

func MakePeer(endpoint string, requestVoteReplyCh chan RequestVoteReplyMessage, appendEntriesReplyCh chan AppendEntriesReplyMessage) Peer {
	return Peer{endpoint, requestVoteReplyCh, appendEntriesReplyCh}
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

	p.requestVoteReplyCh <- RequestVoteReplyMessage{args, reply}
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

	p.appendEntriesReplyCh <- AppendEntriesReplyMessage{args, reply}
}
