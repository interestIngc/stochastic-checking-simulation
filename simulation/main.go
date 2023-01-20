package main

import (
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"net"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/messages"
	"stochastic-checking-simulation/utils"
	"strconv"
	"strings"
)

var (
	nodesStr = flag.String(
		"nodes", "",
		"a string representing bindings host:port separated by comma, e.g. 127.0.0.1:8081,127.0.0.1:8082")
	address        = flag.String("address", "", "current node's address, e.g. 127.0.0.1:8081")
	mainServerAddr = flag.String("mainserver", "", "address of the main server, e.g. 127.0.0.1:8080")
	protocol       = flag.String("protocol", "accountability",
		"A protocol to run, either accountability or broadcast")
	faultyProcesses = flag.Int("f", 0, "max number of faulty processes in the system")
	witnessSetSize = flag.Int("w", 0, "size of the witness set")
	witnessThreshold = flag.Int("u", 0, "witnesses threshold to accept a transaction")
	nodeIdSize = flag.Int("node_id_size", 256, "node id size, default is 256")
	numberOfBins = flag.Int("number_of_bins", 32, "number of bins in history hash, default is 32")
)

func main() {
	flag.Parse()

	nodes := strings.Split(*nodesStr, ",")
	host, portStr, e := net.SplitHostPort(*address)
	if e != nil {
		fmt.Printf("Could not split %s into host and port\n", *address)
		return
	}

	port, e := strconv.Atoi(portStr)
	if e != nil {
		fmt.Printf("Could not convert port string representation into int: %s\n", e)
		return
	}

	config.ProcessCount = len(nodes)
	config.FaultyProcesses = *faultyProcesses
	config.WitnessSetSize = *witnessSetSize
	config.WitnessThreshold = *witnessThreshold
	config.NodeIdSize = *nodeIdSize
	config.NumberOfBins = *numberOfBins

	pids := make([]*actor.PID, len(nodes))
	for i := 0; i < len(nodes); i++ {
		pids[i] = actor.NewPID(nodes[i], "pid")
	}

	mainServer := actor.NewPID(*mainServerAddr,"mainserver")

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(host, port)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	if *protocol == "accountability" {
		process := &CorrectProcess{}
		currPid, e :=
			system.Root.SpawnNamed(
				actor.PropsFromProducer(
					func() actor.Actor {
						return process
					}),
				"pid",
			)
		if e != nil {
			fmt.Printf("Error while generating pid happened: %s\n", e)
			return
		}
		process.InitCorrectProcess(currPid, pids)
		fmt.Printf("%s: started\n", utils.PidToString(currPid))
		system.Root.RequestWithCustomSender(mainServer, &messages.Started{}, currPid)
	} else {
		process := &BrachaProcess{}
		currPid, e :=
			system.Root.SpawnNamed(
				actor.PropsFromProducer(
					func() actor.Actor {
						return process
					}),
				"pid",
			)
		if e != nil {
			fmt.Printf("Error while generating pid happened: %s\n", e)
			return
		}
		process.InitBrachaProcess(currPid, pids)
		fmt.Printf("%s: started\n", utils.PidToString(currPid))
		system.Root.RequestWithCustomSender(mainServer, &messages.Started{}, currPid)
	}

	_, _ = console.ReadLine()
}
