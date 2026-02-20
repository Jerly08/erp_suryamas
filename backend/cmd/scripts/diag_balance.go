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

// 1. Get account info
type AccInfo struct { ID uint; Code string; Name string; Balance float64 }
var acc AccInfo
db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '1101-005' AND deleted_at IS NULL").Scan(&acc)
fmt.Printf("=== COA Account ===\nID=%d, Code=%s, Name=%s, Balance=%.2f\n\n", acc.ID, acc.Code, acc.Name, acc.Balance)

// 2. Get cash_bank info
type CBInfo struct { ID uint; Name string; Balance float64; AccountID uint }
var cb CBInfo
db.Raw("SELECT id, name, balance, account_id FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", acc.ID).Scan(&cb)
fmt.Printf("=== Cash Bank ===\nID=%d, Name=%s, Balance=%.2f, AccountID=%d\n\n", cb.ID, cb.Name, cb.Balance, cb.AccountID)

fmt.Printf("=== SELISIH ===\nCash Bank: %.2f\nCOA:       %.2f\nSelisih:   %.2f\n\n", cb.Balance, acc.Balance, cb.Balance - acc.Balance)

// 3. List all journal entries for this account
type JournalEntry struct {
ID          uint
JournalID   uint
DebitAmount  float64
CreditAmount float64
Description  string
CreatedAt    string
}
var entries []JournalEntry
db.Raw(`SELECT ujl.id, ujl.journal_id, ujl.debit_amount, ujl.credit_amount, ujl.description, ujl.created_at::text 
FROM unified_journal_lines ujl 
WHERE ujl.account_id = ? 
ORDER BY ujl.created_at, ujl.id`, acc.ID).Scan(&entries)

fmt.Println("=== Unified Journal Lines for 1101-005 ===")
var totalDebit, totalCredit float64
for _, e := range entries {
totalDebit += e.DebitAmount
totalCredit += e.CreditAmount
fmt.Printf("  JL#%d J#%d | D:%.2f C:%.2f | %s | %s\n", e.ID, e.JournalID, e.DebitAmount, e.CreditAmount, e.CreatedAt[:19], e.Description)
}
calcBalance := totalDebit - totalCredit
fmt.Printf("\nTotal Debit:  %.2f\nTotal Credit: %.2f\nCalculated Balance (D-C): %.2f\n\n", totalDebit, totalCredit, calcBalance)

// 4. List cash_bank transactions
type CBTx struct {
ID          uint
Type        string
Amount      float64
BalanceAfter float64
Description string
ReferenceNo string
CreatedAt   string
}
var txs []CBTx
db.Raw(`SELECT id, type, amount, balance_after, description, reference_number, created_at::text 
FROM cash_bank_transactions 
WHERE cash_bank_id = ? AND deleted_at IS NULL
ORDER BY created_at, id`, cb.ID).Scan(&txs)

fmt.Println("=== Cash Bank Transactions ===")
for _, t := range txs {
fmt.Printf("  TX#%d | %s | Amount:%.2f | After:%.2f | Ref:%s | %s\n", t.ID, t.Type, t.Amount, t.BalanceAfter, t.ReferenceNo, t.Description)
}

// 5. Check journal ledgers referencing this account
type LedgerInfo struct {
ID          uint
JournalType string
TotalDebit  float64
TotalCredit float64
Description string
CreatedAt   string
}
var ledgers []LedgerInfo
db.Raw(`SELECT DISTINCT ujled.id, ujled.journal_type, ujled.total_debit, ujled.total_credit, ujled.description, ujled.created_at::text
FROM unified_journal_ledgers ujled
JOIN unified_journal_lines ujl ON ujl.journal_id = ujled.id
WHERE ujl.account_id = ?
ORDER BY ujled.created_at`, acc.ID).Scan(&ledgers)

fmt.Println("\n=== Related Journal Ledgers ===")
for _, l := range ledgers {
fmt.Printf("  Ledger#%d | Type:%s | D:%.2f C:%.2f | %s | %s\n", l.ID, l.JournalType, l.TotalDebit, l.TotalCredit, l.CreatedAt[:19], l.Description)
}
}
