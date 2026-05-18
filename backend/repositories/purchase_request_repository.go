package repositories

import (
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

type PurchaseRequestRepository interface {
	Create(pr *models.PurchaseRequest) error
	GetByID(id uint) (*models.PurchaseRequest, error)
	GetAll(department string, status string, requesterID uint, availableForPurchase bool, currentPurchaseID uint) ([]models.PurchaseRequest, error)
	Update(pr *models.PurchaseRequest) error
	Delete(id uint) error
	CreateDocument(document *models.PurchaseRequestDocument) error
	GetDocumentsByPurchaseRequestID(purchaseRequestID uint) ([]models.PurchaseRequestDocument, error)
	GetDocumentByID(id uint) (*models.PurchaseRequestDocument, error)
	DeleteDocument(id uint) error
}

type purchaseRequestRepository struct {
	db *gorm.DB
}

func NewPurchaseRequestRepository(db *gorm.DB) PurchaseRequestRepository {
	return &purchaseRequestRepository{db}
}

func (r *purchaseRequestRepository) Create(pr *models.PurchaseRequest) error {
	return r.db.Create(pr).Error
}

func (r *purchaseRequestRepository) GetByID(id uint) (*models.PurchaseRequest, error) {
	var pr models.PurchaseRequest
	err := r.db.Preload("Requester").
		Preload("Project").
		Preload("Vendor").
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Purchase").
		Preload("Documents").
		Preload("Documents.Uploader").
		Preload("Approver").
		Preload("FinanceStaffApprover").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover").
		First(&pr, id).Error
	return &pr, err
}

func (r *purchaseRequestRepository) GetAll(department string, status string, requesterID uint, availableForPurchase bool, currentPurchaseID uint) ([]models.PurchaseRequest, error) {
	var prs []models.PurchaseRequest
	query := r.db.Preload("Requester").
		Preload("Project").
		Preload("Vendor").
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Purchase").
		Preload("Documents").
		Preload("Documents.Uploader").
		Preload("Approver").
		Preload("FinanceStaffApprover").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover")

	if department != "" {
		query = query.Where("department = ?", department)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if requesterID != 0 {
		query = query.Where("requester_id = ?", requesterID)
	}
	if availableForPurchase {
		currentPurchaseCondition := ""
		args := []interface{}{models.PurchaseStatusCancelled}
		if currentPurchaseID > 0 {
			currentPurchaseCondition = "AND purchases.id <> ?"
			args = append(args, currentPurchaseID)
		}

		query = query.
			Where("purchase_requests.status = ?", models.PRStatusApproved).
			Where(`
				NOT EXISTS (
					SELECT 1
					FROM purchases
					WHERE purchases.purchase_request_id = purchase_requests.id
						AND purchases.deleted_at IS NULL
						AND purchases.status <> ?
						`+currentPurchaseCondition+`
				)
			`, args...)
	}

	err := query.Order("created_at desc").Find(&prs).Error
	return prs, err
}

func (r *purchaseRequestRepository) Update(pr *models.PurchaseRequest) error {
	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(pr).Error
}

func (r *purchaseRequestRepository) Delete(id uint) error {
	return r.db.Delete(&models.PurchaseRequest{}, id).Error
}

func (r *purchaseRequestRepository) CreateDocument(document *models.PurchaseRequestDocument) error {
	return r.db.Create(document).Error
}

func (r *purchaseRequestRepository) GetDocumentsByPurchaseRequestID(purchaseRequestID uint) ([]models.PurchaseRequestDocument, error) {
	var documents []models.PurchaseRequestDocument
	err := r.db.Preload("Uploader").
		Where("purchase_request_id = ?", purchaseRequestID).
		Order("created_at asc").
		Find(&documents).Error

	return documents, err
}

func (r *purchaseRequestRepository) GetDocumentByID(id uint) (*models.PurchaseRequestDocument, error) {
	var document models.PurchaseRequestDocument
	err := r.db.Preload("PurchaseRequest").
		Preload("Uploader").
		First(&document, id).Error

	return &document, err
}

func (r *purchaseRequestRepository) DeleteDocument(id uint) error {
	return r.db.Delete(&models.PurchaseRequestDocument{}, id).Error
}
