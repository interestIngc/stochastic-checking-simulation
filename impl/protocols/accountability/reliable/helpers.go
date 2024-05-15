package reliable

import (
	"crypto/rsa"
	"stochastic-checking-simulation/impl/eventlogger"
)

type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i, _ := range p {
		p[i] = 0
	}
	n = len(p)
	return
}

func encrypt(
	publicKey *rsa.PublicKey,
	share []byte,
	logger *eventlogger.EventLogger,
) []byte {
	//cipher, err := rsa.EncryptPKCS1v15(zeroReader{}, publicKey, share)
	//if err != nil {
	//	logger.Fatal("Error while encrypting a share: " + err.Error())
	//}
	//
	//return cipher

	output := make([]byte, len(share))
	copy(output, share)
	return output
}

func decrypt(
	privateKey *rsa.PrivateKey,
	encryptedShare []byte,
	logger *eventlogger.EventLogger,
) []byte {
	//share, err := rsa.DecryptPKCS1v15(nil, privateKey, encryptedShare)
	//if err != nil {
	//	logger.Fatal("Could not decrypt ciphertext: " + err.Error())
	//}
	//return share

	output := make([]byte, len(encryptedShare))
	copy(output, encryptedShare)
	return output
}

func hash(input []int32) int32 {
	var h int32 = 0

	for _, val := range input {
		h += val
	}

	return h
}
