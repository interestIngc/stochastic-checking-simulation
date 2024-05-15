package hashing

import (
	"fmt"
	"stochastic-checking-simulation/impl/utils"
)

type HistoryHash struct {
	binNum      uint
	binCapacity uint

	hasher Hasher
	bins   *MultiRing

	seed []byte
}

func NewHistoryHash(binNum uint, binCapacity uint, hasher Hasher, seed int32) *HistoryHash {
	hh := new(HistoryHash)
	hh.binNum = binNum
	hh.binCapacity = binCapacity
	hh.hasher = hasher
	hh.bins = NewMultiRing(binCapacity, binNum)

	hh.seed = utils.Int32ToBytes(seed)

	return hh
}

// Insert adds a new transaction into the multiRing representing history hash
func (hh *HistoryHash) Insert(bytes []byte) {
	h := utils.ToUint64(hh.hasher.Hash(append(bytes, hh.seed...)))
	binIndex := h % uint64(hh.binNum)
	direction := 1
	if (h & uint64(hh.binNum)) == 0 {
		direction = -1
	}
	hh.bins.add(binIndex, direction)
}

// ToString converts the current historyHash to string
func (hh *HistoryHash) ToString() string {
	return fmt.Sprintf("%v", hh.bins.vector)
}
