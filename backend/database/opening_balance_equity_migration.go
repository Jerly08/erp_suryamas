package database

import (
	"log"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

const (
	openingBalanceEquityCode = "3104"
	openingBalanceEquityName = "EKUITAS SALDO AWAL"
)

// EnsureOpeningBalanceEquityAccount makes sure the default opening-balance equity
// account exists and is active, so migration entries do not fall back to share capital.
func EnsureOpeningBalanceEquityAccount(db *gorm.DB) {
	log.Println("🔧 Ensuring opening balance equity account exists...")

	var equityHeader models.Account
	if err := db.Where("code = ? AND deleted_at IS NULL", "3000").First(&equityHeader).Error; err != nil {
		log.Printf("⚠️  Skipping opening balance equity account creation because equity header 3000 was not found: %v", err)
		return
	}

	var account models.Account
	err := db.Unscoped().Where("code = ?", openingBalanceEquityCode).First(&account).Error
	if err == nil {
		updates := map[string]interface{}{
			"name":               openingBalanceEquityName,
			"description":        "Akun default untuk jurnal saldo awal / migrasi neraca pembukaan",
			"type":               models.AccountTypeEquity,
			"category":           models.CategoryEquity,
			"parent_id":          equityHeader.ID,
			"level":              2,
			"is_header":          false,
			"is_active":          true,
			"is_system_critical": true,
			"deleted_at":         nil,
		}
		if updateErr := db.Unscoped().Model(&account).Updates(updates).Error; updateErr != nil {
			log.Printf("⚠️  Failed to update opening balance equity account %s: %v", openingBalanceEquityCode, updateErr)
			return
		}
		log.Printf("✅ Opening balance equity account ready: %s - %s", openingBalanceEquityCode, openingBalanceEquityName)
		return
	}
	if err != gorm.ErrRecordNotFound {
		log.Printf("⚠️  Failed to check opening balance equity account %s: %v", openingBalanceEquityCode, err)
		return
	}

	account = models.Account{
		Code:             openingBalanceEquityCode,
		Name:             strings.ToUpper(openingBalanceEquityName),
		Description:      "Akun default untuk jurnal saldo awal / migrasi neraca pembukaan",
		Type:             models.AccountTypeEquity,
		Category:         models.CategoryEquity,
		ParentID:         &equityHeader.ID,
		Level:            2,
		IsHeader:         false,
		IsActive:         true,
		IsSystemCritical: true,
	}
	if err := db.Create(&account).Error; err != nil {
		log.Printf("⚠️  Failed to create opening balance equity account %s: %v", openingBalanceEquityCode, err)
		return
	}

	log.Printf("✅ Created opening balance equity account: %s - %s", openingBalanceEquityCode, openingBalanceEquityName)
}
