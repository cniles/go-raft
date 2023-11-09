package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"os"
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

var minTimeout = flag.Int("t", 2500, "minimum timeout")
var maxTimeout = flag.Int("T", 3000, "maximum timeout")

var command = flag.String("C", "", "Invoke an RPC")
var server = flag.String("S", ":9990", "server to connect to")
var message = flag.String("m", "hello world", "message to append")
var batchSize = flag.Int("b", 100, "size of client batches for benchmarking")

var total = flag.Int("r", 75000, "total number of benchmark requests")
var clients = flag.Int("l", 50, "number of clients for benchmark")
var stateDir = flag.String("d", os.Getenv("HOME")+string(os.PathSeparator)+".goraft", "location to store state (logs, etc)")

var endpointsFlag = flag.String("c", "", "comma separated list of agent endpoints (hostname:port)")

func makeUlid() ulid.ULID {
	t := time.Unix(1000000, 0)
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy)
}

func doServer() {
	endpoints := []string{}
	if *endpointsFlag != "" {
		endpoints = strings.Split(*endpointsFlag, ",")
	}

	log.Println(endpoints)

	agentId := ":" + strconv.FormatInt(int64(*portFlag), 10)

	config := machine.MachineConfig{
		AgentId:    agentId,
		Port:       int64(*portFlag),
		Endpoints:  endpoints,
		MinTimeout: int64(*minTimeout),
		MaxTimeout: int64(*maxTimeout),
		Behaviors: []state.StateBehavior{
			new(machine.Follower),
			&machine.Candidate{CandidateId: agentId},
			&machine.Leader{
				LeaderId:      agentId,
				MinTimeout:    int64(*minTimeout),
				MaxTimeout:    int64(*maxTimeout),
				TimeoutFactor: 0.1,
			},
		},
		StateDir: *stateDir,
	}

	machine.Run(config)

	for {
	}

}

func clientCommand(command []string, client *rpc.Client) string {
	reply := &service.ClientCommandReply{}
	args := &service.ClientCommandArgs{
		Command: command,
	}

	error := client.Call("Agent.ClientCommand", args, reply)

	if error != nil {
		log.Fatal("error from rpc client", error)
	}

	if reply.LastIndex == -1 {
		return reply.Leader
	}

	return ""
}

func addServer(server string, client *rpc.Client) string {
	reply := &service.AddServerReply{}
	args := &service.AddServerArgs{
		NewServer: server,
	}

	log.Println("Running command")

	err := client.Call("Agent.AddServer", args, reply)

	if err != nil {
		log.Fatal("Endpoint returned error", err)
	}

	log.Println("Replied with:", reply.Status)

	if reply.Status == "NOT_LEADER" {
		return reply.LeaderHint
	}

	if reply.Status == "TIMEOUT" {
		log.Println("Timed out updating new machine")
	} else if reply.Status == "OK" {
		log.Println("Machine added")
	} else {
		log.Println("Unexpected result:", reply.Status)
	}

	return ""
}

func removeServer(server string, client *rpc.Client) string {
	reply := &service.RemoveServerReply{}
	args := &service.RemoveServerArgs{
		NewServer: server,
	}

	err := client.Call("Agent.RemoveServer", args, reply)

	if err != nil {
		log.Fatal("Endpoint returned an error", err)
	}

	if reply.LeaderHint != "" {
	}

	if reply.Status == "OK" {
		log.Println("Server removed")
		return ""
	} else {
		log.Println("Returned: ", reply.Status)
	}

	return reply.LeaderHint
}

func doCommands(count int, clientNum int, done chan struct{}) {
	log.Printf("%d starting %d requests", clientNum, count)
	client, err := rpc.DialHTTP("tcp", *server)
	if err != nil {
		log.Fatal("Could not dial client")
	}
	defer client.Close()
	for j := 0; j < count / *batchSize; j++ {
		for {
			commands := make([]string, *batchSize)
			for b := 0; b < *batchSize; b++ {
				commands[b] = fmt.Sprintf("log %d %d %d", j, clientNum, b)
			}
			leader := clientCommand(commands, client)
			if leader == "" {
				break
			}
			client, err = rpc.DialHTTP("tcp", leader)
		}
	}
	log.Printf("Client %d done", clientNum)
	done <- struct{}{}
}

func benchmark() {
	total := *total
	clients := *clients
	count := total / clients
	done := make(chan struct{})
	for i := 0; i < clients; i++ {
		go doCommands(count, i, done)
	}

	for i := 0; i < clients; i++ {
		<-done
	}
}

func doClient() {
	done := false
	for !done {
		client, err := rpc.DialHTTP("tcp", *server)
		if err != nil {
			log.Fatal("Failed to dial server:", *server)
		}
		leader := ""

		switch *command {
		case "clientcommand":
			leader = clientCommand([]string{*message}, client)
		case "addserver":
			leader = addServer(*message, client)
		case "removeserver":
			leader = removeServer(*message, client)
		case "benchmark":
			start := time.Now()
			benchmark()
			finish := time.Now().Sub(start)
			log.Println("Benchmark finished in", finish)
		default:
			log.Println("Unrecognized command", *command)
		}

		if leader != "" {
			log.Println("redirect to leader:", leader)
			*server = leader
		} else {
			done = true
		}

		client.Close()
	}
}

func main() {
	flag.Parse()

	if *command != "" {
		log.Println("Starting command")
		doClient()
	} else {
		log.Println("Starting server")
		doServer()
	}
}
