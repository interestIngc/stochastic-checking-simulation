package main

import (
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/utils"
)

var (
	processCount = flag.Int("n", 0, "number of processes in the system (excluding the main server)")
	times        = flag.Int("times", 5, "number of transactions for each process to broadcast")
	logFile      = flag.String(
		"log_file",
		"",
		"path to the file where to save logs produced by the main server")
)

func main() {
	flag.Parse()

	f := utils.OpenLogFile(*logFile)
	logger := log.New(f, "", log.LstdFlags)

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
		utils.ExitWithError(logger, fmt.Sprintf("Could not start the main server: %s\n", e))
	}

	server.InitMainServer(pid, *processCount, *times, logger)
	logger.Printf("Main server started at: %s:%d\n", config.BaseIpAddress, config.Port)

	_, _ = console.ReadLine()
}
