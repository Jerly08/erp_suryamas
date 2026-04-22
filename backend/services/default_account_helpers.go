package services

import (
	"app-sistem-akuntansi/models"
	"fmt"

	"gorm.io/gorm"
)

func resolveFirstActiveDetailChildAccount(tx *gorm.DB, parentID uint) (*models.Account, error) {
	var child models.Account
	if err := tx.
		Where("parent_id = ? AND is_header = ? AND is_active = ? AND deleted_at IS NULL", parentID, false, true).
		Order("code ASC").
		First(&child).Error; err != nil {
		return nil, err
	}

	return &child, nil
}

func normalizeToDetailRevenueAccountID(tx *gorm.DB, accountID uint) (uint, error) {
	if accountID == 0 {
		return 0, fmt.Errorf("revenue account is required")
	}

	var account models.Account
	if err := tx.
		Where("id = ? AND deleted_at IS NULL", accountID).
		First(&account).Error; err != nil {
		return 0, err
	}

	if !account.IsHeader {
		return account.ID, nil
	}

	child, err := resolveFirstActiveDetailChildAccount(tx, account.ID)
	if err != nil {
		return 0, fmt.Errorf("akun COA pendapatan '%s' adalah header dan tidak memiliki akun detail aktif", account.Code)
	}

	return child.ID, nil
}

func findDefaultRevenueAccountID(tx *gorm.DB) (uint, error) {
	preferredCodes := []string{"4101-001", "4101"}
	for _, code := range preferredCodes {
		var account models.Account
		if err := tx.
			Where("code = ? AND is_active = ? AND deleted_at IS NULL", code, true).
			First(&account).Error; err == nil {
			if account.IsHeader {
				child, childErr := resolveFirstActiveDetailChildAccount(tx, account.ID)
				if childErr == nil {
					return child.ID, nil
				}
			} else {
				return account.ID, nil
			}
		}
	}

	resolver := NewAccountResolver(tx)
	if resolved, err := resolver.GetAccount(AccountTypeSalesRevenue); err == nil && resolved != nil {
		if resolved.IsHeader {
			child, childErr := resolveFirstActiveDetailChildAccount(tx, resolved.ID)
			if childErr == nil {
				return child.ID, nil
			}
		} else if resolved.IsActive {
			return resolved.ID, nil
		}
	}

	var fallback models.Account
	if err := tx.
		Where("type = ? AND is_active = ? AND deleted_at IS NULL", models.AccountTypeRevenue, true).
		Order("is_header ASC, code ASC").
		First(&fallback).Error; err != nil {
		return 0, fmt.Errorf("no default revenue account found")
	}

	if fallback.IsHeader {
		child, childErr := resolveFirstActiveDetailChildAccount(tx, fallback.ID)
		if childErr != nil {
			return 0, fmt.Errorf("default revenue account %s has no active detail child", fallback.Code)
		}
		return child.ID, nil
	}

	return fallback.ID, nil
}
