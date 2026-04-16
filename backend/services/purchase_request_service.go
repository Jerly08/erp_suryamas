package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type PurchaseRequestService interface {
	CreatePurchaseRequest(payload models.PurchaseRequestCreatePayload, userID uint, userRole string) (*models.PurchaseRequest, error)
	GetPurchaseRequests(userID uint, userRole string, department string, status string) ([]models.PurchaseRequest, error)
	GetPurchaseRequestByID(id uint, userID uint, userRole string) (*models.PurchaseRequest, error)
	ReviewPurchaseRequest(id uint, reviewerID uint, reviewerRole string, payload models.PurchaseRequestReviewPayload) (string, error)
	ApprovePurchaseRequest(id uint, approverID uint, approverRole string) (string, error)
	RejectPurchaseRequest(id uint, approverID uint, approverRole string, reason string) (string, error)
	DeletePurchaseRequest(id uint) error
	SyncPurchaseRequestItemsWithPurchase(purchase *models.Purchase) error
	ReleasePurchaseRequestItemsFromPurchase(purchase *models.Purchase) error
}

type purchaseRequestService struct {
	repo                repositories.PurchaseRequestRepository
	productRepo         *repositories.ProductRepository
	notificationService *NotificationService
	db                  *gorm.DB
}

func NewPurchaseRequestService(db *gorm.DB, repo repositories.PurchaseRequestRepository, notificationService *NotificationService) PurchaseRequestService {
	return &purchaseRequestService{
		repo:                repo,
		productRepo:         repositories.NewProductRepository(db),
		notificationService: notificationService,
		db:                  db,
	}
}

// Generate PR Number (Format: PR-YYYYMMDD-ID)
func generatePRNumber(id uint) string {
	now := time.Now()
	return fmt.Sprintf("PR-%s-%04d", now.Format("20060102"), id)
}

func (s *purchaseRequestService) CreatePurchaseRequest(payload models.PurchaseRequestCreatePayload, userID uint, userRole string) (*models.PurchaseRequest, error) {
	var totalEstimatedAmount float64
	var items []models.PurchaseRequestItem
	initialStage := resolveInitialPurchaseRequestStage(userRole)

	for _, rawItem := range payload.Items {
		product, err := s.productRepo.FindByID(rawItem.ProductID)
		if err != nil {
			return nil, fmt.Errorf("invalid product id: %d", rawItem.ProductID)
		}

		totalItemEst := float64(rawItem.Quantity) * rawItem.EstimatedPrice
		totalEstimatedAmount += totalItemEst

		item := models.PurchaseRequestItem{
			ProductID:              product.ID,
			Quantity:               rawItem.Quantity,
			EstimatedPrice:         rawItem.EstimatedPrice,
			TotalEstimatedPrice:    totalItemEst,
			ApprovalStatus:         models.PRItemDecisionPending,
			FinanceStaffDecision:   models.PRItemDecisionPending,
			FinanceManagerDecision: models.PRItemDecisionPending,
			DirectorDecision:       models.PRItemDecisionPending,
			Notes:                  rawItem.Notes,
		}
		items = append(items, item)
	}

	pr := &models.PurchaseRequest{
		RequesterID:          userID,
		Department:           payload.Department,
		ProjectID:            payload.ProjectID,
		TotalEstimatedAmount: totalEstimatedAmount,
		Status:               models.PRStatusPendingApproval,
		CurrentApprovalStage: initialStage,
		Notes:                payload.Notes,
		Items:                items,
	}

	if err := s.repo.Create(pr); err != nil {
		return nil, err
	}

	pr.PRNumber = generatePRNumber(pr.ID)
	if err := s.repo.Update(pr); err != nil {
		return nil, err
	}

	s.notifyPurchaseRequestCreated(pr)

	return s.repo.GetByID(pr.ID)
}

func (s *purchaseRequestService) GetPurchaseRequests(userID uint, userRole string, department string, status string) ([]models.PurchaseRequest, error) {
	var requesterID uint
	if !models.CanViewAllTransactionData(userRole) {
		requesterID = userID
	}

	return s.repo.GetAll(department, status, requesterID)
}

func (s *purchaseRequestService) GetPurchaseRequestByID(id uint, userID uint, userRole string) (*models.PurchaseRequest, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !models.CanViewAllTransactionData(userRole) && pr.RequesterID != userID {
		return nil, gorm.ErrRecordNotFound
	}

	return pr, nil
}

func (s *purchaseRequestService) ReviewPurchaseRequest(id uint, reviewerID uint, reviewerRole string, payload models.PurchaseRequestReviewPayload) (string, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	stage, err := resolvePurchaseRequestReviewStage(pr, reviewerRole)
	if err != nil {
		return "", err
	}

	decisionByItemID, err := buildPurchaseRequestDecisionMap(payload.Items)
	if err != nil {
		return "", err
	}

	now := time.Now()

	switch stage {
	case models.PRApprovalStageFinanceStaff:
		for index := range pr.Items {
			item := &pr.Items[index]
			decision, exists := decisionByItemID[item.ID]
			if !exists {
				return "", fmt.Errorf("finance staff must decide every item before forwarding to finance manager")
			}
			item.FinanceStaffDecision = decision
		}

		pr.Status = models.PRStatusPendingApproval
		pr.CurrentApprovalStage = models.PRApprovalStageFinanceManager
		pr.FinanceStaffApprovedBy = &reviewerID
		pr.FinanceStaffApprovedAt = &now
		pr.RejectionReason = ""

		if err := s.repo.Update(pr); err != nil {
			return "", err
		}

		s.notifyFinanceManagerReview(pr)
		return "Review finance staff berhasil dikirim ke finance manager", nil

	case models.PRApprovalStageFinanceManager:
		approvedCount := 0
		rejectedCount := 0

		for index := range pr.Items {
			item := &pr.Items[index]
			decision, exists := decisionByItemID[item.ID]
			if !exists {
				return "", fmt.Errorf("finance manager must decide every item before final approval")
			}

			item.FinanceManagerDecision = decision
			if payload.EscalateToDirector {
				item.ApprovalStatus = models.PRItemDecisionPending
			} else {
				item.ApprovalStatus = decision
			}

			switch decision {
			case models.PRItemDecisionApproved:
				approvedCount++
			case models.PRItemDecisionRejected:
				rejectedCount++
				item.PurchaseID = nil
				item.PurchasedAt = nil
			}
		}

		pr.FinanceManagerApprovedBy = &reviewerID
		pr.FinanceManagerApprovedAt = &now
		if payload.EscalateToDirector {
			pr.Status = models.PRStatusPendingApproval
			pr.CurrentApprovalStage = models.PRApprovalStageDirector
			pr.ApprovedBy = nil
			pr.ApprovedAt = nil
			pr.RejectionReason = ""

			if err := s.repo.Update(pr); err != nil {
				return "", err
			}

			s.notifyDirectorReview(pr)
			return "Review finance manager berhasil dikirim ke director", nil
		}

		pr.CurrentApprovalStage = models.PRApprovalStageFinalized

		if approvedCount > 0 {
			pr.Status = models.PRStatusApproved
			pr.ApprovedBy = &reviewerID
			pr.ApprovedAt = &now
			pr.RejectionReason = payload.Reason
		} else {
			pr.Status = models.PRStatusRejected
			pr.ApprovedBy = nil
			pr.ApprovedAt = nil
			if payload.Reason != "" {
				pr.RejectionReason = payload.Reason
			} else {
				pr.RejectionReason = "Semua item ditolak oleh finance manager"
			}
		}

		if err := s.repo.Update(pr); err != nil {
			return "", err
		}

		s.notifyRequesterAfterFinalReview(pr, approvedCount, rejectedCount)

		if approvedCount == 0 {
			return "Purchase request ditolak seluruhnya oleh finance manager", nil
		}

		if rejectedCount > 0 {
			return fmt.Sprintf("Final review selesai: %d item disetujui dan %d item ditolak", approvedCount, rejectedCount), nil
		}

		return "Purchase request disetujui seluruhnya oleh finance manager", nil

	case models.PRApprovalStageDirector:
		approvedCount := 0
		rejectedCount := 0

		for index := range pr.Items {
			item := &pr.Items[index]
			decision, exists := decisionByItemID[item.ID]
			if !exists {
				return "", fmt.Errorf("director must decide every item before final approval")
			}

			item.DirectorDecision = decision
			item.ApprovalStatus = decision

			switch decision {
			case models.PRItemDecisionApproved:
				approvedCount++
			case models.PRItemDecisionRejected:
				rejectedCount++
				item.PurchaseID = nil
				item.PurchasedAt = nil
			}
		}

		pr.CurrentApprovalStage = models.PRApprovalStageFinalized
		pr.DirectorApprovedBy = &reviewerID
		pr.DirectorApprovedAt = &now

		if approvedCount > 0 {
			pr.Status = models.PRStatusApproved
			pr.ApprovedBy = &reviewerID
			pr.ApprovedAt = &now
			pr.RejectionReason = payload.Reason
		} else {
			pr.Status = models.PRStatusRejected
			pr.ApprovedBy = nil
			pr.ApprovedAt = nil
			if payload.Reason != "" {
				pr.RejectionReason = payload.Reason
			} else {
				pr.RejectionReason = "Semua item ditolak oleh director"
			}
		}

		if err := s.repo.Update(pr); err != nil {
			return "", err
		}

		s.notifyRequesterAfterFinalReview(pr, approvedCount, rejectedCount)

		if approvedCount == 0 {
			return "Purchase request ditolak seluruhnya oleh director", nil
		}

		if rejectedCount > 0 {
			return fmt.Sprintf("Final review director selesai: %d item disetujui dan %d item ditolak", approvedCount, rejectedCount), nil
		}

		return "Purchase request disetujui seluruhnya oleh director", nil

	default:
		return "", fmt.Errorf("purchase request %s is not in a reviewable stage", pr.PRNumber)
	}
}

func (s *purchaseRequestService) ApprovePurchaseRequest(id uint, approverID uint, approverRole string) (string, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	payload := buildBulkPurchaseRequestReviewPayload(pr, models.PRItemDecisionApproved)
	return s.ReviewPurchaseRequest(id, approverID, approverRole, payload)
}

func (s *purchaseRequestService) RejectPurchaseRequest(id uint, approverID uint, approverRole string, reason string) (string, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	payload := buildBulkPurchaseRequestReviewPayload(pr, models.PRItemDecisionRejected)
	payload.Reason = reason

	return s.ReviewPurchaseRequest(id, approverID, approverRole, payload)
}

func (s *purchaseRequestService) DeletePurchaseRequest(id uint) error {
	return s.repo.Delete(id)
}

func (s *purchaseRequestService) SyncPurchaseRequestItemsWithPurchase(purchase *models.Purchase) error {
	if purchase == nil || purchase.PurchaseRequestID == nil {
		return nil
	}

	selectedItemIDs := make(map[uint]struct{})
	for _, item := range purchase.PurchaseItems {
		if item.PurchaseRequestItemID != nil {
			selectedItemIDs[*item.PurchaseRequestItemID] = struct{}{}
		}
	}

	if len(selectedItemIDs) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var pr models.PurchaseRequest
		if err := tx.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return err
		}

		now := time.Now()
		for index := range pr.Items {
			item := &pr.Items[index]

			if item.PurchaseID != nil && *item.PurchaseID == purchase.ID {
				if _, stillSelected := selectedItemIDs[item.ID]; !stillSelected {
					item.PurchaseID = nil
					item.PurchasedAt = nil
				}
			}

			if _, shouldReserve := selectedItemIDs[item.ID]; shouldReserve {
				if item.ApprovalStatus != models.PRItemDecisionApproved {
					return fmt.Errorf("purchase request item %d is not approved", item.ID)
				}
				if item.PurchaseID != nil && *item.PurchaseID != purchase.ID {
					return fmt.Errorf("purchase request item %d is already linked to another purchase", item.ID)
				}
				item.PurchaseID = &purchase.ID
				item.PurchasedAt = &now
			}
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&pr).Error
	})
}

func (s *purchaseRequestService) ReleasePurchaseRequestItemsFromPurchase(purchase *models.Purchase) error {
	if purchase == nil || purchase.PurchaseRequestID == nil {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var pr models.PurchaseRequest
		if err := tx.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return err
		}

		changed := false
		for index := range pr.Items {
			item := &pr.Items[index]
			if item.PurchaseID != nil && *item.PurchaseID == purchase.ID {
				item.PurchaseID = nil
				item.PurchasedAt = nil
				changed = true
			}
		}

		if !changed {
			return nil
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&pr).Error
	})
}

func resolvePurchaseRequestReviewStage(pr *models.PurchaseRequest, reviewerRole string) (string, error) {
	if pr.Status != models.PRStatusPendingApproval && pr.Status != models.PRStatusDraft {
		return "", fmt.Errorf("purchase request %s is already %s", pr.PRNumber, pr.Status)
	}

	stage := pr.CurrentApprovalStage
	if stage == "" {
		stage = resolveInitialPurchaseRequestStage(pr.Requester.Role)
	}

	roleNorm := models.NormalizeRole(reviewerRole)
	if roleNorm == models.RoleAdmin {
		return stage, nil
	}

	switch stage {
	case models.PRApprovalStageFinanceStaff:
		if models.IsFinanceStaffRole(roleNorm) {
			return stage, nil
		}
		return "", fmt.Errorf("only finance staff can review purchase requests at the first stage")
	case models.PRApprovalStageFinanceManager:
		if models.IsFinanceManagerRole(roleNorm) {
			return stage, nil
		}
		return "", fmt.Errorf("only finance manager can review purchase requests at the finance manager stage")
	case models.PRApprovalStageDirector:
		if roleNorm == models.RoleDirector {
			return stage, nil
		}
		return "", fmt.Errorf("only director can review purchase requests at the director stage")
	default:
		return "", fmt.Errorf("purchase request %s is not in a reviewable stage", pr.PRNumber)
	}
}

func resolveInitialPurchaseRequestStage(requesterRole string) string {
	if models.IsFinanceManagerRole(requesterRole) {
		return models.PRApprovalStageFinanceManager
	}

	return models.PRApprovalStageFinanceStaff
}

func buildPurchaseRequestDecisionMap(items []models.PurchaseRequestReviewItemPayload) (map[uint]string, error) {
	decisionByItemID := make(map[uint]string, len(items))

	for _, item := range items {
		if item.ItemID == 0 {
			return nil, fmt.Errorf("item_id is required")
		}

		decision := normalizePurchaseRequestDecision(item.Decision)
		if decision == "" {
			return nil, fmt.Errorf("invalid decision for item %d", item.ItemID)
		}
		if _, exists := decisionByItemID[item.ItemID]; exists {
			return nil, fmt.Errorf("duplicate decision submitted for item %d", item.ItemID)
		}

		decisionByItemID[item.ItemID] = decision
	}

	return decisionByItemID, nil
}

func buildBulkPurchaseRequestReviewPayload(pr *models.PurchaseRequest, decision string) models.PurchaseRequestReviewPayload {
	payload := models.PurchaseRequestReviewPayload{
		Items: make([]models.PurchaseRequestReviewItemPayload, 0, len(pr.Items)),
	}

	for _, item := range pr.Items {
		payload.Items = append(payload.Items, models.PurchaseRequestReviewItemPayload{
			ItemID:   item.ID,
			Decision: decision,
		})
	}

	return payload
}

func normalizePurchaseRequestDecision(decision string) string {
	switch models.NormalizeRole(decision) {
	case "approved", "approve":
		return models.PRItemDecisionApproved
	case "rejected", "reject":
		return models.PRItemDecisionRejected
	default:
		return ""
	}
}

func (s *purchaseRequestService) notifyPurchaseRequestCreated(pr *models.PurchaseRequest) {
	if s.notificationService == nil {
		return
	}

	switch pr.CurrentApprovalStage {
	case models.PRApprovalStageFinanceManager:
		s.notifyFinanceManagerReview(pr)
		return
	case models.PRApprovalStageDirector:
		s.notifyDirectorReview(pr)
		return
	}

	message := fmt.Sprintf("Purchase request %s needs finance staff review (Amount: %s)",
		pr.PRNumber, fmt.Sprintf("Rp %.2f", pr.TotalEstimatedAmount))

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Finance Staff Review",
		message,
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"total_amount":           pr.TotalEstimatedAmount,
			"department":             pr.Department,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "finance_staff_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleFinanceStaff,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create PR notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyFinanceManagerReview(pr *models.PurchaseRequest) {
	if s.notificationService == nil {
		return
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Final Approval Required",
		fmt.Sprintf("Purchase request %s is ready for finance manager review", pr.PRNumber),
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"total_amount":           pr.TotalEstimatedAmount,
			"department":             pr.Department,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "finance_manager_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleFinance,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create finance manager PR notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyDirectorReview(pr *models.PurchaseRequest) {
	if s.notificationService == nil {
		return
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Director Approval Required",
		fmt.Sprintf("Purchase request %s is ready for director review", pr.PRNumber),
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"total_amount":           pr.TotalEstimatedAmount,
			"department":             pr.Department,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "director_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleDirector,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create director PR notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyRequesterAfterFinalReview(pr *models.PurchaseRequest, approvedCount int, rejectedCount int) {
	if s.notificationService == nil {
		return
	}

	title := "Purchase Request Approved"
	message := fmt.Sprintf("Your purchase request %s has been approved", pr.PRNumber)
	notificationType := models.NotificationTypeApprovalApproved

	if approvedCount == 0 {
		title = "Purchase Request Rejected"
		message = fmt.Sprintf("Your purchase request %s has been rejected", pr.PRNumber)
		notificationType = models.NotificationTypeApprovalRejected
	} else if rejectedCount > 0 {
		message = fmt.Sprintf("Your purchase request %s has been finalized: %d item approved, %d item rejected", pr.PRNumber, approvedCount, rejectedCount)
	}

	if pr.RejectionReason != "" {
		message = fmt.Sprintf("%s. Reason: %s", message, pr.RejectionReason)
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		notificationType,
		title,
		message,
		map[string]interface{}{
			"pr_id":          pr.ID,
			"pr_number":      pr.PRNumber,
			"approved_count": approvedCount,
			"rejected_count": rejectedCount,
			"approved_at":    pr.ApprovedAt,
			"action_type":    "final_review_completed",
		},
		pr.TotalEstimatedAmount,
		models.RoleEmployee,
		pr.RequesterID,
		"",
	); err != nil {
		fmt.Printf("Failed to create requester PR notification: %v\n", err)
	}
}
