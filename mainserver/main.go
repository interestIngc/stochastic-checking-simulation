package main

import (
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"net"
	"stochastic-checking-simulation/config"
	"strconv"
)

var (
	mainServerAddr = flag.String("mainserver", "", "address of the main server, e.g. 127.0.0.1:8080")
	processCount = flag.Int("n", 0, "number of processes in the system (excluding the main server)")
	faultyProcesses = flag.Int("f", 0, "max number of faulty processes in the system")
	witnessSetSize = flag.Int("w", 0, "size of the witness set")
	witnessThreshold = flag.Int("u", 0, "witnesses threshold to accept a transaction")
	nodeIdSize = flag.Int("node_id_size", 256, "node id size, default is 256")
	numberOfBins = flag.Int("number_of_bins", 32, "number of bins in history hash, default is 32")
)

func main() {
	flag.Parse()

	host, portStr, e := net.SplitHostPort(*mainServerAddr)
	if e != nil {
		fmt.Printf("Could not split %s into host and port\n", *mainServerAddr)
		return
	}
	port, e := strconv.Atoi(portStr)
	if e != nil {
		fmt.Printf("Could not convert port string representation into int: %s\n", e)
		return
	}

	config.ProcessCount = *processCount
	config.FaultyProcesses = *faultyProcesses
	config.WitnessSetSize = *witnessSetSize
	config.WitnessThreshold = *witnessThreshold
	config.NodeIdSize = *nodeIdSize
	config.NumberOfBins = *numberOfBins

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(host, port)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	server := &MainServer{}
	pid, e := system.Root.SpawnNamed(
		actor.PropsFromProducer(
			func() actor.Actor {
				return server
			}),
			"mainserver",
	)
	if e != nil {
		fmt.Printf("Could not start a main server: %s\n", e)
		return
	}

	server.InitMainServer(pid)
	fmt.Printf("Main server started at: %s\n", *mainServerAddr)

	_, _ = console.ReadLine()
}
