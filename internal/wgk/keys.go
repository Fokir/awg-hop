package wgk

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

func GenerateKeyPair() (privateKey, publicKey string, err error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)

	return encodeKey(priv[:]), encodeKey(pub[:]), nil
}

func GeneratePresharedKey() (string, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return "", err
	}
	return encodeKey(k[:]), nil
}

func encodeKey(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func MustPublicFromPrivate(privateKey string) string {
	raw, err := decodeKey(privateKey)
	if err != nil {
		return ""
	}
	var priv [32]byte
	copy(priv[:], raw)
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)
	return encodeKey(pub[:])
}

func decodeKey(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != 32 {
		return nil, errors.New("key must be 32 bytes")
	}
	return b, nil
}

func ValidateKey(s string) error {
	_, err := decodeKey(s)
	if err != nil {
		return fmt.Errorf("invalid wireguard key: %w", err)
	}
	return nil
}
