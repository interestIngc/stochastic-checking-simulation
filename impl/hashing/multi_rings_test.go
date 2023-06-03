package hashing

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const modulo = uint(8)
const dimension = uint(4)

func makeDefaultMultiRing() *MultiRing {
	vect := make([]int, dimension)
	for i := uint(0); i < dimension; i++ {
		vect[i] = int(i) % int(modulo)
	}
	return &MultiRing{
		dimension: dimension,
		modulo:    modulo,
		vector:    vect,
	}
}

func TestCopy_simple(t *testing.T) {
	mr := makeDefaultMultiRing()

	mrCopy := mr.copy()

	assert.Equal(t, modulo, mrCopy.modulo)
	assert.Equal(t, dimension, mrCopy.dimension)
	for i := uint(0); i < dimension; i++ {
		assert.Equal(t, int(i%modulo), mrCopy.vector[i])
	}
}

func TestCopy_initialRingModified(t *testing.T) {
	mr := makeDefaultMultiRing()

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
		assert.Equal(t, int(i*10), mr.vector[i])
	}
}

func TestMultiRingDistance_multiRingsEqual(t *testing.T) {
	mr1 := makeDefaultMultiRing()
	mr2 := makeDefaultMultiRing()

	dist, e := multiRingDistance(mr1, mr2)

	assert.Equal(t, nil, e)
	assert.Equal(t, 0.0, dist)
}

func TestMultiRingDistance_OneElementDiff(t *testing.T) {
	mr1 := makeDefaultMultiRing()
	mr2 := makeDefaultMultiRing()
	mr2.set(0, 2)

	dist, e := multiRingDistance(mr1, mr2)

	assert.Equal(t, nil, e)
	assert.Equal(t, 2.0, dist)
}

func TestMultiRingDistance_oneElementDiff_roundDistance(t *testing.T) {
	mr1 := makeDefaultMultiRing()
	mr2 := makeDefaultMultiRing()
	mr2.set(0, 6)

	dist, e := multiRingDistance(mr1, mr2)

	assert.Equal(t, nil, e)
	assert.Equal(t, 2.0, dist)
}

func TestMultiRingDistance_differentModulo(t *testing.T) {
	mr1 := NewMultiRing(2, dimension)
	mr2 := NewMultiRing(3, dimension)

	_, e := multiRingDistance(mr1, mr2)

	assert.NotEqual(t, nil, e)
}
