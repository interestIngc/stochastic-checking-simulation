package main

import (
	"fmt"
	"math"
	"math/rand"
)

// dTorus struct, should always be created with constructor
type dTorus struct {
	dim    int   // Number of dimensions
	dSize  int   // Size of each individual dimension (number of possible points)
	vector []int // dTorus array
}

// dTorus struct constructor
func CreateDefaultTorus(nDim int, dimSize int) dTorus {
	return dTorus{dim: nDim, dSize: dimSize, vector: make([]int, nDim)}
}

// Another dTorus constructor
func CreateTorus(nDim int, dimSize int, values []int) dTorus {

	if len(values) != nDim {
		fmt.Print("Dimensions do not match lenght of given array.")
		//return nil
	}

	for v := range values {
		if v >= dimSize || v < 0 {
			fmt.Print("One of the values in vector is out of range.")
			//return nil
		}
	}

	return dTorus{dim: nDim, dSize: dimSize, vector: values}
}

// Increments a single coordinate by one
func (dT *dTorus) Increment(cell int) {
	vector := make([]int, dT.dim)
	vector[cell] = 1
	dT.vector = AddArayTorus(*dT, CreateTorus(dT.dim, dT.dSize, vector)).vector
}

// Decrements a single coordinate by one
func (dT *dTorus) Decrement(cell int) {
	vector := make([]int, dT.dim)
	vector[cell] = 1
	dT.vector = DiffArayTorus(*dT, CreateTorus(dT.dim, dT.dSize, vector)).vector
}

// Returns distance to origin based on given norm
func (dT *dTorus) DistanceToOrigin(norm string) int {

	origin := CreateDefaultTorus(dT.dim, dT.dSize)

	if norm == "L1" {
		return TorusDistL1(*dT, origin)
	} else if norm == "LInf" {
		return TorusDistLInf(*dT, origin)
	} else {
		fmt.Println("Invalid norm was given.")
		return -1
	}

}

// Adds two arrays in dTorus and returns result
func AddArayTorus(torus1 dTorus, torus2 dTorus) dTorus {

	result := make([]int, torus1.dim)

	if torus1.dim != torus2.dim || torus1.dSize != torus2.dSize {

		fmt.Println("Torus number of dimensions or size does not match.")
		//return nil

	} else {

		for i := 0; i < torus1.dim; i++ {
			result[i] = FitInteger(torus1.vector[i]+torus2.vector[i], torus1.dSize)
		}

	}

	return CreateTorus(torus1.dim, torus1.dSize, result)
}

// Subtracts two arrays in dTorus and returns result
func DiffArayTorus(torus1 dTorus, torus2 dTorus) dTorus {

	result := make([]int, torus1.dim)

	if torus1.dim != torus2.dim || torus1.dSize != torus2.dSize {

		fmt.Println("Torus number of dimensions or size does not match.")
		//return nil

	} else {

		for i := 0; i < torus1.dim; i++ {
			result[i] = FitInteger(torus1.vector[i]-torus2.vector[i], torus1.dSize)
		}

	}

	return CreateTorus(torus1.dim, torus1.dSize, result)

}

// Computes the distance between two dTorus vectors according to L1 norm
func TorusDistL1(torus1 dTorus, torus2 dTorus) int {

	sum := int(0)

	if torus1.dim != torus2.dim || torus1.dSize != torus2.dSize {

		fmt.Println("Torus number of dimensions or size does not match.")
		//return nil

	} else {

		diff := int(0)
		for i := 0; i < torus1.dim; i++ {
			diff = FitInteger(torus1.vector[i]-torus2.vector[i], torus1.dSize)
			sum = sum + Min(diff, torus1.dSize-diff)
		}

	}

	return sum

}

// Computes the distance between two dTorus vectors according to L(Infinity) norm
func TorusDistLInf(torus1 dTorus, torus2 dTorus) int {

	max := int(0)

	if torus1.dim != torus2.dim || torus1.dSize != torus2.dSize {

		fmt.Println("Torus number of dimensions or size does not match.")
		//return nil

	} else {

		diff := int(0)
		for i := 0; i < torus1.dim; i++ {
			diff = FitInteger(torus1.vector[i]-torus2.vector[i], torus1.dSize)
			max = Max(max, Min(diff, torus1.dSize-diff))
		}

	}

	return max

}

// Fits an integer into a ring
func FitInteger(number int, mod int) int {
	x := number % mod
	if x < 0 {
		x = mod + x
	}
	return x
}

// Reuturns max between two integers
func Max(x int, y int) int {
	if x >= y {
		return x
	} else {
		return y
	}
}

// Reuturns min between two integers
func Min(x int, y int) int {
	if x >= y {
		return y
	} else {
		return x
	}
}

func random(min int, max int) int {
	return rand.Intn(max-min) + min
}

func main() {

	rand.Seed(9)

	nBitsTotal := 256
	nDim := 32

	bitsPerDim := float64(nBitsTotal / nDim)
	dimSize := int(math.Pow(2, bitsPerDim))

	torus1 := CreateDefaultTorus(nDim, dimSize)
	//torus2 := CreateTorus(nDim, dimSize, []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536})

	for i := 0; i < 10000; i++ {
		cell := random(0, nDim)
		action := random(0, 2)

		if action%2 == 0 {
			torus1.Increment(cell)
		} else {
			torus1.Decrement(cell)
		}
	}

	fmt.Println(torus1)
	fmt.Println(torus1.DistanceToOrigin("L1"))
	fmt.Println(torus1.DistanceToOrigin("LInf"))

	fmt.Println((nDim * dimSize / 2) / torus1.DistanceToOrigin("L1"))
	fmt.Println((dimSize / 2) / torus1.DistanceToOrigin("LInf"))

	//fmt.Println(torus2.DistanceToOrigin("L1"))
	//fmt.Println(torus2.DistanceToOrigin("LInf"))
}

