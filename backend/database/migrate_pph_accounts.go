package database

import (
	"log"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// MigratePPhPrepaidAccounts ensures PPh-related tax asset accounts exist in the database
// This is a critical migration because journal entries fail if these accounts don't exist
// when sales have PPh deductions (PPh21, PPh23, PPh25)
//
// UPDATED: Uses standard COA codes 1105-001/002/003/004 instead of legacy 1114/1115/1116.
// Also cleans up legacy duplicate accounts if they exist.
func MigratePPhPrepaidAccounts(db *gorm.DB) error {
	log.Println("🔧 [AUTO-MIGRATION] Checking PPh prepaid tax accounts...")

	// Step 1: Clean up legacy duplicate accounts (1114, 1115, 1116)
	// These were created by an older version of this migration and overlap with
	// the standard COA accounts 1105-001, 1105-002, 1105-003.
	legacyCodes := []string{"1114", "1115", "1116"}
	for _, code := range legacyCodes {
		var legacyAccount models.Account
		result := db.Where("code = ? AND deleted_at IS NULL", code).Find(&legacyAccount)
		if result.Error == nil && result.RowsAffected > 0 {
			// Only soft-delete if the account has zero balance (no active transactions)
			if legacyAccount.Balance == 0 {
				now := time.Now()
				if err := db.Model(&legacyAccount).Update("deleted_at", now).Error; err != nil {
					log.Printf("⚠️ Failed to soft-delete legacy account %s: %v", code, err)
				} else {
					log.Printf("🗑️ Soft-deleted legacy duplicate account: %s - %s", code, legacyAccount.Name)
				}
			} else {
				log.Printf("⚠️ Legacy account %s has non-zero balance (%.2f), skipping deletion. Please manually reconcile.", code, legacyAccount.Balance)
			}
		}
	}

	// Step 2: Ensure standard PPh-related asset accounts exist under 1105 (PAJAK DIBAYAR DIMUKA)
	// These codes match the standard COA template in account_templates.go
	pphAccounts := []struct {
		Code        string
		Name        string
		Description string
		ParentCode  string
	}{
		{
			Code:        "1105-001",
			Name:        "PPH PASAL 21 DIBAYAR DIMUKA",
			Description: "Akun untuk mencatat PPh Pasal 21 yang dipotong oleh pelanggan (prepaid withholding tax)",
			ParentCode:  "1105",
		},
		{
			Code:        "1105-002",
			Name:        "PPH PASAL 23 DIBAYAR DIMUKA",
			Description: "Akun untuk mencatat PPh Pasal 23 yang dipotong oleh pelanggan (prepaid withholding tax)",
			ParentCode:  "1105",
		},
		{
			Code:        "1105-003",
			Name:        "PPH PASAL 25 DIBAYAR DIMUKA",
			Description: "Akun untuk mencatat PPh Pasal 25 yang dibayar sendiri (prepaid income tax)",
			ParentCode:  "1105",
		},
		{
			Code:        "1105-004",
			Name:        "KREDIT PAJAK PPH 23",
			Description: "Akun reclass untuk mencatat PPh Pasal 23 yang sudah dikonfirmasi dan dipost terpisah dari tanggal invoice",
			ParentCode:  "1105",
		},
	}

	for _, pph := range pphAccounts {
		var existingAccount models.Account

		// Check if account already exists using Find to suppress 'record not found' logs
		result := db.Where("code = ? AND deleted_at IS NULL", pph.Code).Find(&existingAccount)

		if result.Error == nil && result.RowsAffected == 0 {
			// Account doesn't exist, create it
			newAccount := models.Account{
				Code:        pph.Code,
				Name:        pph.Name,
				Type:        models.AccountTypeAsset,
				Category:    models.CategoryCurrentAsset,
				Level:       4,
				IsHeader:    false,
				IsActive:    true,
				Balance:     0,
				Description: pph.Description,
			}

			if err := db.Create(&newAccount).Error; err != nil {
				log.Printf("❌ Failed to create PPh account %s: %v", pph.Code, err)
				return err
			}

			log.Printf("✅ Created PPh prepaid account: %s - %s", pph.Code, pph.Name)

			// Set parent relationship to 1105 (PAJAK DIBAYAR DIMUKA)
			var parentAccount models.Account
			if parentResult := db.Where("code = ? AND deleted_at IS NULL", pph.ParentCode).Find(&parentAccount); parentResult.Error == nil && parentResult.RowsAffected > 0 {
				db.Model(&newAccount).Update("parent_id", parentAccount.ID)
			} else {
				// Fallback to 1100 (ASET LANCAR)
				if fallbackResult := db.Where("code = '1100' AND deleted_at IS NULL").Find(&parentAccount); fallbackResult.Error == nil && fallbackResult.RowsAffected > 0 {
					db.Model(&newAccount).Update("parent_id", parentAccount.ID)
				}
			}
		} else if result.Error != nil {
			// Unexpected error
			log.Printf("⚠️ Error checking account %s: %v", pph.Code, result.Error)
		}
	}

	log.Println("✅ [AUTO-MIGRATION] PPh prepaid tax accounts check completed")
	return nil
}
