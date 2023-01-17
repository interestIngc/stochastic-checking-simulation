package hashing

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const modulo = uint(8)
const dimension = uint(4)

func TestBaseCopy(t *testing.T) {
	mr := NewMultiRing(modulo, dimension)
	for i := uint(0); i < dimension; i++ {
		mr.set(uint64(i), int(i))
	}

	mrCopy := mr.copy()

	assert.Equal(t, modulo, mrCopy.modulo)
	assert.Equal(t, dimension, mrCopy.dimension)
	for i := 0; i < int(dimension); i++ {
		assert.Equal(t, i, mrCopy.vector[i])
	}
}

func TestCopyInitialRingModified(t *testing.T) {
	mr := NewMultiRing(modulo, dimension)
	for i := uint(0); i < dimension; i++ {
		mr.set(uint64(i), int(i))
	}

	mrCopy := mr.copy()

	mr.set(0, 5)
	mr.set(1, 6)
	assert.Equal(t, mrCopy.vector[0], 0)
	assert.Equal(t, mrCopy.vector[1], 1)
}

func TestMultiRingFromBytes(t *testing.T) {
	byteSize := uint(256)
	bytes := make([]byte, dimension)
	for i := uint(0); i < dimension; i++ {
		bytes[i] = byte(i * 10)
	}

	mr := multiRingFromBytes(byteSize, dimension, bytes)

	assert.Equal(t, byteSize, mr.modulo)
	assert.Equal(t, dimension, mr.dimension)
	for i := uint(0); i < dimension; i++ {
		assert.Equal(t, int(i * 10), mr.vector[i])
	}
}
