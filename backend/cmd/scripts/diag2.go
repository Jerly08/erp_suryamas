package main
import (
"fmt"
"log"
"os"
"gorm.io/driver/postgres"
"gorm.io/gorm"
"gorm.io/gorm/logger"
)
func main() {
db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"), &gorm.Config{
Logger: logger.Default.LogMode(logger.Silent),
})
if err != nil { log.Fatal(err) }

f, _ := os.Create("C:\\temp\\diag_out.txt")
defer f.Close()

w := func(format string, args ...interface{}) { fmt.Fprintf(f, format+"\n", args...) }

// 1
var accBalance float64
var accID uint
db.Raw("SELECT id, balance FROM accounts WHERE code = '1101-005' AND deleted_at IS NULL").Row().Scan(&accID, &accBalance)
w("COA Balance (accounts.balance): %.2f  (ID=%d)", accBalance, accID)

var cbBalance float64
db.Raw("SELECT balance FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", accID).Row().Scan(&cbBalance)
w("Cash Bank Balance: %.2f", cbBalance)
w("Selisih: %.2f", cbBalance - accBalance)
w("")

// 2 - Journal lines
type JL struct {
ID uint; JournalID uint; Debit float64; Credit float64; Desc string
}
var lines []JL
db.Raw(`SELECT id, journal_id, debit_amount as debit, credit_amount as credit, description as desc 
FROM unified_journal_lines WHERE account_id = ? ORDER BY id`, accID).Scan(&lines)

w("=== Journal Lines for OCBC (account_id=%d) ===", accID)
var td, tc float64
for _, l := range lines {
td += l.Debit; tc += l.Credit
w("  JL#%d J#%d D:%.2f C:%.2f | %s", l.ID, l.JournalID, l.Debit, l.Credit, l.Desc)
}
w("")
w("Sum Debit: %.2f", td)
w("Sum Credit: %.2f", tc)
w("Calculated (D-C): %.2f", td-tc)
w("")

// 3 - Cash bank transactions
type CBT struct {
ID uint; Type string; Amount float64; After float64; Desc string; Ref string
}
var txs []CBT
db.Raw(`SELECT id, type, amount, balance_after as after, description as desc, reference_number as ref
FROM cash_bank_transactions WHERE cash_bank_id = (SELECT id FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL)
AND deleted_at IS NULL ORDER BY id`, accID).Scan(&txs)

w("=== Cash Bank Transactions ===")
for _, t := range txs {
w("  TX#%d %s Amount:%.2f After:%.2f Ref:%s | %s", t.ID, t.Type, t.Amount, t.After, t.Ref, t.Desc)
}

fmt.Println("Done - see C:\\temp\\diag_out.txt")
}
