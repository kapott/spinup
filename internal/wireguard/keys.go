// Package wireguard provides WireGuard key generation and tunnel management.
package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// KeyPair represents a WireGuard private/public key pair.
type KeyPair struct {
	// PrivateKey is the base64-encoded WireGuard private key.
	PrivateKey string
	// PublicKey is the base64-encoded WireGuard public key derived from the private key.
	PublicKey string
}

var (
	// ErrInvalidPrivateKey is returned when a private key is invalid or malformed.
	ErrInvalidPrivateKey = errors.New("invalid WireGuard private key")
	// ErrKeyGenerationFailed is returned when key generation fails.
	ErrKeyGenerationFailed = errors.New("failed to generate WireGuard key pair")
)

// GenerateKeyPair generates a new WireGuard private/public key pair.
// The keys are returned as base64-encoded strings, compatible with WireGuard configuration files.
func GenerateKeyPair() (*KeyPair, error) {
	// Generate a new private key using wgtypes
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
	}

	// Derive the public key from the private key
	publicKey := privateKey.PublicKey()

	return &KeyPair{
		PrivateKey: privateKey.String(),
		PublicKey:  publicKey.String(),
	}, nil
}

// PublicKeyFromPrivate derives the public key from a base64-encoded private key.
// This is useful when you have an existing private key and need to compute its public key.
func PublicKeyFromPrivate(privateKeyBase64 string) (string, error) {
	// Parse the private key from base64
	privateKey, err := wgtypes.ParseKey(privateKeyBase64)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPrivateKey, err)
	}

	// Derive the public key
	publicKey := privateKey.PublicKey()

	return publicKey.String(), nil
}

// ValidatePrivateKey checks if a string is a valid WireGuard private key.
// Returns nil if valid, or an error describing the issue.
func ValidatePrivateKey(privateKeyBase64 string) error {
	_, err := wgtypes.ParseKey(privateKeyBase64)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPrivateKey, err)
	}
	return nil
}

// ValidatePublicKey checks if a string is a valid WireGuard public key.
// Returns nil if valid, or an error describing the issue.
func ValidatePublicKey(publicKeyBase64 string) error {
	_, err := wgtypes.ParseKey(publicKeyBase64)
	if err != nil {
		return fmt.Errorf("invalid WireGuard public key: %v", err)
	}
	return nil
}

// GenerateKeyPairRaw generates a WireGuard key pair using raw curve25519 operations.
// This is an alternative implementation that doesn't depend on wgtypes for key generation,
// useful for environments where wgctrl may not be fully available.
func GenerateKeyPairRaw() (*KeyPair, error) {
	// Generate 32 random bytes for the private key
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return nil, fmt.Errorf("%w: failed to generate random bytes: %v", ErrKeyGenerationFailed, err)
	}

	// Apply WireGuard clamping to the private key
	// See: https://cr.yp.to/ecdh.html
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive the public key using curve25519
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	return &KeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey[:]),
	}, nil
}

// KeyPairFromPrivate creates a KeyPair from an existing private key string.
// This is useful when loading keys from configuration.
func KeyPairFromPrivate(privateKeyBase64 string) (*KeyPair, error) {
	publicKey, err := PublicKeyFromPrivate(privateKeyBase64)
	if err != nil {
		return nil, err
	}
	return &KeyPair{
		PrivateKey: privateKeyBase64,
		PublicKey:  publicKey,
	}, nil
}
