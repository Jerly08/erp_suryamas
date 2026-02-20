package main
import (
"fmt"
"log"
"gorm.io/driver/postgres"
"gorm.io/gorm"
)
func main() {
db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"), &gorm.Config{})
if err != nil { log.Fatal(err) }

type CashBankInfo struct {
ID uint
Name string
AccountID *uint
BankName *string
}
var cashBanks []CashBankInfo
db.Raw("SELECT id, name, account_id, bank_name FROM cash_banks WHERE deleted_at IS NULL").Scan(&cashBanks)
fmt.Println("=== Cash Banks ===")
for _, cb := range cashBanks {
accID := "nil"
if cb.AccountID != nil { accID = fmt.Sprintf("%d", *cb.AccountID) }
bankName := ""
if cb.BankName != nil { bankName = *cb.BankName }
fmt.Printf("  ID=%d Name=%s AccountID=%s BankName=%s\n", cb.ID, cb.Name, accID, bankName)
}

// Check 1101-005 account details
type AccInfo struct {
ID uint
Code string
Name string
IsActive bool
ParentID *uint
}
var acc AccInfo
db.Raw("SELECT id, code, name, is_active, parent_id FROM accounts WHERE code = '1101-005' AND deleted_at IS NULL").Scan(&acc)
parentStr := "nil"
if acc.ParentID != nil { parentStr = fmt.Sprintf("%d", *acc.ParentID) }
fmt.Printf("\n1101-005: ID=%d, ParentID=%s, IsActive=%v\n", acc.ID, parentStr, acc.IsActive)

// Check if parent is in cash_banks
if acc.ParentID != nil {
var parentCB int64
db.Raw("SELECT COUNT(*) FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", *acc.ParentID).Scan(&parentCB)
fmt.Printf("Parent (ID=%d) in cash_banks: %d\n", *acc.ParentID, parentCB)
}

// Check if 1101-005 itself is in cash_banks
var selfCB int64
db.Raw("SELECT COUNT(*) FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", acc.ID).Scan(&selfCB)
fmt.Printf("1101-005 (ID=%d) in cash_banks: %d\n", acc.ID, selfCB)
}
