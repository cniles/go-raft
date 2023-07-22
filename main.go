package main

import (
	"flag"
	"math/rand"
	"raft/machine"
	"raft/state"
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

var portFlag = flag.Int("P", 9990, "the port to listen on")

var minTimeout = flag.Int("t", 1000, "minimum leader timeout")
var maxTimeout = flag.Int("T", 1500, "maximum leader timeout")

var endpointsFlag = flag.String("A", "", "comma separated list of agent endpoints (hostname:port)")

var leaderCh = make(chan []Entry, 100)
var termCh = make(chan int64)

var agentEndpoints []string

var currentTerm int64 = 0

var grantedCh = make(chan bool)
var voted = false
var votedFor string
var entryIndex int64
var entries = []Entry{{"", -1}}

func makeUlid() ulid.ULID {
	t := time.Unix(1000000, 0)
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy)
}

var agentId = makeUlid().String()

// func getVote(endpoint string, voteCh chan int, term int64) {
// 	args := &service.RequestVoteArgs{Term: term, CandidateId: agentId, LastLogTerm: entries[entryIndex].Term, LastLogIndex: entryIndex}
// 	reply := service.RequestVoteReply{}
// 	client, err := rpc.DialHTTP("tcp", endpoint)

// 	if err != nil {
// 		// TODO need to retry
// 		log.Println("dialing:", err)
// 		voteCh <- 0
// 		return
// 	}

// 	err = client.Call("Agent.RequestVote", args, &reply)

// 	if err != nil {
// 		// TODO need to retry
// 		voteCh <- 0
// 		return
// 	}

// 	if reply.VoteGranted {
// 		voteCh <- 1
// 	} else {
// 		voteCh <- 0
// 	}
// }

// func runElection() chan bool {
// 	voteCh := make(chan int)
// 	currentTerm++
// 	votedFor = agentId

// 	resultCh := make(chan bool)

// 	for _, endpoint := range agentEndpoints {
// 		go getVote(endpoint, voteCh, currentTerm)
// 	}

// 	go tallyVotes(voteCh, resultCh)

// 	return resultCh
// }

func main() {
	flag.Parse()

	endpoints := strings.Split(*endpointsFlag, ",")

	config := machine.MachineConfig{
		Port:       int64(*portFlag),
		Endpoints:  endpoints,
		MinTimeout: int64(*minTimeout),
		MaxTimeout: int64(*maxTimeout),
		Behaviors:  []state.StateBehavior{new(machine.Follower), new(machine.Candidate), new(machine.Leader)},
	}

	machine.Run(config)

	for {
	}
}
