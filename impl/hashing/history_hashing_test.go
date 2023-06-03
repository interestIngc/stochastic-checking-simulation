package hashing

import (
	"github.com/stretchr/testify/assert"
	"stochastic-checking-simulation/impl/utils"
	"testing"
)

const binCapacity = uint(8)
const binNum = uint(4)

var bytes = make([]byte, 2)

type mockHasher struct {
	value uint64
}

func (h *mockHasher) Hash(bytes []byte) []byte {
	return utils.ToBytes(h.value)
}

func TestInsert_binValueIncremented(t *testing.T) {
	hasher := &mockHasher{value: uint64(5)}
	hh := NewHistoryHash(binNum, binCapacity, hasher)

	hh.Insert(bytes)

	expectedBins := make([]int, binNum)
	expectedBins[1] = 1
	assert.Exactly(t, expectedBins, hh.bins.vector)
}

func TestInsert_binValueDecremented(t *testing.T) {
	hasher := &mockHasher{value: uint64(3)}
	hh := NewHistoryHash(binNum, binCapacity, hasher)

	hh.Insert(bytes)

	expectedBins := make([]int, binNum)
	expectedBins[3] = 7
	assert.Exactly(t, expectedBins, hh.bins.vector)
}

func TestToString(t *testing.T) {
	hasher := &mockHasher{value: uint64(0)}
	hh := NewHistoryHash(binNum, binCapacity, hasher)
	hh.bins.set(0, 1)

	hhString := hh.ToString()

	assert.Equal(t, "[1 0 0 0]", hhString)
}
