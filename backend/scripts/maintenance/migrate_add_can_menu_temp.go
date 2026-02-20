package main

import (
	"log"
	"app-sistem-akuntansi/database"
	"gorm.io/gorm"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Add can_menu column to module_permissions table
	err = db.Exec("ALTER TABLE module_permissions ADD COLUMN can_menu BOOLEAN DEFAULT FALSE").Error
	if err != nil {
		log.Fatal("Failed to add can_menu column:", err)
	}

	// Set default can_menu values based on existing permissions and typical use cases
	// For admin - all menu access
	err = db.Exec(`
		UPDATE module_permissions mp
		SET can_menu = TRUE
		FROM users u
		WHERE mp.user_id = u.id AND u.role = 'admin'
	`).Error
	if err != nil {
		log.Printf("Warning: Failed to set admin default can_menu values: %v", err)
	}

	// For finance - full menu access to financial modules
	err = db.Exec(`
		UPDATE module_permissions mp
		SET can_menu = TRUE
		FROM users u
		WHERE mp.user_id = u.id 
		AND u.role IN ('finance', 'finance_manager')
		AND mp.module IN ('accounts', 'payments', 'cash_bank', 'sales', 'purchases', 'settings')
	`).Error
	if err != nil {
		log.Printf("Warning: Failed to set finance default can_menu values: %v", err)
	}

	// For inventory_manager - menu access to inventory-related modules
	err = db.Exec(`
		UPDATE module_permissions mp
		SET can_menu = TRUE
		FROM users u
		WHERE mp.user_id = u.id 
		AND u.role = 'inventory_manager'
		AND mp.module IN ('products', 'purchases', 'sales', 'assets', 'reports')
	`).Error
	if err != nil {
		log.Printf("Warning: Failed to set inventory_manager default can_menu values: %v", err)
	}

	// For director - menu access to oversight modules
	err = db.Exec(`
		UPDATE module_permissions mp
		SET can_menu = TRUE
		FROM users u
		WHERE mp.user_id = u.id 
		AND u.role = 'director'
		AND mp.module IN ('purchases', 'sales', 'payments', 'cash_bank', 'settings', 'reports')
	`).Error
	if err != nil {
		log.Printf("Warning: Failed to set director default can_menu values: %v", err)
	}

	// For employee - limited menu access (products and purchases only, NOT contacts)
	err = db.Exec(`
		UPDATE module_permissions mp
		SET can_menu = TRUE
		FROM users u
		WHERE mp.user_id = u.id 
		AND u.role = 'employee'
		AND mp.module IN ('products', 'purchases')
	`).Error
	if err != nil {
		log.Printf("Warning: Failed to set employee default can_menu values: %v", err)
	}

	log.Println("Successfully added can_menu column and set default values")
}