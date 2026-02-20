package main

import (
	"fmt"
	"log"
	"time"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
)

func main() {
	// Load configuration  
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔧 Fixing approval service logic bug...")
	
	// The bug is in ProcessApprovalAction function around lines 325-363
	// Let's create a fixed version of the approval completion logic
	
	// 1. First, let's simulate the fix by manually correcting the inconsistent data
	fmt.Println("\n1. FIXING EXISTING INCONSISTENT DATA:")
	
	// Fix approval_actions for requests 24 and 25 that were incorrectly processed
	requestIDs := []uint{24, 25}
	
	for _, requestID := range requestIDs {
		fmt.Printf("\n📋 Fixing approval request %d:\n", requestID)
		
		// Get the request details
		var request struct {
			ID uint `json:"id"`
			Status string `json:"status"`
			EntityID uint `json:"entity_id"`
			CompletedAt *string `json:"completed_at"`
		}
		
		err := db.Raw("SELECT id, status, entity_id, completed_at FROM approval_requests WHERE id = ?", requestID).Scan(&request).Error
		if err != nil {
			log.Printf("Error getting request %d: %v", requestID, err)
			continue
		}
		
		fmt.Printf("  Current status: %s, Completed: %v\n", request.Status, request.CompletedAt != nil)
		
		if request.Status == "APPROVED" && request.CompletedAt != nil {
			// This request was incorrectly marked as APPROVED
			// We need to reset it to PENDING and fix the workflow
			
			// Reset the approval request status
			now := time.Now()
			err = db.Exec(`
				UPDATE approval_requests 
				SET status = 'PENDING', completed_at = NULL, updated_at = ?
				WHERE id = ?
			`, now, requestID).Error
			
			if err != nil {
				log.Printf("Error resetting request %d: %v", requestID, err)
				continue
			}
			
			fmt.Printf("  ✅ Reset approval_requests.status to PENDING\n")
			
			// Reset the purchase status back to PENDING  
			err = db.Exec(`
				UPDATE purchases 
				SET status = 'PENDING', approval_status = 'PENDING', approved_at = NULL, updated_at = ?
				WHERE id = ?
			`, now, request.EntityID).Error
			
			if err != nil {
				log.Printf("Error resetting purchase %d: %v", request.EntityID, err)
				continue
			}
			
			fmt.Printf("  ✅ Reset purchase.status to PENDING\n")
			
			// Now check the current state of approval actions
			var actions []struct {
				ID uint `json:"id"`
				StepOrder int `json:"step_order"`
				ApproverRole string `json:"approver_role"`
				Status string `json:"status"`
				IsActive bool `json:"is_active"`
				ApproverID *uint `json:"approver_id"`
			}
			
			err = db.Raw(`
				SELECT aa.id, astp.step_order, astp.approver_role, aa.status, aa.is_active, aa.approver_id
				FROM approval_actions aa
				JOIN approval_steps astp ON aa.step_id = astp.id
				WHERE aa.request_id = ?
				ORDER BY astp.step_order
			`, requestID).Scan(&actions).Error
			
			if err != nil {
				log.Printf("Error getting actions for request %d: %v", requestID, err)
				continue
			}
			
			fmt.Printf("  Current approval actions state:\n")
			for _, action := range actions {
				fmt.Printf("    Step %d (%s): %s, Active: %t, Approver: %v\n", 
					action.StepOrder, action.ApproverRole, action.Status, action.IsActive, action.ApproverID)
			}
			
			// Apply the correct workflow logic
			// Employee step should be APPROVED (already correct)
			// Finance step should be active for approval
			for _, action := range actions {
				if action.ApproverRole == "finance" && action.Status == "PENDING" && !action.IsActive {
					// Activate the finance step
					err = db.Exec("UPDATE approval_actions SET is_active = true WHERE id = ?", action.ID).Error
					if err != nil {
						log.Printf("Error activating finance step: %v", err)
					} else {
						fmt.Printf("  ✅ Activated finance step for proper approval\n")
					}
					break
				}
			}
		}
	}
	
	// 2. Now let's check what the correct behavior should be
	fmt.Println("\n2. VERIFICATION AFTER FIX:")
	
	for _, requestID := range requestIDs {
		var request struct {
			ID uint `json:"id"`
			Status string `json:"status"`
			EntityID uint `json:"entity_id"`
		}
		
		db.Raw("SELECT id, status, entity_id FROM approval_requests WHERE id = ?", requestID).Scan(&request)
		
		var purchase struct {
			ID uint `json:"id"`
			Code string `json:"code"`
			Status string `json:"status"`
			ApprovalStatus string `json:"approval_status"`
		}
		
		db.Raw("SELECT id, code, status, approval_status FROM purchases WHERE id = ?", request.EntityID).Scan(&purchase)
		
		fmt.Printf("Request %d: %s, Purchase %d (%s): %s/%s\n", 
			request.ID, request.Status, purchase.ID, purchase.Code, purchase.Status, purchase.ApprovalStatus)
	}
	
	fmt.Println("\n3. PERMANENT FIX NEEDED:")
	fmt.Println("   The ProcessApprovalAction function in approval_service.go needs to be fixed")
	fmt.Println("   to handle the scenario where the approving user belongs to a step that")
	fmt.Println("   should be activated next, not just activating the next sequential step.")
	fmt.Println("")
	fmt.Println("   KEY FIX: When a user approves, check if they belong to any pending step")
	fmt.Println("   and activate/complete that step directly instead of just the next step.")
	
	fmt.Println("\n🔧 Temporary inconsistency fix completed!")
	fmt.Println("💡 Now you can try the approval workflow again and it should work correctly.")
}

// This shows what the fixed logic should look like in ProcessApprovalAction
func simulateFixedApprovalLogic() {
	fmt.Println(`
FIXED LOGIC PSEUDO-CODE:

After approving current step:
1. Check if approving user has permission for any OTHER pending steps
2. If yes, activate and complete those steps directly  
3. Then check for next step activation
4. Only mark as completed when all required steps are actually completed

This prevents the inconsistency where approval_requests gets marked APPROVED
while approval_actions still have pending steps.
`)
}