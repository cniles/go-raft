package main

import (
	"flag"
	"log"
	"math/rand"
	"raft/machine"
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

var minTimeout = flag.Int("t", 5000, "minimum timeout")
var maxTimeout = flag.Int("T", 6000, "maximum timeout")

var endpointsFlag = flag.String("P", "", "comma separated list of agent endpoints (hostname:port)")

func makeUlid() ulid.ULID {
	t := time.Unix(1000000, 0)
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy)
}

func main() {
	flag.Parse()

	endpoints := strings.Split(*endpointsFlag, ",")

	log.Println(endpoints)

	agentId := strconv.FormatInt(int64(*portFlag), 10)

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
