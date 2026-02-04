package database

import (
	"log"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// MigratePPhPrepaidAccounts ensures PPh prepaid tax accounts exist in the database
// This is a critical migration because journal entries fail if these accounts don't exist
// when sales have PPh deductions (PPh21, PPh23, PPh25)
func MigratePPhPrepaidAccounts(db *gorm.DB) error {
	log.Println("🔧 [AUTO-MIGRATION] Checking PPh prepaid tax accounts...")

	// Define required PPh prepaid accounts
	// These accounts are essential for sales journal entries when customer withholds taxes
	pphAccounts := []struct {
		Code        string
		Name        string
		Description string
	}{
		{
			Code:        "1114",
			Name:        "PPH PASAL 21 DIBAYAR DIMUKA",
			Description: "Akun untuk mencatat PPh Pasal 21 yang dipotong oleh pelanggan (prepaid withholding tax)",
		},
		{
			Code:        "1115",
			Name:        "PPH PASAL 23 DIBAYAR DIMUKA",
			Description: "Akun untuk mencatat PPh Pasal 23 yang dipotong oleh pelanggan (prepaid withholding tax)",
		},
		{
			Code:        "1116",
			Name:        "PPH PASAL 25 DIBAYAR DIMUKA",
			Description: "Akun untuk mencatat PPh Pasal 25 yang dibayar sendiri (prepaid income tax)",
		},
	}

	for _, pph := range pphAccounts {
		var existingAccount models.Account
		
		// Check if account already exists
		result := db.Where("code = ? AND deleted_at IS NULL", pph.Code).First(&existingAccount)
		
		if result.Error == gorm.ErrRecordNotFound {
			// Account doesn't exist, create it
			newAccount := models.Account{
				Code:        pph.Code,
				Name:        pph.Name,
				Type:        models.AccountTypeAsset,
				Category:    models.CategoryCurrentAsset,
				Level:       3,
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
			
			// Set parent relationship (to 1200 - ACCOUNTS RECEIVABLE or 1100 - CURRENT ASSETS)
			var parentAccount models.Account
			// Try to find 1200 first (ACCOUNTS RECEIVABLE)
			if err := db.Where("code = '1200' AND deleted_at IS NULL").First(&parentAccount).Error; err == nil {
				db.Model(&newAccount).Update("parent_id", parentAccount.ID)
				log.Printf("   → Set parent to %s (%s)", parentAccount.Code, parentAccount.Name)
			} else {
				// Fallback to 1100 (CURRENT ASSETS)
				if err := db.Where("code = '1100' AND deleted_at IS NULL").First(&parentAccount).Error; err == nil {
					db.Model(&newAccount).Update("parent_id", parentAccount.ID)
					log.Printf("   → Set parent to %s (%s)", parentAccount.Code, parentAccount.Name)
				}
			}
		} else if result.Error != nil {
			// Unexpected error
			log.Printf("⚠️ Error checking account %s: %v", pph.Code, result.Error)
		} else {
			// Account exists
			log.Printf("✓ PPh account %s already exists", pph.Code)
		}
	}

	log.Println("✅ [AUTO-MIGRATION] PPh prepaid tax accounts check completed")
	return nil
}
