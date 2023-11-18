package main

import (
	"flag"
	"log"
	"stochastic-checking-simulation/impl/utils"
	"stochastic-checking-simulation/simulation/actor"
)

var (
	processCount = flag.Int("n", 0, "Number of processes in the system (excluding the main server)")
	nodes        = flag.Int("nodes", 1, "Number of nodes on which processes are started")
	logFile      = flag.String(
		"log_file",
		"",
		"Path to the file where to save logs produced by the main server")
	baseIpAddress           = flag.String("base_ip", "10.0.0.1", "Ip address of the main server")
	basePort                = flag.Int("base_port", 5001, "Port on which the main server should be started")
	retransmissionTimeoutNs = flag.Int(
		"retransmission_timeout_ns",
		6000000000,
		"retransmission timeout in ns")
)

func main() {
	flag.Parse()

	f := utils.OpenLogFile(*logFile)
	logger := log.New(f, "", log.LstdFlags)

	n := *processCount

	if (n+1)%*nodes != 0 {
		logger.Fatal(
			"Total number of started processes, including the mainserver, must be divisible by the number of nodes",
		)
	}

	processesPerNode := (n + 1) / *nodes

	pids := utils.GeneratePids(*baseIpAddress, *basePort, *nodes, processesPerNode, logger)

	server := &MainServer{n: n}

	a := actor.Actor{}
	a.InitActor(int32(n), pids, server, logger, *retransmissionTimeoutNs)
}
