package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// generateHMACSecret creates a machine-specific secret key
// This key is derived from hardware identifiers and cannot be easily reproduced
func generateHMACSecret() string {
	var secretMaterial string
	
	// Use multiple sources unique to this machine
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		secretMaterial += string(data)
	}
	
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		secretMaterial += string(data)
	}
	
	hostname, _ := os.Hostname()
	secretMaterial += hostname
	
	// Add a salt that's compiled into the binary
	secretMaterial += "wizado-license-signing-key-v1-go"
	
	// Derive key using SHA-256
	hash := sha256.Sum256([]byte(secretMaterial))
	return hex.EncodeToString(hash[:])
}

// ComputeSignature generates an HMAC-SHA256 signature for license data
func ComputeSignature(key, email, machineID string, activatedAt time.Time) string {
	secretKey := generateHMACSecret()
	
	dataToSign := fmt.Sprintf("%s|%s|%s|%s", key, email, machineID, activatedAt.Format(time.RFC3339))
	
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(dataToSign))
	
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature checks if the license signature is valid
func VerifySignature(license *License) bool {
	if license.Signature == "" {
		return false
	}
	
	expectedSig := ComputeSignature(license.Key, license.Email, license.MachineID, license.ActivatedAt)
	return hmac.Equal([]byte(license.Signature), []byte(expectedSig))
}

