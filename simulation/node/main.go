package main

import (
	"encoding/json"
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"io"
	"log"
	"os"
	"stochastic-checking-simulation/impl/messages"
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
	port         = flag.Int("port", 5001, "Port on which the node should be started")
)

type Input struct {
	Protocol   string                `json:"protocol"`
	Parameters parameters.Parameters `json:"parameters"`
}

func joinWithPort(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func main() {
	flag.Parse()

	lFile := utils.OpenLogFile(*logFile)
	logger := log.New(lFile, "", log.LstdFlags)

	iFile, e := os.Open(*inputFile)
	if e != nil {
		utils.ExitWithError(logger, fmt.Sprintf("Can't read from file %s", *inputFile))
	}

	byteArray, e := io.ReadAll(iFile)
	if e != nil {
		utils.ExitWithError(logger, fmt.Sprintf("Could not read bytes from the input file\n%e", e))
	}

	var input Input
	e = json.Unmarshal(byteArray, &input)
	if e != nil {
		utils.ExitWithError(logger, fmt.Sprintf("Could not parse json from the input file\n%e", e))
	}

	if input.Protocol == "" {
		utils.ExitWithError(logger, "parameter protocol is mandatory")
	}

	processCount := input.Parameters.ProcessCount

	ipBytes := make([]int, Bytes)

	for i, currByte := range strings.Split(*baseIpAddress, ".") {
		ipBytes[i], e = strconv.Atoi(currByte)
		if e != nil {
			utils.ExitWithError(logger, fmt.Sprintf("Byte %d in base ip address is invalid", i))
		}
		if i >= Bytes {
			utils.ExitWithError(logger, "base ip address must be ipv4")
		}
	}

	var processIp string
	//var processPort int
	pids := make([]*actor.PID, processCount)

	//currPort := *port

	for i := 0; i < processCount; i++ {
		//currPort++
		leftByteInd := Bytes - 1
		for ; leftByteInd >= 0 && ipBytes[leftByteInd] == 255; leftByteInd-- {
		}
		if leftByteInd == -1 {
			utils.ExitWithError(
				logger,
				"cannot assign ip addresses, number of processes in the system is too high")
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
			//processPort = currPort
		}
		pids[i] = actor.NewPID(joinWithPort(currIp, *port), "pid")
		//pids[i] = actor.NewPID(joinWithPort(*baseIpAddress, currPort), "pid")
	}

	mainServer := actor.NewPID(joinWithPort(*baseIpAddress, *port), "mainserver")

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
		utils.ExitWithError(logger, fmt.Sprintf("Invalid protocol: %s", input.Protocol))
	}

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(processIp, *port)
	//remoteConfig := remote.Configure(*baseIpAddress, processPort)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	currPid, e :=
		system.Root.SpawnNamed(
			actor.PropsFromProducer(
				func() actor.Actor {
					return process
				}),
			"main",
		)
	if e != nil {
		utils.ExitWithError(logger, fmt.Sprintf("Error while spawning the process happened: %s", e))
	}

	process.InitProcess(
		currPid,
		pids,
		&input.Parameters,
		logger,
		protocols.NewTransactionManager(*transactions, *transactionInitTimeoutNs),
	)
	logger.Printf("Process started: %s, running protocol: %s\n", utils.MakeCustomPid(currPid), input.Protocol)

	system.Root.RequestWithCustomSender(mainServer, &messages.Started{}, currPid)

	_, _ = console.ReadLine()
}
