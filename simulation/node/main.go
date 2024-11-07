package main

import (
	//"crypto/rsa"
	//"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
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
	"stochastic-checking-simulation/simulation/actor"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	//"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
)

var (
	inputFile = flag.String("input_file", "", "Path to the input file in json format")
	logFile   = flag.String("log_file", "",
		"Path to the file where to save logs produced by the process")
	processIndex = flag.Int("i", 0, "Index of the current process in the system")
	nodes        = flag.Int("nodes", 1, "Number of nodes on which processes are started")
	transactions = flag.Int("transactions", 5,
		"number of transactions for the process to broadcast")
	transactionInitTimeoutNs = flag.Int("transaction_init_timeout_ns", 10000000,
		"timeout the process should wait before initialising a new transaction")
	baseIpAddress = flag.String("base_ip", "10.0.0.1",
		"Address of the main server. Ip addresses for nodes are assigned by incrementing base_ip n times")
	basePort                = flag.Int("base_port", 5001, "Port on which the process should be started")
	retransmissionTimeoutNs = flag.Int(
		"retransmission_timeout_ns",
		6000000000,
		"retransmission timeout in ns")
	makeStressTest = flag.Bool(
		"stress_test",
		false,
		"Defines whether to run the stress test. In this case, transactions are sent out infinitely")
	topology = flag.String("topo", "star",
		"Topology used to run the simulation on Mininet. One of: 'star', 'tree'")
	numberOfChildren = flag.Int("number_of_children", 4, "Number of children in a tree topology")
	messageDelay     = flag.Int("message_delay", 0, "Minimal delay between consecutive message dispatches")
	mixingTime = flag.Int(
		"mixing_time",
		2,
		"Mixing time, used in the reveal phase of the protocol")
	keysDirPath = flag.String(
		"keys_dir",
		"keys",
		"Path to the directory with private keys")
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

	var nodeIps []string
	switch *topology {
	case "star":
		nodeIps = utils.GenerateStarTopology(*nodes, *baseIpAddress, logger)
	case "tree":
		nodeIps = utils.GenerateTreeTopology(*numberOfChildren, *nodes, *baseIpAddress, logger)
	default:
		logger.Fatal("This topology is not supported. It should be one of: 'star', 'tree'")
	}

	pids := utils.GeneratePids(nodeIps, *basePort, processCount+1)

	id := *processIndex

	var process protocols.Process

	switch input.Protocol {
	case "reliable_accountability":
		//publicKeys := make([]*rsa.PublicKey, processCount)
		publicKeys := make([]*secp256k1.PublicKey, processCount)
		//var ownPrivateKey *rsa.PrivateKey
		var ownPrivateKey *secp256k1.PrivateKey

		for i := 0; i < processCount; i++ {
			filename := fmt.Sprintf("%s/%d.txt", *keysDirPath, i)
			privateKeyBytes, err := os.ReadFile(filename)

			if err != nil {
				logger.Fatal(
					fmt.Sprintf("Could not read bytes from file %s: %e", filename, err),
				)
			}

			//privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyBytes)
			privateKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
			/*
			if err != nil {
				logger.Fatal(
					fmt.Sprintf("Error while generating a private key from bytes: %e", err),
				)
			}
			*/

			if i == id {
				ownPrivateKey = privateKey
			}

			//publicKeys[i] = &privateKey.PublicKey
			publicKeys[i] = privateKey.PubKey()
		}

		process = &reliable.Process{
			PublicKeys: publicKeys,
			PrivateKey: ownPrivateKey,
			MixingTime: *mixingTime,
		}
	case "consistent_accountability":
		process = &consistent.Process{}
	case "bracha":
		process = &bracha.Process{}
	case "scalable":
		process = &scalable.Process{}
	default:
		logger.Fatalf("Invalid protocol: %s", input.Protocol)
	}

	logger.Printf("Running protocol: %s\n", input.Protocol)

	node := &Node{
		processIndex:             int32(id),
		pids:                     pids,
		parameters:               &input.Parameters,
		transactionsToSendOut:    *transactions,
		transactionInitTimeoutNs: *transactionInitTimeoutNs,
		process:                  process,
		stressTest:               *makeStressTest,
	}

	a := actor.Actor{}
	a.InitActor(int32(id), pids, node, logger, *retransmissionTimeoutNs, *messageDelay)
}
