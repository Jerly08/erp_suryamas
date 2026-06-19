package controllers

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BOMProcurementController struct {
	DB *gorm.DB
}

func NewBOMProcurementController(db *gorm.DB) *BOMProcurementController {
	return &BOMProcurementController{DB: db}
}

// GeneratePR automatically drafts a Purchase Request for missing components in a Project
func (bpc *BOMProcurementController) GeneratePR(c *gin.Context) {
	projectID := c.Param("id")

	// Get current user ID (assumes AuthMiddleware sets "user_id" in context)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case uint64:
		userID = uint(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type in context"})
		return
	}

	var project models.Project
	if err := bpc.DB.First(&project, projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	err := bpc.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Fetch all required components across all attached BOMs for this project
		// We'll group them by ComponentProductID and sum the TotalRequirement
		type ComponentRequirement struct {
			ProductID        uint
			TotalRequirement float64
			UnitPrice        float64
			Notes            string
		}

		var requirements []ComponentRequirement
		err := tx.Model(&models.ProjectBOMItem{}).
			Joins("JOIN project_boms ON project_boms.id = project_bom_items.project_bom_id").
			Where("project_boms.project_id = ?", projectID).
			Select("project_bom_items.component_product_id as product_id, SUM(project_bom_items.total_requirement) as total_requirement, MAX(project_bom_items.unit_price) as unit_price, string_agg(project_bom_items.notes, ', ') as notes").
			Group("project_bom_items.component_product_id").
			Scan(&requirements).Error

		if err != nil {
			return err
		}

		if len(requirements) == 0 {
			return fmt.Errorf("no BOM components found for this project")
		}

		// 2. Perform Inventory Check & Build Shortage List
		var prItems []models.PurchaseRequestItem
		var totalEstimatedAmount float64

		for _, req := range requirements {
			var product models.Product
			if err := tx.First(&product, req.ProductID).Error; err != nil {
				return err
			}

			// Calculate Shortage (Required - Current Stock)
			shortage := req.TotalRequirement - float64(product.Stock)
			if shortage > 0 {
				// We need to buy more
				itemEstimatedPrice := req.UnitPrice
				if itemEstimatedPrice <= 0 {
					itemEstimatedPrice = product.PurchasePrice // Fallback to product master price
				}

				itemTotal := shortage * itemEstimatedPrice
				totalEstimatedAmount += itemTotal

				prItems = append(prItems, models.PurchaseRequestItem{
					ProductID:              &product.ID,
					ItemName:               product.Name,
					Quantity:               int(math.Ceil(shortage)), // Round up to ensure enough is purchased (0.7 → 1)
					EstimatedPrice:         itemEstimatedPrice,
					TotalEstimatedPrice:    itemTotal,
					ApprovalStatus:         models.PRItemDecisionPending,
					FinanceStaffDecision:   models.PRItemDecisionPending,
					FinanceManagerDecision: models.PRItemDecisionPending,
					Notes:                  "Auto-Procurement dari Project BOM: " + req.Notes,
				})
			}
		}

		if len(prItems) == 0 {
			return fmt.Errorf("all required BOM components are already fully stocked in the warehouse")
		}

		// 3. Create Purchase Request (directly as PENDING_APPROVAL for auto-procurement flow)
		now := time.Now()
		projectIDUint := project.ID

		// Generate PR Number with Unix timestamp suffix to prevent daily collision
		// e.g. PR-20260227-PROJ001-1740589366
		prNumber := fmt.Sprintf("PR-%s-%s-%d", now.Format("20060102"), project.Code, now.Unix())

		newPR := models.PurchaseRequest{
			PRNumber:             prNumber,
			RequesterID:          userID,
			Department:           "Projects",
			ProjectID:            &projectIDUint,
			TotalEstimatedAmount: totalEstimatedAmount,
			Status:               models.PRStatusPendingApproval, // Auto-generated PRs go directly to approval queue
			CurrentApprovalStage: models.PRApprovalStageFinanceStaff,
			Notes:                fmt.Sprintf("Auto-generated PR untuk Project: %s (dari BOM shortage)", project.Name),
			Items:                prItems,
		}

		if err := tx.Create(&newPR).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if err.Error() == "all required BOM components are already fully stocked in the warehouse" || err.Error() == "no BOM components found for this project" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PR: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Draft Purchase Request generated successfully from BOM shortages."})
}
