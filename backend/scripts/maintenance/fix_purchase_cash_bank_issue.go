package main

import (
	"fmt"
	"log"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔍 Investigating cash & bank balance issue for purchase...")
	
	// 1. Check recent purchases with immediate payment methods
	fmt.Println("\n1. Recent immediate payment purchases:")
	type PurchaseInfo struct {
		ID uint `json:"id"`
		Code string `json:"code"`
		PaymentMethod string `json:"payment_method"`
		BankAccountID *uint `json:"bank_account_id"`
		TotalAmount float64 `json:"total_amount"`
		Status string `json:"status"`
		ApprovalStatus string `json:"approval_status"`
		CreatedAt string `json:"created_at"`
		ApprovedAt *string `json:"approved_at"`
	}
	
	var purchases []PurchaseInfo
	err := db.Raw(`
		SELECT id, code, payment_method, bank_account_id, total_amount, 
		       status, approval_status, created_at, approved_at
		FROM purchases 
		WHERE payment_method IN ('CASH', 'BANK_TRANSFER', 'CHECK')
		ORDER BY created_at DESC 
		LIMIT 5
	`).Scan(&purchases).Error
	
	if err != nil {
		log.Printf("Error querying purchases: %v", err)
		return
	}
	
	for _, p := range purchases {
		bankID := "NULL"
		if p.BankAccountID != nil {
			bankID = fmt.Sprintf("%d", *p.BankAccountID)
		}
		approvedAt := "NULL"
		if p.ApprovedAt != nil {
			approvedAt = *p.ApprovedAt
		}
		fmt.Printf("  ID: %d, Code: %s, Method: %s, Bank: %s, Amount: %.2f, Status: %s, Approval: %s, Approved: %s\n",
			p.ID, p.Code, p.PaymentMethod, bankID, p.TotalAmount, p.Status, p.ApprovalStatus, approvedAt)
	}
	
	// 2. Check approval requests for these purchases
	fmt.Println("\n2. Approval requests for immediate payment purchases:")
	type ApprovalInfo struct {
		PurchaseID uint `json:"purchase_id"`
		PurchaseCode string `json:"purchase_code"`
		RequestID uint `json:"request_id"`
		RequestCode string `json:"request_code"`
		Status string `json:"status"`
		CreatedAt string `json:"created_at"`
		CompletedAt *string `json:"completed_at"`
	}
	
	var approvals []ApprovalInfo
	err = db.Raw(`
		SELECT 
			p.id as purchase_id,
			p.code as purchase_code,
			ar.id as request_id,
			ar.request_code,
			ar.status,
			ar.created_at,
			ar.completed_at
		FROM purchases p
		LEFT JOIN approval_requests ar ON p.approval_request_id = ar.id
		WHERE p.payment_method IN ('CASH', 'BANK_TRANSFER', 'CHECK')
		ORDER BY p.created_at DESC 
		LIMIT 5
	`).Scan(&approvals).Error
	
	if err != nil {
		log.Printf("Error querying approvals: %v", err)
		return
	}
	
	for _, a := range approvals {
		completedAt := "NULL"
		if a.CompletedAt != nil {
			completedAt = *a.CompletedAt
		}
		fmt.Printf("  Purchase ID: %d (%s), Request ID: %d, Code: %s, Status: %s, Completed: %s\n",
			a.PurchaseID, a.PurchaseCode, a.RequestID, a.RequestCode, a.Status, completedAt)
	}
	
	// 3. Check if approval workflows exist
	fmt.Println("\n3. Checking approval workflows:")
	type WorkflowInfo struct {
		ID uint `json:"id"`
		Name string `json:"name"`
		Module string `json:"module"`
		MinAmount float64 `json:"min_amount"`
		MaxAmount float64 `json:"max_amount"`
		IsActive bool `json:"is_active"`
		StepCount int `json:"step_count"`
	}
	
	var workflows []WorkflowInfo
	err = db.Raw(`
		SELECT 
			w.id, w.name, w.module, w.min_amount, w.max_amount, w.is_active,
			COUNT(s.id) as step_count
		FROM approval_workflows w
		LEFT JOIN approval_workflow_steps s ON w.id = s.workflow_id
		WHERE w.module = 'PURCHASE' AND w.is_active = true
		GROUP BY w.id, w.name, w.module, w.min_amount, w.max_amount, w.is_active
	`).Scan(&workflows).Error
	
	if err != nil {
		log.Printf("Error querying workflows: %v", err)
		return
	}
	
	if len(workflows) == 0 {
		fmt.Println("  ❌ No active purchase approval workflows found!")
		fmt.Println("  💡 This is likely the root cause - running workflow creation...")
		
		// Try to create the workflow using the same logic from auto_migrations.go
		err = createStandardPurchaseApprovalWorkflow(db)
		if err != nil {
			log.Printf("Error creating workflow: %v", err)
			return
		}
		fmt.Println("  ✅ Created Standard Purchase Approval workflow")
		
		// Re-query workflows
		err = db.Raw(`
			SELECT 
				w.id, w.name, w.module, w.min_amount, w.max_amount, w.is_active,
				COUNT(s.id) as step_count
			FROM approval_workflows w
			LEFT JOIN approval_workflow_steps s ON w.id = s.workflow_id
			WHERE w.module = 'PURCHASE' AND w.is_active = true
			GROUP BY w.id, w.name, w.module, w.min_amount, w.max_amount, w.is_active
		`).Scan(&workflows).Error
		
		if err == nil {
			fmt.Println("  📋 Updated workflows:")
			for _, w := range workflows {
				fmt.Printf("    ID: %d, Name: %s, Steps: %d, Range: %.0f-%.0f\n",
					w.ID, w.Name, w.StepCount, w.MinAmount, w.MaxAmount)
			}
		}
	} else {
		for _, w := range workflows {
			fmt.Printf("  ID: %d, Name: %s, Steps: %d, Range: %.0f-%.0f, Active: %t\n",
				w.ID, w.Name, w.StepCount, w.MinAmount, w.MaxAmount, w.IsActive)
		}
	}
	
	// 4. Check cash_bank_transactions for immediate payment purchases
	fmt.Println("\n4. Cash & Bank transactions for recent purchases:")
	type TransactionInfo struct {
		ID uint `json:"id"`
		CashBankName string `json:"cash_bank_name"`
		ReferenceType string `json:"reference_type"`
		ReferenceID uint `json:"reference_id"`
		Amount float64 `json:"amount"`
		BalanceAfter float64 `json:"balance_after"`
		TransactionDate string `json:"transaction_date"`
		Notes string `json:"notes"`
	}
	
	var transactions []TransactionInfo
	err = db.Raw(`
		SELECT 
			cbt.id,
			cb.name as cash_bank_name,
			cbt.reference_type,
			cbt.reference_id,
			cbt.amount,
			cbt.balance_after,
			cbt.transaction_date,
			cbt.notes
		FROM cash_bank_transactions cbt
		JOIN cash_banks cb ON cbt.cash_bank_id = cb.id
		WHERE cbt.reference_type = 'PURCHASE'
		ORDER BY cbt.transaction_date DESC
		LIMIT 10
	`).Scan(&transactions).Error
	
	if err != nil {
		log.Printf("Error querying transactions: %v", err)
		return
	}
	
	if len(transactions) == 0 {
		fmt.Println("  ❌ No cash & bank transactions found for purchases!")
		fmt.Println("  💡 This confirms the issue - cash & bank balance is not being updated")
	} else {
		for _, t := range transactions {
			fmt.Printf("  %s: Ref %s/%d, Amount: %.2f, Balance After: %.2f\n",
				t.CashBankName, t.ReferenceType, t.ReferenceID, t.Amount, t.BalanceAfter)
		}
	}
	
	// 5. Check current cash & bank balances
	fmt.Println("\n5. Current cash & bank balances:")
	type BalanceInfo struct {
		ID uint `json:"id"`
		Name string `json:"name"`
		Balance float64 `json:"balance"`
		UpdatedAt string `json:"updated_at"`
	}
	
	var balances []BalanceInfo
	err = db.Raw(`
		SELECT id, name, balance, updated_at
		FROM cash_banks 
		ORDER BY updated_at DESC
		LIMIT 5
	`).Scan(&balances).Error
	
	if err != nil {
		log.Printf("Error querying balances: %v", err)
		return
	}
	
	for _, b := range balances {
		fmt.Printf("  %s (ID: %d): Balance %.2f, Updated: %s\n",
			b.Name, b.ID, b.Balance, b.UpdatedAt)
	}
	
	// Summary and recommendations
	fmt.Println("\n📋 SUMMARY & RECOMMENDATIONS:")
	
	// Check if we found purchases that should have updated cash balance but didn't
	hasImmediatePurchases := false
	hasApprovedImmediatePurchases := false
	for _, p := range purchases {
		if p.PaymentMethod == "CASH" || p.PaymentMethod == "BANK_TRANSFER" || p.PaymentMethod == "CHECK" {
			hasImmediatePurchases = true
			if p.Status == "APPROVED" {
				hasApprovedImmediatePurchases = true
			}
		}
	}
	
	if !hasImmediatePurchases {
		fmt.Println("  ℹ️  No recent immediate payment purchases found")
	} else if !hasApprovedImmediatePurchases {
		fmt.Println("  ⚠️  Found immediate payment purchases but none are APPROVED")
		fmt.Println("  💡 Purchases need to go through approval workflow first")
		fmt.Println("  🔧 ACTION: Submit purchases for approval via /api/v1/purchases/{id}/submit-approval")
	} else if len(transactions) == 0 {
		fmt.Println("  ❌ Found APPROVED immediate payment purchases but no cash/bank transactions")
		fmt.Println("  💡 This indicates OnPurchaseApproved callback is not being triggered")
		fmt.Println("  🔧 ACTION: Verify approval workflow completion triggers the callback")
	}
	
	if len(workflows) == 0 {
		fmt.Println("  ❌ No approval workflows configured - this will prevent purchases from being approved")
		fmt.Println("  ✅ We've created the workflow above - try creating a new purchase now")
	}
	
	fmt.Println("\n✅ Investigation completed. Check the issues above and follow the recommended actions.")
}

// createStandardPurchaseApprovalWorkflow creates the standard purchase approval workflow
func createStandardPurchaseApprovalWorkflow(db *gorm.DB) error {
	// This mimics the logic from database/auto_migrations.go
	
	type ApprovalWorkflow struct {
		ID              uint    `gorm:"primaryKey"`
		Name            string  `gorm:"not null;size:100"`
		Module          string  `gorm:"not null;size:50"`
		MinAmount       float64 `gorm:"type:decimal(15,2);default:0"`
		MaxAmount       float64 `gorm:"type:decimal(15,2)"`
		IsActive        bool    `gorm:"default:true"`
		RequireDirector bool    `gorm:"default:false"`
		RequireFinance  bool    `gorm:"default:false"`
	}
	
	type ApprovalStep struct {
		ID           uint   `gorm:"primaryKey"`
		WorkflowID   uint   `gorm:"not null;index"`
		StepOrder    int    `gorm:"not null"`
		StepName     string `gorm:"not null;size:100"`
		ApproverRole string `gorm:"not null;size:50"`
		IsOptional   bool   `gorm:"default:false"`
		TimeLimit    int    `gorm:"default:24"`
	}

	// Check if workflow already exists
	var existingWorkflow ApprovalWorkflow
	result := db.Where("name = ? AND module = ?", "Standard Purchase Approval", "PURCHASE").First(&existingWorkflow)
	
	if result.Error == nil {
		return fmt.Errorf("workflow already exists")
	}
	
	// Create workflow
	workflow := ApprovalWorkflow{
		Name:            "Standard Purchase Approval",
		Module:          "PURCHASE", 
		MinAmount:       0,
		MaxAmount:       999999999999,
		IsActive:        true,
		RequireDirector: true,
		RequireFinance:  true,
	}
	
	if err := db.Create(&workflow).Error; err != nil {
		return fmt.Errorf("failed to create workflow: %v", err)
	}
	
	// Create workflow steps
	steps := []ApprovalStep{
		{
			WorkflowID:   workflow.ID,
			StepOrder:    1,
			StepName:     "Employee Submission",
			ApproverRole: "employee",
			IsOptional:   false,
			TimeLimit:    24,
		},
		{
			WorkflowID:   workflow.ID,
			StepOrder:    2, 
			StepName:     "Finance Approval",
			ApproverRole: "finance",
			IsOptional:   false,
			TimeLimit:    48,
		},
		{
			WorkflowID:   workflow.ID,
			StepOrder:    3,
			StepName:     "Director Approval", 
			ApproverRole: "director",
			IsOptional:   true,
			TimeLimit:    72,
		},
	}
	
	for _, step := range steps {
		if err := db.Create(&step).Error; err != nil {
			return fmt.Errorf("failed to create step '%s': %v", step.StepName, err)
		}
	}
	
	return nil
}