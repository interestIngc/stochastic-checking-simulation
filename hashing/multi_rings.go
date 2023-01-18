package hashing

import (
	"errors"
	"math"
)

type MultiRing struct {
	modulo uint
	dimension uint
	vector []int
}

func NewMultiRing(modulo uint, dimension uint) *MultiRing {
	mr := new(MultiRing)
	mr.modulo = modulo
	mr.dimension = dimension
	mr.vector = make([]int, dimension)
	return mr
}

func (r *MultiRing) copy() *MultiRing {
	mr := NewMultiRing(r.modulo, r.dimension)
	for i := uint(0); i < r.dimension; i++ {
		mr.vector[i] = r.vector[i]
	}
	return mr
}

func (r *MultiRing) set(i uint64, value int) {
	for value < 0 {
		value += int(r.modulo)
	}
	r.vector[i] = value % int(r.modulo)
}

func (r *MultiRing) add(i uint64, value int) {
	r.set(i, r.vector[i] + value)
}

func (r *MultiRing) subtract(x int, y int) float64 {
	return float64((int(r.modulo) + x - y) % int(r.modulo))
}

func multiRingDistance(r1 *MultiRing, r2 *MultiRing, norm float64) (float64, error) {
	if r1.modulo != r2.modulo || r1.dimension != r2.dimension {
		return 0.0,
		errors.New("cannot calculate distance between two rings with different modulo or dimension")
	}
	sum := 0.0
	for i := uint(0); i < r1.dimension; i++ {
		diff := math.Min(
			r1.subtract(r1.vector[i], r2.vector[i]),
			r1.subtract(r2.vector[i], r1.vector[i]))
		sum += math.Pow(diff, norm)
	}
	return math.Pow(sum, 1.0 / norm), nil
}

func multiRingFromBytes(modulo uint, dimension uint, bytes []byte) *MultiRing {
	mr := NewMultiRing(modulo, dimension)
	for i := 0; i < len(bytes); i++ {
		mr.vector[i] = int(bytes[i]) % int(modulo)
	}
	return mr
}
