package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func New(prefix string) (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("idgen: read random bytes: %w", err)
	}

	id := hex.EncodeToString(bytes[:])
	if prefix == "" {
		return id, nil
	}

	return prefix + "_" + id, nil
}

func MustNew(prefix string) string {
	id, err := New(prefix)
	if err != nil {
		panic(err)
	}
	return id
}
