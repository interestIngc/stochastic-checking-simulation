package main

import (
	"encoding/json"
	"flag"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"io"
	"log"
	"os"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/protocols/accountability/consistent"
	"stochastic-checking-simulation/impl/protocols/accountability/reliable"
	"stochastic-checking-simulation/impl/protocols/bracha"
	"stochastic-checking-simulation/impl/protocols/scalable"
	"stochastic-checking-simulation/impl/utils"
	"strconv"
	"strings"
)

const Bytes = 4

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

	ipBytes := make([]int, Bytes)

	for i, currByte := range strings.Split(*baseIpAddress, ".") {
		ipBytes[i], e = strconv.Atoi(currByte)
		if e != nil {
			logger.Fatalf("Byte %d in base ip address is invalid", i)
		}
		if i >= Bytes {
			logger.Fatal("Base ip address must be ipv4")
		}
	}

	processIp := *baseIpAddress
	processPort := *port

	pids := make([]*actor.PID, processCount)

	for i := 0; i < processCount; i++ {
		var address string
		if *localRun {
			currPort := *port + i + 1
			if i == *processIndex {
				processPort = currPort
			}
			address = utils.JoinIpAndPort(*baseIpAddress, currPort)
		} else {
			leftByteInd := Bytes - 1
			for ; leftByteInd >= 0 && ipBytes[leftByteInd] == 255; leftByteInd-- {
			}
			if leftByteInd == -1 {
				logger.Fatal("Cannot assign ip addresses, number of processes in the system is too high")
			}
			ipBytes[leftByteInd]++
			for ind := leftByteInd + 1; ind < Bytes; ind++ {
				ipBytes[ind] = 0
			}

			ipBytesAsStr := make([]string, Bytes)
			for ind := 0; ind < Bytes; ind++ {
				ipBytesAsStr[ind] = strconv.Itoa(ipBytes[ind])
			}
			currIp := strings.Join(ipBytesAsStr, ".")

			if i == *processIndex {
				processIp = currIp
			}

			address = utils.JoinIpAndPort(currIp, *port)
		}
		pids[i] = actor.NewPID(address, "main")
	}

	mainServer := actor.NewPID(utils.JoinIpAndPort(*baseIpAddress, *port), "mainserver")

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
		pids[*processIndex],
		pids,
		&input.Parameters,
		logger,
		protocols.NewTransactionManager(*transactions, *transactionInitTimeoutNs),
		mainServer,
	)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(processIp, processPort)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

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
