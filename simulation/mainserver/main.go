package main

import (
	"flag"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/config"
)

var (
	processCount = flag.Int("n", 0, "number of processes in the system (excluding the main server)")
	times        = flag.Int("times", 5, "number of transactions for each process to broadcast")
)

func main() {
	flag.Parse()

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(config.BaseIpAddress, config.Port)
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
		log.Printf("Could not start the main server: %s\n", e)
		return
	}

	server.InitMainServer(pid, *processCount, *times)
	log.Printf("Main server started at: %s:%d\n", config.BaseIpAddress, config.Port)

	_, _ = console.ReadLine()
}
