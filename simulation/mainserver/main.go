package main

import (
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/impl/utils"
)

var (
	processCount = flag.Int("n", 0, "Number of processes in the system (excluding the main server)")
	logFile      = flag.String(
		"log_file",
		"",
		"Path to the file where to save logs produced by the main server")
	ipAddress = flag.String("ip", "10.0.0.0", "Ip address of the main server")
	port      = flag.Int("port", 5001, "Port on which the main server should be started")
)

func main() {
	flag.Parse()

	f := utils.OpenLogFile(*logFile)
	logger := log.New(f, "", log.LstdFlags)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(*ipAddress, *port)
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

	server.InitMainServer(pid, *processCount, logger)
	logger.Printf("Main server started at %s:%d\n", *ipAddress, *port)

	_, _ = console.ReadLine()
}
