package main

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log"
	"os"
)

func main() {
	nodes := 260

	publicKeys := make([]*rsa.PublicKey, nodes)
	for i := 0; i < nodes; i++ {
		filename := fmt.Sprintf("%s/%d.txt", "keys", i)
		privateKeyBytes, err := os.ReadFile(filename)

		if err != nil {
			log.Fatal(
				fmt.Sprintf("Could not read bytes from file %s: %e", filename, err),
			)
		}

		privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyBytes)
		if err != nil {
			log.Fatal(
				fmt.Sprintf("Error while generating a private key from bytes: %e", err),
			)
		}

		publicKeys[i] = &privateKey.PublicKey
		fmt.Println(publicKeys[i])
	}
}
