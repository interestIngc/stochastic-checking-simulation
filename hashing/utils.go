package hashing

import (
	"encoding/binary"
	"github.com/asynkron/protoactor-go/actor"
)

func toUint64(bytes []byte) uint64 {
	return binary.LittleEndian.Uint64(bytes[:])
}

func TransactionToBytes(author *actor.PID, seqNumber int32) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(seqNumber))
	return addBytes([]byte(author.String()), bytes)
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
