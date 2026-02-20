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

// Get all numeric columns from sales table
type ColInfo struct {
ColumnName string
DataType string
}
var cols []ColInfo
db.Raw(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'sales' AND (data_type LIKE '%numeric%' OR data_type LIKE '%double%' OR data_type LIKE '%real%' OR data_type LIKE '%int%' OR column_name LIKE '%ppn%' OR column_name LIKE '%tax%' OR column_name LIKE '%type%' OR column_name LIKE '%payment%') ORDER BY ordinal_position`).Scan(&cols)
fmt.Println("=== Sales table relevant columns ===")
for _, c := range cols { fmt.Printf("  %s (%s)\n", c.ColumnName, c.DataType) }

// Now get PPN values for both sales
fmt.Println("\n=== PPN values for SALE-1 and SALE-2 ===")
type PPNVals struct {
ID uint
Subtotal float64
PPN float64
PPNAmount float64
PPNPercent float64
PPNRate float64
TotalAmount float64
}
var sales []PPNVals
db.Raw(`SELECT id, subtotal, ppn, ppn_amount, ppn_percent, ppn_rate, total_amount FROM sales ORDER BY id`).Scan(&sales)
for _, s := range sales {
fmt.Printf("SALE-%d: Subtotal=%.2f, PPN=%.2f, PPNPercent=%.2f, PPNRate=%.2f, PPNAmount=%.2f, TotalAmount=%.2f\n",
s.ID, s.Subtotal, s.PPN, s.PPNPercent, s.PPNRate, s.PPNAmount, s.TotalAmount)
}
}
