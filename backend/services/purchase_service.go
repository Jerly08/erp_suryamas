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
	"gorm.io/gorm/clause"
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
	stockMonitoringService *StockMonitoringService
}

type PurchaseResult struct {
	Data       []models.Purchase `json:"data"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalPages int               `json:"total_pages"`
}

func roundPurchaseMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func purchasePPhTotal(purchase *models.Purchase) float64 {
	if purchase == nil {
		return 0
	}
	return roundPurchaseMoney(purchase.PPh21Amount + purchase.PPh23Amount)
}

func purchasePPhPaymentStatus(totalPPh float64, paidAmount float64) string {
	totalPPh = roundPurchaseMoney(totalPPh)
	paidAmount = roundPurchaseMoney(paidAmount)
	if totalPPh <= 0.01 {
		return models.PurchasePPhPaymentStatusNone
	}
	if paidAmount >= totalPPh-0.01 {
		return models.PurchasePPhPaymentStatusPaid
	}
	if paidAmount > 0.01 {
		return models.PurchasePPhPaymentStatusPartialPaid
	}
	return models.PurchasePPhPaymentStatusUnpaid
}

func purchaseTermDefaultDueDate(purchase *models.Purchase) time.Time {
	if purchase != nil && !purchase.DueDate.IsZero() {
		return purchase.DueDate
	}
	if purchase != nil && !purchase.Date.IsZero() {
		return purchase.Date
	}
	return time.Now()
}

func buildPurchasePaymentTerms(purchase *models.Purchase, requests []models.PurchasePaymentTermRequest) ([]models.PurchasePaymentTerm, error) {
	if purchase == nil {
		return nil, fmt.Errorf("purchase is required")
	}

	totalAmount := roundPurchaseMoney(purchase.TotalAmount)
	defaultDueDate := purchaseTermDefaultDueDate(purchase)
	if totalAmount <= 0.01 {
		return nil, fmt.Errorf("purchase total amount must be greater than zero before creating payment terms")
	}

	if len(requests) == 0 {
		return []models.PurchasePaymentTerm{{
			PurchaseID:        purchase.ID,
			Sequence:          1,
			Percentage:        100,
			Amount:            totalAmount,
			DueDate:           defaultDueDate,
			Description:       "Termin 1",
			Status:            models.PurchasePaymentTermPending,
			OutstandingAmount: totalAmount,
		}}, nil
	}

	terms := make([]models.PurchasePaymentTerm, 0, len(requests))
	runningTotal := 0.0
	for index, request := range requests {
		sequence := request.Sequence
		if sequence <= 0 {
			sequence = index + 1
		}

		amount := roundPurchaseMoney(request.Amount)
		percentage := request.Percentage
		if amount <= 0 && percentage > 0 {
			amount = roundPurchaseMoney(totalAmount * percentage / 100)
		}
		if amount <= 0 {
			return nil, fmt.Errorf("payment term %d amount must be greater than zero", sequence)
		}
		if percentage <= 0 {
			percentage = roundPurchaseMoney(amount / totalAmount * 100)
		}

		dueDate := request.DueDate
		if dueDate.IsZero() {
			dueDate = defaultDueDate
		}

		description := strings.TrimSpace(request.Description)
		if description == "" {
			description = fmt.Sprintf("Termin %d", sequence)
		}

		terms = append(terms, models.PurchasePaymentTerm{
			PurchaseID:        purchase.ID,
			Sequence:          sequence,
			Percentage:        percentage,
			Amount:            amount,
			DueDate:           dueDate,
			Description:       description,
			Status:            models.PurchasePaymentTermPending,
			OutstandingAmount: amount,
		})
		runningTotal = roundPurchaseMoney(runningTotal + amount)
	}

	if math.Abs(runningTotal-totalAmount) > 0.01 {
		return nil, fmt.Errorf("total payment terms %.2f must equal purchase total %.2f", runningTotal, totalAmount)
	}

	// Absorb one-cent rounding residue into the final term.
	if len(terms) > 0 && runningTotal != totalAmount {
		diff := roundPurchaseMoney(totalAmount - runningTotal)
		terms[len(terms)-1].Amount = roundPurchaseMoney(terms[len(terms)-1].Amount + diff)
	}

	return terms, nil
}

func hydratePurchasePaymentTermsWithAllocations(tx *gorm.DB, purchase *models.Purchase) error {
	if purchase == nil || len(purchase.PaymentTerms) == 0 {
		return nil
	}

	termIDs := make([]uint, 0, len(purchase.PaymentTerms))
	for _, term := range purchase.PaymentTerms {
		termIDs = append(termIDs, term.ID)
	}

	termPaidAmounts := map[uint]float64{}
	if len(termIDs) > 0 {
		type termAllocationTotal struct {
			TermID uint
			Total  float64
		}
		var totals []termAllocationTotal
		if err := tx.Model(&models.PaymentAllocation{}).
			Select("purchase_payment_term_id AS term_id, COALESCE(SUM(allocated_amount), 0) AS total").
			Where("purchase_payment_term_id IN ?", termIDs).
			Group("purchase_payment_term_id").
			Scan(&totals).Error; err != nil {
			return err
		}
		for _, total := range totals {
			termPaidAmounts[total.TermID] = roundPurchaseMoney(total.Total)
		}
	}

	genericPaidAmount := 0.0
	if err := tx.Model(&models.PaymentAllocation{}).
		Where("bill_id = ? AND purchase_payment_term_id IS NULL", purchase.ID).
		Select("COALESCE(SUM(allocated_amount), 0)").
		Scan(&genericPaidAmount).Error; err != nil {
		return err
	}
	genericPaidAmount = roundPurchaseMoney(genericPaidAmount)

	for index := range purchase.PaymentTerms {
		term := &purchase.PaymentTerms[index]
		paidAmount := termPaidAmounts[term.ID]
		if genericPaidAmount > 0 {
			remainingForTerm := roundPurchaseMoney(term.Amount - paidAmount)
			if remainingForTerm > 0 {
				allocation := math.Min(remainingForTerm, genericPaidAmount)
				paidAmount = roundPurchaseMoney(paidAmount + allocation)
				genericPaidAmount = roundPurchaseMoney(genericPaidAmount - allocation)
			}
		}

		if paidAmount > term.Amount && paidAmount-term.Amount <= 0.01 {
			paidAmount = term.Amount
		}
		term.PaidAmount = roundPurchaseMoney(math.Min(paidAmount, term.Amount))
		term.OutstandingAmount = roundPurchaseMoney(math.Max(term.Amount-term.PaidAmount, 0))
		switch {
		case term.OutstandingAmount <= 0.01:
			term.Status = models.PurchasePaymentTermPaid
			term.OutstandingAmount = 0
		case term.PaidAmount > 0.01:
			term.Status = models.PurchasePaymentTermPartial
		default:
			term.Status = models.PurchasePaymentTermPending
		}
	}

	return nil
}

func (s *PurchaseService) loadAndHydratePurchasePaymentTerms(tx *gorm.DB, purchase *models.Purchase) error {
	if purchase == nil {
		return nil
	}
	if !tx.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		return nil
	}
	if err := tx.Where("purchase_id = ?", purchase.ID).
		Order("sequence ASC, due_date ASC, id ASC").
		Find(&purchase.PaymentTerms).Error; err != nil {
		return err
	}
	return hydratePurchasePaymentTermsWithAllocations(tx, purchase)
}

func (s *PurchaseService) syncPurchasePaymentTerms(purchase *models.Purchase, requests []models.PurchasePaymentTermRequest) error {
	if purchase == nil {
		return nil
	}
	if !s.db.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if !models.IsCreditPurchaseMethod(purchase.PaymentMethod) {
			return tx.Where("purchase_id = ?", purchase.ID).Delete(&models.PurchasePaymentTerm{}).Error
		}

		terms, err := buildPurchasePaymentTerms(purchase, requests)
		if err != nil {
			return err
		}
		if err := tx.Where("purchase_id = ?", purchase.ID).Delete(&models.PurchasePaymentTerm{}).Error; err != nil {
			return err
		}
		for index := range terms {
			if err := tx.Create(&terms[index]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *PurchaseService) ensurePurchasePaymentTermsForCredit(purchase *models.Purchase) error {
	if purchase == nil || !models.IsCreditPurchaseMethod(purchase.PaymentMethod) {
		return nil
	}
	if !s.db.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		return nil
	}
	var count int64
	if err := s.db.Model(&models.PurchasePaymentTerm{}).
		Where("purchase_id = ? AND deleted_at IS NULL", purchase.ID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.syncPurchasePaymentTerms(purchase, nil)
}

func (s *PurchaseService) GetPurchasePaymentTermOutstanding(purchaseID uint, termID uint) (float64, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return 0, err
	}
	if err := s.loadAndHydratePurchasePaymentTerms(s.db, purchase); err != nil {
		return 0, err
	}
	for _, term := range purchase.PaymentTerms {
		if term.ID == termID {
			return roundPurchaseMoney(term.OutstandingAmount), nil
		}
	}
	return 0, fmt.Errorf("payment term %d does not belong to purchase %d", termID, purchaseID)
}

func (s *PurchaseService) SyncPurchasePaymentTermProgress(purchaseID uint) error {
	if !s.db.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var purchase models.Purchase
		if err := tx.First(&purchase, purchaseID).Error; err != nil {
			return err
		}
		if err := s.loadAndHydratePurchasePaymentTerms(tx, &purchase); err != nil {
			return err
		}
		for _, term := range purchase.PaymentTerms {
			if err := tx.Model(&models.PurchasePaymentTerm{}).
				Where("id = ?", term.ID).
				Updates(map[string]interface{}{
					"status":     term.Status,
					"updated_at": time.Now(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func syncPurchasePPhPaymentStatus(purchase *models.Purchase) {
	if purchase == nil {
		return
	}
	totalPPh := purchasePPhTotal(purchase)
	if totalPPh <= 0.01 {
		purchase.PPhPaidAmount = 0
		purchase.PPhPaymentStatus = models.PurchasePPhPaymentStatusNone
		return
	}
	purchase.PPhPaidAmount = roundPurchaseMoney(math.Min(math.Max(purchase.PPhPaidAmount, 0), totalPPh))
	purchase.PPhPaymentStatus = purchasePPhPaymentStatus(totalPPh, purchase.PPhPaidAmount)
}

func parsePurchasePPhPaymentDate(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("payment_date is required")
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return &parsed, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
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
	stockMonitoringService *StockMonitoringService,
) *PurchaseService {
	// Initialize SSOT Journal Adapter (wired to unified journal service)
	// Use TaxAccountService so account mapping is flexible (no hardcoded codes)
	taxAccountService := NewTaxAccountService(db)
	ssotAdapter := NewPurchaseSSOTJournalAdapter(db, unifiedJournalService, accountRepo, taxAccountService)

	ps := &PurchaseService{
		db:                     db,
		purchaseRepo:           purchaseRepo,
		productRepo:            productRepo,
		contactRepo:            contactRepo,
		accountRepo:            accountRepo,
		approvalService:        approvalService,
		journalService:         journalService,
		journalRepo:            journalRepo,
		pdfService:             pdfService,
		unifiedJournalService:  unifiedJournalService,
		ssotJournalAdapter:     ssotAdapter,
		journalServiceSSOT:     purchaseJournalServiceSSOT,
		coaService:             coaService,
		stockMonitoringService: stockMonitoringService,
	}
	// Asset capitalization service (reuses existing repos and unified journal)
	ps.assetCapitalizationSvc = NewAssetCapitalizationService(db, accountRepo, unifiedJournalService, journalRepo)

	// Setup post-approval callback
	if approvalService != nil {
		approvalService.SetPostApprovalCallback(ps)
		fmt.Printf("? Post-approval callback setup completed\n")
	}

	return ps
}

// Purchase CRUD Operations

func (s *PurchaseService) GetPurchases(filter models.PurchaseFilter) (*PurchaseResult, error) {
	fmt.Printf("? Retrieving purchases with filter: Status=%s, VendorID=%v, Page=%d, Limit=%d\n",
		filter.Status, filter.VendorID, filter.Page, filter.Limit)
	purchases, total, err := s.purchaseRepo.FindWithFilter(filter)
	if err != nil {
		fmt.Printf("? Failed to retrieve purchases: %v\n", err)
		return nil, fmt.Errorf("failed to retrieve purchases: %v", err)
	}
	for i := range purchases {
		if err := s.loadAndHydratePurchasePaymentTerms(s.db, &purchases[i]); err != nil {
			fmt.Printf("?? Failed to hydrate payment terms for purchase %d: %v\n", purchases[i].ID, err)
		}
	}
	fmt.Printf("? Retrieved %d purchases (total: %d)\n", len(purchases), total)

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
	fmt.Printf("? Retrieving purchase by ID: %d\n", id)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("? Failed to retrieve purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}
	if err := s.loadAndHydratePurchasePaymentTerms(s.db, purchase); err != nil {
		fmt.Printf("?? Failed to hydrate payment terms for purchase %d: %v\n", id, err)
	}
	fmt.Printf("? Purchase %d retrieved: %s (Status: %s)\n", id, purchase.Code, purchase.Status)
	return purchase, nil
}

func (s *PurchaseService) isPurchaseCOAReadOnlyUser(userID uint) bool {
	var user models.User
	if err := s.db.Select("role").First(&user, userID).Error; err != nil {
		fmt.Printf("?? Could not resolve role for purchase COA restriction (user %d): %v\n", userID, err)
		return false
	}
	role := models.NormalizeRole(user.Role)
	return role != models.RoleAdmin && !models.IsAccountingRole(role)
}

func (s *PurchaseService) CreatePurchase(request models.PurchaseCreateRequest, userID uint) (*models.Purchase, error) {
	// Validate vendor exists
	vendor, err := s.contactRepo.GetByID(request.VendorID)
	if err != nil {
		fmt.Printf("? Vendor validation failed for ID %d: %v\n", request.VendorID, err)
		return nil, fmt.Errorf("vendor not found (ID: %d): %v", request.VendorID, err)
	}
	fmt.Printf("? Vendor validation passed: %s (ID: %d)\n", vendor.Name, vendor.ID)

	coaReadOnlyUser := s.isPurchaseCOAReadOnlyUser(userID)
	if coaReadOnlyUser {
		request.CreditAccountID = nil
		request.OtherCostAccountID = nil
		for i := range request.Items {
			request.Items[i].ExpenseAccountID = 0
		}
	}

	request.PaymentMethod = getPaymentMethod(strings.ToUpper(strings.TrimSpace(request.PaymentMethod)))
	if coaReadOnlyUser && request.PaymentMethod == models.PurchasePaymentCredit {
		request.BankAccountID = nil
		request.CreditAccountID = nil
	} else {
		request.BankAccountID, request.CreditAccountID, err = s.normalizePurchasePaymentConfiguration(
			request.PaymentMethod,
			request.BankAccountID,
			request.CreditAccountID,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := s.validatePurchaseOtherCostAccount(request.OtherCostAccountID, request.OtherCostAmount); err != nil {
		return nil, err
	}

	// Generate purchase code
	code, err := s.generatePurchaseCode()
	if err != nil {
		fmt.Printf("? Failed to generate purchase code for vendor %d: %v\n", request.VendorID, err)
		return nil, fmt.Errorf("failed to generate purchase code: %v", err)
	}

	fmt.Printf("? Creating purchase entity with code %s for vendor %d\n", code, request.VendorID)
	// Create purchase entity
	purchase := &models.Purchase{
		Code:     code,
		VendorID: request.VendorID,
		UserID:   userID,
		Date:     request.Date,
		DueDate:  request.DueDate,
		Discount: request.Discount,
		// Payment method fields
		PaymentMethod:    request.PaymentMethod,
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
		OtherTaxAdditions:    request.OtherTaxAdditions,
		PPh21Rate:            getPPhRateFromPointer(request.PPh21Rate),
		PPh23Rate:            getPPhRateFromPointer(request.PPh23Rate),
		OtherTaxDeductions:   request.OtherTaxDeductions,
		FreightCost:          request.FreightCost,
		OtherCostAmount:      request.OtherCostAmount,
		OtherCostDescription: strings.TrimSpace(request.OtherCostDescription),
		OtherCostAccountID:   request.OtherCostAccountID,
		Status:               models.PurchaseStatusDraft,
		Notes:                request.Notes,
		ApprovalStatus:       models.PurchaseApprovalNotStarted,
		RequiresApproval:     false,
		// Initialize payment tracking fields
		PaidAmount:        0,
		OutstandingAmount: 0, // Will be set after total calculation
		MatchingStatus:    models.PurchaseMatchingPending,
	}

	// Assign PR and Project if linked
	purchase.PurchaseRequestID = request.PurchaseRequestID
	purchase.ProjectID = request.ProjectID

	// Calculate totals and create purchase items
	fmt.Printf("? Calculating purchase totals for %d items\n", len(request.Items))
	err = s.calculatePurchaseTotals(purchase, request.Items)
	if err != nil {
		fmt.Printf("? Failed to calculate purchase totals: %v\n", err)
		return nil, fmt.Errorf("failed to calculate purchase totals: %v", err)
	}
	fmt.Printf("? Purchase totals calculated: Subtotal=%.2f, Tax=%.2f, Total=%.2f\n", purchase.SubtotalBeforeDiscount, purchase.TaxAmount, purchase.TotalAmount)

	// INVOICE MATCHING: HARD BLOCK VALIDATION
	// If this purchase is linked to a Purchase Request, validate against the selected approved items.
	if purchase.PurchaseRequestID != nil {
		pr, selectedApprovedBudget, err := s.validateLinkedPurchaseRequestSelection(purchase, false)
		if err != nil {
			return nil, err
		}

		// Inherit ProjectID from PR if the request didn't explicitly set one
		if purchase.ProjectID == nil && pr.ProjectID != nil {
			purchase.ProjectID = pr.ProjectID
		}
		if purchase.DueDate.IsZero() && pr.DueDate != nil && getPaymentMethod(purchase.PaymentMethod) == models.PurchasePaymentCredit {
			purchase.DueDate = *pr.DueDate
		}

		// Menghitung Base Net Amount (Subtotal - Diskon) tanpa PPN dan Pajak Lainnya
		netPurchaseAmount := purchase.SubtotalBeforeDiscount - purchase.Discount

		// Hard Block Rule: Tolerance 0%. Net Invoice must be <= selected approved PR items budget.
		if netPurchaseAmount > selectedApprovedBudget {
			errMsg := fmt.Sprintf("HARD BLOCK: Base Invoice Amount (%.2f) exceeds selected approved Purchase Request budget (%.2f). Note: Taxes are excluded from this limit check. Please revise invoice or request a new PR approval.", netPurchaseAmount, selectedApprovedBudget)
			fmt.Printf("? %s\n", errMsg)
			return nil, errors.New(errMsg)
		}

		// Set Matching Status since it passed
		purchase.MatchingStatus = "MATCHED"
	}

	// Determine approval basis and base amount for later use
	fmt.Printf("? Determining approval requirements for purchase with amount %.2f\n", purchase.TotalAmount)
	if s.approvalService != nil {
		s.setApprovalBasisAndBase(purchase)
		fmt.Printf("? Approval basis set: RequiresApproval=%t, ApprovalBase=%.2f\n", purchase.RequiresApproval, purchase.ApprovalBaseAmount)
	} else {
		// For testing purposes, set default values
		fmt.Printf("? No approval service available, setting default values\n")
		purchase.RequiresApproval = false
		purchase.ApprovalStatus = models.PurchaseApprovalNotRequired
	}

	// Save purchase, status will remain DRAFT
	fmt.Printf("? Saving purchase to database with status DRAFT\n")
	createdPurchase, err := s.purchaseRepo.Create(purchase)
	if err != nil {
		fmt.Printf("? Failed to save purchase to database: %v\n", err)
		return nil, fmt.Errorf("failed to save purchase to database: %v", err)
	}
	fmt.Printf("? Purchase %d saved successfully with code %s\n", createdPurchase.ID, createdPurchase.Code)

	if err := s.syncPurchasePaymentTerms(createdPurchase, request.PaymentTerms); err != nil {
		fmt.Printf("? Failed to sync payment terms for purchase %d: %v\n", createdPurchase.ID, err)
		return nil, fmt.Errorf("failed to sync payment terms: %v", err)
	}

	if err := s.syncLinkedPurchaseRequestItems(createdPurchase); err != nil {
		fmt.Printf("? Failed to sync linked purchase request items for purchase %d: %v\n", createdPurchase.ID, err)
		return nil, fmt.Errorf("failed to reserve linked purchase request items: %v", err)
	}

	// NEW LOGIC: ALL purchases must go through approval workflow
	// No more auto-approval for immediate payments - Employee ? Finance workflow applies to all
	fmt.Printf("?? Purchase %s created (method: %s) - will go through approval workflow\n", purchase.Code, purchase.PaymentMethod)
	fmt.Printf("?? Status remains DRAFT until approval workflow is initiated\n")

	// Note: Bank balance validation and payment processing will happen AFTER approval
	// This ensures proper approval control for all purchase types

	return s.GetPurchaseByID(createdPurchase.ID)
}

func (s *PurchaseService) UpdatePurchase(id uint, request models.PurchaseUpdateRequest, userID uint) (*models.Purchase, error) {
	fmt.Printf("? Starting purchase update for ID %d by user %d\n", id, userID)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("? Failed to find purchase %d for update: %v\n", id, err)
		return nil, fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}

	// Check if purchase can be updated
	fmt.Printf("? Checking if purchase %d can be updated (current status: %s)\n", id, purchase.Status)
	if purchase.Status != models.PurchaseStatusDraft && purchase.Status != models.PurchaseStatusPending {
		fmt.Printf("? Purchase %d cannot be updated in current status: %s\n", id, purchase.Status)
		return nil, fmt.Errorf("purchase cannot be updated in current status: %s", purchase.Status)
	}
	fmt.Printf("? Purchase %d status validation passed for update\n", id)

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
	if request.FreightCost != nil {
		purchase.FreightCost = *request.FreightCost
	}
	if request.OtherCostAmount != nil {
		purchase.OtherCostAmount = *request.OtherCostAmount
	}
	if request.OtherCostDescription != nil {
		purchase.OtherCostDescription = strings.TrimSpace(*request.OtherCostDescription)
	}
	if request.OtherCostAccountID != nil {
		if *request.OtherCostAccountID == 0 {
			purchase.OtherCostAccountID = nil
		} else {
			purchase.OtherCostAccountID = request.OtherCostAccountID
		}
	}
	// Update payment method fields
	if request.PaymentMethod != nil {
		purchase.PaymentMethod = getPaymentMethod(strings.ToUpper(strings.TrimSpace(*request.PaymentMethod)))
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

	coaReadOnlyUser := s.isPurchaseCOAReadOnlyUser(userID)
	if coaReadOnlyUser {
		purchase.CreditAccountID = nil
		purchase.OtherCostAccountID = nil
		if len(request.Items) > 0 {
			for i := range request.Items {
				request.Items[i].ExpenseAccountID = 0
			}
		}
	}

	if coaReadOnlyUser && purchase.PaymentMethod == models.PurchasePaymentCredit {
		purchase.BankAccountID = nil
		purchase.CreditAccountID = nil
	} else {
		purchase.BankAccountID, purchase.CreditAccountID, err = s.normalizePurchasePaymentConfiguration(
			purchase.PaymentMethod,
			purchase.BankAccountID,
			purchase.CreditAccountID,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := s.validatePurchaseOtherCostAccount(purchase.OtherCostAccountID, purchase.OtherCostAmount); err != nil {
		return nil, err
	}

	// Update items if provided
	if len(request.Items) > 0 {
		err = s.updatePurchaseItems(purchase, request.Items)
		if err != nil {
			return nil, err
		}
	}

	// Recalculate totals
	fmt.Printf("? Recalculating totals for updated purchase %d\n", id)
	err = s.recalculatePurchaseTotals(purchase)
	if err != nil {
		fmt.Printf("? Failed to recalculate totals for purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("failed to recalculate purchase totals: %v", err)
	}
	fmt.Printf("? Purchase %d totals recalculated: Subtotal=%.2f, Tax=%.2f, Total=%.2f\n", id, purchase.SubtotalBeforeDiscount, purchase.TaxAmount, purchase.TotalAmount)

	// INVOICE MATCHING: HARD BLOCK VALIDATION FOR UPDATE
	if purchase.PurchaseRequestID != nil {
		_, selectedApprovedBudget, err := s.validateLinkedPurchaseRequestSelection(purchase, true)
		if err != nil {
			return nil, err
		}

		// Menghitung Base Net Amount (Subtotal - Diskon) tanpa PPN dan Pajak Lainnya
		netPurchaseAmount := purchase.SubtotalBeforeDiscount - purchase.Discount

		// Hard Block Rule
		if netPurchaseAmount > selectedApprovedBudget {
			errMsg := fmt.Sprintf("HARD BLOCK: Updated Base Invoice Amount (%.2f) exceeds selected approved Purchase Request budget (%.2f). Note: Taxes are excluded from this limit check. Update rejected.", netPurchaseAmount, selectedApprovedBudget)
			fmt.Printf("? %s\n", errMsg)
			return nil, errors.New(errMsg)
		}
		purchase.MatchingStatus = "MATCHED"
	}

	// Re-evaluate approval base
	s.setApprovalBasisAndBase(purchase)

	// Save updated purchase
	fmt.Printf("? Saving updated purchase %d to database\n", id)
	updatedPurchase, err := s.purchaseRepo.Update(purchase)
	if err != nil {
		fmt.Printf("? Failed to save updated purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("failed to save updated purchase: %v", err)
	}
	fmt.Printf("? Purchase %d updated successfully\n", id)

	if request.PaymentTerms != nil || request.PaymentMethod != nil {
		if err := s.syncPurchasePaymentTerms(updatedPurchase, request.PaymentTerms); err != nil {
			fmt.Printf("? Failed to sync payment terms for updated purchase %d: %v\n", id, err)
			return nil, fmt.Errorf("failed to sync payment terms: %v", err)
		}
	}

	if err := s.syncLinkedPurchaseRequestItems(updatedPurchase); err != nil {
		fmt.Printf("? Failed to sync linked purchase request items for updated purchase %d: %v\n", id, err)
		return nil, fmt.Errorf("failed to sync linked purchase request items: %v", err)
	}

	return s.GetPurchaseByID(updatedPurchase.ID)
}

func (s *PurchaseService) DeletePurchase(id uint) error {
	fmt.Printf("? Starting purchase cancellation for ID %d (cancel instead of delete)\n", id)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("? Failed to find purchase %d for cancellation: %v\n", id, err)
		return fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}

	// If already cancelled, nothing to do
	if purchase.Status == models.PurchaseStatusCancelled {
		fmt.Printf("? Purchase %d is already CANCELLED, nothing to do\n", id)
		return nil
	}

	// Guard: check if purchase date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return err
	}

	// For non-DRAFT purchases that have been approved/completed/paid,
	// perform full accounting reversal (stock, cash/bank, journal, project cost)
	if purchase.Status != models.PurchaseStatusDraft && purchase.Status != models.PurchaseStatusPending {
		fmt.Printf("?? Purchase %d has status %s, performing full accounting reversal via OnPurchaseRejected\n", id, purchase.Status)
		if err := s.OnPurchaseRejected(id); err != nil {
			fmt.Printf("? Post-rejection cleanup failed for purchase %d: %v\n", id, err)
			return fmt.Errorf("failed to reverse approved purchase %d before cancellation: %v", id, err)
		} else {
			fmt.Printf("? Full accounting reversal completed for purchase %d\n", id)
		}
	} else {
		fmt.Printf("? Purchase %d is in %s status, no accounting reversal needed\n", id, purchase.Status)
	}

	// Set status to CANCELLED (do NOT soft-delete - maintain audit trail)
	now := time.Now()
	purchase.Status = models.PurchaseStatusCancelled
	purchase.ApprovalStatus = models.PurchaseApprovalRejected
	purchase.UpdatedAt = now
	purchase.OutstandingAmount = 0
	purchase.PaidAmount = 0

	_, err = s.purchaseRepo.Update(purchase)
	if err != nil {
		fmt.Printf("? Failed to update purchase %d status to CANCELLED: %v\n", id, err)
		return fmt.Errorf("failed to cancel purchase: %v", err)
	}

	if err := s.releaseLinkedPurchaseRequestItems(purchase); err != nil {
		fmt.Printf("? Failed to release linked purchase request items for purchase %d: %v\n", id, err)
		return fmt.Errorf("failed to release linked purchase request items: %v", err)
	}

	fmt.Printf("? Purchase %d cancelled successfully (status: CANCELLED, audit trail preserved)\n", id)
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
	fmt.Printf("? Starting approval submission for purchase %d by user %d\n", id, userID)
	purchase, err := s.purchaseRepo.FindByID(id)
	if err != nil {
		fmt.Printf("? Failed to find purchase %d for approval submission: %v\n", id, err)
		return fmt.Errorf("purchase not found (ID: %d): %v", id, err)
	}

	fmt.Printf("? Checking if purchase %d can be submitted for approval (current status: %s)\n", id, purchase.Status)
	if purchase.Status != models.PurchaseStatusDraft {
		fmt.Printf("? Purchase %d cannot be submitted for approval in current status: %s\n", id, purchase.Status)
		return fmt.Errorf("only draft purchases can be submitted for approval, current status: %s", purchase.Status)
	}

	// Ensure approval base up-to-date
	fmt.Printf("? Setting approval basis for purchase %d (amount: %.2f)\n", id, purchase.TotalAmount)
	s.setApprovalBasisAndBase(purchase)
	// NEW LOGIC: Since all purchases now require approval, this should never auto-approve
	requiresApproval := s.checkIfApprovalRequired(purchase.ApprovalBaseAmount)
	fmt.Printf("? Approval requirement check: RequiresApproval=%t, BaseAmount=%.2f\n", requiresApproval, purchase.ApprovalBaseAmount)

	// With new logic, this should never be false, but adding safety check
	if !requiresApproval {
		fmt.Printf("? WARNING: checkIfApprovalRequired returned false - this should not happen with new logic!\n")
		fmt.Printf("?? Forcing approval requirement to maintain Employee ? Finance workflow\n")
		requiresApproval = true
		purchase.RequiresApproval = true
	}

	// Create approval request
	fmt.Printf("? Creating approval request for purchase %d\n", id)
	err = s.createApprovalRequest(purchase, models.ApprovalPriorityNormal, purchase.UserID)
	if err != nil {
		fmt.Printf("? Failed to create approval request for purchase %d: %v\n", id, err)
		return fmt.Errorf("failed to create approval request: %v", err)
	}
	fmt.Printf("? Approval request created for purchase %d\n", id)

	// The approval workflow now starts from Employee step (step 1)
	// When Employee submits, we immediately progress to the next step (Finance/Manager)
	// We directly update the database instead of calling ProcessApprovalAction
	// to avoid role-check issues when the step preload doesn't work correctly
	submissionComment := "Purchase submitted by requester for approval"
	if userID != purchase.UserID {
		submissionComment = fmt.Sprintf("Purchase submitted for approval on behalf of requester by user %d", userID)
	}
	if err := s.advanceRequesterSubmissionStep(purchase, userID, submissionComment); err != nil {
		return err
	}

	if false && purchase.ApprovalRequestID != nil {
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
			fmt.Printf("?? No active pending step found to auto-approve for request %d\n", requestID)
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
				fmt.Printf("? Finance approval step activated for request %d\n", requestID)
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
		fmt.Printf("? Employee submission step auto-approved for purchase %d\n", id)
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

func (s *PurchaseService) advanceRequesterSubmissionStep(purchase *models.Purchase, actorID uint, comments string) error {
	if purchase == nil || purchase.ApprovalRequestID == nil {
		return nil
	}

	var approvalReq models.ApprovalRequest
	if err := s.db.Preload("ApprovalSteps.Step").First(&approvalReq, *purchase.ApprovalRequestID).Error; err != nil {
		return fmt.Errorf("failed to load approval request for purchase submission: %v", err)
	}

	var requesterAction *models.ApprovalAction
	for i := range approvalReq.ApprovalSteps {
		action := &approvalReq.ApprovalSteps[i]
		if action.IsActive &&
			action.Status == models.ApprovalStatusPending &&
			strings.EqualFold(strings.TrimSpace(action.Step.ApproverRole), "employee") {
			requesterAction = action
			break
		}
	}

	if requesterAction == nil {
		return nil
	}

	now := time.Now()
	if err := s.db.Model(&models.ApprovalAction{}).
		Where("id = ?", requesterAction.ID).
		Updates(map[string]interface{}{
			"status":      models.ApprovalStatusApproved,
			"approver_id": actorID,
			"comments":    comments,
			"action_date": now,
			"is_active":   false,
		}).Error; err != nil {
		return fmt.Errorf("failed to advance requester submission step: %v", err)
	}

	history := models.ApprovalHistory{
		RequestID: approvalReq.ID,
		UserID:    actorID,
		Action:    models.ApprovalActionApproved,
		Comments:  comments,
		Metadata:  "{}",
	}
	if err := s.db.Create(&history).Error; err != nil {
		return fmt.Errorf("failed to create requester submission history: %v", err)
	}

	nextStep := s.approvalService.getNextStep(approvalReq.ApprovalSteps, requesterAction.Step.StepOrder)
	if nextStep == nil {
		return nil
	}

	for i := range approvalReq.ApprovalSteps {
		action := &approvalReq.ApprovalSteps[i]
		if action.StepID == nextStep.ID {
			action.IsActive = true
			if err := s.db.Model(&models.ApprovalAction{}).
				Where("id = ?", action.ID).
				Update("is_active", true).Error; err != nil {
				return fmt.Errorf("failed to activate next approval step: %v", err)
			}
			go s.approvalService.notifyApprovers(&approvalReq, *nextStep)
			break
		}
	}

	fmt.Printf("✅ Request %d advanced from requester submission to %s step\n", approvalReq.ID, nextStep.StepName)
	return nil
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

func (s *PurchaseService) ensureActiveApprovalStepForUser(requestID uint, userID uint, expectedRoles ...string) error {
	if s.approvalService == nil {
		return errors.New("approval service is not available")
	}

	var approvalReq models.ApprovalRequest
	if err := s.db.Preload("ApprovalSteps.Step").First(&approvalReq, requestID).Error; err != nil {
		return fmt.Errorf("failed to load approval request: %v", err)
	}
	if approvalReq.Status != models.ApprovalStatusPending {
		return fmt.Errorf("approval request is no longer pending")
	}

	for i := range approvalReq.ApprovalSteps {
		action := &approvalReq.ApprovalSteps[i]
		if !action.IsActive || action.Status != models.ApprovalStatusPending {
			continue
		}
		if !s.approvalService.canUserApprove(userID, action.Step.ApproverRole) {
			continue
		}

		if len(expectedRoles) == 0 {
			return nil
		}

		activeRole := models.NormalizeRole(action.Step.ApproverRole)
		for _, expectedRole := range expectedRoles {
			if activeRole == models.NormalizeRole(expectedRole) {
				return nil
			}
		}

		return fmt.Errorf("director escalation can only be requested from Finance Manager approval step; active step is %s", action.Step.ApproverRole)
	}

	return errors.New("no active approval step found for your role")
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
	userRoleNorm := models.NormalizeRole(userRole)

	if !approved {
		rejectionComments := strings.TrimSpace(comments)
		if rejectionComments == "" {
			rejectionComments = "Purchase rejected without comment"
		}

		if purchase.ApprovalRequestID != nil {
			action := models.ApprovalActionDTO{
				Action:   "REJECT",
				Comments: rejectionComments,
			}
			if err := s.approvalService.ProcessApprovalAction(*purchase.ApprovalRequestID, userID, action); err != nil {
				return nil, fmt.Errorf("failed to process purchase rejection: %v", err)
			}

			fmt.Printf("🔔 Calling OnPurchaseRejected cleanup for purchase %d\n", purchaseID)
			cleanupErr := s.OnPurchaseRejected(purchaseID)
			if cleanupErr != nil {
				fmt.Printf("⚠️ Warning: Post-rejection cleanup failed for purchase %d: %v\n", purchaseID, cleanupErr)
			} else {
				fmt.Printf("✅ Post-rejection cleanup completed for purchase %d\n", purchaseID)
			}

			result["message"] = "Purchase rejected"
			result["purchase_id"] = purchaseID
			result["status"] = "REJECTED"
			result["rejected_by"] = userID
			result["rejected_at"] = now.Format(time.RFC3339)
			result["rejection_reason"] = rejectionComments
			return result, nil
		}

		// Purchase rejected
		purchase.Status = models.PurchaseStatusCancelled
		purchase.ApprovalStatus = models.PurchaseApprovalRejected
		purchase.UpdatedAt = now

		// If no approval request exists (DRAFT status), create one for history tracking
		if purchase.ApprovalRequestID == nil {
			// Create a minimal approval request for history tracking (without workflow dependency)
			err = s.createMinimalApprovalRequestForRejection(purchase, purchase.UserID)
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
				approvalReq.RejectReason = rejectionComments
				if updateErr := s.approvalService.UpdateApprovalRequest(approvalReq); updateErr != nil {
					fmt.Printf("Failed to update approval request %d after rejection: %v\n", approvalReq.ID, updateErr)
				} else {
					go s.approvalService.notifyRequester(approvalReq, models.ApprovalActionRejected)
				}
			}

			historyComments := rejectionComments

			historyErr := s.approvalService.CreateApprovalHistory(*purchase.ApprovalRequestID, userID, models.ApprovalActionRejected, historyComments)
			if historyErr != nil {
				// Log error but don't fail the entire operation
				fmt.Printf("Failed to create approval history for rejection: %v\n", historyErr)
				// Continue with fallback - directly insert into approval_histories table if needed
			}
		} else {
			fmt.Printf("Warning: Purchase %d rejected but no approval request ID found\n", purchaseID)
		}

		// ?? CRITICAL: Reverse all accounting effects (journal, stock, cash/bank, project cost)
		fmt.Printf("?? Calling OnPurchaseRejected cleanup for purchase %d\n", purchaseID)
		cleanupErr := s.OnPurchaseRejected(purchaseID)
		if cleanupErr != nil {
			fmt.Printf("?? Warning: Post-rejection cleanup failed for purchase %d: %v\n", purchaseID, cleanupErr)
			// Continue - rejection should still succeed even if cleanup has issues
		} else {
			fmt.Printf("? Post-rejection cleanup completed for purchase %d\n", purchaseID)
		}

		result["message"] = "Purchase rejected"
		result["purchase_id"] = purchaseID
		result["status"] = "REJECTED"
		result["rejected_by"] = userID
		result["rejected_at"] = now.Format(time.RFC3339)
		result["rejection_reason"] = rejectionComments
		return result, nil
	}

	// Purchase is approved, check for escalation
	if models.IsFinanceManagerRole(userRoleNorm) && escalateToDirector {
		// If no approval request exists (DRAFT status), create one
		if purchase.ApprovalRequestID == nil {
			err = s.createApprovalRequest(purchase, models.ApprovalPriorityNormal, purchase.UserID)
			if err != nil {
				return nil, fmt.Errorf("failed to create approval request: %v", err)
			}
			// Reload purchase to get the ApprovalRequestID
			purchase, err = s.purchaseRepo.FindByID(purchaseID)
			if err != nil {
				return nil, err
			}
		}

		if err := s.advanceRequesterSubmissionStep(purchase, purchase.UserID, "Purchase submitted by requester for approval"); err != nil {
			return nil, err
		}

		if purchase.ApprovalRequestID != nil {
			if err := s.ensureActiveApprovalStepForUser(*purchase.ApprovalRequestID, userID, models.RoleFinance); err != nil {
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
		err = s.createApprovalRequest(purchase, models.ApprovalPriorityNormal, purchase.UserID)
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

	if err := s.advanceRequesterSubmissionStep(purchase, purchase.UserID, "Purchase submitted by requester for approval"); err != nil {
		return nil, err
	}

	if purchase.ApprovalRequestID != nil {
		action := models.ApprovalActionDTO{
			Action:   "APPROVE",
			Comments: comments,
		}
		if err := s.approvalService.ProcessApprovalAction(*purchase.ApprovalRequestID, userID, action); err != nil {
			return nil, fmt.Errorf("failed to process purchase approval: %v", err)
		}

		purchase, err = s.purchaseRepo.FindByID(purchaseID)
		if err != nil {
			return nil, err
		}

		resultMessage := "Purchase approved successfully"
		if purchase.ApprovalStatus == models.PurchaseApprovalPending || purchase.Status == models.PurchaseStatusPending {
			resultMessage = "Purchase approved at the current step and forwarded to the next approver"
		}

		result["message"] = resultMessage
		result["purchase_id"] = purchaseID
		result["escalated"] = false
		result["status"] = purchase.Status
		result["approval_status"] = purchase.ApprovalStatus
		return result, nil
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
		fmt.Printf("?? Initialized CREDIT purchase payment tracking: Total=%.2f, Outstanding=%.2f, Paid=%.2f\n",
			purchase.TotalAmount, purchase.OutstandingAmount, purchase.PaidAmount)
	}

	_, err = s.purchaseRepo.Update(purchase)
	if err != nil {
		return nil, err
	}
	if err := s.ensurePurchasePaymentTermsForCredit(purchase); err != nil {
		return nil, fmt.Errorf("failed to ensure payment terms: %v", err)
	}

	// NOTE: Stock updates, journal entries, and cash/bank balance updates
	// are now handled by OnPurchaseApproved callback above

	// ? FIXED: Call OnPurchaseApproved callback for complete post-approval processing
	// This ensures cash bank transactions, stock updates, and journal entries are all handled correctly
	fmt.Printf("?? Calling OnPurchaseApproved callback for purchase %d\n", purchaseID)
	err = s.OnPurchaseApproved(purchaseID)
	if err != nil {
		fmt.Printf("?? Warning: Post-approval callback failed for purchase %d: %v\n", purchaseID, err)
		// Continue processing, don't fail the entire approval
	} else {
		fmt.Printf("? Post-approval callback completed successfully for purchase %d\n", purchaseID)
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

	// Allow receipt creation after approval/payment while there are still items to receive.
	// COMPLETED can happen when payment is already fully settled before the goods/service receipt is recorded.
	if purchase.Status != models.PurchaseStatusApproved &&
		purchase.Status != models.PurchaseStatusPending &&
		purchase.Status != "PAID" &&
		purchase.Status != models.PurchaseStatusCompleted {
		return nil, errors.New("can only receive items for approved, pending, paid, or completed purchases")
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

	type validatedReceiptItem struct {
		remaining int
	}
	validatedItems := make(map[uint]validatedReceiptItem)
	for _, itemReq := range request.ReceiptItems {
		if itemReq.QuantityReceived <= 0 {
			return nil, fmt.Errorf("received quantity must be greater than 0 for item %d", itemReq.PurchaseItemID)
		}

		purchaseItem, err := s.purchaseRepo.GetPurchaseItemByID(itemReq.PurchaseItemID)
		if err != nil {
			return nil, err
		}
		if purchaseItem.PurchaseID != request.PurchaseID {
			return nil, errors.New("purchase item does not belong to this purchase")
		}
		if !isGoodsReceiptPurchaseItem(purchaseItem.ItemType) {
			return nil, fmt.Errorf("purchase item %d is %s and does not require a goods receipt", itemReq.PurchaseItemID, purchaseItem.ItemType)
		}

		receivedSoFar, err := s.purchaseRepo.SumReceivedQtyByPurchaseItem(purchaseItem.ID)
		if err != nil {
			return nil, err
		}
		remaining := int(purchaseItem.Quantity) - receivedSoFar
		if remaining <= 0 {
			return nil, fmt.Errorf("purchase item %d is already fully received", itemReq.PurchaseItemID)
		}
		if itemReq.QuantityReceived > remaining {
			return nil, fmt.Errorf("received quantity (%d) exceeds remaining (%d) for item %d", itemReq.QuantityReceived, remaining, itemReq.PurchaseItemID)
		}

		validatedItems[itemReq.PurchaseItemID] = validatedReceiptItem{
			remaining: remaining,
		}
	}

	// Create receipt with items
	createdReceipt, err := s.purchaseRepo.CreateReceipt(receipt)
	if err != nil {
		return nil, err
	}

	// Process receipt items and check completion
	allReceived := true
	for _, itemReq := range request.ReceiptItems {
		validatedItem := validatedItems[itemReq.PurchaseItemID]

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
		if itemReq.QuantityReceived < validatedItem.remaining {
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
			fmt.Printf("?? Warning: Failed to update purchase status to COMPLETED after receipts: %v\n", upErr)
		}
	}

	// Optional: Capitalization journals for items flagged in request
	// This leverages optional fields in PurchaseReceiptItemRequest (CapitalizeAsset, FixedAssetAccountID, SourceAccountOverride)
	if s.assetCapitalizationSvc != nil {
		// Load default accounts
		var inventoryAccountID uint
		if inv, err := s.accountRepo.FindByCode(context.TODO(), "1103-001"); err == nil {
			inventoryAccountID = inv.ID
		}
		var defaultFAAccountID uint
		if fa, err := s.accountRepo.FindByCode(context.TODO(), "1201-004"); err == nil {
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
				fmt.Printf("?? Capitalization skipped: cannot find purchase item %d: %v\n", itemReq.PurchaseItemID, err)
				continue
			}
			// Compute capitalization amount (exclude VAT): unit_price * qty_received
			capAmount := float64(itemReq.QuantityReceived) * purchaseItem.UnitPrice
			if capAmount <= 0 {
				fmt.Printf("?? Capitalization skipped for item %d: amount is 0\n", itemReq.PurchaseItemID)
				continue
			}
			// SAFEGUARD: if purchase item booked to Inventory (expense_account_id == 0), skip capitalization to avoid double handling
			if purchaseItem.ExpenseAccountID == 0 {
				fmt.Printf("?? Capitalization disabled for inventory item %d. Skipping.\n", itemReq.PurchaseItemID)
				continue
			}
			// DOUBLE-POSTING PREVENTION: If the purchase item was already debited to a Fixed Asset account
			// (e.g., 1201-xxx) in the purchase journal, skip capitalization to prevent duplicate entries
			if purchaseItem.ExpenseAccountID != 0 {
				var itemAccount models.Account
				if accErr := s.db.First(&itemAccount, purchaseItem.ExpenseAccountID).Error; accErr == nil {
					if strings.ToUpper(itemAccount.Category) == models.CategoryFixedAsset {
						fmt.Printf("?? Capitalization skipped for item %d: already debited to Fixed Asset account %s (%s) in purchase journal\n",
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
				fmt.Printf("?? Failed to create capitalization journal for receipt item %d: %v\n", itemReq.PurchaseItemID, err)
				// Continue other items; do not fail receipt creation
			} else {
				fmt.Printf("? Capitalization journal created for receipt item %d (amount=%.2f)\n", itemReq.PurchaseItemID, capAmount)
			}
		}
	}

	// Do not update purchase status here - it should remain in current status until payment is made
	// Receipt completion is independent of purchase completion
	receiptBackorderAffectedProductIDs, settlementErr := s.settleBackordersForApprovedPurchase(purchase)
	if settlementErr != nil {
		fmt.Printf("?? Warning: Failed to settle sales backorders after receipt %s for purchase %s: %v\n", createdReceipt.ReceiptNumber, purchase.Code, settlementErr)
	} else {
		s.syncStockAlertsForProducts(receiptBackorderAffectedProductIDs)
	}

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

func (s *PurchaseService) GetPurchasesSummary(filter models.PurchaseFilter) (*models.PurchaseSummary, error) {
	return s.purchaseRepo.GetPurchasesSummary(filter)
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
			fmt.Printf("?? Warning: Failed to set purchase %d to COMPLETED during receipt fetch: %v\n", purchaseID, upErr)
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

func (s *PurchaseService) purchaseNeedsAccountingCOA(purchase *models.Purchase) bool {
	if purchase == nil {
		return false
	}

	if purchase.PaymentMethod == models.PurchasePaymentCredit &&
		(purchase.CreditAccountID == nil || *purchase.CreditAccountID == 0) {
		return true
	}

	for _, item := range purchase.PurchaseItems {
		if item.ExpenseAccountID == 0 {
			return true
		}
	}

	return false
}

func (s *PurchaseService) markPurchasePendingAccounting(purchase *models.Purchase) error {
	if purchase == nil {
		return nil
	}
	if purchase.Status == models.PurchaseStatusPendingAccounting {
		return nil
	}

	purchase.Status = models.PurchaseStatusPendingAccounting
	purchase.UpdatedAt = time.Now()
	_, err := s.purchaseRepo.Update(purchase)
	return err
}

func purchaseItemLabelForAccounting(item models.PurchaseItem) string {
	if strings.TrimSpace(item.Description) != "" {
		return strings.TrimSpace(item.Description)
	}
	if item.Product != nil && strings.TrimSpace(item.Product.Name) != "" {
		return strings.TrimSpace(item.Product.Name)
	}
	return fmt.Sprintf("item #%d", item.ID)
}

func (s *PurchaseService) validatePurchaseDebitAccount(accountID uint) error {
	if accountID == 0 {
		return errors.New("COA debit item wajib dipilih")
	}

	account, err := s.accountRepo.FindByID(context.TODO(), accountID)
	if err != nil || account == nil {
		return fmt.Errorf("COA debit item %d tidak ditemukan", accountID)
	}
	if account.IsHeader {
		return fmt.Errorf("COA %s - %s adalah akun header dan tidak bisa dipakai untuk transaksi", account.Code, account.Name)
	}

	accountType := strings.ToUpper(strings.TrimSpace(account.Type))
	if accountType != models.AccountTypeAsset && accountType != models.AccountTypeExpense {
		return fmt.Errorf("COA %s - %s harus bertipe ASSET atau EXPENSE untuk item purchase", account.Code, account.Name)
	}

	return nil
}

func (s *PurchaseService) validatePurchaseCreditAccount(accountID uint) error {
	if accountID == 0 {
		return errors.New("COA hutang wajib dipilih untuk purchase kredit")
	}

	account, err := s.accountRepo.FindByID(context.TODO(), accountID)
	if err != nil || account == nil {
		return fmt.Errorf("COA hutang %d tidak ditemukan", accountID)
	}
	if account.IsHeader {
		return fmt.Errorf("COA %s - %s adalah akun header dan tidak bisa dipakai untuk transaksi", account.Code, account.Name)
	}
	if !strings.EqualFold(strings.TrimSpace(account.Type), models.AccountTypeLiability) {
		return fmt.Errorf("COA %s - %s harus bertipe LIABILITY untuk hutang purchase", account.Code, account.Name)
	}

	return nil
}

func (s *PurchaseService) validatePurchaseOtherCostAccount(accountID *uint, amount float64) error {
	if amount <= 0 || accountID == nil || *accountID == 0 {
		return nil
	}

	account, err := s.accountRepo.FindByID(context.TODO(), *accountID)
	if err != nil || account == nil {
		return fmt.Errorf("COA biaya lain-lain %d tidak ditemukan", *accountID)
	}
	if account.IsHeader {
		return fmt.Errorf("COA %s - %s adalah akun header dan tidak bisa dipakai untuk biaya lain-lain", account.Code, account.Name)
	}
	if !strings.EqualFold(strings.TrimSpace(account.Type), models.AccountTypeExpense) {
		return fmt.Errorf("COA %s - %s harus bertipe EXPENSE untuk biaya lain-lain", account.Code, account.Name)
	}

	return nil
}

func (s *PurchaseService) UpdatePurchaseFinalReviewConfig(purchaseID uint, request models.PurchaseFinalReviewConfigRequest, userID uint) (*models.Purchase, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("purchase not found: %v", err)
	}
	if purchase.ApprovalStatus != models.PurchaseApprovalApproved {
		return nil, errors.New("purchase belum melewati approval Finance Manager awal")
	}
	if purchase.Status != models.PurchaseStatusPendingFinalApproval {
		return nil, fmt.Errorf("konfigurasi final hanya bisa dikoreksi saat purchase menunggu final approval (status saat ini: %s)", purchase.Status)
	}
	if len(purchase.PurchaseItems) == 0 {
		return nil, errors.New("purchase tidak memiliki item untuk dihitung ulang")
	}
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return nil, err
	}

	if request.PaymentMethod != nil {
		purchase.PaymentMethod = getPaymentMethod(strings.ToUpper(strings.TrimSpace(*request.PaymentMethod)))
	} else {
		purchase.PaymentMethod = getPaymentMethod(strings.ToUpper(strings.TrimSpace(purchase.PaymentMethod)))
	}
	if request.BankAccountID != nil {
		if *request.BankAccountID == 0 {
			purchase.BankAccountID = nil
		} else {
			purchase.BankAccountID = request.BankAccountID
		}
	}
	if request.CreditAccountID != nil {
		if *request.CreditAccountID == 0 {
			purchase.CreditAccountID = nil
		} else {
			purchase.CreditAccountID = request.CreditAccountID
		}
	}
	if request.PaymentReference != nil {
		purchase.PaymentReference = *request.PaymentReference
	}
	if request.FreightCost != nil {
		purchase.FreightCost = *request.FreightCost
	}
	if request.OtherCostAmount != nil {
		purchase.OtherCostAmount = *request.OtherCostAmount
	}
	if request.OtherCostDescription != nil {
		purchase.OtherCostDescription = strings.TrimSpace(*request.OtherCostDescription)
	}
	if request.OtherCostAccountID != nil {
		if *request.OtherCostAccountID == 0 {
			purchase.OtherCostAccountID = nil
		} else {
			purchase.OtherCostAccountID = request.OtherCostAccountID
		}
	}
	if request.PPNRate != nil {
		purchase.PPNRate = *request.PPNRate
	}
	if request.OtherTaxAdditions != nil {
		purchase.OtherTaxAdditions = *request.OtherTaxAdditions
	}
	if request.PPh21Rate != nil {
		purchase.PPh21Rate = *request.PPh21Rate
	}
	if request.PPh23Rate != nil {
		purchase.PPh23Rate = *request.PPh23Rate
	}
	if request.OtherTaxDeductions != nil {
		purchase.OtherTaxDeductions = *request.OtherTaxDeductions
	}
	if request.Notes != nil {
		purchase.Notes = *request.Notes
	}
	if err := s.validatePurchaseOtherCostAccount(purchase.OtherCostAccountID, purchase.OtherCostAmount); err != nil {
		return nil, err
	}

	purchase.BankAccountID, purchase.CreditAccountID, err = s.normalizePurchasePaymentConfiguration(
		purchase.PaymentMethod,
		purchase.BankAccountID,
		purchase.CreditAccountID,
	)
	if err != nil {
		return nil, err
	}

	if err := s.recalculatePurchaseTotals(purchase); err != nil {
		return nil, fmt.Errorf("failed to recalculate final purchase totals: %v", err)
	}
	s.setApprovalBasisAndBase(purchase)

	now := time.Now()
	purchase.UpdatedAt = now
	err = s.db.Transaction(func(tx *gorm.DB) error {
		selectFields := []string{
			"PaymentMethod",
			"BankAccountID",
			"CreditAccountID",
			"PaymentReference",
			"FreightCost",
			"OtherCostAmount",
			"OtherCostDescription",
			"OtherCostAccountID",
			"SubtotalBeforeDiscount",
			"ItemDiscountAmount",
			"OrderDiscountAmount",
			"NetBeforeTax",
			"PPNRate",
			"PPNAmount",
			"OtherTaxAdditions",
			"TotalTaxAdditions",
			"PPh21Rate",
			"PPh21Amount",
			"PPh23Rate",
			"PPh23Amount",
			"OtherTaxDeductions",
			"TotalTaxDeductions",
			"TaxAmount",
			"TotalAmount",
			"PaidAmount",
			"OutstandingAmount",
			"PPhPaymentStatus",
			"PPhPaidAmount",
			"ApprovalAmountBasis",
			"ApprovalBaseAmount",
			"UpdatedAt",
		}
		if request.Notes != nil {
			selectFields = append(selectFields, "Notes")
		}

		if err := tx.Model(&models.Purchase{}).
			Where("id = ?", purchaseID).
			Select(selectFields).
			Updates(purchase).Error; err != nil {
			return fmt.Errorf("failed to update final review configuration: %w", err)
		}

		if purchase.ApprovalRequestID != nil {
			history := models.ApprovalHistory{
				RequestID: *purchase.ApprovalRequestID,
				UserID:    userID,
				Action:    models.ApprovalActionApproved,
				Comments:  "Finance Manager reviewed/updated tax, payment, and GL readiness before final approval",
				Metadata:  "{\"finance_manager_final_config_review\":true,\"waiting_final_approval\":true}",
			}
			if err := tx.Create(&history).Error; err != nil {
				return fmt.Errorf("failed to record final review configuration history: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetPurchaseByID(purchaseID)
}

func (s *PurchaseService) CompleteAccountingCOA(purchaseID uint, request models.PurchaseAccountingCOARequest, userID uint) (*models.Purchase, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("purchase not found: %v", err)
	}

	if purchase.ApprovalStatus != models.PurchaseApprovalApproved {
		return nil, errors.New("COA accounting hanya bisa dilengkapi setelah purchase disetujui Finance Manager")
	}
	isInitialAccountingCOA := purchase.Status == models.PurchaseStatusPendingAccounting
	isFinalReviewCOA := purchase.Status == models.PurchaseStatusPendingFinalApproval
	if !isInitialAccountingCOA && !isFinalReviewCOA {
		return nil, fmt.Errorf("purchase tidak sedang menunggu input/review COA (status saat ini: %s)", purchase.Status)
	}
	if len(purchase.PurchaseItems) == 0 {
		return nil, errors.New("purchase tidak memiliki item untuk diposting")
	}
	if len(request.Items) == 0 {
		return nil, errors.New("minimal satu item COA wajib dikirim")
	}

	itemAccountByID := make(map[uint]uint, len(request.Items))
	for _, item := range request.Items {
		if item.PurchaseItemID == 0 {
			return nil, errors.New("purchase_item_id wajib diisi")
		}
		if err := s.validatePurchaseDebitAccount(item.ExpenseAccountID); err != nil {
			return nil, err
		}
		itemAccountByID[item.PurchaseItemID] = item.ExpenseAccountID
	}

	for _, item := range purchase.PurchaseItems {
		if _, ok := itemAccountByID[item.ID]; !ok {
			return nil, fmt.Errorf("COA untuk %s wajib dipilih", purchaseItemLabelForAccounting(item))
		}
	}

	var creditAccountID *uint
	if purchase.PaymentMethod == models.PurchasePaymentCredit {
		if request.CreditAccountID == nil || *request.CreditAccountID == 0 {
			return nil, errors.New("COA hutang wajib dipilih untuk purchase kredit")
		}
		if err := s.validatePurchaseCreditAccount(*request.CreditAccountID); err != nil {
			return nil, err
		}
		creditAccountID = request.CreditAccountID
	}

	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return nil, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		purchaseUpdates := map[string]interface{}{
			"updated_at": time.Now(),
		}
		if isInitialAccountingCOA {
			purchaseUpdates["status"] = models.PurchaseStatusPendingFinalApproval
		}
		if purchase.PaymentMethod == models.PurchasePaymentCredit {
			purchaseUpdates["credit_account_id"] = *creditAccountID
		}

		if err := tx.Model(&models.Purchase{}).
			Where("id = ?", purchaseID).
			Updates(purchaseUpdates).Error; err != nil {
			return fmt.Errorf("failed to update purchase accounting status: %w", err)
		}

		for purchaseItemID, accountID := range itemAccountByID {
			if err := tx.Model(&models.PurchaseItem{}).
				Where("id = ? AND purchase_id = ?", purchaseItemID, purchaseID).
				Update("expense_account_id", accountID).Error; err != nil {
				return fmt.Errorf("failed to update COA for purchase item %d: %w", purchaseItemID, err)
			}
		}

		if purchase.ApprovalRequestID != nil {
			historyComments := "Accounting completed GL/COA input; waiting for Finance Manager final approval"
			metadata := "{\"accounting_coa_completed\":true,\"waiting_final_approval\":true}"
			if isFinalReviewCOA {
				historyComments = "Finance Manager reviewed/updated GL/COA before final approval"
				metadata = "{\"finance_manager_coa_review\":true,\"waiting_final_approval\":true}"
			}
			historyNote := models.ApprovalHistory{
				RequestID: *purchase.ApprovalRequestID,
				UserID:    userID,
				Action:    models.ApprovalActionApproved,
				Comments:  historyComments,
				Metadata:  metadata,
			}
			if err := tx.Create(&historyNote).Error; err != nil {
				return fmt.Errorf("failed to record accounting COA history: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetPurchaseByID(purchaseID)
}

func (s *PurchaseService) FinalApprovePurchaseAfterAccounting(purchaseID uint, userID uint, comments string) (*models.Purchase, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("purchase not found: %v", err)
	}
	if purchase.ApprovalStatus != models.PurchaseApprovalApproved {
		return nil, errors.New("purchase belum melewati approval Finance Manager awal")
	}
	if purchase.Status != models.PurchaseStatusPendingFinalApproval {
		return nil, fmt.Errorf("purchase belum menunggu final approval Finance Manager (status saat ini: %s)", purchase.Status)
	}
	pendingCOARevision, err := s.HasPendingPurchaseCOARevision(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("gagal memeriksa pengajuan revisi COA: %w", err)
	}
	if pendingCOARevision {
		return nil, errors.New("masih ada pengajuan revisi COA yang menunggu keputusan Finance Manager")
	}
	if s.purchaseNeedsAccountingCOA(purchase) {
		return nil, errors.New("GL/COA belum lengkap, Accounting harus melengkapi COA terlebih dahulu")
	}
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// Serialize final approval with a possible COA revision request. The same
		// row lock is used by the revision workflow, so a journal cannot be posted
		// while a pending revision is being created.
		var lockedPurchase models.Purchase
		if err := tx.Preload("PurchaseItems").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&lockedPurchase, purchaseID).Error; err != nil {
			return err
		}
		if lockedPurchase.ApprovalStatus != models.PurchaseApprovalApproved ||
			lockedPurchase.Status != models.PurchaseStatusPendingFinalApproval {
			return errors.New("purchase tidak lagi siap untuk final approval")
		}
		if s.purchaseNeedsAccountingCOA(&lockedPurchase) {
			return errors.New("GL/COA belum lengkap, Accounting harus melengkapi COA terlebih dahulu")
		}
		if err := CheckClosedPeriod(tx, lockedPurchase.Date); err != nil {
			return err
		}
		var pendingCount int64
		if err := tx.Model(&models.PurchaseCOARevision{}).
			Where("purchase_id = ? AND status = ?", purchaseID, models.PurchaseCOARevisionStatusPendingFinanceManager).
			Count(&pendingCount).Error; err != nil {
			return err
		}
		if pendingCount > 0 {
			return errors.New("masih ada pengajuan revisi COA yang menunggu keputusan Finance Manager")
		}

		updates := map[string]interface{}{
			"status":      models.PurchaseStatusApproved,
			"approved_by": userID,
			"approved_at": now,
			"updated_at":  now,
		}
		if err := tx.Model(&models.Purchase{}).Where("id = ?", purchaseID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to mark purchase final approved: %w", err)
		}
		if lockedPurchase.ApprovalRequestID != nil {
			historyComments := strings.TrimSpace(comments)
			if historyComments == "" {
				historyComments = "Finance Manager final approval after Accounting GL/COA completion"
			}
			history := models.ApprovalHistory{
				RequestID: *lockedPurchase.ApprovalRequestID,
				UserID:    userID,
				Action:    models.ApprovalActionApproved,
				Comments:  historyComments,
				Metadata:  "{\"finance_manager_final_approval\":true}",
			}
			if err := tx.Create(&history).Error; err != nil {
				return fmt.Errorf("failed to record final approval history: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.postFinalApprovedPurchase(purchaseID); err != nil {
		_ = s.db.Model(&models.Purchase{}).
			Where("id = ?", purchaseID).
			Update("status", models.PurchaseStatusPendingFinalApproval).Error
		return nil, fmt.Errorf("final approval tersimpan, tetapi posting purchase gagal: %v", err)
	}
	if err := s.markPurchasePaymentProofReviewed(purchaseID, userID, models.PurchasePaymentProofApproved, comments); err != nil {
		fmt.Printf("?? Warning: Failed to mark purchase payment proof reviewed: %v\n", err)
	}

	return s.GetPurchaseByID(purchaseID)
}

// PostApprovalCallback Implementation

// OnPurchaseApproved implements PostApprovalCallback interface. Initial approval
// now stops at Accounting GL/COA review; posting happens only after final
// Finance Manager approval.
func (s *PurchaseService) OnPurchaseApproved(purchaseID uint) error {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return fmt.Errorf("failed to get purchase for post-approval routing: %v", err)
	}
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return err
	}
	if err := s.markPurchasePendingAccounting(purchase); err != nil {
		return fmt.Errorf("failed to mark purchase pending accounting COA: %v", err)
	}
	fmt.Printf("?? Purchase %s approved by Finance Manager and routed to Accounting GL/COA input\n", purchase.Code)
	return nil
}

func (s *PurchaseService) postFinalApprovedPurchase(purchaseID uint) error {
	fmt.Printf("?? Starting post-approval processing for purchase %d\n", purchaseID)

	// Get the approved purchase
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return fmt.Errorf("failed to get purchase for post-approval processing: %v", err)
	}

	fmt.Printf("?? Processing approved purchase %s (method: %s, amount: %.2f)\n", purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return err
	}

	if s.purchaseNeedsAccountingCOA(purchase) {
		return fmt.Errorf("purchase %s belum lengkap GL/COA accounting", purchase.Code)
	}

	// 1. Update product stock
	fmt.Printf("?? Updating product stock for approved purchase %d\n", purchaseID)
	affectedProductIDs, err := s.updateProductStockOnApproval(purchase)
	if err != nil {
		fmt.Printf("?? Warning: Failed to update product stock: %v\n", err)
		// Continue processing even if stock update fails
	} else {
		fmt.Printf("? Successfully updated product stock\n")
	}

	// 2. Create journal entries - PRIORITIZE SSOT (no double posting)
	fmt.Printf("??? Creating journal entries for approved purchase %s (ONLY using SSOT)\n", purchase.Code)
	purchaseJournalCreated := false

	// Use SSOT service exclusively
	if s.journalServiceSSOT != nil {
		if err := s.journalServiceSSOT.CreatePurchaseJournal(purchase, nil); err != nil {
			return fmt.Errorf("failed to post purchase journal: %w", err)
		} else {
			purchaseJournalCreated = true
			fmt.Printf("? [SSOT] Created purchase journal entries for Balance Sheet integration\n")
		}
	} else {
		// No SSOT service available
		fmt.Printf("?? SSOT journal service not available, skipping journal creation\n")
	}

	// 3. Update cash/bank balance for immediate payment methods
	if isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("?? Updating cash/bank balance for immediate payment purchase\n")
		err = s.updateCashBankBalanceForPurchaseAtomic(purchase)
		if err != nil {
			fmt.Printf("?? Warning: Failed to update cash/bank balance: %v\n", err)
		} else {
			fmt.Printf("? Successfully updated cash/bank balance\n")
		}
	}

	// 4. Initialize payment tracking for credit purchases
	if purchase.PaymentMethod == models.PurchasePaymentCredit {
		purchase.OutstandingAmount = purchase.TotalAmount
		purchase.PaidAmount = 0
		_, err = s.purchaseRepo.Update(purchase)
		if err != nil {
			fmt.Printf("?? Warning: Failed to update credit purchase payment tracking: %v\n", err)
		} else {
			fmt.Printf("?? Initialized credit purchase payment tracking\n")
		}
		if err := s.ensurePurchasePaymentTermsForCredit(purchase); err != nil {
			fmt.Printf("?? Warning: Failed to ensure credit purchase payment terms: %v\n", err)
		}
	} else {
		// For immediate payment, mark as fully paid
		purchase.PaidAmount = purchase.TotalAmount
		purchase.OutstandingAmount = 0
		_, err = s.purchaseRepo.Update(purchase)
		if err != nil {
			fmt.Printf("?? Warning: Failed to update immediate payment tracking: %v\n", err)
		} else {
			fmt.Printf("?? Marked immediate payment as fully paid\n")
		}
		if err := s.ensureImmediatePurchasePaymentRecord(purchase); err != nil {
			fmt.Printf("?? Warning: Failed to create immediate purchase payment proof: %v\n", err)
		}
	}

	// 5. Update Project Actual Cost (if linked to project)
	if purchase.ProjectID != nil {
		fmt.Printf("?? Increasing project actual cost for approved purchase %d\n", purchaseID)
		err = s.updateProjectCostForPurchase(purchase, true)
		if err != nil {
			fmt.Printf("?? Warning: Failed to increase project actual cost: %v\n", err)
		} else {
			fmt.Printf("? Successfully increased project actual cost for project %d\n", *purchase.ProjectID)
		}
	}

	// 6. Auto-create Asset records for items debited to Fixed Asset accounts (PSAK 16 - Accrual Basis)
	// When purchase items target a Fixed Asset account (1201-xxx), automatically create Asset register records
	// so that depreciation starts immediately from purchase date, not after payment/receipt
	s.autoCreateAssetsFromPurchase(purchase)

	// 7. Auto-create Prepaid Expenses
	fmt.Printf("? Mengecek keberadaan item PREPAID untuk purchase %s\n", purchase.Code)
	err = s.processPrepaidExpensesInPurchase(purchase)
	if err != nil {
		fmt.Printf("?? Warning: Gagal memproses data Prepaid Expense: %v\n", err)
		// Tidak mereturn error agar approval parent tetap jalan
	}

	// 8. Settle any open sales backorders now that inventory is available.
	// Even when the purchase journal already existed or was skipped as duplicate,
	// the stock may still need to be allocated to sales waiting fulfillment.
	if !purchaseJournalCreated {
		fmt.Printf("?? Purchase journal was not newly posted for %s; attempting sales backorder settlement against current stock\n", purchase.Code)
	}
	fmt.Printf("?? Settling sales backorders from approved purchase %s\n", purchase.Code)
	backorderAffectedProductIDs, settlementErr := s.settleBackordersForApprovedPurchase(purchase)
	if settlementErr != nil {
		fmt.Printf("?? Warning: Failed to settle sales backorders for purchase %s: %v\n", purchase.Code, settlementErr)
	} else {
		for productID := range backorderAffectedProductIDs {
			affectedProductIDs[productID] = struct{}{}
		}
		fmt.Printf("? Sales backorder settlement completed for purchase %s\n", purchase.Code)
	}

	s.syncStockAlertsForProducts(affectedProductIDs)

	fmt.Printf("? Post-approval processing completed for purchase %s\n", purchase.Code)
	return nil
}

// processPrepaidExpensesInPurchase extracts and schedules PREPAID items into prepaid_expenses and schedules
func (s *PurchaseService) processPrepaidExpensesInPurchase(purchase *models.Purchase) error {
	baseDate := purchase.Date
	if baseDate.IsZero() {
		baseDate = time.Now()
	}
	baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, baseDate.Location())

	for _, item := range purchase.PurchaseItems {
		if item.ItemType == models.PurchaseItemTypePrepaid {
			if item.ExpenseAccountID == 0 {
				fmt.Printf("?? Skipping Prepaid Expense auto-generation for purchase item %d because prepaid COA is empty\n", item.ID)
				continue
			}

			// Durasi harus sudah tervalidasi saat purchase dibuat atau diperbarui.
			months := item.AmortizationMonth
			if months < 1 || months > 120 {
				return fmt.Errorf("item prepaid %d memiliki durasi amortisasi tidak valid: %d bulan", item.ID, months)
			}

			// Hitung Biaya Per Bulan flat
			monthlyAmt := item.TotalPrice / float64(months)

			endDate := baseDate.AddDate(0, months, 0)

			// Penamaan Prepaid Expense
			prepaidName := item.Description
			if prepaidName == "" {
				prepaidName = fmt.Sprintf("Prepaid dari PO %s", purchase.Code)
			}

			// Buat Header Prepaid Expense
			prepaid := models.PrepaidExpense{
				PurchaseID:        &purchase.ID,
				PurchaseItemID:    &item.ID,
				Name:              prepaidName,
				Description:       fmt.Sprintf("Prepaid Expense auto-generated dari PO %s", purchase.Code),
				TotalAmount:       item.TotalPrice,
				AmortizationMonth: months,
				MonthlyAmount:     monthlyAmt,
				StartDate:         baseDate,
				EndDate:           endDate,
				Status:            "ACTIVE",
				PrepaidAccountID:  item.ExpenseAccountID, // Akun sumber Aset Lancar
				ExpenseAccountID:  0,                     // Nanti diupdate di UI modul Amortisasi
			}

			// Save Prepaid Expense
			if err := s.db.Create(&prepaid).Error; err != nil {
				return fmt.Errorf("gagal membuat record prepaid: %v", err)
			}

			// Buat Jadwal Cicilannya sejumlah bulan
			for i := 1; i <= months; i++ {
				scheduleDate := baseDate.AddDate(0, i, 0) // Jatuh tempo bulan ke-i

				schedule := models.PrepaidSchedule{
					PrepaidID:     prepaid.ID,
					MonthNumber:   i,
					ScheduledDate: scheduleDate,
					Amount:        monthlyAmt,
					Status:        "PENDING",
				}
				if err := s.db.Create(&schedule).Error; err != nil {
					fmt.Printf("?? Gagal membuat schedule ke-%d untuk prepaid ID %d\n", i, prepaid.ID)
				}
			}
			fmt.Printf("? Sukses membuat Prepaid Expense '%s' (%d Bulan) sebesar Rp %.2f\n", prepaidName, months, item.TotalPrice)
		}
	}
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
	for _, item := range purchase.PurchaseItems {
		// Determine if this item should create an asset
		isAssetFromFlag := item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset

		if !isAssetFromFlag {
			continue // Not an asset item, skip
		}

		// Check if asset already exists for this purchase item (prevent duplicates)
		var existingCount int64
		s.db.Model(&models.Asset{}).Where(
			"notes LIKE ? AND notes LIKE ? AND deleted_at IS NULL",
			fmt.Sprintf("%%Purchase %s%%", purchase.Code),
			fmt.Sprintf("%%PurchaseItem:%d%%", item.ID),
		).Count(&existingCount)
		if existingCount > 0 {
			fmt.Printf("?? Asset already exists for purchase item %d, skipping\n", item.ID)
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
		categoryCode := ""
		var assetCategoryID *uint
		var assetAccountID *uint

		if item.AssetCategoryID != nil && *item.AssetCategoryID > 0 {
			// User selected asset category from UI
			assetCategoryID = item.AssetCategoryID
			var assetCat models.AssetCategory
			if err := s.db.First(&assetCat, *item.AssetCategoryID).Error; err == nil {
				category = assetCat.Name
				categoryCode = assetCat.Code
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
		usefulLife, depMethod, salvageValue := deriveAutoCreatedAssetDefaults(category, categoryCode, item.TotalPrice)

		// Ensure Quantity is at least 1 (Prevents zero-loop bug & division by zero panic)
		qty := item.Quantity
		if qty <= 0 {
			qty = 1
			fmt.Printf("?? Warning: Auto-fixing item.Quantity=0 for Asset '%s', forcing to Qty=1\n", assetName)
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
				fmt.Printf("?? ?? ?? [FATAL AUTO-ASSET] Failed to create Asset DB record '%s': %v\n", unitAssetName, err)
				continue
			}

			fmt.Printf("? [AUTO-ASSET] Created asset %s (%s) from purchase %s - Rp %.2f, useful life: %d years\n",
				asset.Code, asset.Name, purchase.Code, asset.PurchasePrice, asset.UsefulLife)
		}
	}
}

func deriveAutoCreatedAssetDefaults(categoryName string, categoryCode string, totalPrice float64) (int, string, float64) {
	normalizedName := strings.ToLower(strings.TrimSpace(categoryName))
	normalizedCode := strings.ToUpper(strings.TrimSpace(categoryCode))

	switch {
	case normalizedCode == "LAND" || normalizedName == "tanah" || normalizedName == "land":
		return 0, "", 0
	case normalizedCode == "REAL" || normalizedName == "bangunan" || normalizedName == "building" || normalizedName == "real estate":
		return 20, models.DepreciationMethodStraightLine, totalPrice * 0.10
	case normalizedCode == "VEHICLE" || normalizedName == "kendaraan" || normalizedName == "vehicle":
		return 5, models.DepreciationMethodStraightLine, totalPrice * 0.10
	case normalizedCode == "OFFICE" ||
		normalizedCode == "COMP" ||
		normalizedCode == "IT" ||
		normalizedCode == "FURNITURE" ||
		normalizedName == "peralatan" ||
		normalizedName == "office equipment" ||
		normalizedName == "computer equipment" ||
		normalizedName == "it infrastructure" ||
		normalizedName == "inventaris kantor" ||
		normalizedName == "furniture":
		return 4, models.DepreciationMethodStraightLine, totalPrice * 0.05
	default:
		return 5, models.DepreciationMethodStraightLine, 0
	}
}

func (s *PurchaseService) cleanupAutoCreatedAssetsFromPurchase(purchase *models.Purchase) error {
	if len(purchase.PurchaseItems) == 0 {
		return nil
	}

	assetRepo := repositories.NewAssetRepository(s.db)
	assetService := NewAssetService(assetRepo, s.db)
	assetJournalSvc := NewAssetJournalServiceSSOT(s.db)
	assetSourceTypes := []string{
		models.SSOTSourceTypeAsset,
		models.SSOTSourceTypeOpening,
		models.SSOTSourceTypeDepreciation,
	}

	cleanedAssets := 0

	for _, item := range purchase.PurchaseItems {
		if !item.IsFixedAsset && item.ItemType != models.PurchaseItemTypeAsset {
			continue
		}

		var assets []models.Asset
		if err := s.db.Where(
			"notes LIKE ? AND notes LIKE ? AND deleted_at IS NULL",
			fmt.Sprintf("%%Purchase %s%%", purchase.Code),
			fmt.Sprintf("%%PurchaseItem:%d%%", item.ID),
		).Find(&assets).Error; err != nil {
			return fmt.Errorf("failed to find auto-created assets for purchase item %d: %v", item.ID, err)
		}

		for _, asset := range assets {
			var entries []models.SSOTJournalEntry
			if err := s.db.Where("source_type IN ? AND source_id = ? AND deleted_at IS NULL", assetSourceTypes, asset.ID).Find(&entries).Error; err != nil {
				return fmt.Errorf("failed to load journal entries for asset %s: %v", asset.Code, err)
			}

			for _, entry := range entries {
				if err := CheckClosedPeriod(s.db, entry.EntryDate); err != nil {
					return fmt.Errorf("cannot remove asset %s from purchase %s because journal %s is in a closed period: %w", asset.Code, purchase.Code, entry.EntryNumber, err)
				}
			}

			if err := assetJournalSvc.ReverseAssetJournals(asset.ID); err != nil {
				return fmt.Errorf("failed to reverse asset journals for %s: %v", asset.Code, err)
			}
			if err := assetService.DeleteAsset(asset.ID); err != nil {
				return fmt.Errorf("failed to delete auto-created asset %s: %v", asset.Code, err)
			}

			cleanedAssets++
			fmt.Printf("✅ Auto-created asset %s cleaned up for purchase %s\n", asset.Code, purchase.Code)
		}
	}

	if cleanedAssets > 0 {
		fmt.Printf("✅ Cleaned up %d auto-created assets for purchase %s\n", cleanedAssets, purchase.Code)
	}

	return nil
}

// OnPurchaseRejected reverses all accounting effects when a purchase is rejected.
// This handles cleanup for purchases that were previously approved (journal, stock, cash/bank, project cost).
func (s *PurchaseService) OnPurchaseRejected(purchaseID uint) error {
	fmt.Printf("?? Starting post-rejection cleanup for purchase %d\n", purchaseID)

	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return fmt.Errorf("failed to get purchase for rejection cleanup: %v", err)
	}

	fmt.Printf("?? Cleaning up rejected purchase %s (method: %s, amount: %.2f)\n",
		purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	// Guard: check if purchase date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return err
	}

	// 1. Reset payment tracking amounts
	fmt.Printf("?? Resetting payment tracking for rejected purchase %d\n", purchaseID)
	oldOutstanding := purchase.OutstandingAmount
	oldPaid := purchase.PaidAmount
	purchase.OutstandingAmount = 0
	purchase.PaidAmount = 0
	_, err = s.purchaseRepo.Update(purchase)
	if err != nil {
		fmt.Printf("?? Warning: Failed to reset payment tracking: %v\n", err)
	} else {
		fmt.Printf("? Payment tracking reset: Outstanding %.2f?0, Paid %.2f?0\n", oldOutstanding, oldPaid)
	}

	// 2. Delete SSOT journal entries with COA balance reversal
	fmt.Printf("?? Removing SSOT journal entries for rejected purchase %d\n", purchaseID)
	if s.journalServiceSSOT != nil {
		// Use the SSOT journal service which properly reverses COA balances
		if err := s.journalServiceSSOT.DeletePurchaseJournal(purchaseID, nil); err != nil {
			fmt.Printf("?? Warning: Failed to delete SSOT journal via service for purchase %d: %v\n", purchaseID, err)
		} else {
			fmt.Printf("? SSOT journal entries deleted with COA reversal for purchase %d\n", purchaseID)
		}
	}

	// Fallback: Also soft-delete any remaining SSOT entries that might have been missed
	if err := s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Model(&models.SSOTJournalEntry{}).
		Where("source_type = 'PURCHASE' AND source_id = ? AND deleted_at IS NULL", purchaseID).
		Update("deleted_at", time.Now()).Error; err != nil {
		fmt.Printf("?? Warning: Failed to soft-delete remaining SSOT journal entries: %v\n", err)
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
		fmt.Printf("?? Warning: Failed to soft-delete remaining SSOT journal lines: %v\n", err)
	}

	// 3. Remove any asset register records auto-created from this purchase
	fmt.Printf("??? Cleaning auto-created assets for rejected purchase %s\n", purchase.Code)
	if err := s.cleanupAutoCreatedAssetsFromPurchase(purchase); err != nil {
		return err
	}

	// 4. Reopen any sale backorders that had been settled by this purchase before
	// we subtract the approved stock addition.
	backorderAffectedProductIDs, backorderErr := s.reverseBackorderSettlementsForRejectedPurchase(purchase)
	if backorderErr != nil {
		return backorderErr
	}

	// 5. Reverse product stock (subtract the quantities that were added on approval)
	fmt.Printf("?? Reversing product stock for rejected purchase %s\n", purchase.Code)
	affectedProductIDs := make(map[uint]struct{})
	for productID := range backorderAffectedProductIDs {
		affectedProductIDs[productID] = struct{}{}
	}
	for _, item := range purchase.PurchaseItems {
		// Skip ASSET items - they don't have inventory/stock
		if item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset {
			fmt.Printf("?? Skipping stock reversal for ASSET item '%s' (PurchaseItem:%d)\n", item.Description, item.ID)
			continue
		}
		// Skip items without ProductID
		if item.ProductID == nil || *item.ProductID == 0 {
			continue
		}

		product, err := s.productRepo.FindByID(*item.ProductID)
		if err != nil {
			fmt.Printf("?? Warning: Product %d not found for stock reversal: %v\n", *item.ProductID, err)
			continue
		}

		oldStock := product.Stock
		product.Stock -= item.Quantity
		if product.Stock < 0 {
			fmt.Printf("?? Warning: Stock for product %d would go negative (%d), setting to 0\n", product.ID, product.Stock)
			product.Stock = 0
		}

		err = s.productRepo.Update(context.Background(), product)
		if err != nil {
			fmt.Printf("?? Warning: Failed to reverse stock for product %d: %v\n", product.ID, err)
		} else {
			affectedProductIDs[product.ID] = struct{}{}
			fmt.Printf("? Product %d stock reversed: %d ? %d\n", product.ID, oldStock, product.Stock)
		}
	}

	// 6. Reverse cash/bank balance for immediate payment purchases
	if isImmediatePayment(purchase.PaymentMethod) && purchase.BankAccountID != nil && *purchase.BankAccountID > 0 {
		fmt.Printf("?? Restoring cash/bank balance for rejected immediate payment purchase\n")

		var cashBank models.CashBank
		if err := s.db.First(&cashBank, *purchase.BankAccountID).Error; err != nil {
			fmt.Printf("?? Warning: Failed to find bank account %d for balance restoration: %v\n", *purchase.BankAccountID, err)
		} else {
			restoreAmount := purchase.TotalAmount
			exchangeRate := 1.0
			if cashBank.IsForeignCurrency() {
				nativeAmount, carryRate, calcErr := calculatePurchaseNativeOutflow(&cashBank, purchase.TotalAmount)
				if calcErr != nil {
					fmt.Printf("⚠️ Warning: Failed to calculate reversal native amount for %s: %v\n", cashBank.Name, calcErr)
				} else {
					restoreAmount = nativeAmount
					exchangeRate = carryRate
					cashBank.BookBalanceIDR += purchase.TotalAmount
				}
			}

			oldBalance := cashBank.Balance
			cashBank.Balance += restoreAmount // Add back (restore)

			if err := s.db.Save(&cashBank).Error; err != nil {
				fmt.Printf("?? Warning: Failed to restore cash/bank balance: %v\n", err)
			} else {
				fmt.Printf("? Cash/bank balance restored: %s %.2f ? %.2f (+%.2f)\n",
					cashBank.Name, oldBalance, cashBank.Balance, restoreAmount)
			}

			// Create reversal transaction record
			reversalTx := &models.CashBankTransaction{
				CashBankID:      *purchase.BankAccountID,
				ReferenceType:   "PURCHASE_REVERSAL",
				ReferenceID:     purchase.ID,
				Amount:          restoreAmount, // Positive = incoming (reversal)
				Currency:        normalizeCashBankCurrency(cashBank.Currency),
				ExchangeRate:    exchangeRate,
				BookAmountIDR:   purchase.TotalAmount,
				BalanceAfter:    cashBank.Balance,
				TransactionDate: time.Now(),
				Notes:           fmt.Sprintf("Reversal: Purchase %s rejected", purchase.Code),
			}

			if err := s.db.Create(reversalTx).Error; err != nil {
				fmt.Printf("?? Warning: Failed to create reversal transaction record: %v\n", err)
			} else {
				fmt.Printf("?? Reversal cash/bank transaction record created: ID %d\n", reversalTx.ID)
			}

			guard := NewCashBankCurrencyGuard(s.db)
			if err := guard.EnforceAfterTransaction(nil, *purchase.BankAccountID); err != nil {
				fmt.Printf("?? Warning: Failed to enforce cash/bank invariant after purchase reversal: %v\n", err)
			}
		}
	}

	// 7. Reverse project actual cost (if linked to a project)
	if purchase.ProjectID != nil {
		fmt.Printf("?? Decreasing project actual cost for rejected purchase %d\n", purchaseID)
		err = s.updateProjectCostForPurchase(purchase, false) // false = decrease
		if err != nil {
			fmt.Printf("?? Warning: Failed to decrease project actual cost: %v\n", err)
		} else {
			fmt.Printf("? Project %d actual cost decreased by %.2f\n", *purchase.ProjectID, purchase.NetBeforeTax)
		}
	}

	s.syncStockAlertsForProducts(affectedProductIDs)

	fmt.Printf("? Post-rejection cleanup completed for purchase %s\n", purchase.Code)
	return nil
}

func (s *PurchaseService) syncStockAlertsForProducts(productIDs map[uint]struct{}) {
	if s.stockMonitoringService == nil || len(productIDs) == 0 {
		return
	}

	affectedProductIDs := make([]uint, 0, len(productIDs))
	for productID := range productIDs {
		affectedProductIDs = append(affectedProductIDs, productID)
	}

	if err := s.stockMonitoringService.SyncProductAlerts(affectedProductIDs...); err != nil {
		fmt.Printf("?? Warning: Failed to sync stock alerts for %d products: %v\n", len(affectedProductIDs), err)
	}
}

// Private helper methods

func (s *PurchaseService) checkIfApprovalRequired(amount float64) bool {
	// NEW LOGIC: All purchases require approval regardless of amount
	// This ensures Employee ? Finance ? (optional Director) workflow
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
	fmt.Printf("?? Creating approval request for purchase %s\n", purchase.Code)
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
				if !isGoodsReceiptPurchaseItem(purchaseItem.ItemType) {
					return fmt.Errorf("purchase item %d is %s and does not require a goods receipt", receiptItem.PurchaseItemID, purchaseItem.ItemType)
				}
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

func isGoodsReceiptPurchaseItem(itemType string) bool {
	switch strings.ToUpper(strings.TrimSpace(itemType)) {
	case "", models.PurchaseItemTypeInventory, models.PurchaseItemTypeAsset:
		return true
	default:
		return false
	}
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

func (s *PurchaseService) validatePurchasePaymentConfiguration(paymentMethod string, bankAccountID *uint, creditAccountID *uint) error {
	method := getPaymentMethod(strings.ToUpper(strings.TrimSpace(paymentMethod)))

	switch method {
	case models.PurchasePaymentCredit:
		if bankAccountID != nil && *bankAccountID != 0 {
			return fmt.Errorf("bank/cash account is not allowed for CREDIT purchase method")
		}
		if creditAccountID == nil || *creditAccountID == 0 {
			return fmt.Errorf("credit account is required for CREDIT purchase method")
		}

		var creditAccount models.Account
		if err := s.db.Select("id", "type", "is_header", "is_active").First(&creditAccount, *creditAccountID).Error; err != nil {
			return fmt.Errorf("credit account not found: %v", err)
		}
		if !creditAccount.IsActive {
			return fmt.Errorf("selected credit account is inactive")
		}
		if creditAccount.IsHeader {
			return fmt.Errorf("selected credit account must be a detail account")
		}
		if strings.ToUpper(strings.TrimSpace(creditAccount.Type)) != models.AccountTypeLiability {
			return fmt.Errorf("selected credit account must be a liability account")
		}

	case models.PurchasePaymentCash, models.PurchasePaymentTransfer, models.PurchasePaymentCheck:
		if creditAccountID != nil && *creditAccountID != 0 {
			return fmt.Errorf("credit account is only allowed for CREDIT purchase method")
		}
		if bankAccountID == nil || *bankAccountID == 0 {
			return fmt.Errorf("cash/bank account is required for %s purchase method", method)
		}

		var cashBank models.CashBank
		if err := s.db.Select("id", "type", "is_active").First(&cashBank, *bankAccountID).Error; err != nil {
			return fmt.Errorf("cash/bank account not found: %v", err)
		}
		if !cashBank.IsActive {
			return fmt.Errorf("selected cash/bank account is inactive")
		}

		expectedType := expectedCashBankTypeForPurchaseMethod(method)
		if strings.ToUpper(strings.TrimSpace(cashBank.Type)) != expectedType {
			return fmt.Errorf("selected cash/bank account must be of type %s for %s purchase method", expectedType, method)
		}

	default:
		return fmt.Errorf("unsupported purchase payment method: %s", method)
	}

	return nil
}

func (s *PurchaseService) normalizePurchasePaymentConfiguration(paymentMethod string, bankAccountID *uint, creditAccountID *uint) (*uint, *uint, error) {
	method := getPaymentMethod(strings.ToUpper(strings.TrimSpace(paymentMethod)))

	switch method {
	case models.PurchasePaymentCredit:
		if creditAccountID == nil || *creditAccountID == 0 {
			resolvedCreditAccountID, err := s.resolveDefaultPurchaseCreditAccountID()
			if err != nil {
				return nil, nil, err
			}
			creditAccountID = resolvedCreditAccountID
		} else {
			normalizedCreditAccountID, err := s.normalizeSelectedPurchaseCreditAccountID(creditAccountID)
			if err != nil {
				return nil, nil, err
			}
			creditAccountID = normalizedCreditAccountID
		}
		bankAccountID = nil
	case models.PurchasePaymentCash, models.PurchasePaymentTransfer, models.PurchasePaymentCheck:
		creditAccountID = nil
	}

	if err := s.validatePurchasePaymentConfiguration(method, bankAccountID, creditAccountID); err != nil {
		return nil, nil, err
	}

	return bankAccountID, creditAccountID, nil
}

func (s *PurchaseService) resolveDefaultPurchaseCreditAccountID() (*uint, error) {
	var account models.Account
	if err := s.db.
		Where("code = ? AND deleted_at IS NULL", "2101-001").
		First(&account).Error; err == nil {
		if !account.IsActive {
			return nil, fmt.Errorf("default credit account 2101-001 is inactive")
		}
		if account.IsHeader {
			return nil, fmt.Errorf("default credit account 2101-001 must be a detail account")
		}
		if strings.ToUpper(strings.TrimSpace(account.Type)) != models.AccountTypeLiability {
			return nil, fmt.Errorf("default credit account 2101-001 must be a liability account")
		}
		accountID := account.ID
		return &accountID, nil
	}

	resolver := NewAccountResolver(s.db)
	resolvedAccount, err := resolver.GetAccount(AccountTypeAccountsPayable)
	if err != nil {
		return nil, fmt.Errorf("default credit account not found: %w", err)
	}
	if resolvedAccount == nil || resolvedAccount.ID == 0 {
		return nil, fmt.Errorf("default credit account not found")
	}
	if !resolvedAccount.IsActive {
		return nil, fmt.Errorf("default credit account is inactive")
	}
	if resolvedAccount.IsHeader {
		return nil, fmt.Errorf("default credit account must be a detail account")
	}
	if strings.ToUpper(strings.TrimSpace(resolvedAccount.Type)) != models.AccountTypeLiability {
		return nil, fmt.Errorf("default credit account must be a liability account")
	}

	accountID := resolvedAccount.ID
	return &accountID, nil
}

func (s *PurchaseService) normalizeSelectedPurchaseCreditAccountID(creditAccountID *uint) (*uint, error) {
	if creditAccountID == nil || *creditAccountID == 0 {
		return creditAccountID, nil
	}

	var selectedAccount models.Account
	if err := s.db.Select("id", "code", "type", "is_header", "is_active").First(&selectedAccount, *creditAccountID).Error; err != nil {
		return nil, fmt.Errorf("credit account not found: %v", err)
	}

	if !selectedAccount.IsHeader {
		return creditAccountID, nil
	}

	var childAccount models.Account
	if err := s.db.
		Where("parent_id = ? AND is_active = ? AND is_header = ? AND deleted_at IS NULL", selectedAccount.ID, true, false).
		Order("code ASC").
		First(&childAccount).Error; err != nil {
		return nil, fmt.Errorf("selected credit account %s requires a detail child account", selectedAccount.Code)
	}

	childAccountID := childAccount.ID
	return &childAccountID, nil
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
	cashBankAccountID := *cashBankID

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
	guard := NewCashBankCurrencyGuard(s.db)
	var cashBank models.CashBank
	if err := tx.First(&cashBank, cashBankAccountID).Error; err != nil {
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
		CashBankID:      cashBankAccountID,
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

	if err := guard.EnforceAfterTransaction(tx, cashBankAccountID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after vendor payment: %v", err)
	}

	// Create journal entries for vendor payment
	// Debit: Accounts Payable (reduce liability)
	// Credit: Cash/Bank (reduce asset)
	if err := s.createVendorPaymentJournalEntries(tx, payment, purchase, cashBankAccountID, userID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create journal entries: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	paymentBackorderAffectedProductIDs, settlementErr := s.settleBackordersForApprovedPurchase(purchase)
	if settlementErr != nil {
		fmt.Printf("?? Warning: Failed to settle sales backorders after vendor payment for purchase %s: %v\n", purchase.Code, settlementErr)
	} else {
		s.syncStockAlertsForProducts(paymentBackorderAffectedProductIDs)
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
		Preload("Payment").
		Preload("PurchasePaymentTerm").
		Preload("PaymentProofUploader").
		Preload("PaymentProofReviewer").
		Order("date DESC").
		Find(&payments).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get purchase payments: %v", err)
	}

	return payments, nil
}

func (s *PurchaseService) AttachPurchasePaymentProof(
	purchaseID uint,
	userID uint,
	userRole string,
	fileName string,
	filePath string,
	fileSize int64,
	mimeType string,
) (*models.PurchasePayment, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(filePath) == "" {
		return nil, errors.New("payment proof file is required")
	}

	if !s.db.Migrator().HasTable(&models.PurchasePayment{}) {
		if err := s.db.AutoMigrate(&models.PurchasePayment{}); err != nil {
			return nil, err
		}
	}

	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("purchase not found: %v", err)
	}
	if !isImmediatePayment(purchase.PaymentMethod) {
		return nil, errors.New("bukti pembayaran bank hanya untuk pembelian dengan pembayaran langsung")
	}
	if purchase.BankAccountID == nil {
		return nil, errors.New("cash/bank account must be selected before uploading payment proof")
	}

	if err := s.ensureImmediatePurchasePaymentRecord(purchase); err != nil {
		return nil, fmt.Errorf("failed to prepare purchase payment record: %w", err)
	}

	now := time.Now()
	reviewStatus := models.PurchasePaymentProofPending
	var reviewedBy *uint
	var reviewedAt *time.Time
	reviewNotes := ""
	if models.IsFinanceManagerRole(userRole) && s.canReviewPurchasePaymentProof(*purchase) {
		reviewStatus = models.PurchasePaymentProofApproved
		reviewedBy = &userID
		reviewedAt = &now
		reviewNotes = "Bukti pembayaran bank diunggah oleh Finance Manager setelah purchase approval"
	}

	var payment models.PurchasePayment
	if err := s.db.Where("purchase_id = ? AND payment_id IS NULL AND deleted_at IS NULL", purchaseID).
		Order("created_at DESC").
		First(&payment).Error; err != nil {
		return nil, fmt.Errorf("purchase payment record not found: %v", err)
	}

	updates := map[string]interface{}{
		"payment_proof_file_name":     strings.TrimSpace(fileName),
		"payment_proof_file_path":     strings.TrimSpace(filePath),
		"payment_proof_file_size":     fileSize,
		"payment_proof_mime_type":     strings.TrimSpace(mimeType),
		"payment_proof_uploaded_by":   userID,
		"payment_proof_uploaded_at":   now,
		"payment_proof_review_status": reviewStatus,
		"payment_proof_reviewed_by":   reviewedBy,
		"payment_proof_reviewed_at":   reviewedAt,
		"payment_proof_review_notes":  reviewNotes,
		"updated_at":                  now,
	}
	if err := s.db.Model(&payment).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to attach payment proof: %w", err)
	}

	if err := s.db.Preload("CashBank").
		Preload("User").
		Preload("PaymentProofUploader").
		Preload("PaymentProofReviewer").
		First(&payment, payment.ID).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

// AttachCreditPurchasePaymentProof attaches a bank proof to one completed
// credit-payment allocation. The linked term is mandatory so a proof can never
// be mistaken for another installment on the same purchase order.
func (s *PurchaseService) AttachCreditPurchasePaymentProof(
	purchaseID uint,
	paymentID uint,
	userID uint,
	userRole string,
	fileName string,
	filePath string,
	fileSize int64,
	mimeType string,
) (*models.PurchasePayment, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(filePath) == "" {
		return nil, errors.New("payment proof file is required")
	}
	if paymentID == 0 {
		return nil, errors.New("payment ID is required")
	}

	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("purchase not found: %v", err)
	}
	if !models.IsCreditPurchaseMethod(purchase.PaymentMethod) {
		return nil, errors.New("bukti pembayaran termin hanya untuk pembelian kredit")
	}

	var purchasePayment models.PurchasePayment
	if err := s.db.
		Preload("Payment").
		Where("purchase_id = ? AND payment_id = ? AND deleted_at IS NULL", purchaseID, paymentID).
		First(&purchasePayment).Error; err != nil {
		return nil, fmt.Errorf("payment purchase tidak ditemukan: %v", err)
	}
	if purchasePayment.PurchasePaymentTermID == nil {
		return nil, errors.New("pembayaran ini belum tertaut ke termin secara spesifik")
	}
	if purchasePayment.Payment == nil || purchasePayment.Payment.Status != models.PaymentStatusCompleted {
		return nil, errors.New("bukti hanya dapat diunggah untuk pembayaran yang sudah tercatat")
	}

	var allocationCount int64
	if err := s.db.Model(&models.PaymentAllocation{}).
		Where(
			"payment_id = ? AND bill_id = ? AND purchase_payment_term_id = ?",
			paymentID,
			purchaseID,
			*purchasePayment.PurchasePaymentTermID,
		).
		Count(&allocationCount).Error; err != nil {
		return nil, fmt.Errorf("failed to validate payment allocation: %w", err)
	}
	if allocationCount == 0 {
		return nil, errors.New("pembayaran tidak memiliki alokasi termin yang valid")
	}

	now := time.Now()
	reviewStatus := models.PurchasePaymentProofPending
	var reviewedBy *uint
	var reviewedAt *time.Time
	reviewNotes := ""
	if models.IsFinanceManagerRole(userRole) && s.canReviewPurchasePaymentProof(*purchase) {
		reviewStatus = models.PurchasePaymentProofApproved
		reviewedBy = &userID
		reviewedAt = &now
		reviewNotes = "Bukti pembayaran termin diunggah oleh Finance Manager"
	}

	updates := map[string]interface{}{
		"payment_proof_file_name":     strings.TrimSpace(fileName),
		"payment_proof_file_path":     strings.TrimSpace(filePath),
		"payment_proof_file_size":     fileSize,
		"payment_proof_mime_type":     strings.TrimSpace(mimeType),
		"payment_proof_uploaded_by":   userID,
		"payment_proof_uploaded_at":   now,
		"payment_proof_review_status": reviewStatus,
		"payment_proof_reviewed_by":   reviewedBy,
		"payment_proof_reviewed_at":   reviewedAt,
		"payment_proof_review_notes":  reviewNotes,
		"updated_at":                  now,
	}
	if err := s.db.Model(&purchasePayment).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to attach payment proof: %w", err)
	}

	if err := s.db.
		Preload("Payment").
		Preload("PurchasePaymentTerm").
		Preload("CashBank").
		Preload("User").
		Preload("PaymentProofUploader").
		Preload("PaymentProofReviewer").
		First(&purchasePayment, purchasePayment.ID).Error; err != nil {
		return nil, err
	}

	return &purchasePayment, nil
}

func (s *PurchaseService) GetPurchasePaymentProofForDownload(paymentID uint, userID uint, userRole string) (*models.PurchasePayment, error) {
	var payment models.PurchasePayment
	err := s.db.
		Preload("Purchase").
		Preload("Purchase.PurchaseRequest").
		Preload("PaymentProofUploader").
		Preload("PaymentProofReviewer").
		First(&payment, paymentID).Error
	if err != nil {
		return nil, fmt.Errorf("payment proof not found: %v", err)
	}
	if strings.TrimSpace(payment.PaymentProofFilePath) == "" {
		return nil, errors.New("bukti pembayaran bank belum diunggah")
	}

	role := models.NormalizeRole(userRole)
	if models.CanViewAllTransactionData(role) || role == models.RoleAdmin || role == models.RoleDirector {
		return &payment, nil
	}
	if payment.Purchase.UserID == userID {
		return &payment, nil
	}

	canDownloadFromPR, err := s.userCanAccessPaymentProofFromPurchaseRequest(payment.PurchaseID, userID)
	if err != nil {
		return nil, err
	}
	if canDownloadFromPR && s.canReviewPurchasePaymentProof(payment.Purchase) {
		if strings.ToUpper(strings.TrimSpace(payment.PaymentProofReviewStatus)) != models.PurchasePaymentProofApproved {
			return nil, errors.New("bukti pembayaran bank belum disetujui Finance Manager")
		}
		return &payment, nil
	}

	return nil, errors.New("access denied: you can only download payment proof for your own finalized purchase request")
}

func (s *PurchaseService) ReviewPurchasePaymentProof(paymentID uint, reviewerID uint, reviewerRole string, status string, notes string) (*models.PurchasePayment, error) {
	role := models.NormalizeRole(reviewerRole)
	if !models.IsFinanceManagerRole(role) && role != models.RoleAdmin {
		return nil, errors.New("hanya Finance Manager yang dapat mereview bukti pembayaran bank")
	}

	status = strings.ToUpper(strings.TrimSpace(status))
	switch status {
	case models.PurchasePaymentProofApproved, models.PurchasePaymentProofRejected:
	default:
		return nil, errors.New("status review bukti pembayaran harus APPROVED atau REJECTED")
	}
	if status == models.PurchasePaymentProofRejected && strings.TrimSpace(notes) == "" {
		return nil, errors.New("catatan wajib diisi saat menolak bukti pembayaran bank")
	}

	var payment models.PurchasePayment
	if err := s.db.Preload("Purchase").First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment proof not found: %v", err)
	}
	if strings.TrimSpace(payment.PaymentProofFilePath) == "" {
		return nil, errors.New("bukti pembayaran bank belum diunggah")
	}
	if !s.canReviewPurchasePaymentProof(payment.Purchase) {
		return nil, errors.New("review bukti pembayaran bank hanya untuk purchase yang sudah disetujui Finance Manager")
	}

	now := time.Now()
	if err := s.db.Model(&payment).Updates(map[string]interface{}{
		"payment_proof_review_status": status,
		"payment_proof_reviewed_by":   reviewerID,
		"payment_proof_reviewed_at":   now,
		"payment_proof_review_notes":  strings.TrimSpace(notes),
		"updated_at":                  now,
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to review payment proof: %w", err)
	}

	if err := s.db.Preload("Purchase").
		Preload("CashBank").
		Preload("User").
		Preload("PaymentProofUploader").
		Preload("PaymentProofReviewer").
		First(&payment, payment.ID).Error; err != nil {
		return nil, err
	}

	return &payment, nil
}

func (s *PurchaseService) markPurchasePaymentProofReviewed(purchaseID uint, reviewerID uint, status string, notes string) error {
	now := time.Now()
	return s.db.Model(&models.PurchasePayment{}).
		Where("purchase_id = ? AND payment_proof_file_path <> '' AND deleted_at IS NULL", purchaseID).
		Updates(map[string]interface{}{
			"payment_proof_review_status": strings.TrimSpace(status),
			"payment_proof_reviewed_by":   reviewerID,
			"payment_proof_reviewed_at":   now,
			"payment_proof_review_notes":  strings.TrimSpace(notes),
			"updated_at":                  now,
		}).Error
}

func (s *PurchaseService) userCanAccessPaymentProofFromPurchaseRequest(purchaseID uint, userID uint) (bool, error) {
	var count int64
	err := s.db.Raw(`
		SELECT COUNT(1)
		FROM purchase_requests pr
		WHERE pr.requester_id = ?
			AND pr.deleted_at IS NULL
			AND (
				pr.id = (
					SELECT purchase_request_id
					FROM purchases
					WHERE id = ? AND deleted_at IS NULL
				)
				OR EXISTS (
					SELECT 1
					FROM purchase_request_items pri
					WHERE pri.purchase_request_id = pr.id
						AND pri.purchase_id = ?
						AND pri.deleted_at IS NULL
				)
				OR EXISTS (
					SELECT 1
					FROM purchase_items pi
					JOIN purchase_request_items pri
						ON pri.id = pi.purchase_request_item_id
						AND pri.deleted_at IS NULL
					WHERE pi.purchase_id = ?
						AND pi.deleted_at IS NULL
						AND pri.purchase_request_id = pr.id
				)
			)
	`, userID, purchaseID, purchaseID, purchaseID).Scan(&count).Error
	return count > 0, err
}

func (s *PurchaseService) isPurchaseFinalizedForRequester(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case models.PurchaseStatusApproved, models.PurchaseStatusCompleted, models.PurchaseStatusPaid:
		return true
	default:
		return false
	}
}

func (s *PurchaseService) canReviewPurchasePaymentProof(purchase models.Purchase) bool {
	if s.isPurchaseFinalizedForRequester(purchase.Status) {
		return true
	}

	if strings.ToUpper(strings.TrimSpace(purchase.ApprovalStatus)) != models.PurchaseApprovalApproved {
		return false
	}

	switch strings.ToUpper(strings.TrimSpace(purchase.Status)) {
	case models.PurchaseStatusPendingAccounting, models.PurchaseStatusPendingFinalApproval:
		return true
	default:
		return false
	}
}

// isImmediatePayment checks if payment method requires immediate payment
func isImmediatePayment(paymentMethod string) bool {
	return paymentMethod == models.PurchasePaymentCash ||
		paymentMethod == models.PurchasePaymentTransfer ||
		paymentMethod == models.PurchasePaymentCheck
}

func expectedCashBankTypeForPurchaseMethod(paymentMethod string) string {
	method := strings.ToUpper(strings.TrimSpace(paymentMethod))
	if method == "CASH" || method == "TUNAI" {
		return models.CashBankTypeCash
	}
	return models.CashBankTypeBank
}

func calculatePurchaseNativeOutflow(cashBank *models.CashBank, bookAmountIDR float64) (float64, float64, error) {
	if cashBank == nil {
		return 0, 0, fmt.Errorf("cash/bank account is required")
	}
	if bookAmountIDR <= 0 {
		return 0, 0, fmt.Errorf("book amount must be greater than zero")
	}
	if !cashBank.IsForeignCurrency() {
		return bookAmountIDR, 1, nil
	}
	if cashBank.Balance <= 0 {
		return 0, 0, fmt.Errorf("foreign currency balance is not available")
	}

	bookBalance := cashBank.EffectiveBookBalanceIDR()
	carryRate := 0.0
	if bookBalance > 0 {
		carryRate = bookBalance / cashBank.Balance
	}
	if carryRate <= 0 {
		carryRate = cashBank.EffectiveExchangeRate()
	}
	if carryRate <= 0 {
		return 0, 0, fmt.Errorf("exchange rate is not available for %s", cashBank.Name)
	}

	nativeAmount := bookAmountIDR / carryRate
	if nativeAmount <= 0 {
		return 0, 0, fmt.Errorf("native amount must be greater than zero")
	}
	if nativeAmount-cashBank.Balance > 0.000001 {
		return 0, 0, fmt.Errorf(
			"insufficient balance in %s. Available: %.2f %s, Required: %.2f %s",
			cashBank.Name,
			cashBank.Balance,
			normalizeCashBankCurrency(cashBank.Currency),
			nativeAmount,
			normalizeCashBankCurrency(cashBank.Currency),
		)
	}

	return nativeAmount, carryRate, nil
}

// updateProductStockOnApproval updates product stock when purchase is approved
func (s *PurchaseService) updateProductStockOnApproval(purchase *models.Purchase) (map[uint]struct{}, error) {
	fmt.Printf("?? Starting stock update for purchase %s with %d items\n", purchase.Code, len(purchase.PurchaseItems))
	affectedProductIDs := make(map[uint]struct{})

	for _, item := range purchase.PurchaseItems {
		// Skip ASSET items - they don't have inventory/stock
		if item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset {
			fmt.Printf("?? Skipping stock update for ASSET item '%s' (PurchaseItem:%d)\n", item.Description, item.ID)
			continue
		}

		// Skip items without ProductID (safety check)
		if item.ProductID == nil || *item.ProductID == 0 {
			fmt.Printf("?? Skipping stock update for item without ProductID (PurchaseItem:%d)\n", item.ID)
			continue
		}

		// Get current product data
		product, err := s.productRepo.FindByID(*item.ProductID)
		if err != nil {
			fmt.Printf("? Error finding product %d: %v\n", *item.ProductID, err)
			continue // Skip this item but continue with others
		}

		if product.IsAssembly {
			fmt.Printf("?? Skipping stock update for assembly product %d (%s)\n", product.ID, product.Name)
			continue
		}

		if product.IsService {
			fmt.Printf("?? Skipping stock update for service product %d (%s)\n", product.ID, product.Name)
			continue
		}

		fmt.Printf("?? Product %d (%s): Current stock = %d, Adding quantity = %d\n",
			product.ID, product.Name, product.Stock, item.Quantity)

		purchaseUnitCost := purchaseItemLandedUnitCost(purchase, &item)

		// Update stock quantity (add purchased quantity)
		oldStock := product.Stock
		product.Stock += item.Quantity

		// Update cost price using weighted average if we have existing stock
		if oldStock > 0 {
			// Weighted average: (old_stock * old_price + new_qty * new_price) / total_qty
			totalValue := (float64(oldStock) * product.PurchasePrice) + (float64(item.Quantity) * purchaseUnitCost)
			totalQuantity := oldStock + item.Quantity
			product.PurchasePrice = totalValue / float64(totalQuantity)
			fmt.Printf("?? Updated weighted average price: %.2f (was %.2f)\n",
				product.PurchasePrice, (float64(oldStock)*product.PurchasePrice)/float64(oldStock))
		} else {
			// If no existing stock, use new price
			product.PurchasePrice = purchaseUnitCost
			fmt.Printf("?? Set new purchase price: %.2f\n", product.PurchasePrice)
		}

		// Keep cost_price aligned with the latest weighted purchase basis so sales
		// can use it as estimated HPP for backorders.
		product.CostPrice = product.PurchasePrice

		// Save updated product
		err = s.productRepo.Update(context.Background(), product)
		if err != nil {
			fmt.Printf("? Failed to update product %d stock: %v\n", product.ID, err)
			return affectedProductIDs, fmt.Errorf("failed to update stock for product %d: %v", product.ID, err)
		}
		affectedProductIDs[product.ID] = struct{}{}

		fmt.Printf("? Product %d stock updated: %d ? %d\n", product.ID, oldStock, product.Stock)
	}

	fmt.Printf("?? Stock update completed for purchase %s\n", purchase.Code)
	return affectedProductIDs, nil
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
		fmt.Printf("?? Correcting purchase payment amounts: Outstanding=%.2f, should be %.2f\n",
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

	fmt.Printf("? Accounts payable tracking initialized for credit purchase %s: Outstanding=%.2f\n",
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
	fmt.Printf("?? Updating cash/bank balance for immediate payment purchase %s (method: %s, amount: %.2f)\n",
		purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	// Only update balance for immediate payment methods
	if !isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("? Skipping cash/bank balance update for %s payment method\n", purchase.PaymentMethod)
		return nil // No balance update needed for credit purchases
	}

	// ? CRITICAL FIX: Determine cash/bank account with fallback
	var cashBankID uint

	if purchase.BankAccountID != nil && *purchase.BankAccountID > 0 {
		// Use specified bank account
		cashBankID = *purchase.BankAccountID
		fmt.Printf("?? Using specified bank account ID %d for balance update\n", cashBankID)
	} else {
		// ? FALLBACK: Find default cash_bank account based on payment method
		fmt.Printf("?? No bank account specified, using fallback to default account\n")

		var defaultAccountCode string
		method := strings.ToUpper(strings.TrimSpace(purchase.PaymentMethod))
		if method == "CASH" || method == "TUNAI" {
			defaultAccountCode = "1101" // Kas
		} else {
			defaultAccountCode = "1101" // Bank accounts are also under 1101 (KAS DAN SETARA KAS) header - specific bank is resolved via CashBank entity
		}

		// Find account by code
		var account models.Account
		if err := s.db.Where("code = ? AND deleted_at IS NULL", defaultAccountCode).First(&account).Error; err != nil {
			fmt.Printf("? Default account %s not found: %v\n", defaultAccountCode, err)
			return fmt.Errorf("default account %s not found: %v", defaultAccountCode, err)
		}

		// Find cash_bank linked to this account
		var cashBank models.CashBank
		if err := s.db.Where("account_id = ? AND deleted_at IS NULL", account.ID).First(&cashBank).Error; err != nil {
			fmt.Printf("? Cash/Bank record for account %s not found: %v\n", defaultAccountCode, err)
			return fmt.Errorf("cash_bank record for account %s not found: %v", defaultAccountCode, err)
		}

		cashBankID = cashBank.ID
		fmt.Printf("? Using fallback cash/bank: %s (ID: %d, Code: %s)\n", cashBank.Name, cashBankID, defaultAccountCode)

		// Update purchase.BankAccountID for consistency
		purchase.BankAccountID = &cashBankID
	}

	// Get bank account details using determined cashBankID
	var cashBank models.CashBank
	if err := s.db.First(&cashBank, cashBankID).Error; err != nil {
		fmt.Printf("? Failed to retrieve bank account %d: %v\n", cashBankID, err)
		return fmt.Errorf("bank account not found: %v", err)
	}
	fmt.Printf("?? Bank account retrieved: %s (Current Balance: %.2f)\n", cashBank.Name, cashBank.Balance)

	// Double-check balance is still sufficient (safety check)
	if cashBank.Balance < purchase.TotalAmount {
		fmt.Printf("? Insufficient balance during update: %s has %.2f but requires %.2f\n",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount)
		return fmt.Errorf("insufficient balance in %s. Available: %.2f, Required: %.2f",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount)
	}

	// Update balance - decrease for outgoing payment
	oldBalance := cashBank.Balance
	cashBank.Balance -= purchase.TotalAmount

	// Save updated balance
	if err := s.db.Save(&cashBank).Error; err != nil {
		fmt.Printf("? Failed to update cash/bank balance: %v\n", err)
		return fmt.Errorf("failed to update cash/bank balance: %v", err)
	}
	fmt.Printf("?? Balance updated: %s %.2f ? %.2f (decreased by %.2f)\n",
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
		fmt.Printf("?? Warning: Failed to create cash/bank transaction record: %v\n", err)
		// Don't fail the entire process for transaction record creation failure
	} else {
		fmt.Printf("?? Cash/bank transaction record created: ID %d\n", cashBankTransaction.ID)
	}

	fmt.Printf("? Cash/bank balance update completed successfully for purchase %s\n", purchase.Code)
	return nil
}

// validateBankBalanceForPurchase validates that bank account has sufficient balance for immediate payment purchases
func (s *PurchaseService) validateBankBalanceForPurchase(purchase *models.Purchase) error {
	fmt.Printf("? Validating bank balance for purchase %s (method: %s, amount: %.2f)\n", purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)
	// Only validate for immediate payment methods
	if !isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("? Skipping bank balance validation for %s payment method\n", purchase.PaymentMethod)
		return nil // No validation needed for credit purchases
	}

	// Check if bank account is specified
	fmt.Printf("? Checking if bank account is specified for %s payment\n", purchase.PaymentMethod)
	if purchase.BankAccountID == nil {
		fmt.Printf("? Bank account ID is required but not provided for %s payment method\n", purchase.PaymentMethod)
		return fmt.Errorf("bank account is required for %s payment method", purchase.PaymentMethod)
	}
	fmt.Printf("? Bank account ID %d provided for validation\n", *purchase.BankAccountID)

	// Get bank account details
	fmt.Printf("? Retrieving bank account details for ID %d\n", *purchase.BankAccountID)
	var cashBank models.CashBank
	if err := s.db.First(&cashBank, *purchase.BankAccountID).Error; err != nil {
		fmt.Printf("? Failed to retrieve bank account %d: %v\n", *purchase.BankAccountID, err)
		return fmt.Errorf("bank account not found: %v", err)
	}
	fmt.Printf("? Bank account retrieved: %s (Balance: %.2f)\n", cashBank.Name, cashBank.Balance)

	// Check if balance is sufficient
	fmt.Printf("? Checking balance sufficiency: Available=%.2f, Required=%.2f\n", cashBank.Balance, purchase.TotalAmount)
	if cashBank.Balance < purchase.TotalAmount {
		fmt.Printf("? Insufficient balance: %s has %.2f but requires %.2f (shortfall: %.2f)\n",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount, purchase.TotalAmount-cashBank.Balance)
		return fmt.Errorf("insufficient balance in %s. Available: %.2f, Required: %.2f",
			cashBank.Name, cashBank.Balance, purchase.TotalAmount)
	}

	fmt.Printf("? Bank balance validation passed: %s has sufficient balance (%.2f >= %.2f)\n",
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

	if err := s.SyncPurchasePaymentTermProgress(purchaseID); err != nil {
		return fmt.Errorf("failed to sync purchase payment term progress: %v", err)
	}

	return nil
}

// SSOT Journal Integration Methods

// createSSOTPurchaseJournalEntries creates journal entries using SSOT Journal System
func (s *PurchaseService) createSSOTPurchaseJournalEntries(purchase *models.Purchase, userID uint) error {
	if s.ssotJournalAdapter == nil {
		fmt.Printf("?? SSOT Journal Adapter not initialized, falling back to legacy journal system\n")
		return s.createAndPostPurchaseJournalEntries(purchase, userID)
	}

	// Create SSOT journal entry
	ctx := context.Background()
	journalEntry, err := s.ssotJournalAdapter.CreatePurchaseJournalEntry(ctx, purchase, uint64(userID))
	if err != nil {
		fmt.Printf("?? SSOT create failed (%v).\n", err)
		return fmt.Errorf("failed SSOT: %v", err)
	}

	fmt.Printf("? SSOT Journal Entry created: %s (ID: %d) for purchase %s\n",
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
		fmt.Printf("?? SSOT Journal Adapter not initialized for payment journal\n")
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

	fmt.Printf("? Purchase payment journal entry created successfully for purchase %d\n", purchaseID)
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

// BackfillPurchaseJournal creates the missing SSOT purchase journal for an already-approved purchase.
func (s *PurchaseService) BackfillPurchaseJournal(purchaseID uint) ([]models.SSOTJournalEntry, error) {
	if s.journalServiceSSOT == nil {
		return nil, fmt.Errorf("SSOT journal service not available")
	}

	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get purchase: %v", err)
	}
	if !s.journalServiceSSOT.ShouldPostToJournal(purchase.Status) {
		return nil, fmt.Errorf("purchase status %s cannot be posted to journal", purchase.Status)
	}
	if err := CheckClosedPeriod(s.db, purchase.Date); err != nil {
		return nil, err
	}
	if s.purchaseNeedsAccountingCOA(purchase) {
		if markErr := s.markPurchasePendingAccounting(purchase); markErr != nil {
			return nil, fmt.Errorf("purchase needs GL/COA input and could not be marked pending accounting: %v", markErr)
		}
		return nil, fmt.Errorf("purchase needs Accounting GL/COA input before journal can be created")
	}

	if err := s.journalServiceSSOT.CreatePurchaseJournal(purchase, nil); err != nil {
		return nil, err
	}

	return s.GetPurchaseJournalEntries(purchaseID)
}

// PayPurchasePPh records settlement of purchase-side PPh payable via cash/bank.
func (s *PurchaseService) PayPurchasePPh(purchaseID uint, request models.PurchasePPhPaymentRequest, userID uint) (*models.Purchase, error) {
	paymentDate, err := parsePurchasePPhPaymentDate(request.PaymentDate)
	if err != nil {
		return nil, fmt.Errorf("tanggal pembayaran PPh tidak valid: %v", err)
	}

	amount := roundPurchaseMoney(request.Amount)
	if amount <= 0.01 {
		return nil, fmt.Errorf("nominal pembayaran PPh harus lebih besar dari 0")
	}

	var updatedPurchase models.Purchase
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var purchase models.Purchase
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Vendor").
			Preload("PurchaseItems.Product").
			First(&purchase, purchaseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("purchase not found")
			}
			return fmt.Errorf("failed to load purchase: %v", err)
		}

		status := strings.ToUpper(strings.TrimSpace(purchase.Status))
		if status != models.PurchaseStatusApproved && status != models.PurchaseStatusCompleted && status != models.PurchaseStatusPaid {
			return fmt.Errorf("PPh purchase hanya bisa dibayar setelah purchase approved/completed/paid")
		}
		if err := CheckClosedPeriod(tx, *paymentDate); err != nil {
			return err
		}

		totalPPh := purchasePPhTotal(&purchase)
		if totalPPh <= 0.01 {
			return fmt.Errorf("purchase ini tidak memiliki PPh 21/23 untuk dibayar")
		}
		if purchase.PPhPaidAmount < 0 {
			purchase.PPhPaidAmount = 0
		}
		if purchase.PPhPaidAmount > totalPPh {
			purchase.PPhPaidAmount = totalPPh
		}

		remaining := roundPurchaseMoney(totalPPh - purchase.PPhPaidAmount)
		if remaining <= 0.01 {
			return fmt.Errorf("PPh purchase sudah dibayar")
		}
		if amount > remaining+0.01 {
			return fmt.Errorf("nominal pembayaran melebihi sisa PPh. Sisa: %.2f", remaining)
		}

		sourceID := uint64(purchase.ID)
		var purchaseJournalCount int64
		if err := tx.Model(&models.SSOTJournalEntry{}).
			Where("source_type = ? AND source_id = ? AND deleted_at IS NULL", models.SSOTSourceTypePurchase, sourceID).
			Count(&purchaseJournalCount).Error; err != nil {
			return fmt.Errorf("failed to check purchase journal: %v", err)
		}
		if purchaseJournalCount == 0 {
			if s.journalServiceSSOT == nil {
				return fmt.Errorf("jurnal purchase belum terbentuk dan SSOT journal service tidak tersedia")
			}
			if s.purchaseNeedsAccountingCOA(&purchase) {
				return fmt.Errorf("purchase masih membutuhkan input GL/COA sebelum PPh bisa dibayar")
			}
			if err := s.journalServiceSSOT.CreatePurchaseJournal(&purchase, tx); err != nil {
				return fmt.Errorf("gagal membuat jurnal purchase sebelum bayar PPh: %v", err)
			}
		}

		var cashBank models.CashBank
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL AND is_active = ?", request.CashBankID, true).
			First(&cashBank).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("akun kas/bank tidak ditemukan atau tidak aktif")
			}
			return fmt.Errorf("failed to load cash/bank account: %v", err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("akun kas/bank belum terhubung ke GL/COA")
		}
		if strings.ToUpper(strings.TrimSpace(cashBank.Currency)) != "IDR" {
			return fmt.Errorf("pembayaran PPh purchase hanya didukung dari akun kas/bank IDR")
		}
		if cashBank.AvailableBalance()+0.01 < amount {
			return fmt.Errorf("saldo kas/bank tidak cukup. Saldo tersedia: %.2f", cashBank.AvailableBalance())
		}

		taxHelper := NewTaxAccountHelper(tx)
		type pphComponent struct {
			label   string
			amount  float64
			account *models.Account
		}
		components := make([]pphComponent, 0, 2)
		if purchase.PPh21Amount > 0.01 {
			account, err := taxHelper.GetPurchaseWithholdingTax21PayableAccount(tx)
			if err != nil {
				return fmt.Errorf("akun hutang PPh 21 tidak ditemukan: %v", err)
			}
			components = append(components, pphComponent{label: "PPh 21", amount: roundPurchaseMoney(purchase.PPh21Amount), account: account})
		}
		if purchase.PPh23Amount > 0.01 {
			account, err := taxHelper.GetPurchaseWithholdingTax23PayableAccount(tx)
			if err != nil {
				return fmt.Errorf("akun hutang PPh 23 tidak ditemukan: %v", err)
			}
			components = append(components, pphComponent{label: "PPh 23", amount: roundPurchaseMoney(purchase.PPh23Amount), account: account})
		}
		if len(components) == 0 {
			return fmt.Errorf("purchase ini tidak memiliki komponen PPh 21/23 yang bisa dibayar")
		}

		paidRatio := 0.0
		if totalPPh > 0 {
			paidRatio = math.Min(math.Max(purchase.PPhPaidAmount/totalPPh, 0), 1)
		}
		paymentRatio := amount / remaining
		allocated := 0.0
		lines := make([]JournalLineRequest, 0, len(components)+1)
		for index, component := range components {
			componentRemaining := roundPurchaseMoney(component.amount * (1 - paidRatio))
			lineAmount := roundPurchaseMoney(componentRemaining * paymentRatio)
			if index == len(components)-1 {
				lineAmount = roundPurchaseMoney(amount - allocated)
			}
			if lineAmount <= 0.01 {
				continue
			}
			allocated = roundPurchaseMoney(allocated + lineAmount)
			lines = append(lines, JournalLineRequest{
				AccountID:    uint64(component.account.ID),
				DebitAmount:  decimal.NewFromFloat(lineAmount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Settlement hutang %s purchase - %s", component.label, purchase.Code),
			})
		}
		if math.Abs(allocated-amount) > 0.01 {
			return fmt.Errorf("alokasi pembayaran PPh tidak balance")
		}

		reference := strings.TrimSpace(request.Reference)
		if reference == "" {
			reference = fmt.Sprintf("PAY-PPH-%s", purchase.Code)
		}
		notes := strings.TrimSpace(request.Notes)
		if notes == "" {
			notes = fmt.Sprintf("Pembayaran PPh purchase %s", purchase.Code)
		}

		lines = append(lines, JournalLineRequest{
			AccountID:    uint64(cashBank.AccountID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(amount),
			Description:  fmt.Sprintf("Kas/bank keluar bayar PPh purchase - %s", purchase.Code),
		})

		journalService := NewUnifiedJournalService(tx)
		journalEntry, err := journalService.CreateJournalEntryWithTx(tx, &JournalEntryRequest{
			EntryDate:   *paymentDate,
			Reference:   reference,
			Description: fmt.Sprintf("Pembayaran PPh Purchase - %s", purchase.Code),
			Notes:       notes,
			SourceType:  models.SSOTSourceTypePurchasePPhPayment,
			SourceID:    sourceID,
			AutoPost:    true,
			CreatedBy:   uint64(userID),
			Lines:       lines,
		})
		if err != nil {
			return fmt.Errorf("gagal membuat jurnal pembayaran PPh: %v", err)
		}

		cashBank.Balance = roundPurchaseMoney(cashBank.Balance - amount)
		cashBank.BookBalanceIDR = cashBank.Balance
		cashBank.LastExchangeRate = 1
		if err := tx.Save(&cashBank).Error; err != nil {
			return fmt.Errorf("failed to update cash/bank balance: %v", err)
		}

		cashBankTransaction := &models.CashBankTransaction{
			CashBankID:      cashBank.ID,
			ReferenceType:   models.SSOTSourceTypePurchasePPhPayment,
			ReferenceID:     purchase.ID,
			Amount:          -amount,
			Currency:        "IDR",
			ExchangeRate:    1,
			BookAmountIDR:   -amount,
			BalanceAfter:    cashBank.Balance,
			TransactionDate: *paymentDate,
			Notes:           notes,
		}
		if err := tx.Create(cashBankTransaction).Error; err != nil {
			return fmt.Errorf("failed to record cash/bank transaction: %v", err)
		}

		guard := NewCashBankCurrencyGuard(s.db)
		if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
			return fmt.Errorf("failed to validate cash/bank balance: %v", err)
		}

		paidAmount := roundPurchaseMoney(purchase.PPhPaidAmount + amount)
		paymentJournalID := journalEntry.ID
		updates := map[string]interface{}{
			"pph_paid_amount":              paidAmount,
			"pph_payment_status":           purchasePPhPaymentStatus(totalPPh, paidAmount),
			"pph_payment_date":             *paymentDate,
			"pph_payment_reference":        reference,
			"pph_payment_notes":            notes,
			"pph_payment_journal_entry_id": paymentJournalID,
			"updated_at":                   time.Now(),
		}
		if err := tx.Model(&purchase).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update purchase PPh status: %v", err)
		}

		return tx.
			Preload("Vendor").
			Preload("User").
			Preload("PurchaseItems.Product").
			First(&updatedPurchase, purchase.ID).Error
	})
	if err != nil {
		return nil, err
	}

	return &updatedPurchase, nil
}

// GetDB returns the database connection for external services
func (s *PurchaseService) GetDB() *gorm.DB {
	return s.db
}

func (s *PurchaseService) ensureImmediatePurchasePaymentRecord(purchase *models.Purchase) error {
	if purchase == nil || purchase.ID == 0 || !isImmediatePayment(purchase.PaymentMethod) {
		return nil
	}

	amount := purchase.PaidAmount
	if amount <= 0 {
		amount = purchase.TotalAmount
	}
	if amount <= 0 {
		return nil
	}

	if !s.db.Migrator().HasTable(&models.PurchasePayment{}) {
		if err := s.db.AutoMigrate(&models.PurchasePayment{}); err != nil {
			return err
		}
	}

	var existing models.PurchasePayment
	if err := s.db.Where("purchase_id = ? AND payment_id IS NULL AND deleted_at IS NULL", purchase.ID).
		Find(&existing).Error; err != nil {
		return err
	}

	paymentNumber := fmt.Sprintf("PAY-%s", strings.ReplaceAll(purchase.Code, "/", "-"))
	notes := strings.TrimSpace(purchase.PaymentReference)
	if notes == "" {
		notes = fmt.Sprintf("Pembayaran langsung purchase %s", purchase.Code)
	}

	values := map[string]interface{}{
		"payment_number": paymentNumber,
		"date":           purchase.Date,
		"amount":         amount,
		"method":         purchase.PaymentMethod,
		"reference":      purchase.PaymentReference,
		"notes":          notes,
		"cash_bank_id":   purchase.BankAccountID,
		"user_id":        purchase.UserID,
		"payment_id":     nil,
	}

	if existing.ID != 0 {
		return s.db.Model(&existing).Updates(values).Error
	}

	record := models.PurchasePayment{
		PurchaseID:    purchase.ID,
		PaymentNumber: paymentNumber,
		Date:          purchase.Date,
		Amount:        amount,
		Method:        purchase.PaymentMethod,
		Reference:     purchase.PaymentReference,
		Notes:         notes,
		CashBankID:    purchase.BankAccountID,
		UserID:        purchase.UserID,
	}

	return s.db.Create(&record).Error
}

// CreateAndPostLegacyForCLI exposes legacy posting for CLI scripts (backfill)
func (s *PurchaseService) CreateAndPostLegacyForCLI(purchase *models.Purchase) error {
	return s.createAndPostPurchaseJournalEntries(purchase, purchase.UserID)
}

func (s *PurchaseService) updateCashBankBalanceForPurchaseAtomic(purchase *models.Purchase) error {
	fmt.Printf("Updating cash/bank balance atomically for immediate payment purchase %s (method: %s, amount: %.2f)\n",
		purchase.Code, purchase.PaymentMethod, purchase.TotalAmount)

	if !isImmediatePayment(purchase.PaymentMethod) {
		fmt.Printf("Skipping atomic cash/bank balance update for %s payment method\n", purchase.PaymentMethod)
		return nil
	}

	cashBankID, err := s.resolveCashBankIDForPurchaseAtomic(purchase)
	if err != nil {
		return err
	}
	purchase.BankAccountID = &cashBankID

	guard := NewCashBankCurrencyGuard(s.db)
	return s.db.Transaction(func(tx *gorm.DB) error {
		var cashBank models.CashBank
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("bank account not found: %v", err)
		}

		var existingTx models.CashBankTransaction
		err := tx.
			Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ? AND deleted_at IS NULL", cashBankID, "PURCHASE", purchase.ID).
			Order("id DESC").
			First(&existingTx).Error
		if err == nil {
			fmt.Printf("Purchase %s already has cash/bank transaction ID %d, skipping duplicate posting\n", purchase.Code, existingTx.ID)
			return guard.EnforceAfterTransaction(tx, cashBankID)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to check existing cash/bank transaction: %v", err)
		}

		var txSums struct {
			NativeAmount float64 `gorm:"column:native_amount"`
		}
		if err := tx.Table("cash_bank_transactions").
			Where("cash_bank_id = ? AND deleted_at IS NULL", cashBankID).
			Select("COALESCE(SUM(amount), 0) AS native_amount").
			Scan(&txSums).Error; err != nil {
			return fmt.Errorf("failed to sum existing cash/bank transactions: %v", err)
		}

		bookAmountIDR := purchase.TotalAmount
		nativeAmount := purchase.TotalAmount
		exchangeRate := 1.0
		if cashBank.IsForeignCurrency() {
			nativeAmount, exchangeRate, err = calculatePurchaseNativeOutflow(&cashBank, purchase.TotalAmount)
			if err != nil {
				return err
			}
		}

		repairHistoryOnly := math.Abs(cashBank.Balance-(txSums.NativeAmount-nativeAmount)) <= 0.01
		expectedBalanceAfter := cashBank.Balance - nativeAmount
		if repairHistoryOnly {
			expectedBalanceAfter = cashBank.Balance
			fmt.Printf("Detected partial purchase posting for %s: balance already reduced, backfilling history only\n", purchase.Code)
		} else if cashBank.Balance < nativeAmount {
			return fmt.Errorf("insufficient balance in %s. Available: %.2f, Required: %.2f",
				cashBank.Name, cashBank.Balance, nativeAmount)
		}

		currency := normalizeCashBankCurrency(cashBank.Currency)

		cashBankTransaction := &models.CashBankTransaction{
			CashBankID:      cashBankID,
			ReferenceType:   "PURCHASE",
			ReferenceID:     purchase.ID,
			Amount:          -nativeAmount,
			Currency:        currency,
			ExchangeRate:    exchangeRate,
			BookAmountIDR:   -bookAmountIDR,
			BalanceAfter:    expectedBalanceAfter,
			TransactionDate: purchase.Date,
			Notes:           fmt.Sprintf("Payment for purchase %s - %s", purchase.Code, purchase.PaymentMethod),
		}

		if err := tx.Create(cashBankTransaction).Error; err != nil {
			return fmt.Errorf("failed to create cash/bank transaction record: %v", err)
		}

		if err := guard.EnforceAfterTransaction(tx, cashBankID); err != nil {
			return fmt.Errorf("failed to enforce cash/bank invariant for purchase: %v", err)
		}

		fmt.Printf("Cash/bank transaction record created atomically: ID %d\n", cashBankTransaction.ID)
		return nil
	})
}

func (s *PurchaseService) resolveCashBankIDForPurchaseAtomic(purchase *models.Purchase) (uint, error) {
	expectedType := expectedCashBankTypeForPurchaseMethod(purchase.PaymentMethod)

	if purchase.BankAccountID != nil && *purchase.BankAccountID > 0 {
		var cashBank models.CashBank
		if err := s.db.First(&cashBank, *purchase.BankAccountID).Error; err != nil {
			return 0, fmt.Errorf("cash/bank account not found: %v", err)
		}
		if !strings.EqualFold(strings.TrimSpace(cashBank.Type), expectedType) {
			return 0, fmt.Errorf("%s payment must use a %s account, but %s is %s",
				purchase.PaymentMethod,
				strings.ToLower(expectedType),
				cashBank.Name,
				strings.ToLower(cashBank.Type))
		}
		fmt.Printf("Using specified bank account ID %d for atomic balance update\n", *purchase.BankAccountID)
		return *purchase.BankAccountID, nil
	}

	var cashBank models.CashBank
	if err := s.db.
		Where("type = ? AND is_active = ? AND deleted_at IS NULL", expectedType, true).
		Order("id ASC").
		First(&cashBank).Error; err != nil {
		return 0, fmt.Errorf("active %s account not found: %v", strings.ToLower(expectedType), err)
	}

	fmt.Printf("Using fallback cash/bank: %s (ID: %d, Type: %s)\n", cashBank.Name, cashBank.ID, cashBank.Type)
	return cashBank.ID, nil
}
