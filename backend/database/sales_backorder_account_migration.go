package database

import (
	"log"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

const (
	salesBackorderClearingAccountCode = "2103-009"
	salesBackorderClearingAccountName = "ESTIMATED HPP BACKORDER"
)

// EnsureSalesBackorderClearingAccount guarantees the liability account used by
// estimated-HPP backorder journals exists and is active.
func EnsureSalesBackorderClearingAccount(db *gorm.DB) {
	log.Println("🔧 Ensuring sales backorder clearing account exists...")

	var parent models.Account
	if err := db.Where("code = ? AND deleted_at IS NULL", "2103").First(&parent).Error; err != nil {
		log.Printf("⚠️  Skipping sales backorder clearing account creation because liability header 2103 was not found: %v", err)
		return
	}

	var account models.Account
	err := db.Unscoped().Where("code = ?", salesBackorderClearingAccountCode).First(&account).Error
	if err == nil {
		updates := map[string]interface{}{
			"name":               strings.ToUpper(salesBackorderClearingAccountName),
			"description":        "Liabilitas clearing sementara untuk HPP estimasi atas penjualan backorder",
			"type":               models.AccountTypeLiability,
			"category":           models.CategoryCurrentLiability,
			"parent_id":          parent.ID,
			"level":              4,
			"is_header":          false,
			"is_active":          true,
			"is_system_critical": true,
			"deleted_at":         nil,
		}
		if updateErr := db.Unscoped().Model(&account).Updates(updates).Error; updateErr != nil {
			log.Printf("⚠️  Failed to update sales backorder clearing account %s: %v", salesBackorderClearingAccountCode, updateErr)
			return
		}
		log.Printf("✅ Sales backorder clearing account ready: %s - %s", salesBackorderClearingAccountCode, salesBackorderClearingAccountName)
		return
	}
	if err != gorm.ErrRecordNotFound {
		log.Printf("⚠️  Failed to check sales backorder clearing account %s: %v", salesBackorderClearingAccountCode, err)
		return
	}

	account = models.Account{
		Code:             salesBackorderClearingAccountCode,
		Name:             strings.ToUpper(salesBackorderClearingAccountName),
		Description:      "Liabilitas clearing sementara untuk HPP estimasi atas penjualan backorder",
		Type:             models.AccountTypeLiability,
		Category:         models.CategoryCurrentLiability,
		ParentID:         &parent.ID,
		Level:            4,
		IsHeader:         false,
		IsActive:         true,
		IsSystemCritical: true,
	}
	if err := db.Create(&account).Error; err != nil {
		log.Printf("⚠️  Failed to create sales backorder clearing account %s: %v", salesBackorderClearingAccountCode, err)
		return
	}

	log.Printf("✅ Created sales backorder clearing account: %s - %s", salesBackorderClearingAccountCode, salesBackorderClearingAccountName)
}
