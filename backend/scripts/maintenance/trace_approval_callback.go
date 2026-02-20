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
	
	fmt.Println("🔍 Tracing approval workflow and callback execution...")
	
	// 1. Check approval_actions for purchases 2 and 3
	fmt.Println("\n1. Approval actions for purchases 2 and 3:")
	
	type ApprovalAction struct {
		ID uint `json:"id"`
		RequestID uint `json:"request_id"`
		StepID uint `json:"step_id"`
		StepName string `json:"step_name"`
		ApproverRole string `json:"approver_role"`
		Status string `json:"status"`
		IsActive bool `json:"is_active"`
		ApproverID *uint `json:"approver_id"`
		ActionDate *string `json:"action_date"`
		Comments string `json:"comments"`
	}
	
	var actions []ApprovalAction
	err := db.Raw(`
		SELECT 
			aa.id, aa.request_id, aa.step_id, 
			astp.step_name, astp.approver_role,
			aa.status, aa.is_active, aa.approver_id, aa.action_date, aa.comments
		FROM approval_actions aa
		JOIN approval_steps astp ON aa.step_id = astp.id
		JOIN approval_requests ar ON aa.request_id = ar.id
		JOIN purchases p ON ar.entity_id = p.id
		WHERE p.id IN (2, 3) AND ar.entity_type = 'PURCHASE'
		ORDER BY p.id, astp.step_order
	`).Scan(&actions).Error
	
	if err != nil {
		log.Printf("Error querying approval actions: %v", err)
		return
	}
	
	for _, action := range actions {
		approverID := "NULL"
		if action.ApproverID != nil {
			approverID = fmt.Sprintf("%d", *action.ApproverID)
		}
		actionDate := "NULL"
		if action.ActionDate != nil {
			actionDate = *action.ActionDate
		}
		fmt.Printf("  Request %d, Step: %s (%s), Status: %s, Active: %t, Approver: %s, Date: %s\n",
			action.RequestID, action.StepName, action.ApproverRole, action.Status, action.IsActive, approverID, actionDate)
	}
	
	// 2. Check approval_requests completion status
	fmt.Println("\n2. Approval requests completion:")
	type ApprovalRequest struct {
		ID uint `json:"id"`
		RequestCode string `json:"request_code"`
		EntityID uint `json:"entity_id"`
		Status string `json:"status"`
		CreatedAt string `json:"created_at"`
		CompletedAt *string `json:"completed_at"`
	}
	
	var requests []ApprovalRequest
	err = db.Raw(`
		SELECT 
			ar.id, ar.request_code, ar.entity_id, ar.status,
			ar.created_at, ar.completed_at
		FROM approval_requests ar
		JOIN purchases p ON ar.entity_id = p.id
		WHERE p.id IN (2, 3) AND ar.entity_type = 'PURCHASE'
		ORDER BY p.id
	`).Scan(&requests).Error
	
	if err != nil {
		log.Printf("Error querying approval requests: %v", err)
		return
	}
	
	for _, req := range requests {
		completedAt := "NULL"
		if req.CompletedAt != nil {
			completedAt = *req.CompletedAt
		}
		fmt.Printf("  Purchase %d: Request %s, Status: %s, Completed: %s\n",
			req.EntityID, req.RequestCode, req.Status, completedAt)
	}
	
	// 3. Check approval_histories to see the flow
	fmt.Println("\n3. Approval history flow:")
	type ApprovalHistory struct {
		RequestID uint `json:"request_id"`
		PurchaseID uint `json:"purchase_id"`
		Username string `json:"username"`
		Action string `json:"action"`
		Comments string `json:"comments"`
		CreatedAt string `json:"created_at"`
	}
	
	var histories []ApprovalHistory
	err = db.Raw(`
		SELECT 
			ah.request_id, ar.entity_id as purchase_id,
			u.username, ah.action, ah.comments, ah.created_at
		FROM approval_histories ah
		JOIN approval_requests ar ON ah.request_id = ar.id
		JOIN users u ON ah.user_id = u.id
		WHERE ar.entity_id IN (2, 3) AND ar.entity_type = 'PURCHASE'
		ORDER BY ar.entity_id, ah.created_at
	`).Scan(&histories).Error
	
	if err != nil {
		log.Printf("Error querying approval histories: %v", err)
		return
	}
	
	currentPurchase := uint(0)
	for _, hist := range histories {
		if hist.PurchaseID != currentPurchase {
			fmt.Printf("\n  Purchase %d approval flow:\n", hist.PurchaseID)
			currentPurchase = hist.PurchaseID
		}
		fmt.Printf("    %s: %s by %s - %s\n", hist.CreatedAt, hist.Action, hist.Username, hist.Comments)
	}
	
	// 4. Check if there's any evidence of OnPurchaseApproved being called
	fmt.Println("\n4. Checking evidence of post-approval processing:")
	
	// Check journal entries created for these purchases
	var journalCount int64
	db.Raw(`
		SELECT COUNT(*) FROM journal_entries 
		WHERE reference_type = 'PURCHASE' AND reference_id::text IN ('2', '3')
	`).Scan(&journalCount)
	
	fmt.Printf("  Journal entries for purchases 2,3: %d\n", journalCount)
	
	// Check if SSOT journal entries exist
	var ssotJournalCount int64
	db.Raw(`
		SELECT COUNT(*) FROM unified_journal_ledger 
		WHERE reference LIKE '%PO/2025/10/001%'
	`).Scan(&ssotJournalCount)
	
	fmt.Printf("  SSOT journal entries for purchases 2,3: %d\n", ssotJournalCount)
	
	// 5. The key question: Why is OnPurchaseApproved not being called?
	fmt.Println("\n🔍 ANALYSIS:")
	
	allApproved := true
	for _, req := range requests {
		if req.Status != "APPROVED" || req.CompletedAt == nil {
			allApproved = false
			break
		}
	}
	
	if allApproved {
		fmt.Println("  ✅ All approval requests are APPROVED and completed")
		fmt.Println("  ❌ BUT OnPurchaseApproved() callback was NOT triggered")
		fmt.Println("  💡 Possible causes:")
		fmt.Println("    1. Callback not set up properly in approval service")
		fmt.Println("    2. Callback execution failed silently") 
		fmt.Println("    3. Approval completion process bypassed the callback")
	} else {
		fmt.Println("  ❌ Some approval requests are not properly completed")
	}
	
	fmt.Println("\n📋 NEXT ACTION: Check callback setup and test manual trigger...")
}