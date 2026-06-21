package database

import (
	"app-sistem-akuntansi/models"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedData(db *gorm.DB) {
	log.Println("Starting database seeding...")

	// Seed Users
	seedUsers(db)

	// Ensure module permissions table and defaults are prepared (Manage Permission)
	if err := MigratePermissions(db); err != nil {
		log.Printf("Error migrating/initializing module permissions: %v", err)
	}

	// Seed Accounts (COA) - Using improved version with duplicate protection
	if err := SeedAccountsImproved(db); err != nil {
		log.Printf("Error seeding accounts: %v", err)
	} else {
		log.Println("✅ Accounts seeded successfully with duplicate protection")
	}
	EnsureOpeningBalanceEquityAccount(db)
	// Note: FixAccountHierarchies not needed - SeedAccountsImproved handles parent relationships
	// Ensure PPh-related tax asset accounts exist, including the Kredit Pajak PPh23 reclass account.

	// ✅ CRITICAL: Ensure PPh prepaid tax accounts exist (required for sales with tax deductions)
	// This migration is idempotent and safe to run on every startup
	if err := MigratePPhPrepaidAccounts(db); err != nil {
		log.Printf("Error migrating PPh prepaid accounts: %v", err)
	}

	// Seed Contacts
	SeedContacts(db)

	// Seed Product Categories
	seedProductCategories(db)

	// Seed Product Units
	seedProductUnits(db)

	// Seed Products
	seedProducts(db)

	// Seed Expense Categories
	seedExpenseCategories(db)

	// Seed Cash & Bank accounts (start with zero balances)
	// Disabled per request: do not create default Cash & Bank records during seed
	// seedCashBankAccounts(db)

	// Seed Company Profile
	seedCompanyProfile(db)

	// Seed Report Templates
	seedReportTemplates(db)

	// Seed Permissions
	seedPermissions(db)

	// Seed Role Permissions
	seedRolePermissions(db)

	// Seed default approval workflows for PURCHASE module
	seedApprovalWorkflows(db)

	// REMOVED: Sample sales and purchases seeding to use only real data
	// No dummy data will be generated for sales, purchases, or transactions
	// Dashboard will display only actual business data

	log.Println("Database seeding completed successfully")
}

func seedUsers(db *gorm.DB) {
	log.Println("🔄 Starting user seeding with robust UPSERT...")

	// Seed all users for all roles
	adminSeedPassword := os.Getenv("ADMIN_SEED_PASSWORD")
	if adminSeedPassword == "" {
		adminSeedPassword = "Averyfinance@123"
	}

	adminPassword, _ := bcrypt.GenerateFromPassword([]byte(adminSeedPassword), bcrypt.DefaultCost)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	allUsers := []models.User{
		{
			Username:  "admin",
			Email:     "admin@company.com",
			Password:  string(adminPassword),
			Role:      "admin",
			FirstName: "Admin",
			LastName:  "User",
			IsActive:  true,
		},
		{
			Username:  "finance",
			Email:     "finance@company.com",
			Password:  string(hashedPassword),
			Role:      "finance",
			FirstName: "Finance",
			LastName:  "User",
			IsActive:  true,
		},
		{
			Username:  "inventory",
			Email:     "inventory@company.com",
			Password:  string(hashedPassword),
			Role:      "inventory_manager",
			FirstName: "Inventory",
			LastName:  "User",
			IsActive:  true,
		},
		{
			Username:  "director",
			Email:     "director@company.com",
			Password:  string(hashedPassword),
			Role:      "director",
			FirstName: "Director",
			LastName:  "User",
			IsActive:  true,
		},
		{
			Username:  "employee",
			Email:     "employee@company.com",
			Password:  string(hashedPassword),
			Role:      "employee",
			FirstName: "Employee",
			LastName:  "User",
			IsActive:  true,
		},
		{
			Username:  "hr",
			Email:     "hr@company.com",
			Password:  string(hashedPassword),
			Role:      "hr",
			FirstName: "HR",
			LastName:  "User",
			IsActive:  true,
		},
	}

	// Use PostgreSQL native INSERT with ON CONFLICT DO NOTHING
	// This ensures seed only runs once and doesn't reset edited data
	successCount := 0
	for _, user := range allUsers {
		// Check if user already exists
		var existingUser models.User
		if err := db.Where("username = ?", user.Username).First(&existingUser).Error; err == nil {
			log.Printf("⏭️  User %s already exists, skipping", user.Username)
			continue
		}

		// User doesn't exist, create it
		query := `
			INSERT INTO users (
				username, email, password, role, first_name, last_name,
				phone, address, department, position, salary, is_active,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, '', '', '', '', 0, ?, NOW(), NOW())
			ON CONFLICT (username) DO NOTHING
		`

		result := db.Exec(query,
			user.Username, user.Email, user.Password, user.Role,
			user.FirstName, user.LastName, user.IsActive)

		if result.Error != nil {
			log.Printf("⚠️  Warning: Failed to create user %s: %v", user.Username, result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("✅ User %s created successfully", user.Username)
			successCount++
		}
	}

	log.Printf("📊 User seeding completed: %d/%d users processed successfully", successCount, len(allUsers))
}

func seedProductCategories(db *gorm.DB) {
	// Check if product categories already exist
	var count int64
	db.Model(&models.ProductCategory{}).Count(&count)
	if count > 0 {
		return
	}

	categories := []models.ProductCategory{
		{Code: "CAT001", Name: "Elektronik", Description: "Produk elektronik dan gadget"},
		{Code: "CAT002", Name: "Furniture", Description: "Perabotan dan furniture kantor"},
		{Code: "CAT003", Name: "Alat Tulis", Description: "Alat tulis dan perlengkapan kantor"},
		{Code: "CAT004", Name: "Komputer", Description: "Komputer dan aksesoris"},
	}

	for _, category := range categories {
		db.Create(&category)
	}
}

func seedProducts(db *gorm.DB) {
	// Check if specific seed products already exist (including soft-deleted)
	var seedProductExists int64
	db.Unscoped().Model(&models.Product{}).Where("code IN ?", []string{"PRD001", "PRD002", "PRD003"}).Count(&seedProductExists)
	if seedProductExists > 0 {
		log.Printf("Seed products already exist (%d records), skipping seed", seedProductExists)
		return
	}

	// Get first category for relation
	var category models.ProductCategory
	db.First(&category)

	products := []models.Product{
		{
			Code:          "PRD001",
			Name:          "Laptop Dell XPS 13",
			Description:   "Laptop Dell XPS 13 inch with Intel Core i7",
			CategoryID:    &category.ID,
			Brand:         "Dell",
			Unit:          "pcs",
			PurchasePrice: 12000000,
			SalePrice:     15000000,
			Stock:         10,
			MinStock:      5,
			MaxStock:      50,
			ReorderLevel:  8,
			SKU:           "DELL-XPS13-I7",
			IsActive:      true,
		},
		{
			Code:          "PRD002",
			Name:          "Mouse Wireless Logitech",
			Description:   "Mouse wireless Logitech MX Master 3",
			CategoryID:    &category.ID,
			Brand:         "Logitech",
			Unit:          "pcs",
			PurchasePrice: 800000,
			SalePrice:     1200000,
			Stock:         25,
			MinStock:      10,
			MaxStock:      100,
			ReorderLevel:  15,
			SKU:           "LOG-MX3-WL",
			IsActive:      true,
		},
		{
			Code:          "PRD003",
			Name:          "Kertas A4 80gsm",
			Description:   "Kertas A4 80gsm per rim (500 lembar)",
			CategoryID:    &category.ID,
			Unit:          "rim",
			PurchasePrice: 45000,
			SalePrice:     65000,
			Stock:         100,
			MinStock:      20,
			MaxStock:      500,
			ReorderLevel:  30,
			SKU:           "PAPER-A4-80G",
			IsActive:      true,
		},
	}

	// Create products one by one, checking for existing records first
	successCount := 0
	for _, product := range products {
		// Check if product already exists by code
		var existingProduct models.Product
		if err := db.Where("code = ?", product.Code).First(&existingProduct).Error; err == nil {
			log.Printf("Product %s already exists, skipping", product.Code)
			continue
		}

		// Product doesn't exist, create it
		if err := db.Create(&product).Error; err != nil {
			log.Printf("Error seeding product %s: %v", product.Code, err)
		} else {
			successCount++
			log.Printf("Successfully created product %s", product.Code)
		}
	}
	log.Printf("Successfully seeded %d out of %d products", successCount, len(products))
}

func seedExpenseCategories(db *gorm.DB) {
	// Check if expense categories already exist
	var count int64
	db.Model(&models.ExpenseCategory{}).Count(&count)
	if count > 0 {
		return
	}

	categories := []models.ExpenseCategory{
		{Code: "EXP001", Name: "Operasional", Description: "Biaya operasional harian"},
		{Code: "EXP002", Name: "Marketing", Description: "Biaya pemasaran dan promosi"},
		{Code: "EXP003", Name: "Administrasi", Description: "Biaya administrasi dan umum"},
		{Code: "EXP004", Name: "Transportasi", Description: "Biaya transportasi dan perjalanan"},
	}

	for _, category := range categories {
		db.Create(&category)
	}
}

func seedCashBankAccounts(db *gorm.DB) {
	// Check if cash bank accounts already exist
	var count int64
	db.Model(&models.CashBank{}).Count(&count)
	if count > 0 {
		// Cash bank accounts already exist - don't modify balances
		// Balances should only come from legitimate transactions
		return
	}

	cashBanks := []models.CashBank{
		{
			Code:     "CASH001",
			Name:     "Kas Besar",
			Type:     models.CashBankTypeCash,
			Balance:  0, // Start with 0 - balance comes from transactions
			IsActive: true,
		},
		{
			Code:     "BANK001",
			Name:     "Bank BCA - Operasional",
			Type:     models.CashBankTypeBank,
			Balance:  0, // Start with 0 - balance comes from transactions
			IsActive: true,
		},
		{
			Code:     "BANK002",
			Name:     "Bank Mandiri - Payroll",
			Type:     models.CashBankTypeBank,
			Balance:  0, // Start with 0 - balance comes from transactions
			IsActive: true,
		},
	}

	for _, cashBank := range cashBanks {
		db.Create(&cashBank)
	}
}

func seedCompanyProfile(db *gorm.DB) {
	// Check if company profile already exists
	var count int64
	db.Model(&models.CompanyProfile{}).Count(&count)
	if count > 0 {
		return
	}

	company := models.CompanyProfile{
		Name:               "PT Contoh Perusahaan",
		LegalName:          "PT Contoh Perusahaan Tbk",
		TaxNumber:          "01.234.567.8-901.000",
		RegistrationNumber: "AHU-123456789",
		Industry:           "Perdagangan",
		Address:            "Jl. Contoh No. 123, Jakarta Selatan",
		City:               "Jakarta",
		State:              "DKI Jakarta",
		PostalCode:         "12345",
		Country:            "Indonesia",
		Phone:              "021-12345678",
		Email:              "info@perusahaan.com",
		Website:            "www.perusahaan.com",
		FiscalYearStart:    "01-01",
		Currency:           "IDR",
		IsActive:           true,
	}

	db.Create(&company)
}

func seedReportTemplates(db *gorm.DB) {
	// Check if report templates already exist
	var count int64
	db.Model(&models.ReportTemplate{}).Count(&count)
	if count > 0 {
		return
	}

	// Get first user for relation
	var user models.User
	db.First(&user)

	templates := []models.ReportTemplate{
		{
			Name:        "Neraca Standar",
			Type:        models.ReportTypeBalanceSheet,
			Description: "Template neraca standar dengan format Indonesia",
			Template:    `{"sections":["ASET","KEWAJIBAN","EKUITAS"],"format":"standard"}`,
			IsDefault:   true,
			IsActive:    true,
			UserID:      user.ID,
		},
		{
			Name:        "Laporan Laba Rugi",
			Type:        models.ReportTypeIncomeStatement,
			Description: "Template laporan laba rugi dengan format Indonesia",
			Template:    `{"sections":["PENDAPATAN","BEBAN","LABA_BERSIH"],"format":"standard"}`,
			IsDefault:   true,
			IsActive:    true,
			UserID:      user.ID,
		},
		{
			Name:        "Neraca Saldo",
			Type:        models.ReportTypeTrialBalance,
			Description: "Template neraca saldo untuk semua akun",
			Template:    `{"columns":["code","name","debit","credit"],"format":"detailed"}`,
			IsDefault:   true,
			IsActive:    true,
			UserID:      user.ID,
		},
	}

	for _, template := range templates {
		db.Create(&template)
	}
}

func seedPermissions(db *gorm.DB) {
	// Check if specific seed permissions already exist
	var seedPermissionExists int64
	db.Model(&models.Permission{}).Where("name IN ?", []string{"users:read", "accounts:read", "transactions:read"}).Count(&seedPermissionExists)
	if seedPermissionExists > 0 {
		log.Printf("Seed permissions already exist (%d records), skipping seed", seedPermissionExists)
		return
	}

	permissions := []models.Permission{
		// User permissions
		{Name: "users:read", Resource: "users", Action: "read", Description: "View users"},
		{Name: "users:create", Resource: "users", Action: "create", Description: "Create users"},
		{Name: "users:update", Resource: "users", Action: "update", Description: "Update users"},
		{Name: "users:delete", Resource: "users", Action: "delete", Description: "Delete users"},
		{Name: "users:manage", Resource: "users", Action: "manage", Description: "Full user management"},

		// Account permissions
		{Name: "accounts:read", Resource: "accounts", Action: "read", Description: "View accounts"},
		{Name: "accounts:create", Resource: "accounts", Action: "create", Description: "Create accounts"},
		{Name: "accounts:update", Resource: "accounts", Action: "update", Description: "Update accounts"},
		{Name: "accounts:delete", Resource: "accounts", Action: "delete", Description: "Delete accounts"},

		// Transaction permissions
		{Name: "transactions:read", Resource: "transactions", Action: "read", Description: "View transactions"},
		{Name: "transactions:create", Resource: "transactions", Action: "create", Description: "Create transactions"},
		{Name: "transactions:update", Resource: "transactions", Action: "update", Description: "Update transactions"},
		{Name: "transactions:delete", Resource: "transactions", Action: "delete", Description: "Delete transactions"},

		// Product permissions
		{Name: "products:read", Resource: "products", Action: "read", Description: "View products"},
		{Name: "products:create", Resource: "products", Action: "create", Description: "Create products"},
		{Name: "products:update", Resource: "products", Action: "update", Description: "Update products"},
		{Name: "products:delete", Resource: "products", Action: "delete", Description: "Delete products"},

		// Sales permissions
		{Name: "sales:read", Resource: "sales", Action: "read", Description: "View sales"},
		{Name: "sales:create", Resource: "sales", Action: "create", Description: "Create sales"},
		{Name: "sales:update", Resource: "sales", Action: "update", Description: "Update sales"},
		{Name: "sales:delete", Resource: "sales", Action: "delete", Description: "Delete sales"},

		// Purchase permissions
		{Name: "purchases:read", Resource: "purchases", Action: "read", Description: "View purchases"},
		{Name: "purchases:create", Resource: "purchases", Action: "create", Description: "Create purchases"},
		{Name: "purchases:update", Resource: "purchases", Action: "update", Description: "Update purchases"},
		{Name: "purchases:delete", Resource: "purchases", Action: "delete", Description: "Delete purchases"},

		// Report permissions
		{Name: "reports:read", Resource: "reports", Action: "read", Description: "View reports"},
		{Name: "reports:create", Resource: "reports", Action: "create", Description: "Create reports"},
		{Name: "reports:update", Resource: "reports", Action: "update", Description: "Update reports"},
		{Name: "reports:delete", Resource: "reports", Action: "delete", Description: "Delete reports"},

		// Contact permissions
		{Name: "contacts:read", Resource: "contacts", Action: "read", Description: "View contacts"},
		{Name: "contacts:create", Resource: "contacts", Action: "create", Description: "Create contacts"},
		{Name: "contacts:update", Resource: "contacts", Action: "update", Description: "Update contacts"},
		{Name: "contacts:delete", Resource: "contacts", Action: "delete", Description: "Delete contacts"},

		// Asset permissions
		{Name: "assets:read", Resource: "assets", Action: "read", Description: "View assets"},
		{Name: "assets:create", Resource: "assets", Action: "create", Description: "Create assets"},
		{Name: "assets:update", Resource: "assets", Action: "update", Description: "Update assets"},
		{Name: "assets:delete", Resource: "assets", Action: "delete", Description: "Delete assets"},

		// Budget permissions
		{Name: "budgets:read", Resource: "budgets", Action: "read", Description: "View budgets"},
		{Name: "budgets:create", Resource: "budgets", Action: "create", Description: "Create budgets"},
		{Name: "budgets:update", Resource: "budgets", Action: "update", Description: "Update budgets"},
		{Name: "budgets:delete", Resource: "budgets", Action: "delete", Description: "Delete budgets"},

		// BOM permissions
		{Name: "boms:read", Resource: "boms", Action: "read", Description: "View BOMs"},
		{Name: "boms:create", Resource: "boms", Action: "create", Description: "Create BOMs"},
		{Name: "boms:update", Resource: "boms", Action: "update", Description: "Update BOMs"},
		{Name: "boms:delete", Resource: "boms", Action: "delete", Description: "Delete BOMs"},

		// Project permissions
		{Name: "projects:read", Resource: "projects", Action: "read", Description: "View projects"},
		{Name: "projects:create", Resource: "projects", Action: "create", Description: "Create projects"},
		{Name: "projects:update", Resource: "projects", Action: "update", Description: "Update projects"},
		{Name: "projects:delete", Resource: "projects", Action: "delete", Description: "Delete projects"},

		// Purchase Requests permissions
		{Name: "purchase_requests:read", Resource: "purchase_requests", Action: "read", Description: "View purchase requests"},
		{Name: "purchase_requests:create", Resource: "purchase_requests", Action: "create", Description: "Create purchase requests"},
		{Name: "purchase_requests:update", Resource: "purchase_requests", Action: "update", Description: "Update purchase requests"},
		{Name: "purchase_requests:delete", Resource: "purchase_requests", Action: "delete", Description: "Delete purchase requests"},
		{Name: "purchase_requests:approve", Resource: "purchase_requests", Action: "approve", Description: "Approve purchase requests"},

		// Payroll permissions
		{Name: "payroll:read", Resource: "payroll", Action: "read", Description: "View payroll"},
		{Name: "payroll:create", Resource: "payroll", Action: "create", Description: "Create payroll"},
		{Name: "payroll:update", Resource: "payroll", Action: "update", Description: "Update payroll"},
		{Name: "payroll:approve", Resource: "payroll", Action: "approve", Description: "Approve payroll"},
	}

	// Use PostgreSQL native UPSERT to avoid all duplicate key errors
	successCount := 0
	for _, permission := range permissions {
		// Use ON CONFLICT DO NOTHING to skip existing permissions silently
		query := `
			INSERT INTO permissions (name, resource, action, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
			ON CONFLICT (name) DO NOTHING
		`

		result := db.Exec(query, permission.Name, permission.Resource, permission.Action, permission.Description)
		if result.Error != nil {
			log.Printf("Error seeding permission %s: %v", permission.Name, result.Error)
		} else if result.RowsAffected > 0 {
			successCount++
		}
	}
	log.Printf("Successfully seeded %d out of %d permissions", successCount, len(permissions))
}

func seedRolePermissions(db *gorm.DB) {
	// We no longer abort if count > 0 to allow new role permissions to be added.
	var count int64
	db.Model(&models.RolePermission{}).Count(&count)
	log.Printf("Current role permissions count: %d", count)

	// Get all permissions
	var permissions []models.Permission
	db.Find(&permissions)

	permissionMap := make(map[string]uint)
	for _, perm := range permissions {
		permissionMap[perm.Name] = perm.ID
	}

	rolePermissions := map[string][]string{
		"admin": {
			"users:read", "users:create", "users:update", "users:delete", "users:manage",
			"accounts:read", "accounts:create", "accounts:update", "accounts:delete",
			"transactions:read", "transactions:create", "transactions:update", "transactions:delete",
			"products:read", "products:create", "products:update", "products:delete",
			"sales:read", "sales:create", "sales:update", "sales:delete",
			"purchases:read", "purchases:create", "purchases:update", "purchases:delete",
			"reports:read", "reports:create", "reports:update", "reports:delete",
			"contacts:read", "contacts:create", "contacts:update", "contacts:delete",
			"assets:read", "assets:create", "assets:update", "assets:delete",
			"budgets:read", "budgets:create", "budgets:update", "budgets:delete",
			"boms:read", "boms:create", "boms:update", "boms:delete",
			"projects:read", "projects:create", "projects:update", "projects:delete",
			"purchase_requests:read", "purchase_requests:create", "purchase_requests:update", "purchase_requests:delete", "purchase_requests:approve",
			"payroll:read", "payroll:create", "payroll:update",
		},
		"finance": {
			"accounts:read", "accounts:create", "accounts:update",
			"transactions:read", "transactions:create", "transactions:update",
			"sales:read", "sales:create", "sales:update",
			"purchases:read", "purchases:create", "purchases:update",
			"reports:read", "reports:create",
			"contacts:read", "contacts:update",
			"assets:read", "assets:update",
			"budgets:read", "budgets:create", "budgets:update",
			"boms:read",
			"projects:read",
			"purchase_requests:read", "purchase_requests:approve",
			"payroll:read", "payroll:approve",
		},
		"finance_staff": {
			"accounts:read",
			"transactions:read", "transactions:create", "transactions:update",
			"sales:read", "sales:create", "sales:update",
			"purchases:read", "purchases:create", "purchases:update",
			"reports:read",
			"contacts:read",
			"assets:read",
			"boms:read",
			"projects:read", "projects:create", "projects:update",
			"purchase_requests:read", "purchase_requests:create", "purchase_requests:update", "purchase_requests:approve",
			"payroll:read",
		},
		"accounting": {
			"accounts:read",
			"transactions:read", "transactions:create", "transactions:update",
			"sales:read", "sales:create", "sales:update",
			"purchases:read", "purchases:create", "purchases:update",
			"reports:read",
			"contacts:read",
			"assets:read",
			"boms:read",
			"projects:read", "projects:create", "projects:update",
			"purchase_requests:read", "purchase_requests:create", "purchase_requests:update", "purchase_requests:approve",
			"payroll:read",
		},
		"hr": {
			"payroll:read", "payroll:create", "payroll:update",
		},
		"director": {
			"users:read",
			"accounts:read",
			"transactions:read",
			"products:read",
			"sales:read",
			"purchases:read",
			"reports:read", "reports:create",
			"contacts:read",
			"assets:read",
			"budgets:read", "budgets:create", "budgets:update",
			"boms:read",
			"projects:read",
			"purchase_requests:read", "purchase_requests:approve",
		},
		"inventory_manager": {
			// Core inventory modules
			"products:read", "products:create", "products:update",
			"sales:read", "sales:create", "sales:update",
			"purchases:read", "purchases:create", "purchases:update",
			"contacts:read", "contacts:create", "contacts:update",
			// Supporting modules and reporting access
			"assets:read", "assets:create", "assets:update",
			"accounts:read",
			"reports:read",
			"boms:read", "boms:create", "boms:update", "boms:delete",
			"projects:read",
			"purchase_requests:read",
		},
		"employee": {
			// Align employee defaults with module-level Manage Permissions
			"products:read", "products:create",
			"sales:read", "sales:create",
			"contacts:read",
			// Needed for forms and lookups
			"accounts:read",
			// Allow employees to create and edit their purchase requests
			"purchases:read", "purchases:create", "purchases:update",
			"boms:read",
			"purchase_requests:read", "purchase_requests:create", "purchase_requests:update",
		},
	}

	// Create role permissions
	for role, perms := range rolePermissions {
		for _, permName := range perms {
			if permID, exists := permissionMap[permName]; exists {
				var rp models.RolePermission
				db.Where("role = ? AND permission_id = ?", role, permID).FirstOrCreate(&rp, models.RolePermission{
					Role:         role,
					PermissionID: permID,
				})
			}
		}
	}
}

func seedProductUnits(db *gorm.DB) {
	// Check if product units already exist
	var count int64
	db.Model(&models.ProductUnit{}).Count(&count)
	if count > 0 {
		return
	}

	units := []models.ProductUnit{
		// Count units
		{Code: "pcs", Name: "Pieces", Symbol: "pcs", Type: models.UnitTypeCount, Description: "Individual pieces or items", IsActive: true},
		{Code: "unit", Name: "Unit", Symbol: "unit", Type: models.UnitTypeCount, Description: "Single unit of item", IsActive: true},
		{Code: "set", Name: "Set", Symbol: "set", Type: models.UnitTypeCount, Description: "Set of items sold together", IsActive: true},
		{Code: "pair", Name: "Pair", Symbol: "pr", Type: models.UnitTypeCount, Description: "Pair of items (2 pieces)", IsActive: true},
		{Code: "dozen", Name: "Dozen", Symbol: "dz", Type: models.UnitTypeCount, Description: "12 pieces", IsActive: true},

		// Weight units
		{Code: "kg", Name: "Kilogram", Symbol: "kg", Type: models.UnitTypeWeight, Description: "Weight in kilograms", IsActive: true},
		{Code: "g", Name: "Gram", Symbol: "g", Type: models.UnitTypeWeight, Description: "Weight in grams", IsActive: true},
		{Code: "ton", Name: "Ton", Symbol: "t", Type: models.UnitTypeWeight, Description: "Weight in metric tons", IsActive: true},

		// Volume units
		{Code: "liter", Name: "Liter", Symbol: "L", Type: models.UnitTypeVolume, Description: "Volume in liters", IsActive: true},
		{Code: "ml", Name: "Milliliter", Symbol: "mL", Type: models.UnitTypeVolume, Description: "Volume in milliliters", IsActive: true},

		// Length units
		{Code: "m", Name: "Meter", Symbol: "m", Type: models.UnitTypeLength, Description: "Length in meters", IsActive: true},
		{Code: "cm", Name: "Centimeter", Symbol: "cm", Type: models.UnitTypeLength, Description: "Length in centimeters", IsActive: true},
		{Code: "mm", Name: "Millimeter", Symbol: "mm", Type: models.UnitTypeLength, Description: "Length in millimeters", IsActive: true},

		// Area units
		{Code: "m2", Name: "Square Meter", Symbol: "m²", Type: models.UnitTypeArea, Description: "Area in square meters", IsActive: true},

		// Office/stationery units
		{Code: "rim", Name: "Rim", Symbol: "rim", Type: models.UnitTypeCount, Description: "500 sheets of paper", IsActive: true},
		{Code: "box", Name: "Box", Symbol: "box", Type: models.UnitTypeCount, Description: "Box packaging", IsActive: true},
		{Code: "pack", Name: "Pack", Symbol: "pack", Type: models.UnitTypeCount, Description: "Package of items", IsActive: true},

		// Service units
		{Code: "hour", Name: "Hour", Symbol: "hr", Type: models.UnitTypeTime, Description: "Service time in hours", IsActive: true},
		{Code: "day", Name: "Day", Symbol: "day", Type: models.UnitTypeTime, Description: "Service time in days", IsActive: true},
		{Code: "month", Name: "Month", Symbol: "mo", Type: models.UnitTypeTime, Description: "Service time in months", IsActive: true},
	}

	for _, unit := range units {
		db.Create(&unit)
	}
}

// seedApprovalWorkflows creates default PURCHASE workflows if none exist
func seedApprovalWorkflows(db *gorm.DB) {
	var count int64
	db.Model(&models.ApprovalWorkflow{}).Where("module = ?", models.ApprovalModulePurchase).Count(&count)
	if count > 0 {
		return
	}

	// Tier A: 0 - 25,000,000 (Finance only)
	wfA := models.ApprovalWorkflow{
		Name:            "PO <= 25M",
		Module:          models.ApprovalModulePurchase,
		MinAmount:       0,
		MaxAmount:       25000000,
		IsActive:        true,
		RequireFinance:  true,
		RequireDirector: false,
	}
	db.Create(&wfA)
	stepA1 := models.ApprovalStep{WorkflowID: wfA.ID, StepOrder: 1, StepName: "Finance Approval", ApproverRole: "finance"}
	db.Create(&stepA1)

	// Tier B: >25,000,000 - 100,000,000 (Finance -> Director)
	wfB := models.ApprovalWorkflow{
		Name:            "PO > 25M - 100M",
		Module:          models.ApprovalModulePurchase,
		MinAmount:       25000000.01,
		MaxAmount:       100000000,
		IsActive:        true,
		RequireFinance:  true,
		RequireDirector: true,
	}
	db.Create(&wfB)
	stepB1 := models.ApprovalStep{WorkflowID: wfB.ID, StepOrder: 1, StepName: "Finance Approval", ApproverRole: "finance"}
	stepB2 := models.ApprovalStep{WorkflowID: wfB.ID, StepOrder: 2, StepName: "Director Approval", ApproverRole: "director"}
	db.Create(&stepB1)
	db.Create(&stepB2)

	// Tier C: >100,000,000 (Finance -> Director)
	wfC := models.ApprovalWorkflow{
		Name:            "PO > 100M",
		Module:          models.ApprovalModulePurchase,
		MinAmount:       100000000.01,
		MaxAmount:       0, // no upper bound
		IsActive:        true,
		RequireFinance:  true,
		RequireDirector: true,
	}
	db.Create(&wfC)
	stepC1 := models.ApprovalStep{WorkflowID: wfC.ID, StepOrder: 1, StepName: "Finance Approval", ApproverRole: "finance"}
	stepC2 := models.ApprovalStep{WorkflowID: wfC.ID, StepOrder: 2, StepName: "Director Approval", ApproverRole: "director"}
	db.Create(&stepC1)
	db.Create(&stepC2)
}

// REMOVED: seedSampleSales creates sample sales data for dashboard analytics
// This function is disabled to prevent dummy data generation
/*
func seedSampleSales(db *gorm.DB) {
	// Check if specific sample sales already exist
	var sampleSalesExists int64
	db.Model(&models.Sale{}).Where("code IN ?", []string{"SAL-2024-001", "SAL-2024-002", "SAL-2024-003"}).Count(&sampleSalesExists)
	if sampleSalesExists > 0 {
		log.Printf("Sample sales already exist (%d records), skipping seed", sampleSalesExists)
		return
	}

	// Get first customer and product for relations
	var customer models.Contact
	db.Where("type = ?", models.ContactTypeCustomer).First(&customer)

	var product models.Product
	db.Unscoped().First(&product)

	// Get admin user
	var user models.User
	db.Where("role = ?", "admin").First(&user)

	if customer.ID == 0 || product.ID == 0 || user.ID == 0 {
		log.Println("Required data not found for seeding sales")
		return
	}

	// Create sample sales with varied dates for analytics
	sales := []models.Sale{
		{
			Code:            "SAL-2024-001",
			Type:            models.SaleTypeInvoice,
			CustomerID:      customer.ID,
			UserID:          user.ID,
			Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			DueDate:        time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
			InvoiceNumber:  "INV-2024-001",
			Subtotal:       15000000,
			DiscountPercent: 5,
			DiscountAmount: 750000,
			PPNPercent:     11,
			TaxableAmount:  14250000,
			PPN:            1567500,
			TotalTax:       1567500,
			TotalAmount:    15817500,
			PaidAmount:     15817500,
			OutstandingAmount: 0,
			Status:         models.SaleStatusPaid,
			Notes:          "Sample sales transaction",
		},
		{
			Code:            "SAL-2024-002",
			Type:            models.SaleTypeInvoice,
			CustomerID:      customer.ID,
			UserID:          user.ID,
			Date:           time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC),
			DueDate:        time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC),
			InvoiceNumber:  "INV-2024-002",
			Subtotal:       8500000,
			DiscountPercent: 0,
			DiscountAmount: 0,
			PPNPercent:     11,
			TaxableAmount:  8500000,
			PPN:            935000,
			TotalTax:       935000,
			TotalAmount:    9435000,
			PaidAmount:     0,
			OutstandingAmount: 9435000,
			Status:         models.SaleStatusInvoiced,
			Notes:          "Pending payment",
		},
		{
			Code:            "SAL-2024-003",
			Type:            models.SaleTypeInvoice,
			CustomerID:      customer.ID,
			UserID:          user.ID,
			Date:           time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
			DueDate:        time.Date(2024, 4, 5, 0, 0, 0, 0, time.UTC),
			InvoiceNumber:  "INV-2024-003",
			Subtotal:       12000000,
			DiscountPercent: 3,
			DiscountAmount: 360000,
			PPNPercent:     11,
			TaxableAmount:  11640000,
			PPN:            1280400,
			TotalTax:       1280400,
			TotalAmount:    12920400,
			PaidAmount:     12920400,
			OutstandingAmount: 0,
			Status:         models.SaleStatusPaid,
			Notes:          "Fully paid",
		},
	}

	for _, sale := range sales {
		db.Create(&sale)

		// Create sample sale items for each sale
		saleItem := models.SaleItem{
			SaleID:        sale.ID,
			ProductID:     product.ID,
			Quantity:      2,
			UnitPrice:     product.SalePrice,
			LineTotal:     2 * product.SalePrice,
			Taxable:       true,
			FinalAmount:   2 * product.SalePrice,
		}
		db.Create(&saleItem)
	}
}
*/

// REMOVED: seedSamplePurchases creates sample purchases data for dashboard analytics
// This function is disabled to prevent dummy data generation
/*
func seedSamplePurchases(db *gorm.DB) {
	// Check if specific sample purchases already exist
	var samplePurchasesExists int64
	db.Model(&models.Purchase{}).Where("code IN ?", []string{"PUR-2024-001", "PUR-2024-002", "PUR-2024-003"}).Count(&samplePurchasesExists)
	if samplePurchasesExists > 0 {
		log.Printf("Sample purchases already exist (%d records), skipping seed", samplePurchasesExists)
		return
	}

	// Get first vendor and product for relations
	var vendor models.Contact
	db.Where("type = ?", models.ContactTypeVendor).First(&vendor)

	var product models.Product
	db.Unscoped().First(&product)

	// Get admin user
	var user models.User
	db.Where("role = ?", "admin").First(&user)

	if vendor.ID == 0 || product.ID == 0 || user.ID == 0 {
		log.Println("Required data not found for seeding purchases")
		return
	}

	// Create sample purchases with varied dates for analytics
	purchases := []models.Purchase{
		{
			Code:                   "PUR-2024-001",
			VendorID:               vendor.ID,
			UserID:                 user.ID,
			Date:                   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			DueDate:                time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC),
			SubtotalBeforeDiscount: 10000000,
			Discount:               2,
			OrderDiscountAmount:    200000,
			NetBeforeTax:           9800000,
			PPNRate:                11,
			PPNAmount:              1078000,
			TotalTaxAdditions:      1078000,
			TotalAmount:            10878000,
			PaidAmount:             10878000,
			OutstandingAmount:      0,
			Status:                 models.PurchaseStatusCompleted,
			Notes:                  "Sample purchase transaction",
		},
		{
			Code:                   "PUR-2024-002",
			VendorID:               vendor.ID,
			UserID:                 user.ID,
			Date:                   time.Date(2024, 2, 5, 0, 0, 0, 0, time.UTC),
			DueDate:                time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
			SubtotalBeforeDiscount: 6500000,
			Discount:               0,
			OrderDiscountAmount:    0,
			NetBeforeTax:           6500000,
			PPNRate:                11,
			PPNAmount:              715000,
			TotalTaxAdditions:      715000,
			TotalAmount:            7215000,
			PaidAmount:             0,
			OutstandingAmount:      7215000,
			Status:                 models.PurchaseStatusApproved,
			Notes:                  "Pending payment",
		},
		{
			Code:                   "PUR-2024-003",
			VendorID:               vendor.ID,
			UserID:                 user.ID,
			Date:                   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			DueDate:                time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
			SubtotalBeforeDiscount: 8000000,
			Discount:               5,
			OrderDiscountAmount:    400000,
			NetBeforeTax:           7600000,
			PPNRate:                11,
			PPNAmount:              836000,
			TotalTaxAdditions:      836000,
			TotalAmount:            8436000,
			PaidAmount:             8436000,
			OutstandingAmount:      0,
			Status:                 models.PurchaseStatusCompleted,
			Notes:                  "Fully paid",
		},
	}

	for _, purchase := range purchases {
		db.Create(&purchase)

		// Create sample purchase items for each purchase
		purchaseItem := models.PurchaseItem{
			PurchaseID:    purchase.ID,
			ProductID:     product.ID,
			Quantity:      3,
			UnitPrice:     product.PurchasePrice,
			TotalPrice:    3 * product.PurchasePrice,
		}
		db.Create(&purchaseItem)
	}
}
*/
