package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type PurchaseRequestService interface {
	CreatePurchaseRequest(payload models.PurchaseRequestCreatePayload, userID uint) (*models.PurchaseRequest, error)
	GetPurchaseRequests(department string, status string) ([]models.PurchaseRequest, error)
	GetPurchaseRequestByID(id uint) (*models.PurchaseRequest, error)
	ApprovePurchaseRequest(id uint, approverID uint) error
	RejectPurchaseRequest(id uint, approverID uint, reason string) error
	DeletePurchaseRequest(id uint) error
}

type purchaseRequestService struct {
	repo                repositories.PurchaseRequestRepository
	productRepo         *repositories.ProductRepository // Fix interface incompatibility by using concrete struct type pointer
	notificationService *NotificationService
}

func NewPurchaseRequestService(db *gorm.DB, repo repositories.PurchaseRequestRepository, notificationService *NotificationService) PurchaseRequestService {
	return &purchaseRequestService{
		repo:                repo,
		productRepo:         repositories.NewProductRepository(db),
		notificationService: notificationService,
	}
}

// Generate PR Number (Format: PR-YYYYMMDD-ID)
func generatePRNumber(id uint) string {
	now := time.Now()
	// Pad ID to 4 digits
	return fmt.Sprintf("PR-%s-%04d", now.Format("20060102"), id)
}

func (s *purchaseRequestService) CreatePurchaseRequest(payload models.PurchaseRequestCreatePayload, userID uint) (*models.PurchaseRequest, error) {
	var totalEstimatedAmount float64
	var items []models.PurchaseRequestItem

	// Validate items and calculate total from actual database Product Price
	for _, rawItem := range payload.Items {
		// Fetch product to validate existance and get current prices
		product, err := s.productRepo.FindByID(rawItem.ProductID)
		if err != nil {
			return nil, fmt.Errorf("invalid product id: %d", rawItem.ProductID)
		}

		// Calculate items (Qty * Estimated Price)
		totalItemEst := float64(rawItem.Quantity) * rawItem.EstimatedPrice
		totalEstimatedAmount += totalItemEst

		item := models.PurchaseRequestItem{
			ProductID:           product.ID,
			Quantity:            rawItem.Quantity,
			EstimatedPrice:      rawItem.EstimatedPrice,
			TotalEstimatedPrice: totalItemEst,
			Notes:               rawItem.Notes,
		}
		items = append(items, item)
	}

	pr := &models.PurchaseRequest{
		RequesterID:          userID,
		Department:           payload.Department,
		ProjectID:            payload.ProjectID,
		TotalEstimatedAmount: totalEstimatedAmount,
		Status:               models.PRStatusPendingApproval, // Usually jumps state to finance, based on BRD
		Notes:                payload.Notes,
		Items:                items,
	}

	// Create PR skeleton
	if err := s.repo.Create(pr); err != nil {
		return nil, err
	}

	// Update the PR number string now that we have the auto-increment DB ID
	pr.PRNumber = generatePRNumber(pr.ID)
	if err := s.repo.Update(pr); err != nil {
		return nil, err
	}

	// Trigger smart notification
	if s.notificationService != nil {
		message := fmt.Sprintf("Purchase request %s needs approval (Amount: %s)",
			pr.PRNumber, fmt.Sprintf("Rp %.2f", pr.TotalEstimatedAmount))

		err := s.notificationService.smartService.CreateSmartNotification(
			models.NotificationTypeApprovalPending,
			"Purchase Request Approval Required",
			message,
			map[string]interface{}{
				"pr_id":        pr.ID,
				"pr_number":    pr.PRNumber,
				"total_amount": pr.TotalEstimatedAmount,
				"department":   pr.Department,
				"action_type":  "approval_required",
			},
			pr.TotalEstimatedAmount,
			"finance", // target role
			0,         // target userID (0 means all eligible)
			pr.Department,
		)

		if err != nil {
			fmt.Printf("Failed to create PR notification: %v\n", err)
		}
	}

	return s.GetPurchaseRequestByID(pr.ID) // Return full struct with preloads
}

func (s *purchaseRequestService) GetPurchaseRequests(department string, status string) ([]models.PurchaseRequest, error) {
	return s.repo.GetAll(department, status)
}

func (s *purchaseRequestService) GetPurchaseRequestByID(id uint) (*models.PurchaseRequest, error) {
	return s.repo.GetByID(id)
}

func (s *purchaseRequestService) ApprovePurchaseRequest(id uint, approverID uint) error {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if pr.Status != models.PRStatusPendingApproval && pr.Status != models.PRStatusDraft {
		return fmt.Errorf("purchase request %s is already %s", pr.PRNumber, pr.Status)
	}

	now := time.Now()
	pr.Status = models.PRStatusApproved
	pr.ApprovedBy = &approverID
	pr.ApprovedAt = &now

	err = s.repo.Update(pr)
	if err != nil {
		return err
	}

	// Trigger notification to request creator
	if s.notificationService != nil {
		s.notificationService.smartService.CreateSmartNotification(
			models.NotificationTypeApprovalApproved,
			"Purchase Request Approved",
			fmt.Sprintf("Your purchase request %s has been approved", pr.PRNumber),
			map[string]interface{}{
				"pr_id":       pr.ID,
				"pr_number":   pr.PRNumber,
				"approved_at": pr.ApprovedAt,
				"action_type": "approved",
			},
			pr.TotalEstimatedAmount,
			"employee",     // Role
			pr.RequesterID, // Specifically target the requester
			"",
		)
	}

	return nil
}

func (s *purchaseRequestService) RejectPurchaseRequest(id uint, approverID uint, reason string) error {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if pr.Status != models.PRStatusPendingApproval {
		return fmt.Errorf("only pending purchase requests can be rejected")
	}

	pr.Status = models.PRStatusRejected
	pr.RejectionReason = reason

	err = s.repo.Update(pr)
	if err != nil {
		return err
	}

	// Trigger notification to request creator
	if s.notificationService != nil {
		message := fmt.Sprintf("Your purchase request %s has been rejected", pr.PRNumber)
		if reason != "" {
			message += fmt.Sprintf(". Reason: %s", reason)
		}

		s.notificationService.smartService.CreateSmartNotification(
			models.NotificationTypeApprovalRejected,
			"Purchase Request Rejected",
			message,
			map[string]interface{}{
				"pr_id":       pr.ID,
				"pr_number":   pr.PRNumber,
				"rejected_at": time.Now(),
				"reason":      reason,
				"action_type": "rejected",
			},
			pr.TotalEstimatedAmount,
			"employee",     // Role
			pr.RequesterID, // Target requester directly
			"",
		)
	}

	return nil
}

func (s *purchaseRequestService) DeletePurchaseRequest(id uint) error {
	// Simple deletion logic as skeleton
	return s.repo.Delete(id)
}
