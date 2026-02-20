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
result := db.Exec("UPDATE accounts SET is_active = true WHERE code = '1101-005' AND deleted_at IS NULL")
if result.Error != nil { log.Fatal(result.Error) }
fmt.Printf("Re-activated 1101-005 BANK OCBC: %d rows updated\n", result.RowsAffected)

// Verify
type AccInfo struct { Code string; Name string; IsActive bool; Balance float64 }
var acc AccInfo
db.Raw("SELECT code, name, is_active, balance FROM accounts WHERE code = '1101-005' AND deleted_at IS NULL").Scan(&acc)
fmt.Printf("Status: %s %s is_active=%v balance=%.2f\n", acc.Code, acc.Name, acc.IsActive, acc.Balance)
}
