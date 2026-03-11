package controllers

import (
	"net/http"
	"strconv"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

type PrepaidController struct {
	prepaidSvc *services.PrepaidService
}

func NewPrepaidController(prepaidSvc *services.PrepaidService) *PrepaidController {
	return &PrepaidController{
		prepaidSvc: prepaidSvc,
	}
}

// GetAllPrepaid retrieves a list of prepaid expenses
func (ctrl *PrepaidController) GetAllPrepaid(c *gin.Context) {
	status := c.Query("status")
	prepaids, err := ctrl.prepaidSvc.GetAllPrepaid(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch prepaid expenses", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": prepaids})
}

// GetPrepaidByID retrieves a specific prepaid expense with its amortization schedules
func (ctrl *PrepaidController) GetPrepaidByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	prepaid, err := ctrl.prepaidSvc.GetPrepaidByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prepaid expense not found", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": prepaid})
}

// SetExpenseAccount updates the target expense account for amortization
func (ctrl *PrepaidController) SetExpenseAccount(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req struct {
		ExpenseAccountID uint `json:"expense_account_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ctrl.prepaidSvc.SetExpenseAccount(uint(id), req.ExpenseAccountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update expense account", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Expense account updated successfully"})
}

// AmortizeSchedule executes one month of amortization
func (ctrl *PrepaidController) AmortizeSchedule(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Prepaid ID format"})
		return
	}

	scheduleIDParam := c.Param("schedule_id")
	scheduleID, err := strconv.ParseUint(scheduleIDParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Schedule ID format"})
		return
	}

	// Assuming auth middleware sets the user in context
	user, exists := c.Get("user")
	var userID uint = 1 // Fallback if auth missing
	if exists {
		if authUser, ok := user.(*models.User); ok {
			userID = authUser.ID
		}
	}

	if err := ctrl.prepaidSvc.AmortizeSchedule(uint(id), uint(scheduleID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Amortization failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Amortization posted successfully via SSOT Journal"})
}

// TerminatePrepaid handles early termination
func (ctrl *PrepaidController) TerminatePrepaid(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req struct {
		TerminateType      string `json:"terminate_type" binding:"required"` // "WRITEOFF" or "REFUND"
		DestinationAccount uint   `json:"destination_account_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, exists := c.Get("user")
	var userID uint = 1
	if exists {
		if authUser, ok := user.(*models.User); ok {
			userID = authUser.ID
		}
	}

	if err := ctrl.prepaidSvc.TerminatePrepaid(uint(id), req.TerminateType, req.DestinationAccount, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Termination failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Prepaid expense terminated successfully"})
}
