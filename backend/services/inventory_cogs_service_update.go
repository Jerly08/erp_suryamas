package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// updateAccountBalance updates account.balance field for COA tree view display
func (s *InventoryCOGSService) updateAccountBalance(db *gorm.DB, accountID uint64, debitAmount, creditAmount decimal.Decimal) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("account %d not found: %v", accountID, err)
	}

	// Calculate net change: debit - credit
	debit := debitAmount.InexactFloat64()
	credit := creditAmount.InexactFloat64()
	netChange := debit - credit

	// Update balance based on account type
	switch strings.ToUpper(account.Type) {
	case "ASSET", "EXPENSE":
		// Assets and Expenses: debit increases balance
		account.Balance += netChange
	case "LIABILITY", "EQUITY", "REVENUE":
		// Liabilities, Equity, Revenue: credit increases balance (so debit decreases)
		account.Balance -= netChange
	}

	if err := db.Model(&account).Select("Balance").Updates(models.Account{Balance: account.Balance}).Error; err != nil {
		return fmt.Errorf("failed to save account balance: %v", err)
	}

	log.Printf("💰 [COGS] Updated account %s (%s) balance: Dr=%.2f, Cr=%.2f, Change=%.2f, New Balance=%.2f",
		account.Code, account.Name, debit, credit, netChange, account.Balance)

	return nil
}
