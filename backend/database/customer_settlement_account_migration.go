package database

import (
	"fmt"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// EnsureCustomerSettlementAccounts creates the narrow system COA needed for
// customer overpayment and approved rounding.  It only inserts missing codes;
// existing user COA records are never overwritten.
func EnsureCustomerSettlementAccounts(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		liabilityHeader, err := ensureSettlementAccount(tx, models.Account{
			// 2104 is a legacy PPh payable code in deployed databases. Keep
			// customer advances in their own current-liability family instead.
			Code:             "2190",
			Name:             "UANG MUKA PELANGGAN",
			Type:             models.AccountTypeLiability,
			Category:         models.CategoryCurrentLiability,
			Level:            3,
			IsHeader:         true,
			IsActive:         true,
			IsSystemCritical: true,
		})
		if err != nil {
			return err
		}

		if _, err := ensureSettlementAccount(tx, models.Account{
			Code:             "2190-001",
			Name:             "UANG MUKA PELANGGAN",
			Type:             models.AccountTypeLiability,
			Category:         models.CategoryCurrentLiability,
			ParentID:         &liabilityHeader.ID,
			Level:            4,
			IsHeader:         false,
			IsActive:         true,
			IsSystemCritical: true,
		}); err != nil {
			return err
		}

		if _, err := ensureSettlementAccount(tx, models.Account{
			Code:     "7106",
			Name:     "PENDAPATAN PEMBULATAN",
			Type:     models.AccountTypeRevenue,
			Category: models.CategoryOtherIncome,
			Level:    2,
			IsHeader: false,
			IsActive: true,
		}); err != nil {
			return err
		}

		if _, err := ensureSettlementAccount(tx, models.Account{
			Code:     "8106",
			Name:     "BEBAN PEMBULATAN",
			Type:     models.AccountTypeExpense,
			Category: models.CategoryOtherExpense,
			Level:    2,
			IsHeader: false,
			IsActive: true,
		}); err != nil {
			return err
		}

		return nil
	})
}

func ensureSettlementAccount(tx *gorm.DB, desired models.Account) (*models.Account, error) {
	var existing models.Account
	err := tx.Where("code = ? AND deleted_at IS NULL", desired.Code).First(&existing).Error
	if err == nil {
		if !existing.IsActive {
			return nil, fmt.Errorf("customer settlement COA %s is inactive", desired.Code)
		}
		if !strings.EqualFold(existing.Type, desired.Type) || existing.IsHeader != desired.IsHeader {
			return nil, fmt.Errorf("customer settlement COA %s has incompatible configuration", desired.Code)
		}
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("load customer settlement COA %s: %w", desired.Code, err)
	}

	if err := tx.Create(&desired).Error; err != nil {
		return nil, fmt.Errorf("create customer settlement COA %s: %w", desired.Code, err)
	}
	return &desired, nil
}
