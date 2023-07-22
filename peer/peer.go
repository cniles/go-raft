package peer

import (
	"net/rpc"
	"raft/service"
)

type Peer struct {
	endpoint             string
	requestVoteReplyCh   chan *service.RequestVoteReply
	appendEntriesReplyCh chan *service.AppendEntriesReply
}

func MakePeer(endpoint string, requestVoteReplyCh chan *service.RequestVoteReply, appendEntriesReplyCh chan *service.AppendEntriesReply) Peer {
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

	p.requestVoteReplyCh <- reply
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

	p.appendEntriesReplyCh <- reply
}
