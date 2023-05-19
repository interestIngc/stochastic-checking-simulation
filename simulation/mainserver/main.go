package main

import (
	"flag"
	"log"
	"stochastic-checking-simulation/impl/utils"
	"stochastic-checking-simulation/simulation/actor"
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

	var pids []string
	if *localRun {
		pids = utils.GetLocalPids(*baseIpAddress, *basePort, n)
	} else {
		pids = utils.GetRemotePids(*baseIpAddress, *basePort, n, logger)
	}

	ownAddress := utils.JoinIpAndPort(*baseIpAddress, *basePort)

	server := &MainServer{n: n}

	a := actor.Actor{}
	a.InitActor(int32(n), pids, ownAddress, server, logger, *retransmissionTimeoutNs)
}
