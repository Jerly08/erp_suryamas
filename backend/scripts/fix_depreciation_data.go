// Script to fix the wrong depreciation_account_id and correct journal entries
// Run: go run scripts/fix_depreciation_data.go

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const baseURL = "http://localhost:8080/api/v1"

func getToken() string {
	loginData, _ := json.Marshal(map[string]string{
		"email":    "admin@company.com",
		"password": "password123",
	})
	resp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(loginData))
	if err != nil {
		fmt.Printf("Login error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	token, _ := result["token"].(string)
	return token
}

func authGet(url, token string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

func authPut(url, token string, payload map[string]interface{}) (map[string]interface{}, int, error) {
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, resp.StatusCode, nil
}

func authDelete(url, token string) (int, error) {
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func authPost(url, token string, payload map[string]interface{}) (map[string]interface{}, int, error) {
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, resp.StatusCode, nil
}

func main() {
	fmt.Println("=== FIX DEPRECIATION DATA ===")
	fmt.Println()

	token := getToken()
	fmt.Println("[1/4] Login OK")

	// Step 1: Get current asset data
	assetResp, err := authGet(baseURL+"/assets/1", token)
	if err != nil {
		fmt.Printf("Error getting asset: %v\n", err)
		os.Exit(1)
	}
	assetData := assetResp["data"].(map[string]interface{})
	fmt.Printf("[2/4] Asset: %s (ID: %.0f)\n", assetData["name"], assetData["id"])
	fmt.Printf("       Current depreciation_account_id: %.0f\n", assetData["depreciation_account_id"])
	fmt.Printf("       Current accumulated_depreciation: %.0f\n", assetData["accumulated_depreciation"])

	// Step 2: Update asset with correct depreciation account (6103 = ID 64)
	updatePayload := map[string]interface{}{
		"name":                     assetData["name"],
		"category":                 assetData["category"],
		"purchase_date":            assetData["purchase_date"],
		"purchase_price":           assetData["purchase_price"],
		"useful_life":              assetData["useful_life"],
		"depreciation_method":      assetData["depreciation_method"],
		"status":                   assetData["status"],
		"is_active":                assetData["is_active"],
		"salvage_value":            assetData["salvage_value"],
		"condition":                assetData["condition"],
		"depreciation_account_id":  64, // 6103 - BEBAN PENYUSUTAN (correct!)
		"asset_account_id":         assetData["asset_account_id"],
		"accumulated_depreciation": 0,  // Reset to 0, will be recalculated
	}

	result, status, err := authPut(baseURL+"/assets/1", token, updatePayload)
	if err != nil {
		fmt.Printf("Error updating asset: %v\n", err)
		os.Exit(1)
	}
	if status == 200 {
		fmt.Println("[3/4] Asset depreciation_account_id updated to 64 (6103 - BEBAN PENYUSUTAN)")
		fmt.Println("       Accumulated depreciation reset to 0")
	} else {
		fmt.Printf("[3/4] ERROR updating asset: %d - %v\n", status, result)
	}

	// Step 3: Delete wrong journal entries (DEPRECIATION source type)
	fmt.Println("[4/4] Checking for wrong depreciation journals to clean up...")
	fmt.Println()
	fmt.Println("       NOTE: Wrong journals with account 1202-003 need to be")
	fmt.Println("       manually deleted from the unified journal, or run the")
	fmt.Println("       depreciation script again which will create correct entries.")
	fmt.Println()

	// Step 4: Re-run depreciation for March 2026 to create correct journal
	fmt.Println("=== Re-running depreciation for March 2026 ===")
	depPayload := map[string]interface{}{
		"year":    2026,
		"month":   3,
		"user_id": 1,
	}
	depResult, depStatus, err := authPost(baseURL+"/assets/run-depreciation", token, depPayload)
	if err != nil {
		fmt.Printf("Error running depreciation: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Status: %d\n", depStatus)
	prettyJSON, _ := json.MarshalIndent(depResult, "", "  ")
	fmt.Println(string(prettyJSON))

	// Verify final state
	fmt.Println()
	fmt.Println("=== VERIFICATION ===")
	assetResp2, _ := authGet(baseURL+"/assets/1", token)
	assetData2 := assetResp2["data"].(map[string]interface{})
	fmt.Printf("Asset: %s\n", assetData2["name"])
	fmt.Printf("  depreciation_account_id: %.0f (should be 64)\n", assetData2["depreciation_account_id"])
	fmt.Printf("  accumulated_depreciation: %.0f\n", assetData2["accumulated_depreciation"])
	fmt.Printf("  status: %s\n", assetData2["status"])

	fmt.Println()
	fmt.Println("=== DONE ===")
	fmt.Println("Please check the Journal page to verify the correct entries.")
	fmt.Println("The old wrong journals with 1202-003 should be manually deleted.")
}
