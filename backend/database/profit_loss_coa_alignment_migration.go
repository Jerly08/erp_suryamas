package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

const profitLossCOAAlignmentMigrationID = "profit_loss_coa_alignment_v1.1"

var profitLossAlignmentCodes = map[string]bool{
	"4000": true,
	"4101": true,
	"7000": true,
	"7101": true,
	"7102": true,
	"7103": true,
	"7104": true,
	"7105": true,
	"8000": true,
	"8101": true,
	"8102": true,
	"8103": true,
	"8104": true,
	"8105": true,
	"9000": true,
	"9101": true,
}

var profitLossAlignmentOrder = []string{
	"4000",
	"4101",
	"7000",
	"7101",
	"7102",
	"7103",
	"7104",
	"7105",
	"8000",
	"8101",
	"8102",
	"8103",
	"8104",
	"8105",
	"9000",
	"9101",
}

// ProfitLossCOAAlignmentMigration backfills critical P&L COA metadata and adds
// dedicated non-operating accounts introduced after the original seed.
func ProfitLossCOAAlignmentMigration(db *gorm.DB) {
	log.Println("🔧 Starting profit-loss COA alignment migration...")

	var existingMigration models.MigrationRecord
	if err := db.Where("migration_id = ?", profitLossCOAAlignmentMigrationID).First(&existingMigration).Error; err == nil {
		log.Printf("✅ Profit-loss COA alignment migration already applied at %v", existingMigration.AppliedAt)
		return
	}

	var settings models.Settings
	if err := db.First(&settings).Error; err != nil {
		log.Printf("⚠️  Profit-loss COA alignment skipped: settings not available: %v", err)
		return
	}
	if settings.CompanyType == "" {
		log.Println("⚠️  Profit-loss COA alignment skipped: company type not set yet")
		return
	}

	templates := getProfitLossAlignmentTemplates(settings.CompanyType)
	if len(templates) == 0 {
		log.Printf("⚠️  Profit-loss COA alignment skipped: no target templates for company type %s", settings.CompanyType)
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		log.Printf("❌ Failed to start profit-loss COA alignment migration transaction: %v", tx.Error)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("❌ Profit-loss COA alignment migration rolled back due to panic: %v", r)
		}
	}()

	var fixesApplied []string

	if updatedCount, err := backfillProfitLossCategories(tx, templates); err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to backfill P&L account categories: %v", err)
		return
	} else if updatedCount > 0 {
		fixesApplied = append(fixesApplied, fmt.Sprintf("%d account categories aligned", updatedCount))
	}

	if createdCount, err := ensureProfitLossAccounts(tx, templates); err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to ensure dedicated P&L accounts: %v", err)
		return
	} else if createdCount > 0 {
		fixesApplied = append(fixesApplied, fmt.Sprintf("%d dedicated accounts created", createdCount))
	}

	if err := recordProfitLossCOAAlignmentMigration(tx, fixesApplied); err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to record profit-loss COA alignment migration: %v", err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Failed to commit profit-loss COA alignment migration: %v", err)
		return
	}

	if len(fixesApplied) == 0 {
		fixesApplied = append(fixesApplied, "no data changes were required")
	}
	log.Printf("✅ Profit-loss COA alignment migration completed successfully: %s", strings.Join(fixesApplied, ", "))
}

func getProfitLossAlignmentTemplates(companyType string) map[string]models.Account {
	templates := make(map[string]models.Account)
	for _, account := range GetStandardCOA(companyType) {
		if profitLossAlignmentCodes[account.Code] {
			templates[account.Code] = account
		}
	}
	return templates
}

func backfillProfitLossCategories(tx *gorm.DB, templates map[string]models.Account) (int64, error) {
	template4101, exists := templates["4101"]
	if !exists {
		return 0, nil
	}

	result := tx.Model(&models.Account{}).
		Where("code = ? AND deleted_at IS NULL", "4101").
		Where("COALESCE(category, '') <> ?", template4101.Category).
		Update("category", template4101.Category)
	if result.Error != nil {
		return 0, result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("✅ Backfilled category for account 4101 to %s", template4101.Category)
	}

	return result.RowsAffected, nil
}

func ensureProfitLossAccounts(tx *gorm.DB, templates map[string]models.Account) (int, error) {
	createdCount := 0

	for _, code := range profitLossAlignmentOrder {
		template, exists := templates[code]
		if !exists {
			continue
		}

		var existingAccount models.Account
		result := tx.Where("code = ? AND deleted_at IS NULL", code).Find(&existingAccount)
		if result.Error != nil {
			return createdCount, result.Error
		}

		if result.RowsAffected == 0 {
			newAccount := models.Account{
				Code:        template.Code,
				Name:        strings.ToUpper(template.Name),
				Type:        template.Type,
				Category:    template.Category,
				Level:       template.Level,
				IsHeader:    template.IsHeader,
				IsActive:    template.IsActive,
				Balance:     0,
				Description: template.Description,
			}

			if err := tx.Create(&newAccount).Error; err != nil {
				return createdCount, err
			}

			log.Printf("✅ Created dedicated P&L account: %s - %s", newAccount.Code, newAccount.Name)
			existingAccount = newAccount
			createdCount++
		}

		if err := setTemplateParent(tx, &existingAccount); err != nil {
			return createdCount, err
		}
	}

	return createdCount, nil
}

func setTemplateParent(tx *gorm.DB, account *models.Account) error {
	parentCode, hasParent, err := resolveTemplateParentCode(tx, account.Code)
	if err != nil {
		return err
	}
	if !hasParent {
		return nil
	}

	var parent models.Account
	parentResult := tx.Where("code = ? AND deleted_at IS NULL", parentCode).Find(&parent)
	if parentResult.Error != nil {
		return parentResult.Error
	}
	if parentResult.RowsAffected == 0 {
		return nil
	}

	if account.ParentID != nil && *account.ParentID == parent.ID {
		return nil
	}

	return tx.Model(account).Update("parent_id", parent.ID).Error
}

func resolveTemplateParentCode(tx *gorm.DB, code string) (string, bool, error) {
	parentCode := ""
	if strings.Contains(code, "-") {
		parts := strings.Split(code, "-")
		candidate := parts[0]
		if exists, err := accountCodeExists(tx, candidate); err != nil {
			return "", false, err
		} else if exists {
			return candidate, true, nil
		}
		code = candidate
	}

	if len(code) != 4 {
		return "", false, nil
	}
	if code[1:] == "000" {
		return "", false, nil
	}

	if code[2:] == "00" {
		parentCode = code[:1] + "000"
		return parentCode, true, nil
	}

	candidate := code[:2] + "00"
	if exists, err := accountCodeExists(tx, candidate); err != nil {
		return "", false, err
	} else if exists {
		return candidate, true, nil
	}

	fallback := code[:1] + "000"
	if exists, err := accountCodeExists(tx, fallback); err != nil {
		return "", false, err
	} else if exists {
		return fallback, true, nil
	}

	return "", false, nil
}

func accountCodeExists(tx *gorm.DB, code string) (bool, error) {
	var count int64
	if err := tx.Model(&models.Account{}).
		Where("code = ? AND deleted_at IS NULL", code).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func recordProfitLossCOAAlignmentMigration(tx *gorm.DB, fixesApplied []string) error {
	description := "Profit-loss COA alignment applied"
	if len(fixesApplied) > 0 {
		description = fmt.Sprintf("Profit-loss COA alignment applied: %s", strings.Join(fixesApplied, ", "))
	}

	migrationRecord := models.MigrationRecord{
		MigrationID: profitLossCOAAlignmentMigrationID,
		Description: description,
		Version:     "1.1",
		AppliedAt:   time.Now(),
	}

	return tx.Create(&migrationRecord).Error
}
