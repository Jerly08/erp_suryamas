package controllers

import (
	"app-sistem-akuntansi/models"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type ComponentTemplateController struct {
	DB *gorm.DB
}

func NewComponentTemplateController(db *gorm.DB) *ComponentTemplateController {
	return &ComponentTemplateController{DB: db}
}

func isUniqueConstraintError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return strings.Contains(strings.ToLower(err.Error()), "duplicate key value")
}

// GetTemplates returns all component templates with item count
func (ct *ComponentTemplateController) GetTemplates(c *gin.Context) {
	var templates []models.ComponentTemplate

	if err := ct.DB.
		Preload("Items.ComponentProduct").
		Order("created_at DESC").
		Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// GetTemplate returns a single template with full item + product details
func (ct *ComponentTemplateController) GetTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	var template models.ComponentTemplate
	if err := ct.DB.
		Preload("Items.ComponentProduct").
		First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": template})
}

// CreateTemplate creates a new component template
func (ct *ComponentTemplateController) CreateTemplate(c *gin.Context) {
	var payload models.ComponentTemplateCreateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build items
	var items []models.ComponentTemplateItem
	for _, itemPayload := range payload.Items {
		// Validate product exists
		var product models.Product
		if err := ct.DB.First(&product, itemPayload.ComponentProductID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found: " + strconv.Itoa(int(itemPayload.ComponentProductID))})
			return
		}

		items = append(items, models.ComponentTemplateItem{
			ComponentProductID: itemPayload.ComponentProductID,
			CategoryStr:        itemPayload.CategoryStr,
			RequirementQty:     itemPayload.RequirementQty,
			BaseUnit:           itemPayload.BaseUnit,
			Notes:              itemPayload.Notes,
		})
	}

	template := models.ComponentTemplate{
		Name:        payload.Name,
		Description: payload.Description,
	}

	var existingTemplate models.ComponentTemplate
	lookupErr := ct.DB.Unscoped().Where("name = ?", payload.Name).First(&existingTemplate).Error
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate template name: " + lookupErr.Error()})
		return
	}

	hasExistingTemplate := lookupErr == nil
	if hasExistingTemplate && !existingTemplate.DeletedAt.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "Template name already exists"})
		return
	}

	if err := ct.DB.Transaction(func(tx *gorm.DB) error {
		if hasExistingTemplate {
			template.ID = existingTemplate.ID
			if err := tx.Unscoped().
				Model(&template).
				Updates(map[string]interface{}{
					"name":        payload.Name,
					"description": payload.Description,
					"deleted_at":  nil,
				}).Error; err != nil {
				return err
			}

			if err := tx.Unscoped().Where("template_id = ?", template.ID).Delete(&models.ComponentTemplateItem{}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Create(&template).Error; err != nil {
				// Another request may have created the same template name moments earlier.
				// Reuse that row instead of failing the whole request with a 500.
				if !isUniqueConstraintError(err) {
					return err
				}

				if err := tx.Unscoped().Where("name = ?", payload.Name).First(&template).Error; err != nil {
					return err
				}

				if err := tx.Unscoped().
					Model(&template).
					Updates(map[string]interface{}{
						"name":        payload.Name,
						"description": payload.Description,
						"deleted_at":  nil,
					}).Error; err != nil {
					return err
				}

				if err := tx.Unscoped().Where("template_id = ?", template.ID).Delete(&models.ComponentTemplateItem{}).Error; err != nil {
					return err
				}
			}
		}

		if len(items) > 0 {
			for i := range items {
				items[i].TemplateID = template.ID
			}

			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template: " + err.Error()})
		return
	}

	if err := ct.DB.Preload("Items.ComponentProduct").First(&template, template.ID).Error; err != nil {
		c.JSON(http.StatusCreated, gin.H{
			"message": "Template created successfully, but failed to reload items",
			"data":    template,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Template created successfully", "data": template})
}

// UpdateTemplate updates an existing component template (name, description, and replaces items)
func (ct *ComponentTemplateController) UpdateTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	var template models.ComponentTemplate
	if err := ct.DB.First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	var payload models.ComponentTemplateCreateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build new items
	var items []models.ComponentTemplateItem
	for _, itemPayload := range payload.Items {
		var product models.Product
		if err := ct.DB.First(&product, itemPayload.ComponentProductID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found: " + strconv.Itoa(int(itemPayload.ComponentProductID))})
			return
		}
		items = append(items, models.ComponentTemplateItem{
			TemplateID:         uint(id),
			ComponentProductID: itemPayload.ComponentProductID,
			CategoryStr:        itemPayload.CategoryStr,
			RequirementQty:     itemPayload.RequirementQty,
			BaseUnit:           itemPayload.BaseUnit,
			Notes:              itemPayload.Notes,
		})
	}

	if err := ct.DB.Transaction(func(tx *gorm.DB) error {
		// Delete old items (hard delete)
		if err := tx.Unscoped().Where("template_id = ?", id).Delete(&models.ComponentTemplateItem{}).Error; err != nil {
			return err
		}
		// Update template header
		template.Name = payload.Name
		template.Description = payload.Description
		if err := tx.Save(&template).Error; err != nil {
			return err
		}
		// Insert new items
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template: " + err.Error()})
		return
	}

	// Reload with relations
	ct.DB.Preload("Items.ComponentProduct").First(&template, id)
	c.JSON(http.StatusOK, gin.H{"message": "Template updated successfully", "data": template})
}

// DeleteTemplate soft-deletes a component template
func (ct *ComponentTemplateController) DeleteTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	// Check template exists
	var template models.ComponentTemplate
	if err := ct.DB.First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	// Delete items first, then template
	if err := ct.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", id).Delete(&models.ComponentTemplateItem{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&template).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}
