package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func New(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("id: crypto/rand: %w", err))
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func Session() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("id: crypto/rand: %w", err))
	}
	return hex.EncodeToString(b[:])
}
