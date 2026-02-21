package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type InvoiceTypeService struct {
	db *gorm.DB
}

func NewInvoiceTypeService(db *gorm.DB) *InvoiceTypeService {
	return &InvoiceTypeService{db: db}
}

// GetInvoiceTypes returns all invoice types with optional filtering
func (s *InvoiceTypeService) GetInvoiceTypes(activeOnly bool) ([]models.InvoiceTypeResponse, error) {
	var invoiceTypes []models.InvoiceType
	query := s.db.Preload("Creator")

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Order("name ASC").Find(&invoiceTypes).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch invoice types: %v", err)
	}

	// Convert to response format
	responses := make([]models.InvoiceTypeResponse, len(invoiceTypes))
	for i, it := range invoiceTypes {
		responses[i] = models.InvoiceTypeResponse{
			ID:          it.ID,
			Name:        it.Name,
			Code:        it.Code,
			Description: it.Description,
			IsActive:    it.IsActive,
			CreatedBy:   it.CreatedBy,
			CreatedAt:   it.CreatedAt,
			UpdatedAt:   it.UpdatedAt,
			Creator:     it.Creator,
		}
	}

	return responses, nil
}

// GetInvoiceTypeByID returns a specific invoice type by ID
func (s *InvoiceTypeService) GetInvoiceTypeByID(id uint) (*models.InvoiceTypeResponse, error) {
	var invoiceType models.InvoiceType
	if err := s.db.Preload("Creator").Where("id = ?", id).First(&invoiceType).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invoice type not found")
		}
		return nil, fmt.Errorf("failed to fetch invoice type: %v", err)
	}

	response := &models.InvoiceTypeResponse{
		ID:          invoiceType.ID,
		Name:        invoiceType.Name,
		Code:        invoiceType.Code,
		Description: invoiceType.Description,
		IsActive:    invoiceType.IsActive,
		CreatedBy:   invoiceType.CreatedBy,
		CreatedAt:   invoiceType.CreatedAt,
		UpdatedAt:   invoiceType.UpdatedAt,
		Creator:     invoiceType.Creator,
	}

	return response, nil
}

// CreateInvoiceType creates a new invoice type
func (s *InvoiceTypeService) CreateInvoiceType(request models.InvoiceTypeCreateRequest, createdBy uint) (*models.InvoiceTypeResponse, error) {
	// Validate code uniqueness
	if err := s.validateCodeUniqueness(request.Code, 0); err != nil {
		return nil, err
	}

	// Create new invoice type
	invoiceType := models.InvoiceType{
		Name:        strings.TrimSpace(request.Name),
		Code:        strings.TrimSpace(strings.ToUpper(request.Code)), // Standardize to uppercase
		Description: strings.TrimSpace(request.Description),
		IsActive:    true,
		CreatedBy:   createdBy,
	}

	if err := s.db.Create(&invoiceType).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("invoice type with code '%s' already exists", request.Code)
		}
		return nil, fmt.Errorf("failed to create invoice type: %v", err)
	}

	// Load with creator info
	if err := s.db.Preload("Creator").First(&invoiceType, invoiceType.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load created invoice type: %v", err)
	}

	response := &models.InvoiceTypeResponse{
		ID:          invoiceType.ID,
		Name:        invoiceType.Name,
		Code:        invoiceType.Code,
		Description: invoiceType.Description,
		IsActive:    invoiceType.IsActive,
		CreatedBy:   invoiceType.CreatedBy,
		CreatedAt:   invoiceType.CreatedAt,
		UpdatedAt:   invoiceType.UpdatedAt,
		Creator:     invoiceType.Creator,
	}

	return response, nil
}

// UpdateInvoiceType updates an existing invoice type
func (s *InvoiceTypeService) UpdateInvoiceType(id uint, request models.InvoiceTypeUpdateRequest) (*models.InvoiceTypeResponse, error) {
	// Check if invoice type exists
	var invoiceType models.InvoiceType
	if err := s.db.Where("id = ?", id).First(&invoiceType).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invoice type not found")
		}
		return nil, fmt.Errorf("failed to fetch invoice type: %v", err)
	}

	// Update fields if provided
	if request.Name != nil {
		invoiceType.Name = strings.TrimSpace(*request.Name)
	}
	if request.Code != nil {
		newCode := strings.TrimSpace(strings.ToUpper(*request.Code))
		// Validate code uniqueness only if it's being changed
		if newCode != invoiceType.Code {
			if err := s.validateCodeUniqueness(newCode, id); err != nil {
				return nil, err
			}
		}
		invoiceType.Code = newCode
	}
	if request.Description != nil {
		invoiceType.Description = strings.TrimSpace(*request.Description)
	}
	if request.IsActive != nil {
		invoiceType.IsActive = *request.IsActive
	}

	// Save changes
	if err := s.db.Save(&invoiceType).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("invoice type with this code already exists")
		}
		return nil, fmt.Errorf("failed to update invoice type: %v", err)
	}

	// Load with creator info
	if err := s.db.Preload("Creator").First(&invoiceType, invoiceType.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load updated invoice type: %v", err)
	}

	response := &models.InvoiceTypeResponse{
		ID:          invoiceType.ID,
		Name:        invoiceType.Name,
		Code:        invoiceType.Code,
		Description: invoiceType.Description,
		IsActive:    invoiceType.IsActive,
		CreatedBy:   invoiceType.CreatedBy,
		CreatedAt:   invoiceType.CreatedAt,
		UpdatedAt:   invoiceType.UpdatedAt,
		Creator:     invoiceType.Creator,
	}

	return response, nil
}

// DeleteInvoiceType soft deletes an invoice type (only if not in use)
func (s *InvoiceTypeService) DeleteInvoiceType(id uint) error {
	// Check if invoice type exists
	var invoiceType models.InvoiceType
	if err := s.db.Where("id = ?", id).First(&invoiceType).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("invoice type not found")
		}
		return fmt.Errorf("failed to fetch invoice type: %v", err)
	}

	// Check if invoice type is in use by any sales
	var salesCount int64
	if err := s.db.Model(&models.Sale{}).Where("invoice_type_id = ?", id).Count(&salesCount).Error; err != nil {
		return fmt.Errorf("failed to check invoice type usage: %v", err)
	}

	if salesCount > 0 {
		return fmt.Errorf("cannot delete invoice type: it is being used by %d sales record(s). Consider deactivating instead", salesCount)
	}

	// Note: We deliberately skip checking for existing counter history.
	// Invoice types with counters can still be soft-deleted. The counters will remain in the DB,
	// but the invoice type will no longer be available for new transactions.

	// Soft delete
	if err := s.db.Delete(&invoiceType).Error; err != nil {
		return fmt.Errorf("failed to delete invoice type: %v", err)
	}

	return nil
}

// ToggleInvoiceType toggles the active status of an invoice type
func (s *InvoiceTypeService) ToggleInvoiceType(id uint) (*models.InvoiceTypeResponse, error) {
	var invoiceType models.InvoiceType
	if err := s.db.Where("id = ?", id).First(&invoiceType).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invoice type not found")
		}
		return nil, fmt.Errorf("failed to fetch invoice type: %v", err)
	}

	// Toggle active status
	invoiceType.IsActive = !invoiceType.IsActive

	if err := s.db.Save(&invoiceType).Error; err != nil {
		return nil, fmt.Errorf("failed to toggle invoice type status: %v", err)
	}

	// Load with creator info
	if err := s.db.Preload("Creator").First(&invoiceType, invoiceType.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load updated invoice type: %v", err)
	}

	response := &models.InvoiceTypeResponse{
		ID:          invoiceType.ID,
		Name:        invoiceType.Name,
		Code:        invoiceType.Code,
		Description: invoiceType.Description,
		IsActive:    invoiceType.IsActive,
		CreatedBy:   invoiceType.CreatedBy,
		CreatedAt:   invoiceType.CreatedAt,
		UpdatedAt:   invoiceType.UpdatedAt,
		Creator:     invoiceType.Creator,
	}

	return response, nil
}

// GetActiveInvoiceTypes returns only active invoice types for dropdowns
func (s *InvoiceTypeService) GetActiveInvoiceTypes() ([]models.InvoiceTypeResponse, error) {
	return s.GetInvoiceTypes(true)
}

// Private helper methods

// validateCodeUniqueness checks if a code is unique (excluding current ID)
func (s *InvoiceTypeService) validateCodeUniqueness(code string, excludeID uint) error {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return fmt.Errorf("invoice type code cannot be empty")
	}

	var count int64
	query := s.db.Model(&models.InvoiceType{}).Where("UPPER(code) = ?", code)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("failed to validate code uniqueness: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("invoice type with code '%s' already exists", code)
	}

	return nil
}
