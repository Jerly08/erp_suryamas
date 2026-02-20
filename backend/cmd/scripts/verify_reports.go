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

// Check if 5101 is a header account
type AccInfo struct {
ID uint
Code string
Name string
IsHeader bool
ParentID *uint
Balance float64
}
var accounts []AccInfo
db.Raw(`SELECT id, code, name, is_header, parent_id, balance FROM accounts WHERE code LIKE '5101%' AND deleted_at IS NULL ORDER BY code`).Scan(&accounts)
fmt.Println("=== 5101 Account Hierarchy ===")
for _, a := range accounts {
parentStr := "none"
if a.ParentID != nil { parentStr = fmt.Sprintf("%d", *a.ParentID) }
fmt.Printf("  %s %s | is_header=%v, parent=%s, balance=%.2f\n", a.Code, a.Name, a.IsHeader, parentStr, a.Balance)
}

// Check journal entries posting to 5101 header
var count5101 int64
db.Raw(`SELECT COUNT(*) FROM unified_journal_lines WHERE account_id = (SELECT id FROM accounts WHERE code = '5101' AND deleted_at IS NULL) AND debit_amount > 0`).Scan(&count5101)
fmt.Printf("\nJournal lines posting to 5101 (header): %d\n", count5101)

// Check Balance Sheet equation
fmt.Println("\n=== Balance Sheet Verification ===")
type BalInfo struct { Total float64 }

var assets, liabilities, equity, revenue, expenses BalInfo
db.Raw(`SELECT COALESCE(SUM(CASE WHEN debit_amount > 0 THEN debit_amount ELSE -credit_amount END), 0) as total FROM unified_journal_lines ujl JOIN accounts a ON ujl.account_id = a.id WHERE a.type = 'ASSET' AND a.deleted_at IS NULL`).Scan(&assets)
db.Raw(`SELECT COALESCE(SUM(CASE WHEN credit_amount > 0 THEN credit_amount ELSE -debit_amount END), 0) as total FROM unified_journal_lines ujl JOIN accounts a ON ujl.account_id = a.id WHERE a.type = 'LIABILITY' AND a.deleted_at IS NULL`).Scan(&liabilities)
db.Raw(`SELECT COALESCE(SUM(CASE WHEN credit_amount > 0 THEN credit_amount ELSE -debit_amount END), 0) as total FROM unified_journal_lines ujl JOIN accounts a ON ujl.account_id = a.id WHERE a.type = 'EQUITY' AND a.deleted_at IS NULL`).Scan(&equity)
db.Raw(`SELECT COALESCE(SUM(CASE WHEN credit_amount > 0 THEN credit_amount ELSE -debit_amount END), 0) as total FROM unified_journal_lines ujl JOIN accounts a ON ujl.account_id = a.id WHERE (a.type = 'REVENUE' OR a.type = 'INCOME') AND a.deleted_at IS NULL`).Scan(&revenue)
db.Raw(`SELECT COALESCE(SUM(CASE WHEN debit_amount > 0 THEN debit_amount ELSE -credit_amount END), 0) as total FROM unified_journal_lines ujl JOIN accounts a ON ujl.account_id = a.id WHERE (a.type = 'EXPENSE' OR a.type = 'COGS') AND a.deleted_at IS NULL`).Scan(&expenses)

netIncome := revenue.Total - expenses.Total
fmt.Printf("Assets: %.2f\n", assets.Total)
fmt.Printf("Liabilities: %.2f\n", liabilities.Total)
fmt.Printf("Equity: %.2f\n", equity.Total)
fmt.Printf("Revenue: %.2f\n", revenue.Total)
fmt.Printf("Expenses: %.2f\n", expenses.Total)
fmt.Printf("Net Income: %.2f\n", netIncome)
fmt.Printf("L + E + NI: %.2f\n", liabilities.Total + equity.Total + netIncome)
fmt.Printf("Balance: A = L + E + NI  %.2f = %.2f\n", assets.Total, liabilities.Total + equity.Total + netIncome)

if assets.Total == liabilities.Total + equity.Total + netIncome {
fmt.Println(" BALANCED!")
} else {
fmt.Printf(" SELISIH: %.2f\n", assets.Total - (liabilities.Total + equity.Total + netIncome))
}
}
