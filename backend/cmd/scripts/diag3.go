package main
import (
"fmt"
"log"
"gorm.io/driver/postgres"
"gorm.io/gorm"
"gorm.io/gorm/logger"
)
func main() {
db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"), &gorm.Config{
Logger: logger.Default.LogMode(logger.Silent),
})
if err != nil { log.Fatal(err) }

f, _ := fmt.Println("== Payments table (all) ==")
_ = f

type PayInfo struct {
ID uint; Code string; Amount float64; Status string; Method string; JournalEntryID *uint; ContactID uint; Notes string
}
var pays []PayInfo
db.Raw("SELECT id, code, amount, status, method, journal_entry_id, contact_id, notes FROM payments WHERE deleted_at IS NULL ORDER BY id").Scan(&pays)
for _, p := range pays {
jid := "nil"
if p.JournalEntryID != nil { jid = fmt.Sprintf("%d", *p.JournalEntryID) }
fmt.Printf("  PAY#%d Code=%s Amount=%.2f Status=%s Method=%s JournalID=%s Contact=%d | %s\n", 
p.ID, p.Code, p.Amount, p.Status, p.Method, jid, p.ContactID, p.Notes)
}

fmt.Println("\n== Purchase Payments (purchase_payments table) ==")
type PPInfo struct {
ID uint; PurchaseID uint; PaymentNumber string; Amount float64; Method string; CashBankID *uint; PaymentID *uint
}
var pps []PPInfo
db.Raw("SELECT id, purchase_id, payment_number, amount, method, cash_bank_id, payment_id FROM purchase_payments WHERE deleted_at IS NULL ORDER BY id").Scan(&pps)
for _, pp := range pps {
cbid := "nil"
if pp.CashBankID != nil { cbid = fmt.Sprintf("%d", *pp.CashBankID) }
pid := "nil"
if pp.PaymentID != nil { pid = fmt.Sprintf("%d", *pp.PaymentID) }
fmt.Printf("  PP#%d Purchase=%d PayNum=%s Amount=%.2f Method=%s CashBankID=%s PaymentID=%s\n", 
pp.ID, pp.PurchaseID, pp.PaymentNumber, pp.Amount, pp.Method, cbid, pid)
}

fmt.Println("\n== Unified Journal Ledgers ==")
type JLInfo struct {
ID uint; JournalType string; SourceType string; Reference string; Description string; TotalDebit float64; TotalCredit float64
}
var jls []JLInfo
db.Raw("SELECT id, journal_type, source_type, reference, description, total_debit, total_credit FROM unified_journal_ledgers ORDER BY id").Scan(&jls)
for _, j := range jls {
fmt.Printf("  J#%d Type=%s Source=%s Ref=%s D:%.2f C:%.2f | %s\n", 
j.ID, j.JournalType, j.SourceType, j.Reference, j.TotalDebit, j.TotalCredit, j.Description)
}

fmt.Println("\n== All Unified Journal Lines (showing account IDs) ==")
type UJLInfo struct {
ID uint; JournalID uint; AccountID uint; DebitAmount float64; CreditAmount float64; Description string
}
var ujls []UJLInfo
db.Raw("SELECT id, journal_id, account_id, debit_amount, credit_amount, description FROM unified_journal_lines ORDER BY journal_id, id").Scan(&ujls)
for _, u := range ujls {
fmt.Printf("  UJL#%d J#%d Acc=%d D:%.2f C:%.2f | %s\n", 
u.ID, u.JournalID, u.AccountID, u.DebitAmount, u.CreditAmount, u.Description)
}
}
