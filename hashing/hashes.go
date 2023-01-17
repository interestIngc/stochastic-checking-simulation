package hashing

import (
	"crypto/sha256"
	"crypto/sha512"
)

type Hasher interface {
	Hash([]byte) []byte
}

type HashSHA256 struct {}

func (h HashSHA256) Hash(data []byte) []byte {
	bytes := sha256.Sum256(data)
	return bytes[:]
}

type HashSHA512 struct {}

func (h HashSHA512) Hash(data []byte) []byte {
	bytes := sha512.Sum512(data)
	return bytes[:]
}
