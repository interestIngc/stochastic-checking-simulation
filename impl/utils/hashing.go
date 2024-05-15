package utils

import (
	"encoding/binary"
)

func ToUint64(bytes []byte) uint64 {
	size := len(bytes)
	if size > 8 {
		size = 8
	}

	data := make([]byte, 8)
	for i := 0; i < size; i++ {
		data[i] = bytes[i]
	}

	return binary.LittleEndian.Uint64(data)
}

func ToInt32(bytes []byte) int32 {
	return int32(binary.LittleEndian.Uint32(bytes))
}

func TransactionToBytes(author string, seqNumber int64) []byte {
	return addBytes([]byte(author), ToBytes(uint64(seqNumber)))
}

func ToBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, value)
	return bytes
}

func Int32ToBytes(value int32) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(value))
	return bytes
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

func AreEqual(fst []byte, snd []byte) bool {
	if len(fst) != len(snd) {
		return false
	}

	for i := 0; i < len(fst); i++ {
		if fst[i] != snd[i] {
			return false
		}
	}

	return true
}
