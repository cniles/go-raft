package service

import (
	"fmt"
	"net/rpc"
	"strconv"
	"testing"
)

func makeClient(port int64, t *testing.T) *rpc.Client {
	client, err := rpc.DialHTTP("tcp", ":"+strconv.FormatInt(port, 10))
	if err != nil {
		t.Fatal("Failed to create client: ", err)
	}
	return client
}

func runAgent(port int64, t *testing.T) *Agent {
	agent, err := RunAgent(port)

	if err != nil {
		t.Fatal("listen err:", err)
	}

	return agent
}

func TestRequestVote(t *testing.T) {
	agent := runAgent(9999, t)
	defer agent.Stop()
	client := makeClient(9999, t)
	defer client.Close()

	// Prepare request
	var done chan *rpc.Call = make(chan *rpc.Call, 10)
	var reply = RequestVoteReply{}
	args := RequestVoteArgs{1, "1", 1, 1}

	// Make RPC
	client.Go("Agent.RequestVote", args, &reply, done)

	// Receive message
	message := <-agent.RequestVoteCh

	// Send reply
	sent := RequestVoteReply{1, true}
	message.ReplyCh <- &sent

	// RPC client should have completed
	<-done

	if *message.Args != args {
		t.Fatal("wrong args received.")
	}

	if reply != sent {
		t.Fatal("wrong reply received.")
	}
}

func TestAppendEntries(t *testing.T) {
	agent := runAgent(9999, t)
	defer agent.Stop()
	client := makeClient(9999, t)
	defer client.Close()

	// Prepare request
	var done chan *rpc.Call = make(chan *rpc.Call, 10)
	var reply = AppendEntriesReply{}
	args := AppendEntriesArgs{1, "", 1, 1, []Entry{}, 1}

	// Make RPC
	client.Go("Agent.AppendEntries", args, &reply, done)

	// Receive message
	message := <-agent.AppendEntriesCh
	// Send reply
	sent := AppendEntriesReply{1, true}
	message.ReplyCh <- &sent

	// RPC client should have completed
	<-done

	if reply != sent {
		t.Fatal("wrong reply received.")
	}
}

func TestMultipleAgents(t *testing.T) {
	agent1 := runAgent(9990, t)
	defer agent1.Stop()

	agent2 := runAgent(9991, t)
	defer agent2.Stop()

	client1 := makeClient(9990, t)
	defer client1.Close()

	client2 := makeClient(9991, t)
	defer client2.Close()
}

func TestClientCommand(t *testing.T) {
	agent := runAgent(9998, t)
	defer agent.Stop()
	client := makeClient(9998, t)
	defer client.Close()

	args := &ClientCommandArgs{
		Command: "",
	}
	reply := &ClientCommandReply{}

	done := make(chan *rpc.Call, 10)

	client.Go("Agent.ClientCommand", args, reply, done)

	message := <-agent.ClientCommandCh

	message.ReplyCh <- &ClientCommandReply{}
}
