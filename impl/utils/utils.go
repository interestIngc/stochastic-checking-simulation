package utils

import (
	"encoding/binary"
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
)

func MakeCustomPid(pid *actor.PID) string {
	return fmt.Sprintf("Address:%s Id:%s", pid.Address, pid.Id)
}

func ToUint64(bytes []byte) uint64 {
	return binary.LittleEndian.Uint64(bytes)
}

func TransactionToBytes(author string, seqNumber int64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(seqNumber))
	return addBytes([]byte(author), bytes)
}

func addBytes(fst []byte, snd []byte) []byte {
	if len(snd) > len(fst) {
		fst, snd = snd, fst
	}
	res := make([]byte, len(fst))
	for i, val := range fst {
		res[i] = val
	}
	for i, val := range snd {
		res[i] += val
	}
	return res
}
