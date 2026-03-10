package services

import (
	config2 "app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type PurchaseService struct {
	db              *gorm.DB
	purchaseRepo    *repositories.PurchaseRepository
	productRepo     *repositories.ProductRepository
	contactRepo     repositories.ContactRepository
	accountRepo     repositories.AccountRepository
	approvalService *ApprovalService
	journalService  JournalServiceInterface
	journalRepo     repositories.JournalEntryRepository
	pdfService      PDFServiceInterface
	// SSOT Journal Integration
	ssotJournalAdapter    *PurchaseSSOTJournalAdapter
	unifiedJournalService *UnifiedJournalService
	// Asset capitalization
	assetCapitalizationSvc *AssetCapitalizationService
	journalServiceSSOT     *PurchaseJournalServiceSSOT // NEW: unified_journal_ledger (for Balance Sheet)
	coaService             *COAService
}

type PurchaseResult struct {
	Data       []models.Purchase `json:"data"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalPages int               `json:"total_pages"`
}

func NewPurchaseService(
	db *gorm.DB,
	purchaseRepo *repositories.PurchaseRepository,
	productRepo *repositories.ProductRepository,
	contactRepo repositories.ContactRepository,
	accountRepo repositories.AccountRepository,
	approvalService *ApprovalService,
	journalService JournalServiceInterface,
	journalRepo repositories.JournalEntryRepository,
	pdfService PDFServiceInterface,
	unifiedJournalService *UnifiedJournalService,
	coaService *COAService,
	purchaseJournalServiceSSOT *PurchaseJournalServiceSSOT,
) *PurchaseService {
	// Initialize SSOT Journal Adapter (wired to unified journal service)
	// Use TaxAccountService so account mapping is flexible (no hardcoded codes)
	taxAccountService := NewTaxAccountService(db)
	ssotAdapter := NewPurchaseSSOTJournalAdapter(db, unifiedJournalService, accountRepo, taxAccountService)

	ps := &PurchaseService{
		db:                    db,
		purchaseRepo:          purchaseRepo,
		productRepo:           productRepo,
		contactRepo:           contactRepo,
		accountRepo:           accountRepo,
		approvalService:       approvalService,
		journalService:        journalService,
		journalRepo:           journalRepo,
		pdfService:            pdfService,
		unifiedJournalService: unifiedJournalService,
		ssotJournalAdapter:    ssotAdapter,
		journalServiceSSOT:    purchaseJournalServiceSSOT,
		coaService:            coaService,
	}
	// Asset capitalization service (reuses existing repos and unified journal)
	ps.assetCapitalizationSvc = NewAssetCapitalizationService(db, accountRepo, unifiedJournalService, journalRepo)

	// Setup post-approval callback
	if approvalService != nil {
		approvalService.SetPostApprovalCallback(ps)
		fmt.Printf("✅ Post-approval callback setup completed\n")
	}

	return ps
}

// Purchase CRUD Operations

func (s *PurchaseService) GetPurchases(filter models.PurchaseFilter) (*PurchaseResult, error) {
	fmt.Printf("ℹ Retrieving purchases with filter: Status=%s, VendorID=%d, Page=%d, Limit=%d\n",
		filter.Status, filter.VendorID, filter.Page, filter.Limit)
	purchases, total, err := s.purchaseRepo.FindWithFilter(filter)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve purchases: %v\n", err)
		return nil, fmt.Errorf("failed to retrieve purchases: %v", err)
	}
	fmt.Printf("✅ Retrieved %d purchases (total: %d)\n", len(purchases), total)

	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))

	return &PurchaseResult{
		Data:       purchases,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

func (s *PurchaseService) GetPurchaseByID(id uint) (*models.Purchase, error) {
	fmt.Printf("ℹ Retrieving purchase by ID: %d\n", id)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}
	fmt.Printf("✅ Purchase %d retrieved: %s (Status: %s)\n", id, purchase.Code, purchase.Status)
	return purchase, nil
}

func (s *PurchaseService) CreatePurchase(request models.PurchaseCreateRequest, userID uint) (*models.Purchase, error) {
	// Validate vendor exists
	vendor, err := s.contactRepo.GetByID(request.VendorID)
	if err != nil {
		fmt.Printf("❌ Vendor validation failed for ID %d: %v\n", request.VendorID, err)
		return nil, fmt.Errorf("vendor not found (ID: %d): %v", request.VendorID, err)
	}
	fmt.Printf("✅ Vendor validation passed: %s (ID: %d)\n", vendor.Name, vendor.ID)

	// Generate purchase code
	code, err := s.generatePurchaseCode()
	if err != nil {
		fmt.Printf("❌ Failed to generate purchase code for vendor %d: %v\n", request.VendorID, err)
		return nil, fmt.Errorf("failed to generate purchase code: %v", err)
	}

	fmt.Printf("ℹ Creating purchase entity with code %s for vendor %d\n", code, request.VendorID)
	// Create purchase entity
	purchase := &models.Purchase{
		Code:     code,
		VendorID: request.VendorID,
		UserID:   userID,
		Date:     request.Date,
		DueDate:  request.DueDate,
		Discount: request.Discount,
		// Payment method fields
		PaymentMethod:    getPaymentMethod(request.PaymentMethod),
		BankAccountID:    request.BankAccountID,
		CreditAccountID:  request.CreditAccountID,
		PaymentReference: request.PaymentReference,
		// Tax rates from request - use pointers to distinguish null vs zero
		PPNRate: func() float64 {
			if request.PPNRate == nil {
				return 11.0
			}
			return *request.PPNRate
		}(),
		OtherTaxAdditions:  request.OtherTaxAdditions,
		PPh21Rate:          getPPhRateFromPointer(request.PPh21Rate),
		PPh23Rate:          getPPhRateFromPointer(request.PPh23Rate),
		OtherTaxDeductions: request.OtherTaxDeductions,
		Status:             models.PurchaseStatusDraft,
		Notes:              request.Notes,
		ApprovalStatus:     models.PurchaseApprovalNotStarted,
		RequiresApproval:   false,
		// Initialize payment tracking fields
		PaidAmount:        0,
		OutstandingAmount: 0, // Will be set after total calculation
		MatchingStatus:    models.PurchaseMatchingPending,
	}

	// Assign PR and Project if linked
	purchase.PurchaseRequestID = request.PurchaseRequestID
	purchase.ProjectID = request.ProjectID

	// Calculate totals and create purchase items
	fmt.Printf("ℹ Calculating purchase totals for %d items\n", len(request.Items))
	err = s.calculatePurchaseTotals(purchase, request.Items)
	if err != nil {
		fmt.Printf("❌ Failed to calculate purchase totals: %v\n", err)
		return nil, fmt.Errorf("failed to calculate purchase totals: %v", err)
	}
	fmt.Printf("✅ Purchase totals calculated: Subtotal=%.2f, Tax=%.2f, Total=%.2f\n", purchase.SubtotalBeforeDiscount, purchase.TaxAmount, purchase.TotalAmount)

	// INVOICE MATCHING: HARD BLOCK VALIDATION
	// If this purchase is linked to a Purchase Request, validate against its estimated amount
	if purchase.PurchaseRequestID != nil {
		var pr models.PurchaseRequest
		if err := s.db.First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return nil, fmt.Errorf("linked purchase request not found: %v", err)
		}

		// Ensure the PR is approved
		if pr.Status != "APPROVED" {
			return nil, fmt.Errorf("cannot create purchase from unapproved request (Status: %s)", pr.Status)
		}

		// Inherit ProjectID from PR if the request didn't explicitly set one
		if purchase.ProjectID == nil && pr.ProjectID != nil {
			purchase.ProjectID = pr.ProjectID
		}

		// Menghitung Base Net Amount (Subtotal - Diskon) tanpa PPN dan Pajak Lainnya
		netPurchaseAmount := purchase.SubtotalBeforeDiscount - purchase.Discount

		// Hard Block Rule: Tolerance 0%. Net Invoice must be <= PR Estimate.
		if netPurchaseAmount > pr.TotalEstimatedAmount {
			errMsg := fmt.Sprintf("HARD BLOCK: Base Invoice Amount (%.2f) exceeds Approved Purchase Request Amount (%.2f). Note: Taxes are excluded from this limit check. Please revise invoice or request a new PR.", netPurchaseAmount, pr.TotalEstimatedAmount)
			fmt.Printf("❌ %s\n", errMsg)
			return nil, errors.New(errMsg)
		}

		// Set Matching Status since it passed
		purchase.MatchingStatus = "MATCHED"
	}

	// Determine approval basis and base amount for later use
	fmt.Printf("ℹ Determining approval requirements for purchase with amount %.2f\n", purchase.TotalAmount)
	if s.approvalService != nil {
		s.setApprovalBasisAndBase(purchase)
		fmt.Printf("✅ Approval basis set: RequiresApproval=%t, ApprovalBase=%.2f\n", purchase.RequiresApproval, purchase.ApprovalBaseAmount)
	} else {
		// For testing purposes, set default values
		fmt.Printf("⚠ No approval service available, setting default values\n")
		purchase.RequiresApproval = false
		purchase.ApprovalStatus = models.PurchaseApprovalNotRequired
	}

	// Save purchase, status will remain DRAFT
	fmt.Printf("ℹ Saving purchase to database with status DRAFT\n")
	createdPurchase, err := s.purchaseRepo.Create(purchase)
	if err != nil {
		fmt.Printf("❌ Failed to save purchase to database: %v\n", err)
		return nil, fmt.Errorf("failed to save purchase to database: %v", err)
	}
	fmt.Printf("✅ Purchase %d saved successfully with code %s\n", createdPurchase.ID, createdPurchase.Code)

	// NEW LOGIC: ALL purchases must go through approval workflow
	// No more auto-approval for immediate payments - Employee → Finance workflow applies to all
	fmt.Printf("📋 Purchase %s created (method: %s) - will go through approval workflow\n", purchase.Code, purchase.PaymentMethod)
	fmt.Printf("💡 Status remains DRAFT until approval workflow is initiated\n")

	// Note: Bank balance validation and payment processing will happen AFTER approval
	// This ensures proper approval control for all purchase types

	return s.GetPurchaseByID(createdPurchase.ID)
}

func (s *PurchaseService) UpdatePurchase(id uint, request models.PurchaseUpdateRequest, userID uint) (*models.Purchase, error) {
	fmt.Printf("ℹ Starting purchase update for ID %d by user %d\n", id, userID)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("❌ Failed to find purchase %d for update: %v\n", id, err)
		return nil, fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}

	// Check if purchase can be updated
	fmt.Printf("ℹ Checking if purchase %d can be updated (current status: %s)\n", id, purchase.Status)
	if purchase.Status != models.PurchaseStatusDraft && purchase.Status != models.PurchaseStatusPending {
		fmt.Printf("❌ Purchase %d cannot be updated in current status: %s\n", id, purchase.Status)
		return nil, fmt.Errorf("purchase cannot be updated in current status: %s", purchase.Status)
	}
	fmt.Printf("✅ Purchase %d status validation passed for update\n", id)

	// Update fields if provided
	if request.VendorID != nil {
		purchase.VendorID = *request.VendorID
	}
	if request.Date != nil {
		purchase.Date = *request.Date
	}
	if request.DueDate != nil {
		purchase.DueDate = *request.DueDate
	}
	if request.Discount != nil {
		purchase.Discount = *request.Discount
	}
	// Update tax rates from request using double pointers for update requests
	if request.PPNRate != nil {
		if *request.PPNRate == nil {
			purchase.PPNRate = 11.0
		} else {
			purchase.PPNRate = **request.PPNRate
		}
	}
	if request.OtherTaxAdditions != nil {
		purchase.OtherTaxAdditions = *request.OtherTaxAdditions
	}
	if request.PPh21Rate != nil {
		purchase.PPh21Rate = getPPhRateFromDoublePointer(request.PPh21Rate)
	}
	if request.PPh23Rate != nil {
		purchase.PPh23Rate = getPPhRateFromDoublePointer(request.PPh23Rate)
	}
	if request.OtherTaxDeductions != nil {
		purchase.OtherTaxDeductions = *request.OtherTaxDeductions
	}
	if request.Notes != nil {
		purchase.Notes = *request.Notes
	}
	// Update payment method fields
	if request.PaymentMethod != nil {
		purchase.PaymentMethod = *request.PaymentMethod
	}
	if request.BankAccountID != nil {
		purchase.BankAccountID = request.BankAccountID
	}
	if request.CreditAccountID != nil {
		purchase.CreditAccountID = request.CreditAccountID
	}
	if request.PaymentReference != nil {
		purchase.PaymentReference = *request.PaymentReference
	}

	// Update items if provided
	if len(request.Items) > 0 {
		err = s.updatePurchaseItems(purchase, request.Items)
		if err != nil {
			return nil, err
		}
	}

	// Recalculate totals
	fmt.Printf("ℹ Recalculating totals for updated purchase %d\n", id)
	err = s.recalculatePurchaseTotals(purchase)
	if err != nil {
		fmt.Printf("❌ Failed to recalculate totals for purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("failed to recalculate purchase totals: %v", err)
	}
	fmt.Printf("✅ Purchase %d totals recalculated: Subtotal=%.2f, Tax=%.2f, Total=%.2f\n", id, purchase.SubtotalBeforeDiscount, purchase.TaxAmount, purchase.TotalAmount)

	// INVOICE MATCHING: HARD BLOCK VALIDATION FOR UPDATE
	if purchase.PurchaseRequestID != nil {
		var pr models.PurchaseRequest
		if err := s.db.First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return nil, fmt.Errorf("linked purchase request not found: %v", err)
		}

		// Menghitung Base Net Amount (Subtotal - Diskon) tanpa PPN dan Pajak Lainnya
		netPurchaseAmount := purchase.SubtotalBeforeDiscount - purchase.Discount

		// Hard Block Rule
		if netPurchaseAmount > pr.TotalEstimatedAmount {
			errMsg := fmt.Sprintf("HARD BLOCK: Updated Base Invoice Amount (%.2f) exceeds Approved Purchase Request Amount (%.2f). Note: Taxes are excluded from this limit check. Update rejected.", netPurchaseAmount, pr.TotalEstimatedAmount)
			fmt.Printf("❌ %s\n", errMsg)
			return nil, errors.New(errMsg)
		}
		purchase.MatchingStatus = "MATCHED"
	}

	// Re-evaluate approval base
	s.setApprovalBasisAndBase(purchase)

	// Save updated purchase
	fmt.Printf("ℹ Saving updated purchase %d to database\n", id)
	updatedPurchase, err := s.purchaseRepo.Update(purchase)
	if err != nil {
		fmt.Printf("❌ Failed to save updated purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("failed to save updated purchase: %v", err)
	}
	fmt.Printf("✅ Purchase %d updated successfully\n", id)

	return s.GetPurchaseByID(updatedPurchase.ID)
}

func (s *PurchaseService) DeletePurchase(id uint) error {
	fmt.Printf("ℹ Starting purchase cancellation for ID %d (cancel instead of delete)\n", id)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("❌ Failed to find purchase %d for cancellation: %v\n", id, err)
		return fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}

	// If already cancelled, nothing to do
	if purchase.Status == models.PurchaseStatusCancelled {
		fmt.Printf("ℹ Purchase %d is already CANCELLED, nothing to do\n", id)
		return nil
	}

	// Guard: check if purchase date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return err
	}

	// For non-DRAFT purchases that have been approved/completed/paid,
	// perform full accounting reversal (stock, cash/bank, journal, project cost)
	if purchase.Status != models.PurchaseStatusDraft && purchase.Status != models.PurchaseStatusPending {
		fmt.Printf("🔄 Purchase %d has status %s, performing full accounting reversal via OnPurchaseRejected\n", id, purchase.Status)
		if err := s.OnPurchaseRejected(id); err != nil {
			fmt.Printf("⚠️ Warning: Post-rejection cleanup had issues for purchase %d: %v\n", id, err)
			// Continue with cancellation even if cleanup has partial issues
		} else {
			fmt.Printf("✅ Full accounting reversal completed for purchase %d\n", id)
		}
	} else {
		fmt.Printf("✅ Purchase %d is in %s status, no accounting reversal needed\n", id, purchase.Status)
	}

	// Set status to CANCELLED (do NOT soft-delete — maintain audit trail)
	now := time.Now()
	purchase.Status = models.PurchaseStatusCancelled
	purchase.ApprovalStatus = models.PurchaseApprovalRejected
	purchase.UpdatedAt = now
	purchase.OutstandingAmount = 0
	purchase.PaidAmount = 0

	_, err = s.purchaseRepo.Update(purchase)
	if err != nil {
		fmt.Printf("❌ Failed to update purchase %d status to CANCELLED: %v\n", id, err)
		return fmt.Errorf("failed to cancel purchase: %v", err)
	}

	fmt.Printf("✅ Purchase %d cancelled successfully (status: CANCELLED, audit trail preserved)\n", id)
	return nil
}

// updateProjectCostForPurchase updates the total_actual_cost for a linked project.
// If increase=true, it adds to actual cost. If increase=false, it subtracts.
// It uses NetBeforeTax to trace the core actual cost.
func (s *PurchaseService) updateProjectCostForPurchase(purchase *models.Purchase, increase bool) error {
	if purchase.ProjectID == nil {
		// Not linked to any project
		return nil
	}

	cost := purchase.NetBeforeTax
	if !increase {
		cost = -cost
	}

	result := s.db.Exec("UPDATE projects SET total_actual_cost = total_actual_cost + ? WHERE id = ?", cost, *purchase.ProjectID)
	return result.Error
}

// Approval Integration

func (s *PurchaseService) SubmitForApproval(id uint, userID uint) error {
	fmt.Printf("ℹ Starting approval submission for purchase %d by user %d\n", id, userID)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("❌ Failed to find purchase %d for approval submission: %v\n", id, err)
		return fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}

	fmt.Printf("ℹ Checking if purchase %d can be submitted for approval (current status: %s)\n", id, purchase.Status)
	if purchase.Status != models.PurchaseStatusDraft {
		fmt.Printf("❌ Purchase %d cannot be submitted for approval in current status: %s\n", id, purchase.Status)
		return fmt.Errorf("only draft purchases can be submitted for approval, current status: %s", purchase.Status)
	}

	// Ensure approval base up-to-date
	fmt.Printf("ℹ Setting approval basis for purchase %d (amount: %.2f)\n", id, purchase.TotalAmount)
	s.setApprovalBasisAndBase(purchase)
	// NEW LOGIC: Since all purchases now require approval, this should never auto-approve
	requiresApproval := s.checkIfApprovalRequired(purchase.ApprovalBaseAmount)
	fmt.Printf("ℹ Approval requirement check: RequiresApproval=%t, BaseAmount=%.2f\n", requiresApproval, purchase.ApprovalBaseAmount)

	// With new logic, this should never be false, but adding safety check
	if !requiresApproval {
		fmt.Printf("⚠ WARNING: checkIfApprovalRequired returned false - this should not happen with new logic!\n")
		fmt.Printf("🔄 Forcing approval requirement to maintain Employee → Finance workflow\n")
		requiresApproval = true
		purchase.RequiresApproval = true
	}

	// Create approval request
	fmt.Printf("ℹ Creating approval request for purchase %d\n", id)
	err = s.createApprovalRequest(purchase, models.ApprovalPriorityNormal, userID)
	if err != nil {
		fmt.Printf("❌ Failed to create approval request for purchase %d: %v\n", id, err)
		return fmt.Errorf("failed to create approval request: %v", err)
	}
	fmt.Printf("✅ Approval request created for purchase %d\n", id)

	// The approval workflow now starts from Employee step (step 1)
	// When Employee submits, we immediately progress to the next step (Finance/Manager)
	// We directly update the database instead of calling ProcessApprovalAction
	// to avoid role-check issues when the step preload doesn't work correctly
	if purchase.ApprovalRequestID != nil {
		now := time.Now()
		requestID := *purchase.ApprovalRequestID

		// Step 1: Auto-approve the employee submission step (step_order = 1)
		result := s.db.Model(&models.ApprovalAction{}).
			Where("request_id = ? AND is_active = ? AND status = ?", requestID, true, models.ApprovalStatusPending).
			Updates(map[string]interface{}{
				"status":      models.ApprovalStatusApproved,
				"approver_id": userID,
				"comments":    "Purchase submitted by Employee for approval",
				"action_date": now,
				"is_active":   false,
			})
		if result.Error != nil {
			return fmt.Errorf("failed to auto-approve employee step: %v", result.Error)
		}
		if result.RowsAffected == 0 {
			fmt.Printf("⚠️ No active pending step found to auto-approve for request %d\n", requestID)
		}

		// Step 2: Activate the next step (finance approval, step_order = 2)
		// Find the step_id for step_order = 2 in this workflow
		var approvalReq models.ApprovalRequest
		if err := s.db.First(&approvalReq, requestID).Error; err == nil {
			var financeStep models.ApprovalStep
			if err := s.db.Where("workflow_id = ? AND step_order = ?", approvalReq.WorkflowID, 2).First(&financeStep).Error; err == nil {
				s.db.Model(&models.ApprovalAction{}).
					Where("request_id = ? AND step_id = ?", requestID, financeStep.ID).
					Update("is_active", true)
				fmt.Printf("✅ Finance approval step activated for request %d\n", requestID)
			}
		}

		// Step 3: Create approval history entry
		history := models.ApprovalHistory{
			RequestID: requestID,
			UserID:    userID,
			Action:    models.ApprovalActionApproved,
			Comments:  "Purchase submitted by Employee for approval",
			Metadata:  "{}",
		}
		s.db.Create(&history)
		fmt.Printf("✅ Employee submission step auto-approved for purchase %d\n", id)
	}

	// Update purchase status
	now := time.Now()
	purchase.Status = models.PurchaseStatusPending // Change to PENDING instead of PENDING_APPROVAL
	purchase.ApprovalStatus = models.PurchaseApprovalPending
	purchase.RequiresApproval = true
	purchase.UpdatedAt = now

	_, err = s.purchaseRepo.Update(purchase)
	return err
}

func (s *PurchaseService) ProcessPurchaseApproval(purchaseID uint, approved bool, userID uint) error {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return err
	}

	if purchase.ApprovalStatus != models.PurchaseApprovalPending {
		return errors.New("purchase is not pending approval")
	}

	now := time.Now()
	if approved {
		// Purchase approved
		purchase.Status = models.PurchaseStatusApproved
		purchase.ApprovalStatus = models.PurchaseApprovalApproved
		purchase.ApprovedAt = &now
		purchase.ApprovedBy = &userID
	} else {
		// Purchase rejected
		purchase.Status = models.PurchaseStatusCancelled
		purchase.ApprovalStatus = models.PurchaseApprovalRejected
	}

	purchase.UpdatedAt = now
	_, err = s.purchaseRepo.Update(purchase)
	return err
}

// ProcessPurchaseApprovalWithEscalation processes purchase approval with escalation logic
func (s *PurchaseService) ProcessPurchaseApprovalWithEscalation(purchaseID uint, approved bool, userID uint, userRole, comments string, escalateToDirector bool) (map[string]interface{}, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, err
	}

	// Allow approval/rejection of DRAFT purchases (Finance approving new purchases)
	// and PENDING purchases (Director approving escalated purchases)
	// Also allow NOT_STARTED for rejection
	if purchase.Status != models.PurchaseStatusDraft &&
		purchase.Status != models.PurchaseStatusPending &&
		purchase.ApprovalStatus != models.PurchaseApprovalPending &&
		purchase.ApprovalStatus != models.PurchaseApprovalNotStarted {
		return nil, errors.New("purchase cannot be approved in current status")
	}

	now := time.Now()
	result := make(map[string]interface{})

	if !approved {
		// Purchase rejected
		purchase.Status = models.PurchaseStatusCancelled
		purchase.ApprovalStatus = models.PurchaseApprovalRejected
		purchase.UpdatedAt = now

		// If no approval request exists (DRAFT status), create one for history tracking
		if purchase.ApprovalRequestID == nil {
			// Create a minimal approval request for history tracking (without workflow dependency)
			err = s.createMinimalApprovalRequestForRejection(purchase, userID)
			if err != nil {
				// Continue even if this fails - the rejection should still proceed
			}
		}

		_, err = s.purchaseRepo.Update(purchase)
		if err != nil {
			return nil, err
		}

		// Create approval history record for rejection
		if purchase.ApprovalRequestID != nil {
			// First update the approval request status to rejected
			if approvalReq, err := s.approvalService.GetApprovalRequest(*purchase.ApprovalRequestID); err == nil {
				approvalReq.Status = models.ApprovalStatusRejected
				approvalReq.CompletedAt = &now
				approvalReq.RejectReason = comments
				s.approvalService.UpdateApprovalRequest(approvalReq)
			}

			// Ensure comments are not empty for rejection history
			historyComments := comments
			if historyComments == "" {
				historyComments = "Purchase rejected without comment"
			}

			historyErr := s.approvalService.CreateApprovalHistory(*purchase.ApprovalRequestID, userID, models.ApprovalActionRejected, historyComments)
			if historyErr != nil {
				// Log error but don't fail the entire operation
				fmt.Printf("Failed to create approval history for rejection: %v\n", historyErr)
				// Continue with fallback - directly insert into approval_histories table if needed
			}
		} else {
			fmt.Printf("Warning: Purchase %d rejected but no approval request ID found\n", purchaseID)
		}

		// 🚫 CRITICAL: Reverse all accounting effects (journal, stock, cash/bank, project cost)
		fmt.Printf("🔔 Calling OnPurchaseRejected cleanup for purchase %d\n", purchaseID)
		cleanupErr := s.OnPurchaseRejected(purchaseID)
		if cleanupErr != nil {
			fmt.Printf("⚠️ Warning: Post-rejection cleanup failed for purchase %d: %v\n", purchaseID, cleanupErr)
			// Continue — rejection should still succeed even if cleanup has issues
		} else {
			fmt.Printf("✅ Post-rejection cleanup completed for purchase %d\n", purchaseID)
		}

		result["message"] = "Purchase rejected"
		result["purchase_id"] = purchaseID
		result["status"] = "REJECTED"
		result["rejected_by"] = userID
		result["rejected_at"] = now.Format(time.RFC3339)
		result["rejection_reason"] = comments
		return result, nil
	}

	// Purchase is approved, check for escalation
	if userRole == "finance" && escalateToDirector {
		// If no approval request exists (DRAFT status), create one
		if purchase.ApprovalRequestID == nil {
			err = s.createApprovalRequest(purchase, models.ApprovalPriorityNormal, userID)
			if err != nil {
				return nil, fmt.Errorf("failed to create approval request: %v", err)
			}
			// Reload purchase to get the ApprovalRequestID
			purchase, err = s.purchaseRepo.FindByID(purchaseID)
			if err != nil {
				return nil, err
			}
		}

		// IMPORTANT: Escalate to director FIRST before processing approval
		// This ensures the request stays PENDING for director approval
		if purchase.ApprovalRequestID != nil {
			// First escalate to director to add director step
			err = s.approvalService.EscalateToDirector(*purchase.ApprovalRequestID, userID, "Requires Director approval as requested by Finance")
			if err != nil {
				return nil, fmt.Errorf("failed to escalate to director: %v", err)
			}

			// Then process the finance approval
			action := models.ApprovalActionDTO{
				Action:   "APPROVE",
				Comments: fmt.Sprintf("%s (Escalated to Director for final approval)", comments),
			}
			err = s.approvalService.ProcessApprovalAction(*purchase.ApprovalRequestID, userID, action)
			if err != nil {
				return nil, fmt.Errorf("failed to process finance approval: %v", err)
			}
		}

		// Purchase stays PENDING for director review
		purchase.Status = models.PurchaseStatusPending           // Keep as PENDING
		purchase.ApprovalStatus = models.PurchaseApprovalPending // Set to PENDING for director
		purchase.RequiresApproval = true                         // Mark as requiring approval

		purchase.UpdatedAt = now
		_, err = s.purchaseRepo.Update(purchase)
		if err != nil {
			return nil, err
		}

		result["message"] = "Purchase approved by Finance and escalated to Director for final approval"
		result["purchase_id"] = purchaseID
		result["escalated"] = true
		result["status"] = "PENDING"          // Status is PENDING
		result["approval_status"] = "PENDING" // But approval status is PENDING
		return result, nil
	}

	// Direct approval (no escalation needed)
	// SAFETY CHECK: Prevent direct approval of DRAFT purchases - they should go through SubmitForApproval first
	if purchase.Status == models.PurchaseStatusDraft {
		return nil, fmt.Errorf("DRAFT purchases cannot be directly approved - use SubmitForApproval first to initiate workflow")
	}

	// If no approval request exists for non-DRAFT status, create one for history tracking
	if purchase.ApprovalRequestID == nil {
		err = s.createApprovalRequest(purchase, models.ApprovalPriorityNormal, userID)
		if err != nil {
			fmt.Printf("Failed to create approval request: %v\n", err)
			// Continue even if this fails - the approval should still proceed
		}
		// Reload purchase to get the ApprovalRequestID
		purchase, err = s.purchaseRepo.FindByID(purchaseID)
		if err != nil {
			return nil, err
		}
	}

	// Create approval history
	if purchase.ApprovalRequestID != nil {
		// Update the approval request status to approved
		if approvalReq, err := s.approvalService.GetApprovalRequest(*purchase.ApprovalRequestID); err == nil {
			approvalReq.Status = models.ApprovalStatusApproved
			approvalReq.CompletedAt = &now
			s.approvalService.UpdateApprovalRequest(approvalReq)
		}

		historyErr := s.approvalService.CreateApprovalHistory(*purchase.ApprovalRequestID, userID, models.ApprovalActionApproved, comments)
		if historyErr != nil {
			fmt.Printf("Failed to create approval history: %v\n", historyErr)
		}
	}

	purchase.Status = models.PurchaseStatusApproved
	purchase.ApprovalStatus = models.PurchaseApprovalApproved
	purchase.ApprovedAt = &now
	purchase.ApprovedBy = &userID
	purchase.UpdatedAt = now

	// CRITICAL FIX: Initialize payment amounts for CREDIT purchases when approved
	if purchase.PaymentMethod == models.PurchasePaymentCredit {
		// Set outstanding amount to total amount (nothing paid yet)
		purchase.OutstandingAmount = purchase.TotalAmount
		purchase.PaidAmount = 0
		fmt.Printf("💳 Initialized CREDIT purchase payment tracking: Total=%.2f, Outstanding=%.2f, Paid=%.2f\n",
			purchase.TotalAmount, purchase.OutstandingAmount, purchase.PaidAmount)
	}

	_, err = s.purchaseRepo.Update(purchase)
	if err != nil {
		return nil, err
	}

	// NOTE: Stock updates, journal entries, and cash/bank balance updates
	// are now handled by OnPurchaseApproved callback above

	// ✅ FIXED: Call OnPurchaseApproved callback for complete post-approval processing
	// This ensures cash bank transactions, stock updates, and journal entries are all handled correctly
	fmt.Printf("🔔 Calling OnPurchaseApproved callback for purchase %d\n", purchaseID)
	err = s.OnPurchaseApproved(purchaseID)
	if err != nil {
		fmt.Printf("⚠️ Warning: Post-approval callback failed for purchase %d: %v\n", purchaseID, err)
		// Continue processing, don't fail the entire approval
	} else {
		fmt.Printf("✅ Post-approval callback completed successfully for purchase %d\n", purchaseID)
	}

	result["message"] = "Purchase approved successfully"
	result["purchase_id"] = purchaseID
	result["escalated"] = false
	result["status"] = "APPROVED"
	result["approval_status"] = "APPROVED"
	return result, nil
}

// Receipt Management

func (s *PurchaseService) CreatePurchaseReceipt(request models.PurchaseReceiptRequest, userID uint) (*models.PurchaseReceipt, error) {
	purchase, err := s.purchaseRepo.FindByID(request.PurchaseID)
	if err != nil {
		return nil, err
	}

	// Allow receipt creation for APPROVED, PENDING, and PAID purchases
	// PAID status might exist if payment was recorded before receipt creation
	if purchase.Status != models.PurchaseStatusApproved && purchase.Status != models.PurchaseStatusPending && purchase.Status != "PAID" {
		return nil, errors.New("can only receive items for approved, pending, or paid purchases")
	}

	// Generate receipt number
	receiptNumber := s.generateReceiptNumber()

	// Create receipt
	receipt := &models.PurchaseReceipt{
		PurchaseID:    request.PurchaseID,
		ReceiptNumber: receiptNumber,
		ReceivedDate:  request.ReceivedDate,
		ReceivedBy:    userID,
		Status:        models.ReceiptStatusPending,
		Notes:         request.Notes,
	}

	// Validate receipt items
	err = s.validateReceiptItems(request.ReceiptItems, purchase.PurchaseItems)
	if err != nil {
		return nil, err
	}

	// Create receipt with items
	createdReceipt, err := s.purchaseRepo.CreateReceipt(receipt)
	if err != nil {
		return nil, err
	}

	// Process receipt items and check completion
	allReceived := true
	for _, itemReq := range request.ReceiptItems {
		purchaseItem, err := s.purchaseRepo.GetPurchaseItemByID(itemReq.PurchaseItemID)
		if err != nil {
			return nil, err
		}

		if purchaseItem.PurchaseID != request.PurchaseID {
			return nil, errors.New("purchase item does not belong to this purchase")
		}

		// Validate remaining quantity for this item
		receivedSoFar, err := s.purchaseRepo.SumReceivedQtyByPurchaseItem(purchaseItem.ID)
		if err != nil {
			return nil, err
		}
		remaining := int(purchaseItem.Quantity) - receivedSoFar
		if remaining <= 0 {
			// Nothing left to receive for this item; skip
			allReceived = allReceived && true
			continue
		}
		if itemReq.QuantityReceived > remaining {
			return nil, fmt.Errorf("received quantity (%d) exceeds remaining (%d) for item %d", itemReq.QuantityReceived, remaining, itemReq.PurchaseItemID)
		}

		// Create receipt item
		receiptItem := &models.PurchaseReceiptItem{
			ReceiptID:        createdReceipt.ID,
			PurchaseItemID:   itemReq.PurchaseItemID,
			QuantityReceived: itemReq.QuantityReceived,
			Condition:        s.getDefaultCondition(itemReq.Condition),
			Notes:            itemReq.Notes,
		}

		err = s.purchaseRepo.CreateReceiptItem(receiptItem)
		if err != nil {
			return nil, err
		}

		// Check if all items are fully received
		if itemReq.QuantityReceived < purchaseItem.Quantity {
			allReceived = false
		}
	}

	// Update receipt status only - do not change purchase status based on receipt completion
	if allReceived {
		createdReceipt.Status = models.ReceiptStatusComplete
		// DO NOT set purchase to COMPLETED here - purchase should only be COMPLETED when fully paid
		// Keep purchase in its current status (APPROVED) until payment is made
	} else {
		createdReceipt.Status = models.ReceiptStatusPartial
	}

	// Update receipt status
	createdReceipt, err = s.purchaseRepo.UpdateReceipt(createdReceipt)
	if err != nil {
		return nil, err
	}

	// If all items of the purchase are fully received, mark purchase as COMPLETED (receipt-wise)
	allItemsReceived, err := s.purchaseRepo.AreAllItemsFullyReceived(request.PurchaseID)
	if err == nil && allItemsReceived {
		// Update purchase status to COMPLETED but do not change payment amounts
		purchase.Status = models.PurchaseStatusCompleted
		if _, upErr := s.purchaseRepo.Update(purchase); upErr != nil {
			fmt.Printf("⚠️ Warning: Failed to update purchase status to COMPLETED after receipts: %v\n", upErr)
		}
	}

	// Optional: Capitalization journals for items flagged in request
	// This leverages optional fields in PurchaseReceiptItemRequest (CapitalizeAsset, FixedAssetAccountID, SourceAccountOverride)
	if s.assetCapitalizationSvc != nil {
		// Load default accounts
		var inventoryAccountID uint
		if inv, err := s.accountRepo.FindByCode(nil, "1103-001"); err == nil {
			inventoryAccountID = inv.ID
		}
		var defaultFAAccountID uint
		if fa, err := s.accountRepo.FindByCode(nil, "1201-004"); err == nil {
			defaultFAAccountID = fa.ID
		}
		// Map request items by PurchaseItemID for quick lookup of flags
		reqFlags := make(map[uint]models.PurchaseReceiptItemRequest)
		for _, ri := range request.ReceiptItems {
			reqFlags[ri.PurchaseItemID] = ri
		}
		// Iterate over receipt items again using the request list for flags
		for _, itemReq := range request.ReceiptItems {
			if !itemReq.CapitalizeAsset {
				continue
			}
			// Find purchase item to compute amount
			purchaseItem, err := s.purchaseRepo.GetPurchaseItemByID(itemReq.PurchaseItemID)
			if err != nil {
				fmt.Printf("⚠️ Capitalization skipped: cannot find purchase item %d: %v\n", itemReq.PurchaseItemID, err)
				continue
			}
			// Compute capitalization amount (exclude VAT): unit_price * qty_received
			capAmount := float64(itemReq.QuantityReceived) * purchaseItem.UnitPrice
			if capAmount <= 0 {
				fmt.Printf("ℹ️ Capitalization skipped for item %d: amount is 0\n", itemReq.PurchaseItemID)
				continue
			}
			// SAFEGUARD: if purchase item booked to Inventory (expense_account_id == 0), skip capitalization to avoid double handling
			if purchaseItem.ExpenseAccountID == 0 {
				fmt.Printf("ℹ️ Capitalization disabled for inventory item %d. Skipping.\n", itemReq.PurchaseItemID)
				continue
			}
			// DOUBLE-POSTING PREVENTION: If the purchase item was already debited to a Fixed Asset account
			// (e.g., 1201-xxx) in the purchase journal, skip capitalization to prevent duplicate entries
			if purchaseItem.ExpenseAccountID != 0 {
				var itemAccount models.Account
				if accErr := s.db.First(&itemAccount, purchaseItem.ExpenseAccountID).Error; accErr == nil {
					if strings.ToUpper(itemAccount.Category) == models.CategoryFixedAsset {
						fmt.Printf("⏭️ Capitalization skipped for item %d: already debited to Fixed Asset account %s (%s) in purchase journal\n",
							itemReq.PurchaseItemID, itemAccount.Code, itemAccount.Name)
						continue
					}
				}
			}
			// Determine source account (override > item expense > inventory)
			sourceAccountID := inventoryAccountID
			if itemReq.SourceAccountOverride != nil && *itemReq.SourceAccountOverride != 0 {
				sourceAccountID = *itemReq.SourceAccountOverride
			} else if purchaseItem.ExpenseAccountID != 0 {
				sourceAccountID = purchaseItem.ExpenseAccountID
			}
			// Determine fixed asset account
			fixedAssetAccountID := defaultFAAccountID
			if itemReq.FixedAssetAccountID != nil && *itemReq.FixedAssetAccountID != 0 {
				fixedAssetAccountID = *itemReq.FixedAssetAccountID
			}
			// Build input
			capInput := CapitalizationInput{
				AssetID:             0, // asset record may be created separately from frontend; link to receipt for now
				Amount:              capAmount,
				Date:                request.ReceivedDate,
				Description:         fmt.Sprintf("Capitalization from Receipt %s - Purchase %s", createdReceipt.ReceiptNumber, purchase.Code),
				Reference:           createdReceipt.ReceiptNumber,
				SourceAccountID:     sourceAccountID,
				FixedAssetAccountID: fixedAssetAccountID,
				UserID:              userID,
				ReferenceType:       "RECEIPT",
				ReferenceID:         createdReceipt.ID,
			}
			if err := s.assetCapitalizationSvc.Capitalize(capInput); err != nil {
				fmt.Printf("⚠️ Failed to create capitalization journal for receipt item %d: %v\n", itemReq.PurchaseItemID, err)
				// Continue other items; do not fail receipt creation
			} else {
				fmt.Printf("✅ Capitalization journal created for receipt item %d (amount=%.2f)\n", itemReq.PurchaseItemID, capAmount)
			}
		}
	}

	// Do not update purchase status here - it should remain in current status until payment is made
	// Receipt completion is independent of purchase completion

	return s.purchaseRepo.FindReceiptByID(createdReceipt.ID)
}

// Document Management

func (s *PurchaseService) UploadDocument(purchaseID uint, documentType, fileName, filePath string, fileSize int64, mimeType string, userID uint) error {
	_, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return err
	}

	document := &models.PurchaseDocument{
		PurchaseID:   purchaseID,
		DocumentType: documentType,
		FileName:     fileName,
		FilePath:     filePath,
		FileSize:     fileSize,
		MimeType:     mimeType,
		UploadedBy:   userID,
	}

	return s.purchaseRepo.CreateDocument(document)
}

func (s *PurchaseService) GetPurchaseDocuments(purchaseID uint) ([]models.PurchaseDocument, error) {
	return s.purchaseRepo.FindDocumentsByPurchaseID(purchaseID)
}

func (s *PurchaseService) DeleteDocument(documentID uint) error {
	return s.purchaseRepo.DeleteDocument(documentID)
}

// Three-way Matching

func (s *PurchaseService) GetPurchaseMatching(purchaseID uint) (*models.PurchaseMatchingData, error) {
	return s.purchaseRepo.GetPurchaseForMatching(purchaseID)
}

func (s *PurchaseService) ValidateThreeWayMatching(purchaseID uint) (bool, error) {
	matching, err := s.purchaseRepo.GetPurchaseForMatching(purchaseID)
	if err != nil {
		return false, err
	}

	// Validate Purchase Order exists
	if matching.Purchase.ID == 0 {
		return false, errors.New("purchase order not found")
	}

	// Validate Receipt exists and is complete
	hasCompleteReceipt := false
	for _, receipt := range matching.Receipts {
		if receipt.Status == models.ReceiptStatusComplete {
			hasCompleteReceipt = true
			break
		}
	}

	if !hasCompleteReceipt {
		return false, errors.New("complete receipt required")
	}

	// Validate Invoice exists
	hasInvoice := false
	for _, doc := range matching.Documents {
		if doc.DocumentType == models.PurchaseDocumentInvoice {
			hasInvoice = true
			break
		}
	}

	if !hasInvoice {
		return false, errors.New("invoice document required")
	}

	// Update matching status
	err = s.purchaseRepo.UpdateMatchingStatus(purchaseID, "MATCHED")
	if err != nil {
		return false, err
	}

	return true, nil
}

// Analytics and Reporting

func (s *PurchaseService) GetPurchasesSummary(startDate, endDate string) (*models.PurchaseSummary, error) {
	return s.purchaseRepo.GetPurchasesSummary(startDate, endDate)
}

func (s *PurchaseService) GetVendorPurchaseSummary(vendorID uint) (*models.VendorPurchaseSummary, error) {
	return s.purchaseRepo.GetVendorPurchaseSummary(vendorID)
}

// Receipt Management - Additional methods

// GetPurchaseReceipts returns all receipts for a purchase
func (s *PurchaseService) GetPurchaseReceipts(purchaseID uint) ([]models.PurchaseReceipt, error) {
	// Verify purchase exists
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, err
	}

	// Get receipts from repository
	receipts, err := s.purchaseRepo.FindReceiptsByPurchaseID(purchaseID)
	if err != nil {
		return nil, err
	}

	// Auto-sync status: if seluruh item sudah diterima, set COMPLETED
	allReceived, err2 := s.purchaseRepo.AreAllItemsFullyReceived(purchaseID)
	if err2 == nil && allReceived && purchase.Status != models.PurchaseStatusCompleted {
		purchase.Status = models.PurchaseStatusCompleted
		if _, upErr := s.purchaseRepo.Update(purchase); upErr != nil {
			fmt.Printf("⚠️ Warning: Failed to set purchase %d to COMPLETED during receipt fetch: %v\n", purchaseID, upErr)
		}
	}

	return receipts, nil
}

// GetCompletedPurchaseReceipts returns only completed receipts for a purchase
func (s *PurchaseService) GetCompletedPurchaseReceipts(purchaseID uint) ([]models.PurchaseReceipt, error) {
	// Verify purchase exists
	_, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, err
	}

	// Get completed receipts from repository
	return s.purchaseRepo.FindCompletedReceiptsByPurchaseID(purchaseID)
}

// GenerateReceiptPDF generates PDF for a specific receipt
func (s *PurchaseService) GenerateReceiptPDF(receiptID uint) ([]byte, *models.PurchaseReceipt, error) {
	// Get receipt with all related data
	receipt, err := s.purchaseRepo.FindReceiptByID(receiptID)
	if err != nil {
		return nil, nil, err
	}

	// Check if PDF service is available
	if s.pdfService == nil {
		return nil, nil, errors.New("PDF service not available")
	}

	// Generate PDF using the PDF service
	pdfBytes, err := s.pdfService.GenerateReceiptPDF(receipt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate receipt PDF: %v", err)
	}

	return pdfBytes, receipt, nil
}

// GenerateAllReceiptsPDF generates combined PDF for all receipts of a purchase
func (s *PurchaseService) GenerateAllReceiptsPDF(purchaseID uint) ([]byte, *models.Purchase, error) {
	// Get purchase with all related data
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, nil, err
	}

	// Get all receipts for this purchase
	receipts, err := s.GetPurchaseReceipts(purchaseID)
	if err != nil {
		return nil, nil, err
	}

	if len(receipts) == 0 {
		return nil, nil, errors.New("no receipts found for this purchase")
	}

	// Check if PDF service is available
	if s.pdfService == nil {
		return nil, nil, errors.New("PDF service not available")
	}

	// Generate combined PDF using the PDF service
	// Create data structure for the PDF service
	data := map[string]interface{}{
		"purchase": purchase,
		"receipts": receipts,
	}
	pdfBytes, err := s.pdfService.GenerateAllReceiptsPDF(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate combined receipts PDF: %v", err)
	}

	return pdfBytes, purchase, nil
}

// PostApprovalCallback Implementation

// OnPurchaseApproved implements PostApprovalCallback interface - handles business logic after purchase approval
func (s *PurchaseService) OnPurchaseApproved(purchaseID uint) error {
	fmt.Printf("📦 Starting post-approval processing for purchase %d\n", purchaseID)

	// Get the approved purchase
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return fmt.Errorf("failed to get purchase for post-approval processing: %v", err)
	}

	fmt.Printf("💰 Processing approved purchase %s (method: %s, amount: %.2f)\n", purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	// 1. Update product stock
	fmt.Printf("🔄 Updating product stock for approved purchase %d\n", purchaseID)
	err = s.updateProductStockOnApproval(purchase)
	if err != nil {
		fmt.Printf("⚠️ Warning: Failed to update product stock: %v\n", err)
		// Continue processing even if stock update fails
	} else {
		fmt.Printf("✅ Successfully updated product stock\n")
	}

	// 2. Create journal entries - PRIORITIZE SSOT (no double posting)
	fmt.Printf("🏗️ Creating journal entries for approved purchase %s (ONLY using SSOT)\n", purchase.Code)

	// Use SSOT service exclusively
	if s.journalServiceSSOT != nil {
		if err := s.journalServiceSSOT.CreatePurchaseJournal(purchase, nil); err != nil {
			fmt.Printf("⚠️ Warning: SSOT journal creation failed: %v\n", err)
			// Continue processing - journal can be recreated later
		} else {
			fmt.Printf("✅ [SSOT] Created purchase journal entries for Balance Sheet integration\n")
		}
	} else {
		// No SSOT service available
		fmt.Printf("⚠️ SSOT journal service not available, skipping journal creation\n")
	}

	// 3. Update cash/bank balance for immediate payment methods
	if isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("💰 Updating cash/bank balance for immediate payment purchase\n")
		err = s.updateCashBankBalanceForPurchase(purchase)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to update cash/bank balance: %v\n", err)
		} else {
			fmt.Printf("✅ Successfully updated cash/bank balance\n")
		}
	}

	// 4. Initialize payment tracking for credit purchases
	if purchase.PaymentMethod == models.PurchasePaymentCredit {
		purchase.OutstandingAmount = purchase.TotalAmount
		purchase.PaidAmount = 0
		_, err = s.purchaseRepo.Update(purchase)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to update credit purchase payment tracking: %v\n", err)
		} else {
			fmt.Printf("💳 Initialized credit purchase payment tracking\n")
		}
	} else {
		// For immediate payment, mark as fully paid
		purchase.PaidAmount = purchase.TotalAmount
		purchase.OutstandingAmount = 0
		_, err = s.purchaseRepo.Update(purchase)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to update immediate payment tracking: %v\n", err)
		} else {
			fmt.Printf("💵 Marked immediate payment as fully paid\n")
		}
	}

	// 5. Update Project Actual Cost (if linked to project)
	if purchase.ProjectID != nil {
		fmt.Printf("📈 Increasing project actual cost for approved purchase %d\n", purchaseID)
		err = s.updateProjectCostForPurchase(purchase, true)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to increase project actual cost: %v\n", err)
		} else {
			fmt.Printf("✅ Successfully increased project actual cost for project %d\n", *purchase.ProjectID)
		}
	}

	// 6. Auto-create Asset records for items debited to Fixed Asset accounts (PSAK 16 - Accrual Basis)
	// When purchase items target a Fixed Asset account (1201-xxx), automatically create Asset register records
	// so that depreciation starts immediately from purchase date, not after payment/receipt
	s.autoCreateAssetsFromPurchase(purchase)

	fmt.Printf("✅ Post-approval processing completed for purchase %s\n", purchase.Code)
	return nil
}

// autoCreateAssetsFromPurchase creates Asset records for purchase items flagged as Fixed Asset
// or items debited to Fixed Asset accounts (PSAK 16 accrual basis).
func (s *PurchaseService) autoCreateAssetsFromPurchase(purchase *models.Purchase) {
	if len(purchase.PurchaseItems) == 0 {
		return
	}

	assetRepo := repositories.NewAssetRepository(s.db)
	assetService := NewAssetService(assetRepo, s.db)
	assetJournalSvc := NewAssetJournalServiceSSOT(s.db)

	for _, item := range purchase.PurchaseItems {
		// Determine if this item should create an asset
		isAssetFromFlag := item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset
		
		if !isAssetFromFlag {
			continue // Not an asset item, skip
		}

		// Check if asset already exists for this purchase item (prevent duplicates)
		var existingCount int64
		s.db.Model(&models.Asset{}).Where(
			"notes LIKE ? AND deleted_at IS NULL",
			fmt.Sprintf("%%PurchaseItem:%d%%", item.ID),
		).Count(&existingCount)
		if existingCount > 0 {
			fmt.Printf("ℹ️ Asset already exists for purchase item %d, skipping\n", item.ID)
			continue
		}

		// Determine asset name
		assetName := item.Description
		if assetName == "" {
			// Fallback: use product name if available
			if item.ProductID != nil && *item.ProductID > 0 {
				var product models.Product
				if err := s.db.First(&product, *item.ProductID).Error; err == nil {
					assetName = product.Name
				}
			}
			if assetName == "" {
				assetName = fmt.Sprintf("Aset dari PO %s", purchase.Code)
			}
		}

		// Determine category from AssetCategoryID (user-selected)
		category := "Peralatan" // Default
		var assetCategoryID *uint
		var assetAccountID *uint

		if item.AssetCategoryID != nil && *item.AssetCategoryID > 0 {
			// User selected asset category from UI
			assetCategoryID = item.AssetCategoryID
			var assetCat models.AssetCategory
			if err := s.db.First(&assetCat, *item.AssetCategoryID).Error; err == nil {
				category = assetCat.Name
			}
		}

		// Firmly use ExpenseAccountID as the debit Asset Account
		if item.ExpenseAccountID != 0 {
			assetAccountID = &item.ExpenseAccountID
		}

		// Resolve depreciation expense account (Beban Penyusutan)
		var depExpAccount models.Account
		var depExpID *uint
		if err := s.db.Where("category = ? AND type = 'EXPENSE' AND deleted_at IS NULL", models.CategoryDepreciationExp).First(&depExpAccount).Error; err == nil {
			depExpID = &depExpAccount.ID
		} else if err := s.db.Where("code LIKE '6103%' AND type = 'EXPENSE' AND deleted_at IS NULL").First(&depExpAccount).Error; err == nil {
			depExpID = &depExpAccount.ID
		}

		// Resolve accumulated depreciation account (1202-xxx mirrors 1201-xxx)
		var accumDepID *uint
		if assetAccountID != nil {
			var assetAcct models.Account
			if err := s.db.First(&assetAcct, *assetAccountID).Error; err == nil {
				accumDepCode := strings.Replace(assetAcct.Code, "1201", "1202", 1)
				var accumDepAccount models.Account
				if err := s.db.Where("code = ? AND deleted_at IS NULL", accumDepCode).First(&accumDepAccount).Error; err == nil {
					accumDepID = &accumDepAccount.ID
				}
			}
		}

		// Set default useful life and depreciation method based on category
		usefulLife := 5 // Default 5 years
		depMethod := models.DepreciationMethodStraightLine
		salvageValue := 0.0

		switch category {
		case "Tanah":
			usefulLife = 0 // Tanah tidak disusutkan
			depMethod = ""
		case "Bangunan":
			usefulLife = 20 // 20 tahun
			salvageValue = item.TotalPrice * 0.10
		case "Kendaraan":
			usefulLife = 5 // 5 tahun
			salvageValue = item.TotalPrice * 0.10
		case "Peralatan":
			usefulLife = 4 // 4 tahun
			salvageValue = item.TotalPrice * 0.05
		case "Inventaris Kantor":
			usefulLife = 4 // 4 tahun
			salvageValue = item.TotalPrice * 0.05
		}

		// Ensure Quantity is at least 1 (Prevents zero-loop bug & division by zero panic)
		qty := item.Quantity
		if qty <= 0 {
			qty = 1
			fmt.Printf("⚠️ Warning: Auto-fixing item.Quantity=0 for Asset '%s', forcing to Qty=1\n", assetName)
		}

		// Create asset for each unit quantity
		for q := 0; q < qty; q++ {
			unitAssetName := assetName
			if qty > 1 {
				unitAssetName = fmt.Sprintf("%s (#%d)", assetName, q+1)
			}

			// Ensure per-unit price is calculated properly
			perUnitPrice := item.UnitPrice
			if perUnitPrice <= 0 && item.TotalPrice > 0 {
				perUnitPrice = item.TotalPrice / float64(qty)
			}

			asset := &models.Asset{
				Name:               unitAssetName,
				Category:           category,
				CategoryID:         assetCategoryID,
				Status:             models.AssetStatusPendingSetup,
				PurchaseDate:       purchase.Date,
				PurchasePrice:      perUnitPrice, // Per-unit price
				SalvageValue:       salvageValue / float64(qty),
				UsefulLife:         usefulLife,
				DepreciationMethod: depMethod,
				IsActive:           false, // Inactive until user completes setup
				Notes: fmt.Sprintf(
					"Auto-created from Purchase %s (PurchaseItem:%d). Silakan lengkapi masa manfaat, nilai sisa, dan metode penyusutan lalu aktifkan aset.",
					purchase.Code, item.ID,
				),
				AssetAccountID:                   assetAccountID,
				DepreciationAccountID:            depExpID,
				AccumulatedDepreciationAccountID: accumDepID,
			}

			// Create asset via service (generates code automatically)
			if err := assetService.CreateAsset(asset); err != nil {
				// We don't continue out of the whole function, just print loud error to track!
				fmt.Printf("🚨 🚨 🚨 [FATAL AUTO-ASSET] Failed to create Asset DB record '%s': %v\n", unitAssetName, err)
				continue
			}

			fmt.Printf("✅ [AUTO-ASSET] Created asset %s (%s) from purchase %s - Rp %.2f, useful life: %d years\n",
				asset.Code, asset.Name, purchase.Code, asset.PurchasePrice, asset.UsefulLife)
		}

		// NOTE: SSOT journal for this asset is NOT created here because the purchase
		// journal (step 2) already handles: Dr 1201-xxx / Cr Hutang/Kas/Bank
		_ = assetJournalSvc // Available if needed for future use
	}
}

// OnPurchaseRejected reverses all accounting effects when a purchase is rejected.
// This handles cleanup for purchases that were previously approved (journal, stock, cash/bank, project cost).
func (s *PurchaseService) OnPurchaseRejected(purchaseID uint) error {
	fmt.Printf("🚫 Starting post-rejection cleanup for purchase %d\n", purchaseID)

	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return fmt.Errorf("failed to get purchase for rejection cleanup: %v", err)
	}

	fmt.Printf("🔄 Cleaning up rejected purchase %s (method: %s, amount: %.2f)\n",
		purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	// Guard: check if purchase date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return err
	}

	// 1. Reset payment tracking amounts
	fmt.Printf("💰 Resetting payment tracking for rejected purchase %d\n", purchaseID)
	oldOutstanding := purchase.OutstandingAmount
	oldPaid := purchase.PaidAmount
	purchase.OutstandingAmount = 0
	purchase.PaidAmount = 0
	_, err = s.purchaseRepo.Update(purchase)
	if err != nil {
		fmt.Printf("⚠️ Warning: Failed to reset payment tracking: %v\n", err)
	} else {
		fmt.Printf("✅ Payment tracking reset: Outstanding %.2f→0, Paid %.2f→0\n", oldOutstanding, oldPaid)
	}

	// 2. Delete SSOT journal entries with COA balance reversal
	fmt.Printf("📒 Removing SSOT journal entries for rejected purchase %d\n", purchaseID)
	if s.journalServiceSSOT != nil {
		// Use the SSOT journal service which properly reverses COA balances
		if err := s.journalServiceSSOT.DeletePurchaseJournal(purchaseID, nil); err != nil {
			fmt.Printf("⚠️ Warning: Failed to delete SSOT journal via service for purchase %d: %v\n", purchaseID, err)
		} else {
			fmt.Printf("✅ SSOT journal entries deleted with COA reversal for purchase %d\n", purchaseID)
		}
	}

	// Fallback: Also soft-delete any remaining SSOT entries that might have been missed
	if err := s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Model(&models.SSOTJournalEntry{}).
		Where("source_type = 'PURCHASE' AND source_id = ? AND deleted_at IS NULL", purchaseID).
		Update("deleted_at", time.Now()).Error; err != nil {
		fmt.Printf("⚠️ Warning: Failed to soft-delete remaining SSOT journal entries: %v\n", err)
	}

	// Also soft-delete any remaining unified_journal_lines
	if err := s.db.Exec(`
		UPDATE unified_journal_lines 
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE deleted_at IS NULL AND journal_id IN (
			SELECT id FROM unified_journal_ledger 
			WHERE source_type = 'PURCHASE' AND source_id = ?
		)
	`, purchaseID).Error; err != nil {
		fmt.Printf("⚠️ Warning: Failed to soft-delete remaining SSOT journal lines: %v\n", err)
	}

	// 3. Reverse product stock (subtract the quantities that were added on approval)
	fmt.Printf("📦 Reversing product stock for rejected purchase %s\n", purchase.Code)
	for _, item := range purchase.PurchaseItems {
		// Skip ASSET items - they don't have inventory/stock
		if item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset {
			fmt.Printf("ℹ️ Skipping stock reversal for ASSET item '%s' (PurchaseItem:%d)\n", item.Description, item.ID)
			continue
		}
		// Skip items without ProductID
		if item.ProductID == nil || *item.ProductID == 0 {
			continue
		}

		product, err := s.productRepo.FindByID(*item.ProductID)
		if err != nil {
			fmt.Printf("⚠️ Warning: Product %d not found for stock reversal: %v\n", *item.ProductID, err)
			continue
		}

		oldStock := product.Stock
		product.Stock -= item.Quantity
		if product.Stock < 0 {
			fmt.Printf("⚠️ Warning: Stock for product %d would go negative (%d), setting to 0\n", product.ID, product.Stock)
			product.Stock = 0
		}

		err = s.productRepo.Update(context.Background(), product)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to reverse stock for product %d: %v\n", product.ID, err)
		} else {
			fmt.Printf("✅ Product %d stock reversed: %d → %d\n", product.ID, oldStock, product.Stock)
		}
	}

	// 4. Reverse cash/bank balance for immediate payment purchases
	if isImmediatePayment(purchase.PaymentMethod) && purchase.BankAccountID != nil && *purchase.BankAccountID > 0 {
		fmt.Printf("💰 Restoring cash/bank balance for rejected immediate payment purchase\n")

		var cashBank models.CashBank
		if err := s.db.First(&cashBank, *purchase.BankAccountID).Error; err != nil {
			fmt.Printf("⚠️ Warning: Failed to find bank account %d for balance restoration: %v\n", *purchase.BankAccountID, err)
		} else {
			oldBalance := cashBank.Balance
			cashBank.Balance += purchase.TotalAmount // Add back (restore)

			if err := s.db.Save(&cashBank).Error; err != nil {
				fmt.Printf("⚠️ Warning: Failed to restore cash/bank balance: %v\n", err)
			} else {
				fmt.Printf("✅ Cash/bank balance restored: %s %.2f → %.2f (+%.2f)\n",
					cashBank.Name, oldBalance, cashBank.Balance, purchase.TotalAmount)
			}

			// Create reversal transaction record
			reversalTx := &models.CashBankTransaction{
				CashBankID:      *purchase.BankAccountID,
				ReferenceType:   "PURCHASE_REVERSAL",
				ReferenceID:     purchase.ID,
				Amount:          purchase.TotalAmount, // Positive = incoming (reversal)
				BalanceAfter:    cashBank.Balance,
				TransactionDate: time.Now(),
				Notes:           fmt.Sprintf("Reversal: Purchase %s rejected", purchase.Code),
			}

			if err := s.db.Create(reversalTx).Error; err != nil {
				fmt.Printf("⚠️ Warning: Failed to create reversal transaction record: %v\n", err)
			} else {
				fmt.Printf("📝 Reversal cash/bank transaction record created: ID %d\n", reversalTx.ID)
			}
		}

		// Soft-delete the original cash/bank transaction
		if err := s.db.Exec(`
			UPDATE cash_bank_transactions 
			SET deleted_at = CURRENT_TIMESTAMP
			WHERE reference_type = 'PURCHASE' AND reference_id = ? AND deleted_at IS NULL
		`, purchaseID).Error; err != nil {
			fmt.Printf("⚠️ Warning: Failed to soft-delete original cash/bank transaction: %v\n", err)
		}
	}

	// 5. Reverse project actual cost (if linked to a project)
	if purchase.ProjectID != nil {
		fmt.Printf("📉 Decreasing project actual cost for rejected purchase %d\n", purchaseID)
		err = s.updateProjectCostForPurchase(purchase, false) // false = decrease
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to decrease project actual cost: %v\n", err)
		} else {
			fmt.Printf("✅ Project %d actual cost decreased by %.2f\n", *purchase.ProjectID, purchase.NetBeforeTax)
		}
	}

	fmt.Printf("✅ Post-rejection cleanup completed for purchase %s\n", purchase.Code)
	return nil
}

// Private helper methods

func (s *PurchaseService) checkIfApprovalRequired(amount float64) bool {
	// NEW LOGIC: All purchases require approval regardless of amount
	// This ensures Employee → Finance → (optional Director) workflow
	return true
}

func (s *PurchaseService) createApprovalRequest(purchase *models.Purchase, priority string, userID uint) error {
	// Ensure vendor is loaded
	vendorName := "Unknown"
	if purchase.Vendor.ID != 0 {
		// Vendor is already loaded
		vendorName = purchase.Vendor.Name
	} else {
		// Try to load vendor if not already loaded
		vendor, err := s.contactRepo.GetByID(purchase.VendorID)
		if err == nil {
			vendorName = vendor.Name
		}
	}

	// Debug logging before creating approval request
	fmt.Printf("🔍 Creating approval request for purchase %s\n", purchase.Code)
	fmt.Printf("   TotalAmount: %.2f\n", purchase.TotalAmount)
	fmt.Printf("   ApprovalBaseAmount: %.2f\n", purchase.ApprovalBaseAmount)
	fmt.Printf("   SubtotalBeforeDiscount: %.2f\n", purchase.SubtotalBeforeDiscount)

	// Create approval request
	approvalReq := models.CreateApprovalRequestDTO{
		EntityType:     models.EntityTypePurchase,
		EntityID:       purchase.ID,
		Amount:         purchase.ApprovalBaseAmount,
		Priority:       priority,
		RequestTitle:   fmt.Sprintf("Purchase Approval - %s (Vendor: %s)", purchase.Code, vendorName),
		RequestMessage: fmt.Sprintf("Approval request for purchase %s with base amount %.2f (basis: %s)", purchase.Code, purchase.ApprovalBaseAmount, purchase.ApprovalAmountBasis),
	}

	// Determine priority based on amount
	if purchase.TotalAmount > 50000000 { // 50M IDR
		approvalReq.Priority = models.ApprovalPriorityUrgent
	} else if purchase.TotalAmount > 25000000 { // 25M IDR
		approvalReq.Priority = models.ApprovalPriorityHigh
	} else {
		approvalReq.Priority = models.ApprovalPriorityNormal
	}

	approvalRequest, err := s.approvalService.CreateApprovalRequest(approvalReq, userID)
	if err != nil {
		// Log the error for debugging
		fmt.Printf("Failed to create approval request: %v\n", err)
		return fmt.Errorf("failed to create approval request: %v", err)
	}

	// Check if approvalRequest is nil
	if approvalRequest == nil {
		return errors.New("approval request creation returned nil")
	}

	// Update purchase with approval request ID
	purchase.ApprovalRequestID = &approvalRequest.ID
	_, err = s.purchaseRepo.Update(purchase)
	return err
}

func (s *PurchaseService) validateReceiptItems(receiptItems []models.PurchaseReceiptItemRequest, purchaseItems []models.PurchaseItem) error {
	for _, receiptItem := range receiptItems {
		found := false
		for _, purchaseItem := range purchaseItems {
			if purchaseItem.ID == receiptItem.PurchaseItemID {
				if receiptItem.QuantityReceived > purchaseItem.Quantity {
					return fmt.Errorf("received quantity cannot exceed ordered quantity for item %d", receiptItem.PurchaseItemID)
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("purchase item %d not found", receiptItem.PurchaseItemID)
		}
	}
	return nil
}

func (s *PurchaseService) updateReceiptStatus(receiptID uint) error {
	receipt, err := s.purchaseRepo.FindReceiptByID(receiptID)
	if err != nil {
		return err
	}

	// Get the purchase to check completion status
	purchase, err := s.purchaseRepo.FindByID(receipt.PurchaseID)
	if err != nil {
		return err
	}

	// Get all receipt items for this receipt
	receiptItems, err := s.purchaseRepo.GetReceiptItems(receiptID)
	if err != nil {
		return err
	}

	// Check if all purchase items are fully received
	allReceived := true
	for _, purchaseItem := range purchase.PurchaseItems {
		totalReceived := 0
		// Sum up all received quantities for this purchase item across all receipts
		for _, receiptItem := range receiptItems {
			if receiptItem.PurchaseItemID == purchaseItem.ID {
				totalReceived += receiptItem.QuantityReceived
			}
		}
		if totalReceived < purchaseItem.Quantity {
			allReceived = false
			break
		}
	}

	// Update receipt status
	if allReceived {
		receipt.Status = models.ReceiptStatusComplete
		// DO NOT automatically set purchase to COMPLETED here
		// Purchase should only be COMPLETED when both received AND paid
		// Keep purchase in APPROVED status until payment is complete
		// Payment completion logic in CreateIntegratedPayment will handle final status
	} else {
		receipt.Status = models.ReceiptStatusPartial
	}

	// Update receipt only - do not change purchase status based on receipt completion
	_, err = s.purchaseRepo.UpdateReceipt(receipt)
	if err != nil {
		return err
	}

	// Do not update purchase status here - it should only change when payment is made
	// The purchase status should remain APPROVED until fully paid

	return nil
}

func (s *PurchaseService) getDefaultCondition(condition string) string {
	if condition == "" {
		return models.ReceiptConditionGood
	}
	return condition
}

// createMinimalApprovalRequestForRejection creates a minimal approval request for rejection tracking without workflow dependency
func (s *PurchaseService) createMinimalApprovalRequestForRejection(purchase *models.Purchase, userID uint) error {
	// Ensure vendor is loaded
	vendorName := "Unknown"
	if purchase.Vendor.ID != 0 {
		// Vendor is already loaded
		vendorName = purchase.Vendor.Name
	} else {
		// Try to load vendor if not already loaded
		vendor, err := s.contactRepo.GetByID(purchase.VendorID)
		if err == nil {
			vendorName = vendor.Name
		}
	}

	// Create approval request directly in approval service without workflow dependency
	return s.approvalService.CreateMinimalApprovalRequestForRejection(
		models.EntityTypePurchase,
		purchase.ID,
		purchase.ApprovalBaseAmount,
		fmt.Sprintf("Purchase Rejection Tracking - %s (Vendor: %s)", purchase.Code, vendorName),
		userID,
		purchase,
	)
}

// getApprovalBasis reads basis from env var APPROVAL_AMOUNT_BASIS
func getApprovalBasis() string {
	basis := os.Getenv("APPROVAL_AMOUNT_BASIS")
	if basis == "" {
		return "SUBTOTAL_BEFORE_DISCOUNT"
	}
	return basis
}

// getPaymentMethod returns default payment method if empty
func getPaymentMethod(paymentMethod string) string {
	if paymentMethod == "" {
		return models.PurchasePaymentCredit // Default to credit
	}
	return paymentMethod
}

// getPPNRateFromPointer returns default PPN rate of 11% if nil, otherwise respects the provided value
// nil = default to 11%, 0 = no VAT (explicit zero), any other value = use as-is
func getPPNRateFromPointer(requestedRate *float64) float64 {
	if requestedRate == nil {
		// No rate provided in request - use configurable default from settings
		cfg := config2.GetAccountingConfig()
		if cfg != nil && cfg.TaxRates.DefaultPPN > 0 {
			return cfg.TaxRates.DefaultPPN
		}
		return 11.0
	}
	// Explicit rate provided (could be 0 for no VAT) - use as-is
	return *requestedRate
}

// getPPhRateFromPointer returns 0% if nil, otherwise respects the provided value
// nil = 0% (default), any other value = use as-is
func getPPhRateFromPointer(requestedRate *float64) float64 {
	if requestedRate == nil {
		// No rate provided - default to 0%
		return 0.0
	}
	// Explicit rate provided - use as-is
	return *requestedRate
}

// getPPNRateFromDoublePointer handles double pointer for update operations
func getPPNRateFromDoublePointer(requestedRate **float64) float64 {
	if requestedRate == nil || *requestedRate == nil {
		// No update to rate - use configurable default
		cfg := config2.GetAccountingConfig()
		if cfg != nil && cfg.TaxRates.DefaultPPN > 0 {
			return cfg.TaxRates.DefaultPPN
		}
		return 11.0 // fallback default
	}
	// Explicit rate provided - use as-is
	return **requestedRate
}

// getPPhRateFromDoublePointer handles double pointer for PPh rates in update operations
func getPPhRateFromDoublePointer(requestedRate **float64) float64 {
	if requestedRate == nil || *requestedRate == nil {
		// No update to rate - return 0% default
		return 0.0
	}
	// Explicit rate provided - use as-is
	return **requestedRate
}

// purchaseHasJournalEntries checks if a purchase already has associated journal entries
func (s *PurchaseService) purchaseHasJournalEntries(purchaseID uint) (bool, error) {
	if s.journalRepo == nil {
		return false, errors.New("journal repository not available")
	}

	// Use FindByReferenceID which is specifically designed for finding entries by reference
	ctx := context.Background()
	existingEntry, err := s.journalRepo.FindByReferenceID(ctx, models.JournalRefPurchase, purchaseID)
	if err != nil {
		return false, err
	}

	return existingEntry != nil, nil
}

// Purchase Payment Integration Methods

// CreateIntegratedPayment creates a payment in both Purchase and Payment Management systems
func (s *PurchaseService) CreateIntegratedPayment(
	purchaseID uint,
	amount float64,
	date time.Time,
	method string,
	cashBankID *uint,
	reference string,
	notes string,
	userID uint,
) (map[string]interface{}, error) {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get purchase with vendor details
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("purchase not found: %v", err)
	}

	// Validate purchase can receive payment
	if purchase.Status != models.PurchaseStatusApproved {
		tx.Rollback()
		return nil, errors.New("purchase must be approved to receive payment")
	}

	if purchase.PaymentMethod != models.PurchasePaymentCredit {
		tx.Rollback()
		return nil, errors.New("only credit purchases can receive payments")
	}

	if purchase.OutstandingAmount <= 0 {
		tx.Rollback()
		return nil, errors.New("purchase is already fully paid")
	}

	if amount <= 0 {
		tx.Rollback()
		return nil, errors.New("payment amount must be greater than zero")
	}

	if amount > purchase.OutstandingAmount {
		tx.Rollback()
		return nil, fmt.Errorf("payment amount (%.2f) exceeds outstanding amount (%.2f)", amount, purchase.OutstandingAmount)
	}

	// Validate cash/bank account is provided
	if cashBankID == nil {
		tx.Rollback()
		return nil, errors.New("cash/bank account is required for payment")
	}

	// Create payment record directly in Payment Management
	// Generate payment code
	paymentCode := fmt.Sprintf("PAY/%04d/%02d/%04d", date.Year(), date.Month(), time.Now().Unix()%9999)

	// Create payment record
	payment := &models.Payment{
		Code:      paymentCode,
		ContactID: purchase.VendorID,
		UserID:    userID,
		Date:      date,
		Amount:    amount,
		Method:    method,
		Reference: reference,
		Status:    models.PaymentStatusCompleted,
		Notes:     fmt.Sprintf("Payment for purchase %s. %s", purchase.Code, notes),
	}

	if err := tx.Create(payment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment: %v", err)
	}

	// Create payment allocation
	paymentAllocation := &models.PaymentAllocation{
		PaymentID:       uint64(payment.ID),
		BillID:          &purchaseID,
		AllocatedAmount: amount,
	}

	if err := tx.Create(paymentAllocation).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment allocation: %v", err)
	}

	// Create cross-reference record in purchase_payments
	purchasePayment := &models.PurchasePayment{
		PurchaseID:    purchaseID,
		PaymentNumber: payment.Code,
		Date:          date,
		Amount:        amount,
		Method:        method,
		Reference:     reference,
		Notes:         notes,
		CashBankID:    cashBankID,
		UserID:        userID,
		PaymentID:     &payment.ID, // Cross-reference to payments table
	}

	// Save purchase payment
	if err := tx.Create(purchasePayment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create purchase payment record: %v", err)
	}

	// Update purchase payment tracking
	purchase.PaidAmount += amount
	purchase.OutstandingAmount -= amount

	// Update purchase status if fully paid
	if purchase.OutstandingAmount <= 0.01 { // Allow for rounding errors
		purchase.Status = models.PurchaseStatusPaid
		purchase.OutstandingAmount = 0 // Ensure exact zero
	}

	// Save updated purchase
	if err := tx.Save(purchase).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update purchase: %v", err)
	}

	// Update cash/bank balance (decrease for payment OUT)
	if cashBankID != nil {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, *cashBankID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("cash/bank account not found: %v", err)
		}

		// Check sufficient balance for outgoing payment
		if cashBank.Balance < amount {
			tx.Rollback()
			return nil, fmt.Errorf("insufficient balance. Available: %.2f, Required: %.2f", cashBank.Balance, amount)
		}

		// Decrease cash/bank balance for payment OUT
		cashBank.Balance -= amount
		if err := tx.Save(&cashBank).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update cash/bank balance: %v", err)
		}

		// Create cash/bank transaction record
		cashBankTransaction := &models.CashBankTransaction{
			CashBankID:      *cashBankID,
			ReferenceType:   "PAYMENT",
			ReferenceID:     payment.ID,
			Amount:          -amount, // Negative for outgoing payment
			BalanceAfter:    cashBank.Balance,
			TransactionDate: date,
			Notes:           fmt.Sprintf("Vendor payment - %s", purchase.Code),
		}

		if err := tx.Create(cashBankTransaction).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create cash/bank transaction: %v", err)
		}
	}

	// Create journal entries for vendor payment
	// Debit: Accounts Payable (reduce liability)
	// Credit: Cash/Bank (reduce asset)
	if cashBankID != nil {
		if err := s.createVendorPaymentJournalEntries(tx, payment, purchase, *cashBankID, userID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create journal entries: %v", err)
		}
	} else {
		// Handle case where no cash/bank account specified
		// This shouldn't happen in normal flow but we need to handle it gracefully
		tx.Rollback()
		return nil, errors.New("cash/bank account is required for payment")
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Return success response
	return map[string]interface{}{
		"payment": map[string]interface{}{
			"id":     payment.ID,
			"code":   payment.Code,
			"amount": payment.Amount,
			"status": payment.Status,
		},
		"updated_purchase": map[string]interface{}{
			"id":                 purchase.ID,
			"status":             purchase.Status,
			"paid_amount":        purchase.PaidAmount,
			"outstanding_amount": purchase.OutstandingAmount,
		},
		"message": "Payment created successfully via Payment Management",
	}, nil
}

// GetPurchasePayments returns all payments for a purchase
func (s *PurchaseService) GetPurchasePayments(purchaseID uint) ([]models.PurchasePayment, error) {
	var payments []models.PurchasePayment
	err := s.db.Where("purchase_id = ?", purchaseID).
		Preload("CashBank").
		Preload("User").
		Order("date DESC").
		Find(&payments).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get purchase payments: %v", err)
	}

	return payments, nil
}

// isImmediatePayment checks if payment method requires immediate payment
func isImmediatePayment(paymentMethod string) bool {
	return paymentMethod == models.PurchasePaymentCash ||
		paymentMethod == models.PurchasePaymentTransfer ||
		paymentMethod == models.PurchasePaymentCheck
}

// updateProductStockOnApproval updates product stock when purchase is approved
func (s *PurchaseService) updateProductStockOnApproval(purchase *models.Purchase) error {
	fmt.Printf("📦 Starting stock update for purchase %s with %d items\n", purchase.Code, len(purchase.PurchaseItems))

	for _, item := range purchase.PurchaseItems {
		// Skip ASSET items - they don't have inventory/stock
		if item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset {
			fmt.Printf("ℹ️ Skipping stock update for ASSET item '%s' (PurchaseItem:%d)\n", item.Description, item.ID)
			continue
		}

		// Skip items without ProductID (safety check)
		if item.ProductID == nil || *item.ProductID == 0 {
			fmt.Printf("ℹ️ Skipping stock update for item without ProductID (PurchaseItem:%d)\n", item.ID)
			continue
		}

		// Get current product data
		product, err := s.productRepo.FindByID(*item.ProductID)
		if err != nil {
			fmt.Printf("❌ Error finding product %d: %v\n", *item.ProductID, err)
			continue // Skip this item but continue with others
		}

		fmt.Printf("📋 Product %d (%s): Current stock = %d, Adding quantity = %d\n",
			product.ID, product.Name, product.Stock, item.Quantity)

		// Update stock quantity (add purchased quantity)
		oldStock := product.Stock
		product.Stock += item.Quantity

		// Update cost price using weighted average if we have existing stock
		if oldStock > 0 {
			// Weighted average: (old_stock * old_price + new_qty * new_price) / total_qty
			totalValue := (float64(oldStock) * product.PurchasePrice) + (float64(item.Quantity) * item.UnitPrice)
			totalQuantity := oldStock + item.Quantity
			product.PurchasePrice = totalValue / float64(totalQuantity)
			fmt.Printf("💰 Updated weighted average price: %.2f (was %.2f)\n",
				product.PurchasePrice, (float64(oldStock)*product.PurchasePrice)/float64(oldStock))
		} else {
			// If no existing stock, use new price
			product.PurchasePrice = item.UnitPrice
			fmt.Printf("💰 Set new purchase price: %.2f\n", product.PurchasePrice)
		}

		// Save updated product
		err = s.productRepo.Update(context.Background(), product)
		if err != nil {
			fmt.Printf("❌ Failed to update product %d stock: %v\n", product.ID, err)
			return fmt.Errorf("failed to update stock for product %d: %v", product.ID, err)
		}

		fmt.Printf("✅ Product %d stock updated: %d → %d\n", product.ID, oldStock, product.Stock)
	}

	fmt.Printf("🎉 Stock update completed for purchase %s\n", purchase.Code)
	return nil
}

// createPaymentTrackingForCreditPurchase creates clean accounts payable tracking for approved credit purchases
// FIXED: No longer creates dummy PAYABLE records that pollute payment reports
func (s *PurchaseService) createPaymentTrackingForCreditPurchase(purchase *models.Purchase, userID uint) error {
	// Check if payment tracking already exists for this purchase
	existingPayments, err := s.GetPurchasePayments(purchase.ID)
	if err != nil {
		return fmt.Errorf("failed to check existing payments: %v", err)
	}

	// If payments already exist, don't create duplicates
	if len(existingPayments) > 0 {
		fmt.Printf("Payment tracking already exists for purchase %d, skipping creation\n", purchase.ID)
		return nil
	}

	// FIXED: Instead of creating dummy payment records, we just ensure
	// the purchase has correct outstanding amounts for accounts payable tracking
	// The purchase itself IS the accounts payable record

	// Verify purchase amounts are correctly set
	if purchase.OutstandingAmount != purchase.TotalAmount {
		fmt.Printf("🔧 Correcting purchase payment amounts: Outstanding=%.2f, should be %.2f\n",
			purchase.OutstandingAmount, purchase.TotalAmount)

		// Update purchase to have correct outstanding amount
		err := s.db.Model(purchase).Updates(map[string]interface{}{
			"outstanding_amount": purchase.TotalAmount,
			"paid_amount":        0,
		}).Error

		if err != nil {
			return fmt.Errorf("failed to update purchase payment amounts: %v", err)
		}
	}

	fmt.Printf("✅ Accounts payable tracking initialized for credit purchase %s: Outstanding=%.2f\n",
		purchase.Code, purchase.TotalAmount)

	return nil
}

// createVendorPaymentJournalEntries creates journal entries for vendor payment
// Debit: Accounts Payable (reduce liability)
// Credit: Cash/Bank (reduce asset)
func (s *PurchaseService) createVendorPaymentJournalEntries(tx *gorm.DB, payment *models.Payment, purchase *models.Purchase, cashBankID uint, userID uint) error {
	// Get accounts payable account (liability account)
	var apAccount models.Account
	if err := tx.Where("code = ?", "2101").First(&apAccount).Error; err != nil {
		// Fallback to search by name
		if err := tx.Where("LOWER(name) LIKE ?", "%utang%usaha%").First(&apAccount).Error; err != nil {
			return fmt.Errorf("accounts payable account not found: %v", err)
		}
	}

	// Get cash/bank account linked to GL
	var cashBank models.CashBank
	if err := tx.Preload("Account").First(&cashBank, cashBankID).Error; err != nil {
		return fmt.Errorf("cash/bank account not found: %v", err)
	}

	if cashBank.AccountID == 0 {
		return fmt.Errorf("cash/bank account %d is not linked to chart of accounts", cashBankID)
	}

	// Create journal entry
	journalEntry := &models.JournalEntry{
		EntryDate:       payment.Date,
		Description:     fmt.Sprintf("Vendor Payment %s - %s", payment.Code, purchase.Code),
		ReferenceType:   models.JournalRefPayment,
		ReferenceID:     &payment.ID,
		Reference:       payment.Code,
		UserID:          userID,
		Status:          models.JournalStatusPosted,
		TotalDebit:      payment.Amount,
		TotalCredit:     payment.Amount,
		IsAutoGenerated: true,
	}

	// Journal lines
	journalLines := []models.JournalLine{
		// Debit Accounts Payable (reduce liability)
		{
			AccountID:    apAccount.ID,
			Description:  fmt.Sprintf("Vendor payment - %s", payment.Code),
			DebitAmount:  payment.Amount,
			CreditAmount: 0,
		},
		// Credit Cash/Bank (reduce asset)
		{
			AccountID:    cashBank.AccountID,
			Description:  fmt.Sprintf("Payment out - %s", payment.Code),
			DebitAmount:  0,
			CreditAmount: payment.Amount,
		},
	}

	journalEntry.JournalLines = journalLines

	if err := tx.Create(journalEntry).Error; err != nil {
		return fmt.Errorf("failed to create journal entry: %v", err)
	}

	// Update account balances
	// Accounts Payable: Decrease by debiting (normal credit balance account)
	// UPDATE accounts SET balance = balance - amount WHERE id = ap_account_id
	if err := tx.Exec("UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		payment.Amount, apAccount.ID).Error; err != nil {
		return fmt.Errorf("failed to update accounts payable balance: %v", err)
	}

	// Cash/Bank: Decrease by crediting (normal debit balance account)
	// UPDATE accounts SET balance = balance - amount WHERE id = cash_account_id
	if err := tx.Exec("UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		payment.Amount, cashBank.AccountID).Error; err != nil {
		return fmt.Errorf("failed to update cash/bank account balance: %v", err)
	}

	return nil
}

// updateCashBankBalanceForPurchase updates cash/bank balance for immediate payment purchases
// This function should be called AFTER successful journal entry creation
func (s *PurchaseService) updateCashBankBalanceForPurchase(purchase *models.Purchase) error {
	fmt.Printf("💰 Updating cash/bank balance for immediate payment purchase %s (method: %s, amount: %.2f)\n",
		purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	// Only update balance for immediate payment methods
	if !isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("✅ Skipping cash/bank balance update for %s payment method\n", purchase.PaymentMethod)
		return nil // No balance update needed for credit purchases
	}

	// ✅ CRITICAL FIX: Determine cash/bank account with fallback
	var cashBankID uint

	if purchase.BankAccountID != nil && *purchase.BankAccountID > 0 {
		// Use specified bank account
		cashBankID = *purchase.BankAccountID
		fmt.Printf("ℹ️ Using specified bank account ID %d for balance update\n", cashBankID)
	} else {
		// ✅ FALLBACK: Find default cash_bank account based on payment method
		fmt.Printf("⚠️ No bank account specified, using fallback to default account\n")

		var defaultAccountCode string
		method := strings.ToUpper(strings.TrimSpace(purchase.PaymentMethod))
		if method == "CASH" || method == "TUNAI" {
			defaultAccountCode = "1101" // Kas
		} else {
			defaultAccountCode = "1101" // Bank accounts are also under 1101 (KAS DAN SETARA KAS) header — specific bank is resolved via CashBank entity
		}

		// Find account by code
		var account models.Account
		if err := s.db.Where("code = ? AND deleted_at IS NULL", defaultAccountCode).First(&account).Error; err != nil {
			fmt.Printf("❌ Default account %s not found: %v\n", defaultAccountCode, err)
			return fmt.Errorf("default account %s not found: %v", defaultAccountCode, err)
		}

		// Find cash_bank linked to this account
		var cashBank models.CashBank
		if err := s.db.Where("account_id = ? AND deleted_at IS NULL", account.ID).First(&cashBank).Error; err != nil {
			fmt.Printf("❌ Cash/Bank record for account %s not found: %v\n", defaultAccountCode, err)
			return fmt.Errorf("cash_bank record for account %s not found: %v", defaultAccountCode, err)
		}

		cashBankID = cashBank.ID
		fmt.Printf("✅ Using fallback cash/bank: %s (ID: %d, Code: %s)\n", cashBank.Name, cashBankID, defaultAccountCode)

		// Update purchase.BankAccountID for consistency
		purchase.BankAccountID = &cashBankID
	}

	// Get bank account details using determined cashBankID
	var cashBank models.CashBank
	if err := s.db.First(&cashBank, cashBankID).Error; err != nil {
		fmt.Printf("❌ Failed to retrieve bank account %d: %v\n", cashBankID, err)
		return fmt.Errorf("bank account not found: %v", err)
	}
	fmt.Printf("📋 Bank account retrieved: %s (Current Balance: %.2f)\n", cashBank.Name, cashBank.Balance)

	// Double-check balance is still sufficient (safety check)
	if cashBank.Balance < purchase.TotalAmount {
		fmt.Printf("❌ Insufficient balance during update: %s has %.2f but requires %.2f\n",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount)
		return fmt.Errorf("insufficient balance in %s. Available: %.2f, Required: %.2f",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount)
	}

	// Update balance - decrease for outgoing payment
	oldBalance := cashBank.Balance
	cashBank.Balance -= purchase.TotalAmount

	// Save updated balance
	if err := s.db.Save(&cashBank).Error; err != nil {
		fmt.Printf("❌ Failed to update cash/bank balance: %v\n", err)
		return fmt.Errorf("failed to update cash/bank balance: %v", err)
	}
	fmt.Printf("💰 Balance updated: %s %.2f → %.2f (decreased by %.2f)\n",
		cashBank.Name, oldBalance, cashBank.Balance, purchase.TotalAmount)

	// Create cash/bank transaction record for audit trail
	cashBankTransaction := &models.CashBankTransaction{
		CashBankID:      *purchase.BankAccountID,
		ReferenceType:   "PURCHASE",
		ReferenceID:     purchase.ID,
		Amount:          -purchase.TotalAmount, // Negative for outgoing payment
		BalanceAfter:    cashBank.Balance,
		TransactionDate: purchase.Date,
		Notes:           fmt.Sprintf("Payment for purchase %s - %s", purchase.Code, purchase.PaymentMethod),
	}

	if err := s.db.Create(cashBankTransaction).Error; err != nil {
		fmt.Printf("⚠️ Warning: Failed to create cash/bank transaction record: %v\n", err)
		// Don't fail the entire process for transaction record creation failure
	} else {
		fmt.Printf("📝 Cash/bank transaction record created: ID %d\n", cashBankTransaction.ID)
	}

	fmt.Printf("✅ Cash/bank balance update completed successfully for purchase %s\n", purchase.Code)
	return nil
}

// validateBankBalanceForPurchase validates that bank account has sufficient balance for immediate payment purchases
func (s *PurchaseService) validateBankBalanceForPurchase(purchase *models.Purchase) error {
	fmt.Printf("ℹ Validating bank balance for purchase %s (method: %s, amount: %.2f)\n", purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)
	// Only validate for immediate payment methods
	if !isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("✅ Skipping bank balance validation for %s payment method\n", purchase.PaymentMethod)
		return nil // No validation needed for credit purchases
	}

	// Check if bank account is specified
	fmt.Printf("ℹ Checking if bank account is specified for %s payment\n", purchase.PaymentMethod)
	if purchase.BankAccountID == nil {
		fmt.Printf("❌ Bank account ID is required but not provided for %s payment method\n", purchase.PaymentMethod)
		return fmt.Errorf("bank account is required for %s payment method", purchase.PaymentMethod)
	}
	fmt.Printf("✅ Bank account ID %d provided for validation\n", *purchase.BankAccountID)

	// Get bank account details
	fmt.Printf("ℹ Retrieving bank account details for ID %d\n", *purchase.BankAccountID)
	var cashBank models.CashBank
	if err := s.db.First(&cashBank, *purchase.BankAccountID).Error; err != nil {
		fmt.Printf("❌ Failed to retrieve bank account %d: %v\n", *purchase.BankAccountID, err)
		return fmt.Errorf("bank account not found: %v", err)
	}
	fmt.Printf("✅ Bank account retrieved: %s (Balance: %.2f)\n", cashBank.Name, cashBank.Balance)

	// Check if balance is sufficient
	fmt.Printf("ℹ Checking balance sufficiency: Available=%.2f, Required=%.2f\n", cashBank.Balance, purchase.TotalAmount)
	if cashBank.Balance < purchase.TotalAmount {
		fmt.Printf("❌ Insufficient balance: %s has %.2f but requires %.2f (shortfall: %.2f)\n",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount, purchase.TotalAmount-cashBank.Balance)
		return fmt.Errorf("insufficient balance in %s. Available: %.2f, Required: %.2f",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount)
	}

	fmt.Printf("✅ Bank balance validation passed: %s has sufficient balance (%.2f >= %.2f)\n",
		cashBank.Name, cashBank.Balance, purchase.TotalAmount)

	return nil
}

// UpdatePurchasePaymentAmounts updates purchase paid amounts and status after payment
func (s *PurchaseService) UpdatePurchasePaymentAmounts(purchaseID uint, paidAmount, outstandingAmount float64, status string) error {
	// Update purchase payment fields
	err := s.db.Model(&models.Purchase{}).Where("id = ?", purchaseID).Updates(map[string]interface{}{
		"paid_amount":        paidAmount,
		"outstanding_amount": outstandingAmount,
		"status":             status,
		"updated_at":         time.Now(),
	}).Error

	if err != nil {
		return fmt.Errorf("failed to update purchase payment amounts: %v", err)
	}

	return nil
}

// SSOT Journal Integration Methods

// createSSOTPurchaseJournalEntries creates journal entries using SSOT Journal System
func (s *PurchaseService) createSSOTPurchaseJournalEntries(purchase *models.Purchase, userID uint) error {
	if s.ssotJournalAdapter == nil {
		fmt.Printf("⚠️ SSOT Journal Adapter not initialized, falling back to legacy journal system\n")
		return s.createAndPostPurchaseJournalEntries(purchase, userID)
	}

	// Create SSOT journal entry
	ctx := context.Background()
	journalEntry, err := s.ssotJournalAdapter.CreatePurchaseJournalEntry(ctx, purchase, uint64(userID))
	if err != nil {
		fmt.Printf("⚠️ SSOT create failed (%v).\n", err)
		return fmt.Errorf("failed SSOT: %v", err)
	}

	fmt.Printf("✅ SSOT Journal Entry created: %s (ID: %d) for purchase %s\n",
		journalEntry.EntryNumber, journalEntry.ID, purchase.Code)

	return nil
}

// purchaseHasSSOTJournalEntries checks if SSOT journal entries exist for a purchase
func (s *PurchaseService) purchaseHasSSOTJournalEntries(purchaseID uint) (bool, error) {
	if s.ssotJournalAdapter == nil {
		// Fall back to legacy check
		return s.purchaseHasJournalEntries(purchaseID)
	}

	ctx := context.Background()
	entries, err := s.ssotJournalAdapter.GetPurchaseJournalEntries(ctx, uint64(purchaseID))
	if err != nil {
		return false, err
	}

	return len(entries) > 0, nil
}

// CreatePurchasePaymentJournal creates journal entry for purchase payment using SSOT
func (s *PurchaseService) CreatePurchasePaymentJournal(
	purchaseID uint,
	paymentAmount float64,
	bankAccountID uint,
	reference string,
	notes string,
	userID uint,
) error {
	purchase, err := s.GetPurchaseByID(purchaseID)
	if err != nil {
		return fmt.Errorf("failed to get purchase: %v", err)
	}

	if s.ssotJournalAdapter == nil {
		fmt.Printf("⚠️ SSOT Journal Adapter not initialized for payment journal\n")
		return fmt.Errorf("SSOT journal adapter not available")
	}

	ctx := context.Background()
	paymentAmountDecimal := decimal.NewFromFloat(paymentAmount)

	_, err = s.ssotJournalAdapter.CreatePurchasePaymentJournalEntry(
		ctx,
		purchase,
		paymentAmountDecimal,
		uint64(bankAccountID),
		uint64(userID),
		reference,
		notes,
	)

	if err != nil {
		return fmt.Errorf("failed to create payment journal entry: %v", err)
	}

	fmt.Printf("✅ Purchase payment journal entry created successfully for purchase %d\n", purchaseID)
	return nil
}

// GetPurchaseJournalEntries retrieves SSOT journals related to a purchase
// We include both PURCHASE journals and PAYMENT journals tied to the purchase ID
func (s *PurchaseService) GetPurchaseJournalEntries(purchaseID uint) ([]models.SSOTJournalEntry, error) {
	if s.ssotJournalAdapter == nil {
		return nil, fmt.Errorf("SSOT journal adapter not available")
	}

	ctx := context.Background()
	entries, err := s.ssotJournalAdapter.GetPurchaseRelatedJournalEntries(ctx, uint64(purchaseID), true)
	if err != nil {
		return nil, fmt.Errorf("failed to get purchase-related journal entries: %v", err)
	}

	return entries, nil
}

// GetDB returns the database connection for external services
func (s *PurchaseService) GetDB() *gorm.DB {
	return s.db
}

// CreateAndPostLegacyForCLI exposes legacy posting for CLI scripts (backfill)
func (s *PurchaseService) CreateAndPostLegacyForCLI(purchase *models.Purchase) error {
	return s.createAndPostPurchaseJournalEntries(purchase, purchase.UserID)
}
