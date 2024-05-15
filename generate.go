package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log"
	"os"
)

func main() {
	nodes := 260

	for i := 0; i < nodes; i++ {
		outputFile, err := os.Create(fmt.Sprintf("keys/%d.txt", i))
		if err != nil {
			log.Fatal("Error while creating an output file")
		}

		privateKey, err := rsa.GenerateKey(rand.Reader, 128)
		if err != nil {
			log.Fatalf("Error while generating a private key: %e", err)
		}
		privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

		n, err := outputFile.Write(privateKeyBytes)
		if err != nil || n < len(privateKeyBytes) {
			log.Fatalf("Error while writing private key to the output file %e", err)
		}
	}
}
