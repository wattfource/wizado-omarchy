package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// API client for wizado.app license server

type verifyRequest struct {
	Email   string `json:"email"`
	License string `json:"license"`
}

type verifyResponse struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

type activateRequest struct {
	Email     string `json:"email"`
	License   string `json:"license"`
	MachineID string `json:"machineId"`
}

type activateResponse struct {
	Activated  bool   `json:"activated"`
	Email      string `json:"email,omitempty"`
	SlotsUsed  int    `json:"slotsUsed,omitempty"`
	SlotsTotal int    `json:"slotsTotal,omitempty"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

// VerifyAPI calls the license verification API
func VerifyAPI(email, licenseKey string) (bool, error) {
	client := &http.Client{Timeout: apiTimeout}
	
	reqBody := verifyRequest{
		Email:   email,
		License: licenseKey,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return false, err
	}
	
	req, err := http.NewRequest("POST", apiURL+"/license/verify", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return false, ErrNetworkError
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 500 {
		return false, ErrNetworkError
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	
	var result verifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	
	return result.Valid, nil
}

// ActivateAPI calls the license activation API
func ActivateAPI(email, licenseKey, machineID string) (*ActivationResult, error) {
	client := &http.Client{Timeout: apiTimeout * 2}
	
	reqBody := activateRequest{
		Email:     email,
		License:   licenseKey,
		MachineID: machineID,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", apiURL+"/license/activate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return &ActivationResult{
			Success: false,
			Message: fmt.Sprintf("Network error: %v", err),
		}, ErrNetworkError
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var apiResp activateResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}
	
	result := &ActivationResult{
		Success:    apiResp.Activated,
		Email:      apiResp.Email,
		SlotsUsed:  apiResp.SlotsUsed,
		SlotsTotal: apiResp.SlotsTotal,
	}
	
	if apiResp.Message != "" {
		result.Message = apiResp.Message
	} else if apiResp.Error != "" {
		result.Message = apiResp.Error
	} else if !apiResp.Activated {
		result.Message = "Activation failed"
	}
	
	return result, nil
}

// RecoverAPI retrieves a license by email
func RecoverAPI(email string) (string, error) {
	client := &http.Client{Timeout: apiTimeout}
	
	reqBody := map[string]string{"email": email}
	jsonData, _ := json.Marshal(reqBody)
	
	req, err := http.NewRequest("POST", apiURL+"/license/recover", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return "", ErrNetworkError
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	
	if license, ok := result["license"].(string); ok {
		return license, nil
	}
	
	if errMsg, ok := result["error"].(string); ok {
		return "", fmt.Errorf("%s", errMsg)
	}
	
	return "", fmt.Errorf("license not found")
}

