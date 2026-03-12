package models

import (
	"time"

	"gorm.io/gorm"
)

// These models are used specifically for auto-migration of additional structures
// that are created in the enhanced cash bank migration.

type CashBankTransferMigration struct {
	ID                       uint   `gorm:"primaryKey"`
	TransferNumber           string `gorm:"unique;not null;size:50"`
	FromAccountID            uint   `gorm:"not null;index"`
	ToAccountID              uint   `gorm:"not null;index"`
	Date                     time.Time
	Amount                   float64 `gorm:"type:decimal(15,2)"`
	ExchangeRate             float64 `gorm:"type:decimal(12,6);default:1"`
	ConvertedAmount          float64 `gorm:"type:decimal(15,2)"`
	SourceBookAmountIDR      float64 `gorm:"type:decimal(20,2);default:0"`
	DestinationBookAmountIDR float64 `gorm:"type:decimal(20,2);default:0"`
	ForexGainLossAmount      float64 `gorm:"type:decimal(20,2);default:0"`
	ForexGainLossType        string  `gorm:"size:10"`
	Reference                string  `gorm:"size:100"`
	Notes                    string  `gorm:"type:text"`
	Status                   string  `gorm:"size:20;default:PENDING"`
	UserID                   uint    `gorm:"not null;index"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
	DeletedAt                gorm.DeletedAt `gorm:"index"`
}

func (CashBankTransferMigration) TableName() string {
	return "cash_bank_transfers"
}

type BankReconciliationMigration struct {
	ID               uint `gorm:"primaryKey"`
	CashBankID       uint `gorm:"not null;index"`
	ReconcileDate    time.Time
	StatementBalance float64 `gorm:"type:decimal(15,2)"`
	SystemBalance    float64 `gorm:"type:decimal(15,2)"`
	Difference       float64 `gorm:"type:decimal(15,2)"`
	Status           string  `gorm:"size:20;default:PENDING"`
	UserID           uint    `gorm:"not null;index"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (BankReconciliationMigration) TableName() string {
	return "bank_reconciliations"
}

type ReconciliationItemMigration struct {
	ID               uint   `gorm:"primaryKey"`
	ReconciliationID uint   `gorm:"not null;index"`
	TransactionID    uint   `gorm:"not null;index"`
	IsCleared        bool   `gorm:"default:false"`
	Notes            string `gorm:"type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (ReconciliationItemMigration) TableName() string {
	return "reconciliation_items"
}

// MigrationRecord tracks applied migrations to prevent re-running.
type MigrationRecord struct {
	ID          uint      `gorm:"primaryKey"`
	MigrationID string    `gorm:"unique;not null;size:100"`
	Description string    `gorm:"type:text"`
	Version     string    `gorm:"size:20"`
	AppliedAt   time.Time `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func MigrateCashBankTables(db *gorm.DB) error {
	err := db.AutoMigrate(
		&CashBankTransferMigration{},
		&BankReconciliationMigration{},
		&ReconciliationItemMigration{},
	)
	if err != nil {
		return err
	}
	return nil
}
