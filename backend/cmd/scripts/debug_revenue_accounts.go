package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("DB connect failed:", err)
	}

	fmt.Println("=== 1. Check accounts 4101 and 4101-001 ===")
	type AccInfo struct {
		ID       uint
		Code     string
		Name     string
		IsHeader *bool
		ParentID *uint
	}
	var accs []AccInfo
	db.Raw("SELECT id, code, name, is_header, parent_id FROM accounts WHERE code IN ('4101', '4101-001') AND deleted_at IS NULL ORDER BY code").Scan(&accs)
	for _, a := range accs {
		isHeader := false
		if a.IsHeader != nil {
			isHeader = *a.IsHeader
		}
		fmt.Printf("  ID=%d Code=%s Name=%s IsHeader=%v ParentID=%v\n", a.ID, a.Code, a.Name, isHeader, a.ParentID)
	}

	fmt.Println("\n=== 2. Check tax_account_settings.sales_revenue_account_id ===")
	type TaxSetting struct {
		ID                    uint
		SalesRevenueAccountID uint
	}
	var ts TaxSetting
	db.Raw("SELECT id, sales_revenue_account_id FROM tax_account_settings WHERE is_active = true LIMIT 1").Scan(&ts)
	fmt.Printf("  Settings ID=%d, SalesRevenueAccountID=%d\n", ts.ID, ts.SalesRevenueAccountID)

	fmt.Println("\n=== 3. Check sale_items.revenue_account_id ===")
	type SaleItemInfo struct {
		ID               uint
		SaleID           uint
		ProductID        uint
		Description      string
		RevenueAccountID uint
	}
	var items []SaleItemInfo
	db.Raw("SELECT id, sale_id, product_id, description, revenue_account_id FROM sale_items WHERE deleted_at IS NULL ORDER BY sale_id, id").Scan(&items)
	for _, item := range items {
		fmt.Printf("  SaleItem ID=%d SaleID=%d ProductID=%d RevenueAccountID=%d Desc=%s\n",
			item.ID, item.SaleID, item.ProductID, item.RevenueAccountID, item.Description)
	}

	fmt.Println("\n=== 4. Check unified_journal_lines for SALE entries with revenue accounts ===")
	type JournalLine struct {
		JournalID    uint
		AccountID    uint
		AccountCode  string
		AccountName  string
		DebitAmount  float64
		CreditAmount float64
		Description  string
	}
	var lines []JournalLine
	db.Raw(`SELECT ujl.journal_id, ujl.account_id, a.code as account_code, a.name as account_name,
		ujl.debit_amount, ujl.credit_amount, ujl.description
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		JOIN accounts a ON a.id = ujl.account_id
		WHERE uje.source_type = 'SALE'
		ORDER BY ujl.journal_id, ujl.id`).Scan(&lines)
	for _, l := range lines {
		dt := ""
		if l.DebitAmount > 0 {
			dt = fmt.Sprintf("D:%.0f", l.DebitAmount)
		}
		ct := ""
		if l.CreditAmount > 0 {
			ct = fmt.Sprintf("C:%.0f", l.CreditAmount)
		}
		fmt.Printf("  J#%d Acc=%d (%s - %s) %s %s | %s\n",
			l.JournalID, l.AccountID, l.AccountCode, l.AccountName, dt, ct, l.Description)
	}

	fmt.Println("\n=== 5. Check simple_ssot_journal_items for SALE entries ===")
	type SSOTItem struct {
		JournalID   uint
		AccountID   uint
		AccountCode string
		AccountName string
		Debit       float64
		Credit      float64
		Description string
	}
	var ssotItems []SSOTItem
	db.Raw(`SELECT si.journal_id, si.account_id, si.account_code, si.account_name,
		si.debit, si.credit, si.description
		FROM simple_ssot_journal_items si
		JOIN simple_ssot_journals sj ON sj.id = si.journal_id
		WHERE sj.transaction_type = 'SALES'
		ORDER BY si.journal_id, si.id`).Scan(&ssotItems)
	for _, s := range ssotItems {
		dt := ""
		if s.Debit > 0 {
			dt = fmt.Sprintf("D:%.0f", s.Debit)
		}
		ct := ""
		if s.Credit > 0 {
			ct = fmt.Sprintf("C:%.0f", s.Credit)
		}
		fmt.Printf("  J#%d Acc=%d (%s - %s) %s %s | %s\n",
			s.JournalID, s.AccountID, s.AccountCode, s.AccountName, dt, ct, s.Description)
	}
}
