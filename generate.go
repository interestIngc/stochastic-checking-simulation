package main

import (
	//"crypto/rand"
	//"crypto/rsa"
	//"crypto/x509"
	"fmt"
	"log"
	"os"
	"stochastic-checking-simulation/impl/utils"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	//"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
)

func main() {
	nodes := 260

	for i := 0; i < nodes; i++ {
		outputFile, err := os.Create(fmt.Sprintf("keys/%d.txt", i))
		if err != nil {
			log.Fatal("Error while creating an output file")
		}

		//privateKey, err := rsa.GenerateKey(rand.Reader, 128)
		//if err != nil {
		//	log.Fatalf("Error while generating a private key: %e", err)
		//}
		
		//privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		
		pkBytes := utils.Int32ToBytes(int32(i+1))
		privateKey := secp256k1.PrivKeyFromBytes(pkBytes)
		
		privateKeyBytes := privateKey.Serialize()

		n, err := outputFile.Write(privateKeyBytes)
		if err != nil || n < len(privateKeyBytes) {
			log.Fatalf("Error while writing private key to the output file %e", err)
		}
	}
}
