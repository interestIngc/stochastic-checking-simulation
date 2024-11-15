package main

import (
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"log"
	"os"
	"stochastic-checking-simulation/impl/utils"
)

func main() {
	nodes := 260

	for i := 0; i < nodes; i++ {
		outputFile, err := os.Create(fmt.Sprintf("keys/%d.txt", i))
		if err != nil {
			log.Fatal("Error while creating an output file")
		}

		pkBytes := utils.Int32ToBytes(int32(i + 1))
		privateKey := secp256k1.PrivKeyFromBytes(pkBytes)

		privateKeyBytes := privateKey.Serialize()

		n, err := outputFile.Write(privateKeyBytes)
		if err != nil || n < len(privateKeyBytes) {
			log.Fatalf("Error while writing private key to the output file %e", err)
		}
	}
}
