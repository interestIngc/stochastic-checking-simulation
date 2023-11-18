package utils

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"os"
	"path/filepath"
	"stochastic-checking-simulation/impl/messages"
	"strconv"
	"strings"
	"time"
)

func JoinIpAndPort(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func GeneratePids(
	baseIp string,
	basePort int,
	nodes int,
	processesPerNode int,
	logger *log.Logger,
) []string {
	const Bytes = 4

	ipBytes := make([]int, Bytes)

	var e error
	for i, currByte := range strings.Split(baseIp, ".") {
		ipBytes[i], e = strconv.Atoi(currByte)
		if e != nil || ipBytes[i] < 0 || ipBytes[i] > 255 {
			logger.Fatalf("Byte %d in base ip address is invalid", i)
		}
		if i >= Bytes {
			logger.Fatal("Base ip address must be ipv4")
		}
	}

	nodeIps := make([]string, nodes)

	nodeIps[0] = baseIp
	for i := 1; i < nodes; i++ {
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
		nodeIps[i] = strings.Join(ipBytesAsStr, ".")
	}

	pids := make([]string, nodes*processesPerNode)

	for i, ip := range nodeIps {
		for j := 0; j < processesPerNode; j++ {
			pids[i*processesPerNode+j] = JoinIpAndPort(ip, basePort+j)
		}
	}

	return pids
}

func OpenLogFile(logFile string) *os.File {
	dir, _ := filepath.Split(logFile)
	e := os.MkdirAll(dir, os.ModePerm)
	if e != nil {
		log.Printf("Could not create parent directories for %s, error: %e", logFile, e)
	}

	f, e := os.Create(logFile)
	if e != nil {
		log.Printf("Could not open file %s to write logs into, error: %e", logFile, e)
	}

	return f
}

func GetNow() int64 {
	return time.Now().UnixNano()
}

func Unmarshal(data []byte) (*messages.Message, error) {
	msg := &messages.Message{}

	err := proto.Unmarshal(data, msg)
	if err != nil {
		log.Printf("Could not unmarshal message: %v", data)
		return nil, err
	}
	return msg, nil
}

func Marshal(message *messages.Message) ([]byte, error) {
	data, e := proto.Marshal(message)
	if e != nil {
		log.Printf("Could not marshal message, %e", e)
	}
	return data, e
}
