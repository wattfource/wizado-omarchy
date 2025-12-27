// Package license handles license validation, activation, and storage
package license

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the current license state
type Status string

const (
	StatusValid          Status = "valid"
	StatusOfflineGrace   Status = "offline_grace"
	StatusNoLicense      Status = "no_license"
	StatusInvalid        Status = "invalid"
	StatusExpired        Status = "expired"
	StatusMachineMismatch Status = "machine_mismatch"
	StatusOfflineExpired Status = "offline_expired"
	StatusTampered       Status = "tampered"
	StatusClockTampered  Status = "clock_tampered"
)

// Configuration constants (compiled, not overridable)
const (
	apiURL           = "https://wizado.app/api"
	gracePeriodDays  = 14
	reverifyDays     = 7
	apiTimeout       = 5 * time.Second
	clockDriftTolerance = 5 * time.Minute
)

// License represents a stored license
type License struct {
	Key          string    `json:"license"`
	Email        string    `json:"email"`
	MachineID    string    `json:"machineId"`
	ActivatedAt  time.Time `json:"activatedAt"`
	LastVerified time.Time `json:"lastVerified"`
	Signature    string    `json:"signature"`
}

// Result holds the result of a license check
type Result struct {
	Status  Status
	License *License
	Error   error
}

// ActivationResult holds the result of a license activation
type ActivationResult struct {
	Success   bool
	Email     string
	SlotsUsed int
	SlotsTotal int
	Message   string
}

var (
	ErrNoLicense      = errors.New("no license found")
	ErrInvalidLicense = errors.New("invalid license")
	ErrTampered       = errors.New("license file tampered")
	ErrClockTampered  = errors.New("system clock manipulation detected")
	ErrMachineMismatch = errors.New("license activated on different machine")
	ErrNetworkError   = errors.New("network error")
)

// Paths returns the license directory and file paths
func Paths() (dir string, file string, timestampFile string) {
	home, _ := os.UserHomeDir()
	dir = filepath.Join(home, ".config", "wizado")
	file = filepath.Join(dir, "license.json")
	timestampFile = filepath.Join(dir, ".last_known_time")
	return
}

// Load reads the license from disk
func Load() (*License, error) {
	_, licenseFile, _ := Paths()
	
	data, err := os.ReadFile(licenseFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoLicense
		}
		return nil, err
	}
	
	var license License
	if err := json.Unmarshal(data, &license); err != nil {
		return nil, err
	}
	
	return &license, nil
}

// Save writes the license to disk with HMAC signature
func Save(license *License) error {
	dir, licenseFile, _ := Paths()
	
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	
	// Compute signature
	license.Signature = ComputeSignature(license.Key, license.Email, license.MachineID, license.ActivatedAt)
	
	data, err := json.MarshalIndent(license, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(licenseFile, data, 0600); err != nil {
		return err
	}
	
	// Save timestamp for clock protection
	saveTimestamp()
	
	return nil
}

// Clear removes the stored license
func Clear() error {
	_, licenseFile, timestampFile := Paths()
	os.Remove(licenseFile)
	os.Remove(timestampFile)
	return nil
}

// Check validates the stored license
func Check() Result {
	// Check clock manipulation first
	if !clockIsValid() {
		return Result{Status: StatusClockTampered, Error: ErrClockTampered}
	}
	
	license, err := Load()
	if err != nil {
		if errors.Is(err, ErrNoLicense) {
			return Result{Status: StatusNoLicense, Error: ErrNoLicense}
		}
		return Result{Status: StatusInvalid, Error: err}
	}
	
	// Verify HMAC signature
	if !VerifySignature(license) {
		Clear()
		return Result{Status: StatusTampered, Error: ErrTampered}
	}
	
	// Check machine ID
	currentMachineID := GenerateMachineID()
	if license.MachineID != currentMachineID {
		return Result{Status: StatusMachineMismatch, License: license, Error: ErrMachineMismatch}
	}
	
	// Check if re-verification is needed
	if needsReverification(license) {
		result, err := VerifyAPI(license.Email, license.Key)
		if err != nil {
			// Network error - check grace period
			if withinGracePeriod(license) {
				return Result{Status: StatusOfflineGrace, License: license}
			}
			return Result{Status: StatusOfflineExpired, License: license, Error: err}
		}
		
		if !result {
			Clear()
			return Result{Status: StatusInvalid, Error: ErrInvalidLicense}
		}
		
		// Update timestamp
		license.LastVerified = time.Now().UTC()
		Save(license)
		return Result{Status: StatusValid, License: license}
	}
	
	// Within re-verify window
	if withinGracePeriod(license) {
		return Result{Status: StatusValid, License: license}
	}
	
	return Result{Status: StatusExpired, License: license, Error: ErrInvalidLicense}
}

// Activate activates a new license
func Activate(email, key string) (*ActivationResult, error) {
	machineID := GenerateMachineID()
	
	result, err := ActivateAPI(email, key, machineID)
	if err != nil {
		return nil, err
	}
	
	if !result.Success {
		return result, errors.New(result.Message)
	}
	
	// Save the license
	now := time.Now().UTC()
	license := &License{
		Key:          key,
		Email:        email,
		MachineID:    machineID,
		ActivatedAt:  now,
		LastVerified: now,
	}
	
	if err := Save(license); err != nil {
		return nil, err
	}
	
	return result, nil
}

// needsReverification checks if we need to re-verify with the server
func needsReverification(license *License) bool {
	daysSince := time.Since(license.LastVerified).Hours() / 24
	return daysSince >= reverifyDays
}

// withinGracePeriod checks if we're within the offline grace period
func withinGracePeriod(license *License) bool {
	daysSince := time.Since(license.LastVerified).Hours() / 24
	return daysSince < gracePeriodDays
}

// clockIsValid checks for clock manipulation
func clockIsValid() bool {
	_, _, timestampFile := Paths()
	
	data, err := os.ReadFile(timestampFile)
	if err != nil {
		return true // No reference timestamp, assume valid
	}
	
	var lastKnown int64
	if _, err := fmt.Sscanf(string(data), "%d", &lastKnown); err != nil {
		return true
	}
	
	now := time.Now().Unix()
	drift := time.Duration(lastKnown-now) * time.Second
	
	return drift <= clockDriftTolerance
}

// saveTimestamp saves the current time for clock protection
func saveTimestamp() {
	dir, _, timestampFile := Paths()
	os.MkdirAll(dir, 0700)
	now := time.Now().Unix()
	os.WriteFile(timestampFile, []byte(fmt.Sprintf("%d", now)), 0600)
}

