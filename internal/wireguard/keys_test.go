package wireguard

import (
	"encoding/base64"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Verify keys are not empty
	if kp.PrivateKey == "" {
		t.Error("PrivateKey is empty")
	}
	if kp.PublicKey == "" {
		t.Error("PublicKey is empty")
	}

	// Verify keys are valid base64
	privBytes, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Errorf("PrivateKey is not valid base64: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("PrivateKey decoded length = %d, want 32", len(privBytes))
	}

	pubBytes, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Errorf("PublicKey is not valid base64: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("PublicKey decoded length = %d, want 32", len(pubBytes))
	}

	// Verify keys are unique (generate another pair)
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Second GenerateKeyPair() error = %v", err)
	}
	if kp.PrivateKey == kp2.PrivateKey {
		t.Error("Two generated private keys are identical")
	}
	if kp.PublicKey == kp2.PublicKey {
		t.Error("Two generated public keys are identical")
	}
}

func TestPublicKeyFromPrivate(t *testing.T) {
	// Generate a key pair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Derive public key from private key
	derivedPubKey, err := PublicKeyFromPrivate(kp.PrivateKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivate() error = %v", err)
	}

	// Verify derived public key matches original
	if derivedPubKey != kp.PublicKey {
		t.Errorf("PublicKeyFromPrivate() = %v, want %v", derivedPubKey, kp.PublicKey)
	}
}

func TestPublicKeyFromPrivate_InvalidKey(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
	}{
		{"empty", ""},
		{"invalid base64", "not-valid-base64!!!"},
		{"wrong length", "YWJj"}, // "abc" in base64, only 3 bytes
		{"too long", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, // 39 bytes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PublicKeyFromPrivate(tt.privateKey)
			if err == nil {
				t.Error("PublicKeyFromPrivate() expected error, got nil")
			}
		})
	}
}

func TestValidatePrivateKey(t *testing.T) {
	// Generate a valid key pair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Valid key should pass validation
	if err := ValidatePrivateKey(kp.PrivateKey); err != nil {
		t.Errorf("ValidatePrivateKey() error = %v for valid key", err)
	}

	// Invalid keys should fail
	if err := ValidatePrivateKey(""); err == nil {
		t.Error("ValidatePrivateKey() expected error for empty key")
	}
	if err := ValidatePrivateKey("invalid"); err == nil {
		t.Error("ValidatePrivateKey() expected error for invalid key")
	}
}

func TestValidatePublicKey(t *testing.T) {
	// Generate a valid key pair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Valid key should pass validation
	if err := ValidatePublicKey(kp.PublicKey); err != nil {
		t.Errorf("ValidatePublicKey() error = %v for valid key", err)
	}

	// Invalid keys should fail
	if err := ValidatePublicKey(""); err == nil {
		t.Error("ValidatePublicKey() expected error for empty key")
	}
	if err := ValidatePublicKey("invalid"); err == nil {
		t.Error("ValidatePublicKey() expected error for invalid key")
	}
}

func TestGenerateKeyPairRaw(t *testing.T) {
	kp, err := GenerateKeyPairRaw()
	if err != nil {
		t.Fatalf("GenerateKeyPairRaw() error = %v", err)
	}

	// Verify keys are not empty
	if kp.PrivateKey == "" {
		t.Error("PrivateKey is empty")
	}
	if kp.PublicKey == "" {
		t.Error("PublicKey is empty")
	}

	// Verify keys are valid base64 with correct length
	privBytes, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Errorf("PrivateKey is not valid base64: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("PrivateKey decoded length = %d, want 32", len(privBytes))
	}

	pubBytes, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Errorf("PublicKey is not valid base64: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("PublicKey decoded length = %d, want 32", len(pubBytes))
	}

	// Verify the public key derivation is correct
	// By deriving again from the private key
	derivedPubKey, err := PublicKeyFromPrivate(kp.PrivateKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivate() error = %v", err)
	}
	if derivedPubKey != kp.PublicKey {
		t.Errorf("Raw generated public key doesn't match derived: got %v, derived %v", kp.PublicKey, derivedPubKey)
	}
}

func TestKeyPairFromPrivate(t *testing.T) {
	// Generate a key pair
	original, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Create key pair from private key
	restored, err := KeyPairFromPrivate(original.PrivateKey)
	if err != nil {
		t.Fatalf("KeyPairFromPrivate() error = %v", err)
	}

	// Verify keys match
	if restored.PrivateKey != original.PrivateKey {
		t.Errorf("PrivateKey = %v, want %v", restored.PrivateKey, original.PrivateKey)
	}
	if restored.PublicKey != original.PublicKey {
		t.Errorf("PublicKey = %v, want %v", restored.PublicKey, original.PublicKey)
	}
}

func TestKeyPairFromPrivate_InvalidKey(t *testing.T) {
	_, err := KeyPairFromPrivate("invalid-key")
	if err == nil {
		t.Error("KeyPairFromPrivate() expected error for invalid key")
	}
}

// Test that generated keys are compatible with WireGuard
func TestKeyCompatibility(t *testing.T) {
	// Test that both generation methods produce compatible keys
	kpWgTypes, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	kpRaw, err := GenerateKeyPairRaw()
	if err != nil {
		t.Fatalf("GenerateKeyPairRaw() error = %v", err)
	}

	// Both should pass validation
	if err := ValidatePrivateKey(kpWgTypes.PrivateKey); err != nil {
		t.Errorf("wgtypes key failed validation: %v", err)
	}
	if err := ValidatePrivateKey(kpRaw.PrivateKey); err != nil {
		t.Errorf("raw key failed validation: %v", err)
	}

	// Both should have derivable public keys
	pub1, err := PublicKeyFromPrivate(kpWgTypes.PrivateKey)
	if err != nil {
		t.Errorf("wgtypes public key derivation failed: %v", err)
	}
	if pub1 != kpWgTypes.PublicKey {
		t.Errorf("wgtypes public key mismatch")
	}

	pub2, err := PublicKeyFromPrivate(kpRaw.PrivateKey)
	if err != nil {
		t.Errorf("raw public key derivation failed: %v", err)
	}
	if pub2 != kpRaw.PublicKey {
		t.Errorf("raw public key mismatch")
	}
}
