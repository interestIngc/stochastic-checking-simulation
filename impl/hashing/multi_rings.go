package hashing

import (
	"errors"
	"math"
	"stochastic-checking-simulation/impl/utils"
)

type MultiRing struct {
	modulo    uint
	dimension uint
	vector    []int
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

func (r *MultiRing) fit(value int) int {
	return ((value % int(r.modulo)) + int(r.modulo)) % int(r.modulo)
}

func (r *MultiRing) set(i uint64, value int) {
	r.vector[i] = r.fit(value)
}

func (r *MultiRing) add(i uint64, value int) {
	r.set(i, r.vector[i]+value)
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
	return math.Pow(sum, 1.0/norm), nil
}

func multiRingDistanceLInf(r1 *MultiRing, r2 *MultiRing) (float64, error) {
	if r1.modulo != r2.modulo || r1.dimension != r2.dimension {
		return 0.0,
			errors.New("cannot calculate distance between two rings with different modulo or dimension")
	}
	
	max := 0.0
	for i := uint(0); i < r1.dimension; i++ {
		diff := math.Min(
			r1.subtract(r1.vector[i], r2.vector[i]),
			r1.subtract(r2.vector[i], r1.vector[i]))
		max = math.Max(max, diff)
	}
	return max, nil
		
}

func multiRingFromBytes(modulo uint, dimension uint, bytes []byte) *MultiRing {
	mr := NewMultiRing(modulo, dimension)
	bytesPerDimension := uint(len(bytes) / int(dimension))
	if bytesPerDimension > 8 {
		bytesPerDimension = 8
	}
	value := make([]byte, 8)
	for i := uint(0); i < dimension; i++ {
		for j := uint(0); j < bytesPerDimension; j++ {
			value[j] = bytes[i*bytesPerDimension+j]
		}
		mr.set(uint64(i), int(utils.ToUint64(value)))
	}
	return mr
}

// Adds another multiRing to the current one, item-wise
func (r *MultiRing) merge(rx *MultiRing) {
	for i := 0; i < int(r.dimension); i++ {
		r.add(uint64(i), rx.vector[i])
	}
}
