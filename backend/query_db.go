package main

import (
	"app-sistem-akuntansi/config"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.LoadConfig()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	type Result struct {
		ReqID    uint
		ActionID uint
		Status   string
		IsActive bool
		StepName string
		ReqTitle string
	}

	var results []Result
	db.Raw(`
		SELECT ar.id as req_id, aa.id as action_id, aa.status, aa.is_active, as_.step_name, ar.request_title 
		FROM approval_requests ar 
		JOIN approval_actions aa ON ar.id = aa.request_id 
		JOIN approval_steps as_ ON aa.step_id = as_.id
		ORDER BY ar.id DESC
		LIMIT 20
	`).Scan(&results)

	for _, r := range results {
		fmt.Printf("ReqID: %d | ActionID: %d | Status: %s | Active: %v | Step: %s | Title: %s\n",
			r.ReqID, r.ActionID, r.Status, r.IsActive, r.StepName, r.ReqTitle)
	}
}
