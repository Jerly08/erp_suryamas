package controllers

import (
	"app-sistem-akuntansi/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProjectBOMController struct {
	DB *gorm.DB
}

func NewProjectBOMController(db *gorm.DB) *ProjectBOMController {
	return &ProjectBOMController{DB: db}
}

// GetProjectBOMs fetches all BOMs attached to a specific project
func (pbc *ProjectBOMController) GetProjectBOMs(c *gin.Context) {
	projectID := c.Param("id")

	var projectBOMs []models.ProjectBOM
	if err := pbc.DB.Preload("Items").Preload("TargetProduct").Preload("BOMTemplate").Where("project_id = ?", projectID).Find(&projectBOMs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch project BOMs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": projectBOMs})
}

// AddBOMToProject attaches a snapshot of a Master BOM to a Project
func (pbc *ProjectBOMController) AddBOMToProject(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var payload models.ProjectBOMAddPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = pbc.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Fetch Master BOM
		var masterBOM models.BOM
		if err := tx.Preload("Items").First(&masterBOM, payload.BOMID).Error; err != nil {
			return err
		}

		// 2. Create ProjectBOM Snapshot
		// BOM.TotalCost = biaya per 1 unit (Cost / Unit di halaman Master BOM).
		// Proyek butuh payload.Quantity unit → TotalCost = BOM.TotalCost * Quantity.
		projectBOM := models.ProjectBOM{
			ProjectID:       uint(projectID),
			BOMID:           masterBOM.ID,
			BOMName:         masterBOM.Name,
			TargetProductID: masterBOM.TargetProductID,
			Quantity:        payload.Quantity,
			UnitCost:        masterBOM.TotalCost,
			TotalCost:       masterBOM.TotalCost * float64(payload.Quantity),
		}

		if err := tx.Create(&projectBOM).Error; err != nil {
			return err
		}

		// 3. Create ProjectBOMItem Snapshots
		// RequirementQuantity sudah per unit (karena BOM.TotalCost = sum item costs = cost per unit).
		// Total kebutuhan untuk proyek = RequirementQuantity * payload.Quantity.
		for _, masterItem := range masterBOM.Items {
			reqQty := masterItem.RequirementQuantity * float64(payload.Quantity)
			totalItemCost := reqQty * masterItem.UnitPrice

			projectItem := models.ProjectBOMItem{
				ProjectBOMID:       projectBOM.ID,
				ComponentProductID: masterItem.ComponentProductID,
				CategoryStr:        masterItem.CategoryStr,
				BaseQuantity:       masterItem.BaseQuantity,
				BaseUnit:           masterItem.BaseUnit,
				Notes:              masterItem.Notes,
				TotalRequirement:   reqQty,
				UnitPrice:          masterItem.UnitPrice,
				TotalCost:          totalItemCost,
			}
			if err := tx.Create(&projectItem).Error; err != nil {
				return err
			}
		}

		// 4. Recalculate Project Total Budget
		var project models.Project
		if err := tx.First(&project, projectID).Error; err != nil {
			return err
		}

		var totalProjectBudget float64
		// Sum all ProjectBOMs for this project
		tx.Model(&models.ProjectBOM{}).Where("project_id = ?", projectID).Select("COALESCE(SUM(total_cost), 0)").Scan(&totalProjectBudget)

		project.TotalBudget = totalProjectBudget
		if err := tx.Save(&project).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add BOM to Project: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "BOM successfully added to Project"})
}

// RemoveBOMFromProject deletes a ProjectBOM and recalculates budget
func (pbc *ProjectBOMController) RemoveBOMFromProject(c *gin.Context) {
	projectID := c.Param("id")
	projectBOMID := c.Param("bomId")

	err := pbc.DB.Transaction(func(tx *gorm.DB) error {
		// Verify ProjectBOM belongs to Project
		var pb models.ProjectBOM
		if err := tx.Where("id = ? AND project_id = ?", projectBOMID, projectID).First(&pb).Error; err != nil {
			return err
		}

		// Delete items first
		if err := tx.Where("project_bom_id = ?", pb.ID).Delete(&models.ProjectBOMItem{}).Error; err != nil {
			return err
		}

		// Delete ProjectBOM
		if err := tx.Delete(&models.ProjectBOM{}, pb.ID).Error; err != nil {
			return err
		}

		// Recalculate Project Total Budget
		var project models.Project
		if err := tx.First(&project, projectID).Error; err != nil {
			return err
		}

		var totalProjectBudget float64
		tx.Model(&models.ProjectBOM{}).Where("project_id = ?", projectID).Select("COALESCE(SUM(total_cost), 0)").Scan(&totalProjectBudget)

		project.TotalBudget = totalProjectBudget
		if err := tx.Save(&project).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove BOM from project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "BOM removed from Project and budget updated"})
}
