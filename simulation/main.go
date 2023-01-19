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
)

func main() {
	flag.Parse()

	nodes := strings.Split(*nodesStr, ",")
	host, portStr, e := net.SplitHostPort(*address)
	if e != nil {
		fmt.Printf("Could not split %s into host and port\n", *address)
		return
	}
	if len(nodes) != config.ProcessCount {
		fmt.Printf(
			"Number of bindings defined in nodes flag must be equal to the number of processes in the system\n")
		return
	}

	port, e := strconv.Atoi(portStr)
	if e != nil {
		fmt.Printf("Could not convert port string representation into int: %s\n", e)
		return
	}

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
