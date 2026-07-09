package database

import (
	"log"
	
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// AddSettingsTable creates the settings table in the database
func AddSettingsTable(db *gorm.DB) error {
	// Create the settings table
	if err := db.AutoMigrate(&models.Settings{}); err != nil {
		log.Printf("Error creating settings table: %v", err)
		return err
	}
	
	log.Println("✅ Settings table created successfully")
	
	// Check if default settings exist
	var count int64
	if err := db.Model(&models.Settings{}).Count(&count).Error; err != nil {
		log.Printf("Error checking settings count: %v", err)
		return err
	}
	
	// If no settings exist, create default settings
	if count == 0 {
		defaultSettings := models.Settings{
			CompanyName:        "PT. Sistem Akuntansi Indonesia",
			CompanyAddress:     "Jl. Sudirman Kav. 45-46, Jakarta Pusat 10210, Indonesia",
			CompanyPhone:       "+62-21-5551234",
			CompanyEmail:       "info@sistemakuntansi.co.id",
			CompanyWebsite:     "https://sistemakuntansi.co.id",
			CompanyLogoScale:   100,
			TaxNumber:          "01.234.567.8-901.000",
			Currency:           "IDR",
			DateFormat:         "DD/MM/YYYY",
			FiscalYearStart:    "January 1",
			DefaultTaxRate:     11.0,
			Language:           "id",
			Timezone:           "Asia/Jakarta",
			ThousandSeparator:  ".",
			DecimalSeparator:   ",",
			DecimalPlaces:      2,
			InvoicePrefix:      "INV",
			QuotePrefix:        "QT",
			PurchasePrefix:     "PO",
			UpdatedBy:          1, // System user
		}
		
		if err := db.Create(&defaultSettings).Error; err != nil {
			log.Printf("Error creating default settings: %v", err)
			return err
		}
		
		log.Println("✅ Default settings created successfully")
	}
	
	return nil
}

// RunSettingsMigration executes the settings migration
func RunSettingsMigration(db *gorm.DB) {
	log.Println("🔄 Running settings table migration...")
	
	if err := AddSettingsTable(db); err != nil {
		log.Fatalf("❌ Failed to run settings migration: %v", err)
	}
	
	log.Println("✅ Settings migration completed successfully")
}
