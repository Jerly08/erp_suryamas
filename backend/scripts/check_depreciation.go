// Script untuk monitor dan test penyusutan aset
// Jalankan: go run scripts/check_depreciation.go
// Dengan trigger manual: go run scripts/check_depreciation.go --run
// Dengan bulan tertentu: go run scripts/check_depreciation.go --run 2026 3
//
// Fitur:
// 1. Menampilkan semua aset yang aktif dan bisa disusutkan
// 2. Menghitung penyusutan bulanan per aset
// 3. Menampilkan jurnal penyusutan yang sudah tercatat
// 4. Simulasi penyusutan bulan depan
// 5. Bisa trigger depreciation run via API

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"
)

const baseURL = "http://localhost:8080/api/v1"

type Asset struct {
	ID                      uint    `json:"id"`
	Code                    string  `json:"code"`
	Name                    string  `json:"name"`
	Category                string  `json:"category"`
	Status                  string  `json:"status"`
	IsActive                bool    `json:"is_active"`
	PurchaseDate            string  `json:"purchase_date"`
	PurchasePrice           float64 `json:"purchase_price"`
	SalvageValue            float64 `json:"salvage_value"`
	UsefulLife              int     `json:"useful_life"`
	DepreciationMethod      string  `json:"depreciation_method"`
	AccumulatedDepreciation float64 `json:"accumulated_depreciation"`
}

func formatCurrency(amount float64) string {
	str := fmt.Sprintf("%.0f", amount)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += "."
		}
		result += string(c)
	}
	return "Rp " + result
}

// Login and get JWT token
func getAuthToken() (string, error) {
	loginData := map[string]string{
		"email":    "admin@company.com",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	resp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var loginResp map[string]interface{}
	json.Unmarshal(body, &loginResp)

	token, ok := loginResp["token"].(string)
	if !ok {
		return "", fmt.Errorf("no token in login response: %s", string(body))
	}
	return token, nil
}

// Make authenticated GET request
func authGet(url string, token string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Make authenticated POST request
func authPost(url string, token string, payload []byte) ([]byte, int, error) {
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║         📊 DEPRECIATION MONITOR & TEST SCRIPT              ║")
	fmt.Println("║         Monitor Penyusutan Aset Tetap                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	now := time.Now()
	fmt.Printf("📅 Tanggal sekarang: %s\n", now.Format("02 January 2006 15:04"))
	fmt.Println()

	// Login first
	fmt.Print("🔐 Login... ")
	token, err := getAuthToken()
	if err != nil {
		fmt.Printf("\n❌ Login gagal: %v\n", err)
		fmt.Println("   Pastikan backend sudah jalan dan credentials benar")
		os.Exit(1)
	}
	fmt.Println("OK ✅")
	fmt.Println()

	// 1. Fetch all assets
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📋 DAFTAR SEMUA ASET")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	body, err := authGet(baseURL+"/assets", token)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	var assetsResp struct {
		Data []Asset `json:"data"`
	}
	json.Unmarshal(body, &assetsResp)

	if len(assetsResp.Data) == 0 {
		fmt.Println("⚠️  Tidak ada aset ditemukan.")
		os.Exit(0)
	}

	depreciableCount := 0
	var totalMonthlyDep float64

	for _, a := range assetsResp.Data {
		statusIcon := "⚪"
		switch a.Status {
		case "ACTIVE":
			statusIcon = "🟢"
		case "PENDING":
			statusIcon = "⏳"
		case "INACTIVE":
			statusIcon = "⚪"
		case "SOLD":
			statusIcon = "🔴"
		}

		fmt.Printf("\n%s [%s] %s\n", statusIcon, a.Code, a.Name)
		fmt.Printf("   Kategori: %-20s Status: %-15s IsActive: %v\n", a.Category, a.Status, a.IsActive)
		fmt.Printf("   Harga Beli: %-20s Nilai Sisa: %s\n", formatCurrency(a.PurchasePrice), formatCurrency(a.SalvageValue))
		fmt.Printf("   Masa Manfaat: %d tahun          Metode: %s\n", a.UsefulLife, a.DepreciationMethod)
		fmt.Printf("   Akumulasi Penyusutan: %s\n", formatCurrency(a.AccumulatedDepreciation))

		if a.IsActive && a.DepreciationMethod != "" && a.UsefulLife > 0 {
			depreciableCount++
			bookValue := a.PurchasePrice - a.AccumulatedDepreciation
			var monthlyDep float64

			if a.DepreciationMethod == "STRAIGHT_LINE" {
				monthlyDep = (a.PurchasePrice - a.SalvageValue) / float64(a.UsefulLife*12)
			} else if a.DepreciationMethod == "DECLINING_BALANCE" {
				rate := 2.0 / float64(a.UsefulLife)
				monthlyDep = bookValue * rate / 12
			}

			if bookValue-monthlyDep < a.SalvageValue {
				monthlyDep = math.Max(0, bookValue-a.SalvageValue)
			}

			totalMonthlyDep += monthlyDep
			fmt.Printf("   📊 Penyusutan/bulan: %s\n", formatCurrency(monthlyDep))
			fmt.Printf("   📊 Nilai Buku: %s\n", formatCurrency(bookValue))

			totalMonths := a.UsefulLife * 12
			depMonths := 0
			if monthlyDep > 0 {
				depMonths = int(a.AccumulatedDepreciation / monthlyDep)
			}
			remainingMonths := totalMonths - depMonths
			fmt.Printf("   📊 Progress: %d/%d bulan (%d bulan tersisa)\n", depMonths, totalMonths, remainingMonths)
		} else {
			reason := ""
			if !a.IsActive {
				reason = "is_active=false"
			} else if a.DepreciationMethod == "" {
				reason = "tidak ada metode penyusutan"
			} else if a.UsefulLife <= 0 {
				reason = "masa manfaat = 0"
			}
			fmt.Printf("   ⏸️  TIDAK DISUSUTKAN (%s)\n", reason)
		}
	}

	// 2. Summary
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 RINGKASAN PENYUSUTAN")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   Total aset: %d\n", len(assetsResp.Data))
	fmt.Printf("   Aset yang disusutkan: %d\n", depreciableCount)
	fmt.Printf("   Total penyusutan/bulan: %s\n", formatCurrency(totalMonthlyDep))
	fmt.Printf("   Total penyusutan/tahun: %s\n", formatCurrency(totalMonthlyDep*12))

	// 3. Scheduler status
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("⏰ STATUS SCHEDULER")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	prevMonth := now.AddDate(0, -1, 0)
	nextMonth := now.AddDate(0, 1, 0)

	fmt.Printf("   Bulan lalu: %s %d\n", prevMonth.Month().String(), prevMonth.Year())
	fmt.Printf("   Bulan ini:  %s %d\n", now.Month().String(), now.Year())
	fmt.Printf("   Bulan depan: %s %d\n", nextMonth.Month().String(), nextMonth.Year())
	fmt.Println()

	if now.Day() <= 5 {
		fmt.Println("   🟢 Sekarang tanggal 1-5, scheduler AKTIF untuk proses bulan lalu!")
	} else {
		fmt.Printf("   ⏳ Scheduler akan berjalan otomatis pada 1-5 %s %d\n", nextMonth.Month().String(), nextMonth.Year())
		fmt.Println("   ℹ️  Gunakan --run untuk trigger manual sekarang")
	}

	// 4. Simulation
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔮 SIMULASI PENYUSUTAN BULAN INI (Maret 2026)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if depreciableCount > 0 {
		fmt.Printf("   Aset yang akan disusutkan: %d\n", depreciableCount)
		fmt.Printf("   Total penyusutan: %s\n\n", formatCurrency(totalMonthlyDep))
		fmt.Println("   Jurnal yang akan tercatat per aset:")
		fmt.Println("   ┌──────────────────────────────────────────────────────┐")
		fmt.Printf("   │  Debit:  6103 Beban Penyusutan   %18s │\n", formatCurrency(totalMonthlyDep))
		fmt.Printf("   │  Credit: 1201 Akum. Penyusutan   %18s │\n", formatCurrency(totalMonthlyDep))
		fmt.Println("   └──────────────────────────────────────────────────────┘")
	} else {
		fmt.Println("   ⚠️  Tidak ada aset aktif yang bisa disusutkan!")
		fmt.Println("   ℹ️  Pastikan status aset = ACTIVE dan is_active = true")
	}

	// 5. Manual trigger option
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🚀 TRIGGER PENYUSUTAN MANUAL")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	currentYear := now.Year()
	currentMonth := int(now.Month())

	if len(os.Args) > 1 && os.Args[1] == "--run" {
		year := currentYear
		month := currentMonth
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &year)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &month)
		}

		fmt.Printf("\n🚀 Menjalankan penyusutan untuk %d-%02d...\n\n", year, month)

		payload, _ := json.Marshal(map[string]interface{}{
			"year":    year,
			"month":   month,
			"user_id": 1,
		})

		respBody, statusCode, err := authPost(baseURL+"/assets/run-depreciation", token, payload)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("   HTTP Status: %d\n", statusCode)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)
		prettyJSON, _ := json.MarshalIndent(result, "   ", "  ")
		fmt.Printf("   Response:\n   %s\n", string(prettyJSON))

		if statusCode == 200 {
			fmt.Println("\n   ✅ Penyusutan berhasil dijalankan!")
			fmt.Println("   📒 Cek halaman Jurnal untuk melihat entri penyusutan")
		} else {
			fmt.Println("\n   ⚠️  Ada masalah, cek response di atas")
		}
	} else {
		fmt.Println()
		fmt.Println("   Gunakan perintah berikut untuk trigger manual:")
		fmt.Println()
		fmt.Printf("   go run scripts/check_depreciation.go --run              (bulan ini: %d-%02d)\n", currentYear, currentMonth)
		fmt.Printf("   go run scripts/check_depreciation.go --run %d %d   (bulan tertentu)\n", currentYear, currentMonth)
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════")
}
