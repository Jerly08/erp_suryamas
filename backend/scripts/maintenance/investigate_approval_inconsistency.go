package main

import (
	"fmt"
	"log"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
)

func main() {
	// Load configuration  
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔍 Deep investigation of approval workflow inconsistency...")
	
	// 1. Check the exact inconsistency: approval_requests vs approval_actions
	fmt.Println("\n1. INCONSISTENCY CHECK - Approval Requests vs Actions:")
	
	type InconsistentData struct {
		PurchaseID uint `json:"purchase_id"`
		PurchaseCode string `json:"purchase_code"`
		RequestID uint `json:"request_id"`
		RequestStatus string `json:"request_status"`
		CompletedAt *string `json:"completed_at"`
		FinanceActionStatus string `json:"finance_action_status"`
		FinanceApproverID *uint `json:"finance_approver_id"`
		FinanceActionDate *string `json:"finance_action_date"`
		DirectorActionStatus string `json:"director_action_status"`
	}
	
	var inconsistencies []InconsistentData
	err := db.Raw(`
		SELECT 
			p.id as purchase_id,
			p.code as purchase_code,
			ar.id as request_id,
			ar.status as request_status,
			ar.completed_at,
			fa.status as finance_action_status,
			fa.approver_id as finance_approver_id,
			fa.action_date as finance_action_date,
			da.status as director_action_status
		FROM purchases p
		JOIN approval_requests ar ON p.approval_request_id = ar.id
		LEFT JOIN approval_actions fa ON ar.id = fa.request_id 
			AND fa.step_id IN (SELECT id FROM approval_steps WHERE approver_role = 'finance')
		LEFT JOIN approval_actions da ON ar.id = da.request_id 
			AND da.step_id IN (SELECT id FROM approval_steps WHERE approver_role = 'director')
		WHERE p.id IN (2, 3) 
			AND ar.entity_type = 'PURCHASE'
		ORDER BY p.id
	`).Scan(&inconsistencies).Error
	
	if err != nil {
		log.Printf("Error investigating inconsistencies: %v", err)
		return
	}
	
	for _, inc := range inconsistencies {
		fmt.Printf("\n📋 Purchase %d (%s):\n", inc.PurchaseID, inc.PurchaseCode)
		fmt.Printf("  Request ID: %d, Status: %s\n", inc.RequestID, inc.RequestStatus)
		
		completedAt := "NULL"
		if inc.CompletedAt != nil {
			completedAt = *inc.CompletedAt
		}
		fmt.Printf("  Completed At: %s\n", completedAt)
		fmt.Printf("  Finance Action: %s", inc.FinanceActionStatus)
		
		if inc.FinanceApproverID != nil {
			fmt.Printf(" (by user %d)", *inc.FinanceApproverID)
		}
		if inc.FinanceActionDate != nil {
			fmt.Printf(" at %s", *inc.FinanceActionDate)
		}
		fmt.Printf("\n")
		fmt.Printf("  Director Action: %s\n", inc.DirectorActionStatus)
		
		// Identify the inconsistency
		if inc.RequestStatus == "APPROVED" && inc.CompletedAt != nil && inc.FinanceActionStatus == "PENDING" {
			fmt.Printf("  🚨 INCONSISTENCY FOUND: Request marked APPROVED but Finance action still PENDING!\n")
		}
	}
	
	// 2. Check how approval completion process works
	fmt.Println("\n2. APPROVAL COMPLETION INVESTIGATION:")
	fmt.Println("   Let's trace how these requests got marked as APPROVED...")
	
	// Check approval histories to see what happened
	type ApprovalFlow struct {
		RequestID uint `json:"request_id"`
		PurchaseID uint `json:"purchase_id"`
		Action string `json:"action"`
		UserID uint `json:"user_id"`
		Username string `json:"username"`
		UserRole string `json:"user_role"`
		Comments string `json:"comments"`
		CreatedAt string `json:"created_at"`
	}
	
	var flows []ApprovalFlow
	err = db.Raw(`
		SELECT 
			ah.request_id, ar.entity_id as purchase_id,
			ah.action, ah.user_id, u.username, u.role as user_role,
			ah.comments, ah.created_at
		FROM approval_histories ah
		JOIN approval_requests ar ON ah.request_id = ar.id
		JOIN users u ON ah.user_id = u.id
		WHERE ar.entity_id IN (2, 3) AND ar.entity_type = 'PURCHASE'
		ORDER BY ar.entity_id, ah.created_at
	`).Scan(&flows).Error
	
	if err != nil {
		log.Printf("Error querying approval flows: %v", err)
		return
	}
	
	currentPurchase := uint(0)
	for _, flow := range flows {
		if flow.PurchaseID != currentPurchase {
			fmt.Printf("\n  Purchase %d approval history:\n", flow.PurchaseID)
			currentPurchase = flow.PurchaseID
		}
		fmt.Printf("    %s: %s by %s (%s, ID:%d) - %s\n", 
			flow.CreatedAt, flow.Action, flow.Username, flow.UserRole, flow.UserID, flow.Comments)
	}
	
	// 3. Check what triggers approval completion
	fmt.Println("\n3. APPROVAL COMPLETION TRIGGER ANALYSIS:")
	
	// Check if there are any approval actions that should have triggered completion but didn't
	var pendingActions []struct {
		RequestID uint `json:"request_id"`
		PurchaseID uint `json:"purchase_id"`
		StepName string `json:"step_name"`
		ApproverRole string `json:"approver_role"`
		Status string `json:"status"`
		IsActive bool `json:"is_active"`
		StepOrder int `json:"step_order"`
	}
	
	err = db.Raw(`
		SELECT 
			ar.id as request_id, ar.entity_id as purchase_id,
			astp.step_name, astp.approver_role, aa.status, aa.is_active, astp.step_order
		FROM approval_requests ar
		JOIN approval_actions aa ON ar.id = aa.request_id
		JOIN approval_steps astp ON aa.step_id = astp.id
		WHERE ar.entity_id IN (2, 3) 
			AND ar.entity_type = 'PURCHASE'
			AND ar.status = 'APPROVED'
		ORDER BY ar.entity_id, astp.step_order
	`).Scan(&pendingActions).Error
	
	if err != nil {
		log.Printf("Error querying pending actions: %v", err)
		return
	}
	
	fmt.Println("  Current state of approval actions for APPROVED requests:")
	currentRequest := uint(0)
	for _, action := range pendingActions {
		if action.RequestID != currentRequest {
			fmt.Printf("\n    Request %d (Purchase %d):\n", action.RequestID, action.PurchaseID)
			currentRequest = action.RequestID
		}
		fmt.Printf("      Step %d: %s (%s) - Status: %s, Active: %t\n", 
			action.StepOrder, action.StepName, action.ApproverRole, action.Status, action.IsActive)
	}
	
	// 4. Root cause analysis
	fmt.Println("\n4. ROOT CAUSE ANALYSIS:")
	
	for _, inc := range inconsistencies {
		if inc.RequestStatus == "APPROVED" && inc.CompletedAt != nil && inc.FinanceActionStatus == "PENDING" {
			fmt.Printf("\n🔍 Purchase %d Problem Analysis:\n", inc.PurchaseID)
			fmt.Printf("  ❌ approval_requests.status = 'APPROVED' (completed_at set)\n")
			fmt.Printf("  ❌ approval_actions finance step still 'PENDING'\n")
			fmt.Printf("  💡 This suggests approval completion bypassed proper workflow\n")
			fmt.Printf("  🔧 Likely cause: Manual approval or workflow shortcut\n")
		}
	}
	
	// 5. Check specific approval method used
	fmt.Println("\n5. APPROVAL METHOD INVESTIGATION:")
	fmt.Println("   Checking if purchases were approved via direct API vs workflow...")
	
	// Look for any direct purchase updates that might have bypassed workflow
	var directUpdates []struct {
		PurchaseID uint `json:"purchase_id"`
		LastDirectApprovalTime *string `json:"last_direct_update"`
	}
	
	// This is harder to track without audit logs, but let's see if purchase.approved_at 
	// is different from approval_requests.completed_at
	err = db.Raw(`
		SELECT 
			p.id as purchase_id,
			CASE WHEN p.approved_at != ar.completed_at THEN p.approved_at::text 
				ELSE NULL END as last_direct_update
		FROM purchases p
		JOIN approval_requests ar ON p.approval_request_id = ar.id
		WHERE p.id IN (2, 3) AND p.status = 'APPROVED'
	`).Scan(&directUpdates).Error
	
	if err == nil {
		for _, update := range directUpdates {
			if update.LastDirectApprovalTime != nil {
				fmt.Printf("  🚨 Purchase %d: Direct approval detected at %s\n", 
					update.PurchaseID, *update.LastDirectApprovalTime)
			} else {
				fmt.Printf("  ✅ Purchase %d: Approval times consistent\n", update.PurchaseID)
			}
		}
	}
	
	fmt.Println("\n📋 CONCLUSIONS & RECOMMENDATIONS:")
	fmt.Println("  1. The inconsistency shows approval_requests completed but approval_actions still pending")
	fmt.Println("  2. This happens when approval completion logic doesn't properly update all approval_actions") 
	fmt.Println("  3. The PostApprovalCallback is only triggered when approval_actions complete properly")
	fmt.Println("  4. Need to fix the approval completion process to be atomic and consistent")
	fmt.Println("\n🔧 NEXT: Examine approval service completion logic and fix the inconsistency")
}