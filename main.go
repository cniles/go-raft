package main

import (
	"flag"
	"log"
	"math/rand"
	"net/rpc"
	"raft/machine"
	"raft/service"
	"raft/state"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid"
)

type AgentState int

const (
	Follower AgentState = iota
	Candidate
	Leader
)

type Entry struct {
	Command string
	Term    int64
}

var portFlag = flag.Int("p", 9990, "the port to listen on")

var minTimeout = flag.Int("t", 2000, "minimum timeout")
var maxTimeout = flag.Int("T", 2500, "maximum timeout")

var command = flag.String("C", "", "Run a client command instead")
var server = flag.String("S", ":9990", "server to connect to")

var endpointsFlag = flag.String("P", "", "comma separated list of agent endpoints (hostname:port)")

func makeUlid() ulid.ULID {
	t := time.Unix(1000000, 0)
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy)
}

func doServer() {

	endpoints := strings.Split(*endpointsFlag, ",")

	log.Println(endpoints)

	agentId := ":" + strconv.FormatInt(int64(*portFlag), 10)

	config := machine.MachineConfig{
		Port:       int64(*portFlag),
		Endpoints:  endpoints,
		MinTimeout: int64(*minTimeout),
		MaxTimeout: int64(*maxTimeout),
		Behaviors: []state.StateBehavior{
			new(machine.Follower),
			&machine.Candidate{CandidateId: agentId},
			&machine.Leader{LeaderId: agentId, MinTimeout: 1000},
		},
	}

	machine.Run(config)

	for {
	}

}

func doClient() {
	log.Println("Client command")

	done := false

	for !done {
		client, err := rpc.DialHTTP("tcp", *server)
		if err != nil {
			log.Fatal("Failed to dial server:", *server)
		}

		reply := &service.ClientCommandReply{}

		args := &service.ClientCommandArgs{
			Command: *command,
		}

		client.Call("Agent.ClientCommand", args, reply)

		if reply.LastIndex == -1 {
			log.Println("redirect to leader:", reply.Leader)
			*server = reply.Leader
		} else {
			log.Println("Appended to index", reply.LastIndex)
			done = true
		}

		client.Close()
	}
}

func main() {
	flag.Parse()

	if *command != "" {
		doClient()
	} else {
		doServer()
	}
}
