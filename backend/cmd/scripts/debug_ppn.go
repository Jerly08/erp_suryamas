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

fmt.Println("=== SALE-2 (Credit Sale) PPN Analysis ===")
type SaleInfo struct {
ID uint
InvoiceNumber string
PaymentType string
Subtotal float64
PPN float64
PPNPercent float64
PPNRate float64
PPNAmount float64
TotalAmount float64
TaxableAmount float64
}
var sale SaleInfo
db.Raw(`SELECT id, invoice_number, payment_type, subtotal, ppn, ppn_percent, ppn_rate, ppn_amount, total_amount, taxable_amount FROM sales WHERE id = 2`).Scan(&sale)
fmt.Printf("Sale: %+v\n", sale)

fmt.Println("\n=== SaleItem Taxable flags ===")
type ItemInfo struct {
ID uint
ProductID uint
Taxable bool
PPNAmount float64
LineTotal float64
}
var items []ItemInfo
db.Raw(`SELECT id, product_id, taxable, ppn_amount, line_total FROM sale_items WHERE sale_id = 2`).Scan(&items)
for _, item := range items {
fmt.Printf("  Item %d: Taxable=%v, PPNAmount=%.2f, LineTotal=%.2f\n", item.ID, item.Taxable, item.PPNAmount, item.LineTotal)
}

fmt.Println("\n=== Unified Journal Lines for SALE-2 ===")
type JLine struct {
ID uint
AccountID uint
DebitAmount float64
CreditAmount float64
Description string
}
var lines []JLine
db.Raw(`SELECT ujl.id, ujl.account_id, ujl.debit_amount, ujl.credit_amount, ujl.description
FROM unified_journal_lines ujl
JOIN unified_journal_ledger ujle ON ujl.journal_id = ujle.id
WHERE ujle.source_type = 'SALE' AND ujle.source_id = 2
ORDER BY ujl.id`).Scan(&lines)
for _, line := range lines {
var accCode, accName string
db.Raw("SELECT code, name FROM accounts WHERE id = ?", line.AccountID).Row().Scan(&accCode, &accName)
fmt.Printf("  J#%d Acc=%d (%s - %s) D:%.0f C:%.0f | %s\n", line.ID, line.AccountID, accCode, accName, line.DebitAmount, line.CreditAmount, line.Description)
}

fmt.Println("\n=== Compare SALE-1 (Cash) PPN ===")
var sale1 SaleInfo
db.Raw(`SELECT id, invoice_number, payment_type, subtotal, ppn, ppn_percent, ppn_rate, ppn_amount, total_amount, taxable_amount FROM sales WHERE id = 1`).Scan(&sale1)
fmt.Printf("SALE-1: PPN=%.2f, PPNPercent=%.2f, PPNRate=%.2f, PPNAmount=%.2f, Subtotal=%.2f\n", sale1.PPN, sale1.PPNPercent, sale1.PPNRate, sale1.PPNAmount, sale1.Subtotal)
fmt.Printf("SALE-2: PPN=%.2f, PPNPercent=%.2f, PPNRate=%.2f, PPNAmount=%.2f, Subtotal=%.2f\n", sale.PPN, sale.PPNPercent, sale.PPNRate, sale.PPNAmount, sale.Subtotal)
}
