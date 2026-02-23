package repositories

import (
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

type PurchaseRequestRepository interface {
	Create(pr *models.PurchaseRequest) error
	GetByID(id uint) (*models.PurchaseRequest, error)
	GetAll(department string, status string) ([]models.PurchaseRequest, error)
	Update(pr *models.PurchaseRequest) error
	Delete(id uint) error
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
		Preload("Items").
		Preload("Items.Product").
		Preload("Approver").
		First(&pr, id).Error
	return &pr, err
}

func (r *purchaseRequestRepository) GetAll(department string, status string) ([]models.PurchaseRequest, error) {
	var prs []models.PurchaseRequest
	query := r.db.Preload("Requester").Preload("Project")

	if department != "" {
		query = query.Where("department = ?", department)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at desc").Find(&prs).Error
	return prs, err
}

func (r *purchaseRequestRepository) Update(pr *models.PurchaseRequest) error {
	// GORM's Save updates all fields, including associations if they are included
	return r.db.Save(pr).Error
}

func (r *purchaseRequestRepository) Delete(id uint) error {
	return r.db.Delete(&models.PurchaseRequest{}, id).Error
}
