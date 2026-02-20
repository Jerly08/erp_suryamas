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

	// Check for triggers on journal tables
	fmt.Println("=== Database Triggers ===")
	type Trigger struct {
		TriggerName string
		EventManip  string
		TableName   string
		ActionStmt  string
	}
	var triggers []Trigger
	db.Raw(`SELECT trigger_name, event_manipulation as event_manip, 
		event_object_table as table_name,
		action_statement as action_stmt
		FROM information_schema.triggers
		WHERE event_object_schema = 'public'
		ORDER BY event_object_table, trigger_name`).Scan(&triggers)

	if len(triggers) == 0 {
		fmt.Println("No triggers found")
	}
	for _, t := range triggers {
		fmt.Printf("TRIGGER: %s ON %s (%s)\n  Action: %s\n\n", t.TriggerName, t.TableName, t.EventManip, t.ActionStmt)
	}

	// Also check trigger functions
	fmt.Println("=== Trigger Functions ===")
	type TrigFunc struct {
		FuncName string
		FuncBody string
	}
	var funcs []TrigFunc
	db.Raw(`SELECT routine_name as func_name, 
		LEFT(routine_definition, 500) as func_body
		FROM information_schema.routines
		WHERE routine_schema = 'public'
		AND routine_type = 'FUNCTION'
		AND data_type = 'trigger'
		ORDER BY routine_name`).Scan(&funcs)

	if len(funcs) == 0 {
		fmt.Println("No trigger functions found")
	}
	for _, f := range funcs {
		fmt.Printf("FUNCTION: %s\n  Body: %s\n\n", f.FuncName, f.FuncBody)
	}

	// Check pg_trigger directly for more detail
	fmt.Println("=== pg_trigger entries ===")
	type PgTrig struct {
		TgName    string
		RelName   string
		TgEnabled string
		ProcName  string
	}
	var pgTrigs []PgTrig
	db.Raw(`SELECT t.tgname as tg_name, c.relname as rel_name, 
		t.tgenabled as tg_enabled,
		p.proname as proc_name
		FROM pg_trigger t
		JOIN pg_class c ON t.tgrelid = c.oid
		JOIN pg_proc p ON t.tgfoid = p.oid
		WHERE NOT t.tgisinternal
		ORDER BY c.relname, t.tgname`).Scan(&pgTrigs)

	if len(pgTrigs) == 0 {
		fmt.Println("No non-internal triggers found")
	}
	for _, t := range pgTrigs {
		fmt.Printf("  %s ON %s (enabled=%s) -> %s()\n", t.TgName, t.RelName, t.TgEnabled, t.ProcName)
	}
}
