package main

import (
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.LoadConfig()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	var workflows []models.ApprovalWorkflow
	db.Preload("Steps").Find(&workflows)

	for _, w := range workflows {
		fmt.Printf("Workflow: %s (Module: %s, Min: %.2f, Max: %.2f)\n", w.Name, w.Module, w.MinAmount, w.MaxAmount)
		for _, s := range w.Steps {
			fmt.Printf("  Step %d: %s (Role: %s, Optional: %v)\n", s.StepOrder, s.StepName, s.ApproverRole, s.IsOptional)
		}
	}
}
