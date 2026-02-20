package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	// List all non-internal triggers with their source code
	type TrigDetail struct {
		TriggerName string
		TableName   string
		ProcName    string
		Events      string
		ProcSrc     string
	}
	var trigs []TrigDetail
	db.Raw(`
		SELECT t.tgname as trigger_name, 
			   c.relname as table_name, 
			   p.proname as proc_name,
			   CASE 
				 WHEN (t.tgtype & 2)::bool THEN 'BEFORE '
				 WHEN (t.tgtype & 64)::bool THEN 'INSTEAD OF '
				 ELSE 'AFTER '
			   END ||
			   CASE WHEN (t.tgtype & 4)::bool THEN 'INSERT ' ELSE '' END ||
			   CASE WHEN (t.tgtype & 8)::bool THEN 'DELETE ' ELSE '' END ||
			   CASE WHEN (t.tgtype & 16)::bool THEN 'UPDATE ' ELSE '' END
			   as events,
			   p.prosrc as proc_src
		FROM pg_trigger t
		JOIN pg_class c ON t.tgrelid = c.oid
		JOIN pg_proc p ON t.tgfoid = p.oid
		WHERE NOT t.tgisinternal
		  AND c.relname IN ('journal_lines', 'journal_entries', 'unified_journal_lines', 'unified_journal_ledger', 'accounts')
		ORDER BY c.relname, t.tgname
	`).Scan(&trigs)

	for _, t := range trigs {
		fmt.Printf("=== %s ON %s [%s] → %s() ===\n", t.TriggerName, t.TableName, t.Events, t.ProcName)
		fmt.Println(t.ProcSrc)
		fmt.Println()
	}

	if len(trigs) == 0 {
		fmt.Println("No triggers found on these tables")
	}

	fmt.Printf("Total triggers found: %d\n", len(trigs))
}
