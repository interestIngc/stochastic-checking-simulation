package main

import (
	"encoding/json"
	"flag"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"io"
	"log"
	"net"
	"os"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/protocols/accountability/consistent"
	"stochastic-checking-simulation/impl/protocols/accountability/reliable"
	"stochastic-checking-simulation/impl/protocols/bracha"
	"stochastic-checking-simulation/impl/protocols/scalable"
	"stochastic-checking-simulation/impl/utils"
	"strconv"
)

var (
	inputFile = flag.String("input_file", "", "Path to the input file in json format")
	logFile   = flag.String("log_file", "",
		"Path to the file where to save logs produced by the process")
	processIndex = flag.Int("i", 0, "Index of the current process in the system")
	transactions = flag.Int("transactions", 5,
		"number of transactions for the process to broadcast")
	transactionInitTimeoutNs = flag.Int("transaction_init_timeout_ns", 10000000,
		"timeout the process should wait before initialising a new transaction")
	baseIpAddress = flag.String("base_ip", "10.0.0.1",
		"Address of the main server. Ip addresses for nodes are assigned by incrementing base_ip n times")
	port     = flag.Int("port", 5001, "Port on which the node should be started")
	localRun = flag.Bool("local_run", false,
		"Defines whether to start the simulation locally, i.e. on a single machine, or in a distributed system")
)

type Input struct {
	Protocol   string                `json:"protocol"`
	Parameters parameters.Parameters `json:"parameters"`
}

func main() {
	flag.Parse()

	lFile := utils.OpenLogFile(*logFile)
	logger := log.New(lFile, "", log.LstdFlags)

	iFile, e := os.Open(*inputFile)
	if e != nil {
		logger.Fatalf("Can't read from file %s", *inputFile)
	}

	byteArray, e := io.ReadAll(iFile)
	if e != nil {
		logger.Fatalf("Could not read bytes from the input file: %e", e)
	}

	var input Input
	e = json.Unmarshal(byteArray, &input)
	if e != nil {
		logger.Fatalf("Could not parse json from the input file\n%e", e)
	}

	if input.Protocol == "" {
		logger.Fatal("Parameter protocol is mandatory")
	}

	processCount := input.Parameters.ProcessCount

	var pids []*actor.PID
	if *localRun {
		pids = utils.GetLocalPids(*baseIpAddress, *port, processCount)
	} else {
		pids = utils.GetRemotePids(*baseIpAddress, *port, processCount, logger)
	}

	mainServer := actor.NewPID(utils.JoinIpAndPort(*baseIpAddress, *port), "mainserver")

	logger.Printf("Mainserver: %s\n", mainServer.String())
	for _, pid := range pids {
		logger.Println(pid.String())
	}

	var process protocols.Process

	switch input.Protocol {
	case "reliable_accountability":
		process = &reliable.Process{}
	case "consistent_accountability":
		process = &consistent.CorrectProcess{}
	case "bracha":
		process = &bracha.Process{}
	case "scalable":
		process = &scalable.Process{}
	default:
		logger.Fatalf("Invalid protocol: %s", input.Protocol)
	}

	process.InitProcess(
		int64(*processIndex),
		pids,
		&input.Parameters,
		logger,
		protocols.NewTransactionManager(*transactions, *transactionInitTimeoutNs),
		mainServer,
	)

	processIp, processPortStr, _ := net.SplitHostPort(pids[*processIndex].Address)
	processPort, _ := strconv.Atoi(processPortStr)

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

	remoteConfig := remote.Configure(processIp, processPort)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()
	actor.Unbounded()

	_, e =
		system.Root.SpawnNamed(
			actor.PropsFromProducer(
				func() actor.Actor {
					return process
				}),
			"main",
		)
	if e != nil {
		logger.Fatalf("Error while spawning the process happened: %s", e)
	}

	logger.Printf("Running protocol: %s\n", input.Protocol)

	select {}
}
