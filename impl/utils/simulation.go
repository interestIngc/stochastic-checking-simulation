package utils

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func MakeCustomPid(pid *actor.PID) string {
	return fmt.Sprintf("%s,%s", pid.Address, pid.Id)
}

func JoinIpAndPort(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func GetLocalPids(baseIp string, basePort int, processCount int) []*actor.PID {
	pids := make([]*actor.PID, processCount)

	for i := 0; i < processCount; i++ {
		currPort := basePort + i + 1
		pids[i] = actor.NewPID(JoinIpAndPort(baseIp, currPort), "main")
	}

	return pids
}

func GetRemotePids(baseIp string, basePort int, processCount int, logger *log.Logger) []*actor.PID {
	const Bytes = 4

	ipBytes := make([]int, Bytes)

	var e error
	for i, currByte := range strings.Split(baseIp, ".") {
		ipBytes[i], e = strconv.Atoi(currByte)
		if e != nil {
			logger.Fatalf("Byte %d in base ip address is invalid", i)
		}
		if i >= Bytes {
			logger.Fatal("Base ip address must be ipv4")
		}
	}

	pids := make([]*actor.PID, processCount)

	for i := 0; i < processCount; i++ {
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

		pids[i] = actor.NewPID(JoinIpAndPort(currIp, basePort),"main")
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
