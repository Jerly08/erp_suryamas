package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"

	"github.com/gin-gonic/gin"
)

// ContactController handles HTTP requests for Contacts
type ContactController struct {
	contactService services.ContactService
}

// PurchaseVendorRequest intentionally exposes only the vendor fields available
// in the quick-create modal on the Purchase page. Type and active status are
// controlled by the server so this endpoint cannot create another contact type.
type PurchaseVendorRequest struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
	PICName    string `json:"pic_name"`
	ExternalID string `json:"external_id"`
	Address    string `json:"address"`
	Notes      string `json:"notes"`
}

// NewContactController creates a new ContactController
func NewContactController(contactService services.ContactService) *ContactController {
	return &ContactController{contactService: contactService}
}

// GetContacts returns a list of contacts
func (cc *ContactController) GetContacts(c *gin.Context) {
	// Check if type query parameter is provided
	contactType := c.Query("type")

	var contacts []models.Contact
	var err error

	if contactType != "" {
		// If type is specified, filter by type
		contacts, err = cc.contactService.GetContactsByType(contactType)
	} else {
		// Otherwise, get all contacts
		contacts, err = cc.contactService.GetAllContacts()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": contacts})
}

// GetContact returns a contact by ID
func (cc *ContactController) GetContact(c *gin.Context) {
	id := c.Param("id")
	contact, err := cc.contactService.GetContactByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contact)
}

// CreateContact creates a new contact
func (cc *ContactController) CreateContact(c *gin.Context) {
	var contact models.Contact
	if err := c.ShouldBindJSON(&contact); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	newContact, err := cc.contactService.CreateContact(contact)
	if err != nil {
		respondContactError(c, err, "Unable to save contact. Please try again.")
		return
	}
	c.JSON(http.StatusCreated, newContact)
}

// GetVendorsForPurchase returns vendor options for the Purchase form without
// requiring access to the Contact module.
func (cc *ContactController) GetVendorsForPurchase(c *gin.Context) {
	contacts, err := cc.contactService.GetContactsByType(models.ContactTypeVendor)
	if err != nil {
		respondContactError(c, err, "Unable to load vendors. Please try again.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": contacts})
}

// CreateVendorForPurchase handles the quick-create vendor modal embedded in
// Create Purchase. Authorization is provided by the Purchase route.
func (cc *ContactController) CreateVendorForPurchase(c *gin.Context) {
	var request PurchaseVendorRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Vendor data is invalid. Check the entered fields and try again.",
			"code":  utils.ErrorTypeBadRequest,
		})
		return
	}

	vendor := models.Contact{
		Code:       request.Code,
		Name:       request.Name,
		Type:       models.ContactTypeVendor,
		Email:      request.Email,
		Phone:      request.Phone,
		Mobile:     request.Mobile,
		PICName:    request.PICName,
		ExternalID: request.ExternalID,
		Address:    request.Address,
		Notes:      request.Notes,
		IsActive:   true,
	}

	createdVendor, err := cc.contactService.CreateContact(vendor)
	if err != nil {
		respondContactError(c, err, "Unable to save vendor. Please try again.")
		return
	}

	c.JSON(http.StatusCreated, createdVendor)
}

// UpdateContact updates an existing contact by ID
func (cc *ContactController) UpdateContact(c *gin.Context) {
	id := c.Param("id")
	var contact models.Contact
	if err := c.ShouldBindJSON(&contact); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedContact, err := cc.contactService.UpdateContact(id, contact)
	if err != nil {
		respondContactError(c, err, "Unable to update contact. Please try again.")
		return
	}
	c.JSON(http.StatusOK, updatedContact)
}

func respondContactError(c *gin.Context, err error, fallbackMessage string) {
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": fallbackMessage,
		"code":  utils.ErrorTypeInternal,
	})
}

// DeleteContact deletes a contact by ID
func (cc *ContactController) DeleteContact(c *gin.Context) {
	id := c.Param("id")
	if err := cc.contactService.DeleteContact(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// GetContactsByType returns contacts filtered by type
func (cc *ContactController) GetContactsByType(c *gin.Context) {
	contactType := c.Param("type")
	contacts, err := cc.contactService.GetContactsByType(contactType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": contacts})
}

// SearchContacts searches contacts by query
func (cc *ContactController) SearchContacts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query is required"})
		return
	}

	contacts, err := cc.contactService.SearchContacts(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": contacts})
}

// ImportContacts imports contacts from JSON
func (cc *ContactController) ImportContacts(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var contacts []models.Contact
	if err := json.Unmarshal(body, &contacts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if err := cc.contactService.ImportContacts(contacts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contacts imported successfully", "count": len(contacts)})
}

// ExportContacts exports all contacts as JSON
func (cc *ContactController) ExportContacts(c *gin.Context) {
	contacts, err := cc.contactService.ExportContacts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=contacts.json")
	c.JSON(http.StatusOK, contacts)
}

// AddContactAddress adds a new address to a contact
func (cc *ContactController) AddContactAddress(c *gin.Context) {
	id := c.Param("id")
	var address models.ContactAddress
	if err := c.ShouldBindJSON(&address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newAddress, err := cc.contactService.AddContactAddress(id, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, newAddress)
}

// UpdateContactAddress updates an existing contact address
func (cc *ContactController) UpdateContactAddress(c *gin.Context) {
	contactID := c.Param("id")
	addressID := c.Param("address_id")
	var address models.ContactAddress
	if err := c.ShouldBindJSON(&address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedAddress, err := cc.contactService.UpdateContactAddress(contactID, addressID, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updatedAddress)
}

// DeleteContactAddress deletes a contact address
func (cc *ContactController) DeleteContactAddress(c *gin.Context) {
	contactID := c.Param("id")
	addressID := c.Param("address_id")

	if err := cc.contactService.DeleteContactAddress(contactID, addressID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
