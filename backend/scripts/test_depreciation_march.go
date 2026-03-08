// +build ignore

package main

import (
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load env
	godotenv.Load()

	// Connect to database
	db := database.ConnectDB()

	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("   TEST PENYUSUTAN BULANAN - MARET 2026")
	fmt.Println("════════════════════════════════════════════════════════════")

	// Run depreciation for March 2026
	assetJournalSvc := services.NewAssetJournalServiceSSOT(db)
	result, err := assetJournalSvc.RunMonthlyDepreciation(2026, 3, 1) // year=2026, month=3, userID=1
	if err != nil {
		log.Fatalf("❌ Failed to run depreciation: %v", err)
	}

	fmt.Println()
	fmt.Println("📊 HASIL RUN PENYUSUTAN:")
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Printf("   Periode          : %s\n", result.Period)
	fmt.Printf("   Total Aset       : %d\n", result.TotalAssets)
	fmt.Printf("   Diproses         : %d\n", result.ProcessedAssets)
	fmt.Printf("   Dilewati (Skip)  : %d\n", result.SkippedAssets)
	fmt.Printf("   Total Depresiasi : Rp %.2f\n", result.TotalDepreciation)
	if len(result.Errors) > 0 {
		fmt.Println("   Errors:")
		for _, e := range result.Errors {
			fmt.Printf("     ⚠️ %s\n", e)
		}
	}

	// Now query the journals that were created
	fmt.Println()
	fmt.Println("📒 JURNAL PENYUSUTAN YANG TERBENTUK:")
	fmt.Println("────────────────────────────────────────────────────────────")

	type JournalResult struct {
		EntryID     uint    `gorm:"column:entry_id"`
		EntryNumber string  `gorm:"column:entry_number"`
		EntryDate   string  `gorm:"column:entry_date"`
		Description string  `gorm:"column:description"`
		Reference   string  `gorm:"column:reference"`
		Status      string  `gorm:"column:status"`
		LineNumber  int     `gorm:"column:line_number"`
		AccountCode string  `gorm:"column:account_code"`
		AccountName string  `gorm:"column:account_name"`
		Debit       float64 `gorm:"column:debit"`
		Credit      float64 `gorm:"column:credit"`
		LineDesc    string  `gorm:"column:line_desc"`
	}

	var journals []JournalResult
	query := `
		SELECT 
			e.id as entry_id,
			e.entry_number,
			e.entry_date,
			e.description,
			e.reference,
			e.status,
			l.line_number,
			a.code as account_code,
			a.name as account_name,
			l.debit_amount as debit,
			l.credit_amount as credit,
			l.description as line_desc
		FROM ssot_journal_entries e
		JOIN ssot_journal_lines l ON l.journal_id = e.id
		JOIN accounts a ON a.id = l.account_id
		WHERE e.source_type = 'DEPRECIATION'
		  AND e.reference LIKE '%-DEP-2026-03'
		  AND e.deleted_at IS NULL
		ORDER BY e.id, l.line_number
	`

	if err := db.Raw(query).Scan(&journals).Error; err != nil {
		// Try alternative approach
		fmt.Printf("   (Query via ssot_journal_entries: %v)\n", err)
		fmt.Println("   Trying unified_journal_ledger...")
		
		query2 := `
			SELECT 
				e.id as entry_id,
				e.entry_number,
				e.entry_date,
				e.description,
				e.reference,
				e.status,
				l.line_number,
				a.code as account_code,
				a.name as account_name,
				l.debit_amount as debit,
				l.credit_amount as credit,
				l.description as line_desc
			FROM unified_journal_ledger e
			JOIN unified_journal_lines l ON l.journal_id = e.id
			JOIN accounts a ON a.id = l.account_id
			WHERE e.source_type = 'DEPRECIATION'
			  AND e.reference LIKE '%-DEP-2026-03'
			  AND e.deleted_at IS NULL
			ORDER BY e.id, l.line_number
		`
		if err2 := db.Raw(query2).Scan(&journals).Error; err2 != nil {
			fmt.Printf("   ❌ Error querying journals: %v\n", err2)
			os.Exit(1)
		}
	}

	if len(journals) == 0 {
		fmt.Println("   ⚠️ Tidak ada jurnal ditemukan untuk periode 2026-03")
		fmt.Println("   Mungkin sudah pernah dijalankan sebelumnya (skip duplicate)")
	} else {
		currentEntry := uint(0)
		for _, j := range journals {
			if j.EntryID != currentEntry {
				currentEntry = j.EntryID
				fmt.Println()
				fmt.Printf("   📋 Jurnal #%d: %s\n", j.EntryID, j.EntryNumber)
				fmt.Printf("      Tanggal   : %s\n", j.EntryDate[:10])
				fmt.Printf("      Deskripsi : %s\n", j.Description)
				fmt.Printf("      Reference : %s\n", j.Reference)
				fmt.Printf("      Status    : %s\n", j.Status)
				fmt.Println("      ┌──────┬──────────────────────────────────────┬──────────────────┬──────────────────┐")
				fmt.Println("      │ D/K  │ Akun                                 │ Debit            │ Kredit           │")
				fmt.Println("      ├──────┼──────────────────────────────────────┼──────────────────┼──────────────────┤")
			}

			dk := "Dr"
			if j.Credit > 0 {
				dk = "Cr"
			}
			fmt.Printf("      │ %-4s │ %-36s │ %16.2f │ %16.2f │\n",
				dk, j.AccountCode+" - "+j.AccountName, j.Debit, j.Credit)
		}
		fmt.Println("      └──────┴──────────────────────────────────────┴──────────────────┴──────────────────┘")
	}

	// Also show current asset state
	fmt.Println()
	fmt.Println("🏢 STATUS ASET SETELAH PENYUSUTAN:")
	fmt.Println("────────────────────────────────────────────────────────────")

	type AssetStatus struct {
		Code                    string  `gorm:"column:code"`
		Name                    string  `gorm:"column:name"`
		Category                string  `gorm:"column:category"`
		PurchasePrice           float64 `gorm:"column:purchase_price"`
		SalvageValue            float64 `gorm:"column:salvage_value"`
		AccumulatedDepreciation float64 `gorm:"column:accumulated_depreciation"`
		UsefulLife              int     `gorm:"column:useful_life"`
		Method                  string  `gorm:"column:depreciation_method"`
	}

	var assetStatuses []AssetStatus
	db.Raw(`
		SELECT code, name, category, purchase_price, salvage_value, 
		       accumulated_depreciation, useful_life, depreciation_method
		FROM assets 
		WHERE is_active = true AND deleted_at IS NULL
		ORDER BY code
	`).Scan(&assetStatuses)

	for _, a := range assetStatuses {
		bookValue := a.PurchasePrice - a.AccumulatedDepreciation
		fmt.Printf("   📦 %s - %s (%s)\n", a.Code, a.Name, a.Category)
		fmt.Printf("      Harga Perolehan     : Rp %15.2f\n", a.PurchasePrice)
		fmt.Printf("      Nilai Sisa          : Rp %15.2f\n", a.SalvageValue)
		fmt.Printf("      Akum. Penyusutan    : Rp %15.2f\n", a.AccumulatedDepreciation)
		fmt.Printf("      Nilai Buku          : Rp %15.2f\n", bookValue)
		fmt.Printf("      Masa Manfaat        : %d tahun\n", a.UsefulLife)
		fmt.Printf("      Metode              : %s\n", a.Method)
		if a.UsefulLife > 0 {
			monthlyDep := (a.PurchasePrice - a.SalvageValue) / (float64(a.UsefulLife) * 12)
			fmt.Printf("      Depresiasi/Bulan    : Rp %15.2f\n", monthlyDep)
		}
		fmt.Println()
	}

	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("   ✅ TEST SELESAI")
	fmt.Println("════════════════════════════════════════════════════════════")
}
