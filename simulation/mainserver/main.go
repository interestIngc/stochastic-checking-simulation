package main

import (
	"flag"
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
	baseIpAddress = flag.String("ip", "10.0.0.1", "Ip address of the main server")
	basePort      = flag.Int("port", 5001, "Port on which the main server should be started")
	localRun      = flag.Bool("local_run", false,
		"Defines whether to start the simulation locally, i.e. on a single machine, or in a distributed system")
)

func main() {
	flag.Parse()

	f := utils.OpenLogFile(*logFile)
	logger := log.New(f, "", log.LstdFlags)

	system := actor.NewActorSystem()
	system.EventStream.Subscribe(
		func(event interface{}) {
			deadLetter, ok := event.(*actor.DeadLetterEvent)
			if ok {
				logger.Printf(
					"Dead letter detected. To: %s\n",
					deadLetter.PID.String())
			}
		},
	)

	remoteConfig := remote.Configure(*baseIpAddress, *basePort)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	var pids []*actor.PID
	if *localRun {
		pids = utils.GetLocalPids(*baseIpAddress, *basePort, *processCount)
	} else {
		pids = utils.GetRemotePids(*baseIpAddress, *basePort, *processCount, logger)
	}

	server := &MainServer{}
	server.InitMainServer(pids, logger)

	_, e := system.Root.SpawnNamed(
		actor.PropsFromProducer(
			func() actor.Actor {
				return server
			}),
		"mainserver",
	)
	if e != nil {
		logger.Fatalf("Could not start the main server: %s\n", e)
	}

	logger.Printf(
		"Main server started at %s\n",
		utils.JoinIpAndPort(*baseIpAddress, *basePort),
	)

	select {}
}
