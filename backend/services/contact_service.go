package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/utils"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
)

// ContactService provides business logic for contacts
type ContactService interface {
	GetAllContacts() ([]models.Contact, error)
	GetContactByID(id string) (*models.Contact, error)
	CreateContact(contact models.Contact) (*models.Contact, error)
	UpdateContact(id string, contact models.Contact) (*models.Contact, error)
	DeleteContact(id string) error
	GetContactsByType(contactType string) ([]models.Contact, error)
	SearchContacts(query string) ([]models.Contact, error)
	ImportContacts(contacts []models.Contact) error
	ExportContacts() ([]models.Contact, error)
	AddContactAddress(contactID string, address models.ContactAddress) (*models.ContactAddress, error)
	UpdateContactAddress(contactID string, addressID string, address models.ContactAddress) (*models.ContactAddress, error)
	DeleteContactAddress(contactID string, addressID string) error
}

// contactService implements ContactService
type contactService struct {
	repo repositories.ContactRepository
}

// NewContactService creates a new ContactService
func NewContactService(repo repositories.ContactRepository) ContactService {
	return &contactService{repo: repo}
}

// GetAllContacts returns all contacts
func (s *contactService) GetAllContacts() ([]models.Contact, error) {
	return s.repo.GetAll()
}

// GetContactByID returns a contact by ID
func (s *contactService) GetContactByID(id string) (*models.Contact, error) {
	contactID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid contact ID: %v", err)
	}
	return s.repo.GetByID(uint(contactID))
}

// CreateContact creates a new contact
func (s *contactService) CreateContact(contact models.Contact) (*models.Contact, error) {
	var err error
	contact, err = prepareContact(contact)
	if err != nil {
		return nil, err
	}

	// Validate contact type early
	if !isValidContactType(contact.Type) {
		return nil, utils.NewValidationError("Contact type is invalid", map[string]string{
			"type": "Contact type must be CUSTOMER, VENDOR, or EMPLOYEE",
		})
	}

	// If client didn't provide a code, generate one
	if contact.Code == "" {
		code, err := s.generateContactCode(contact.Type)
		if err != nil {
			return nil, err
		}
		contact.Code = code
	} else {
		// If a code was provided, check if it already exists (including soft-deleted); if so, auto-generate a next available code
		exists, err := s.repo.CodeExists(contact.Code)
		if err != nil {
			return nil, fmt.Errorf("failed to check if code exists: %v", err)
		}
		if exists {
			generated, genErr := s.generateContactCode(contact.Type)
			if genErr != nil {
				return nil, fmt.Errorf("contact code already exists and failed to generate new code: %v", genErr)
			}
			contact.Code = generated
		}
	}

	// Try create; if duplicate happens (race condition), generate next code and retry once
	created, err := s.repo.Create(contact)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
		// Collision: generate next code and retry once
		newCode, genErr := s.generateContactCode(contact.Type)
		if genErr != nil {
			return nil, genErr
		}
		contact.Code = newCode
		created, err = s.repo.Create(contact)
	}
	if err != nil {
		return nil, friendlyContactSaveError(contact.Type, err)
	}
	return created, nil
}

// UpdateContact updates an existing contact
func (s *contactService) UpdateContact(id string, contact models.Contact) (*models.Contact, error) {
	contactID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid contact ID: %v", err)
	}

	contact, err = prepareContact(contact)
	if err != nil {
		return nil, err
	}

	// Validate contact type
	if !isValidContactType(contact.Type) {
		return nil, utils.NewValidationError("Contact type is invalid", map[string]string{
			"type": "Contact type must be CUSTOMER, VENDOR, or EMPLOYEE",
		})
	}

	contact.ID = uint(contactID)
	updated, err := s.repo.Update(contact)
	if err != nil {
		return nil, friendlyContactSaveError(contact.Type, err)
	}
	return updated, nil
}

// prepareContact normalizes user input and validates only the fields that are
// required by the contact form. Optional fields are intentionally allowed to
// remain empty.
func prepareContact(contact models.Contact) (models.Contact, error) {
	contact.Code = strings.TrimSpace(contact.Code)
	contact.Name = strings.TrimSpace(contact.Name)
	contact.Type = strings.ToUpper(strings.TrimSpace(contact.Type))
	contact.Category = strings.TrimSpace(contact.Category)
	contact.Email = strings.TrimSpace(contact.Email)
	contact.Phone = strings.TrimSpace(contact.Phone)
	contact.Mobile = strings.TrimSpace(contact.Mobile)
	contact.Fax = strings.TrimSpace(contact.Fax)
	contact.Website = strings.TrimSpace(contact.Website)
	contact.TaxNumber = strings.TrimSpace(contact.TaxNumber)
	contact.PICName = strings.TrimSpace(contact.PICName)
	contact.ExternalID = strings.TrimSpace(contact.ExternalID)
	contact.Address = strings.TrimSpace(contact.Address)
	contact.Notes = strings.TrimSpace(contact.Notes)

	contactLabel := "Contact"
	if contact.Type == models.ContactTypeVendor {
		contactLabel = "Vendor"
	}

	if contact.Name == "" {
		return contact, utils.NewValidationError(contactLabel+" name is required", map[string]string{
			"name": contactLabel + " name is required",
		})
	}
	if len(contact.Name) > 100 {
		return contact, utils.NewValidationError(contactLabel+" name is too long", map[string]string{
			"name": contactLabel + " name must be at most 100 characters",
		})
	}

	// The quick-create vendor form marks email with an asterisk, so it is the
	// only user-entered field besides name that is mandatory for a vendor.
	if contact.Type == models.ContactTypeVendor && contact.Email == "" {
		return contact, utils.NewValidationError("Vendor email is required", map[string]string{
			"email": "Vendor email is required",
		})
	}
	if contact.Email != "" {
		parsed, parseErr := mail.ParseAddress(contact.Email)
		if parseErr != nil || parsed.Address != contact.Email {
			return contact, utils.NewValidationError("Email format is invalid", map[string]string{
				"email": "Enter an email such as vendor@company.com",
			})
		}
	}

	fieldLimits := []struct {
		field string
		value string
		limit int
	}{
		{"code", contact.Code, 20},
		{"email", contact.Email, 100},
		{"phone", contact.Phone, 20},
		{"mobile", contact.Mobile, 20},
		{"pic_name", contact.PICName, 100},
		{"external_id", contact.ExternalID, 50},
	}
	for _, item := range fieldLimits {
		if len(item.value) > item.limit {
			return contact, utils.NewValidationError("One of the fields is too long", map[string]string{
				item.field: fmt.Sprintf("Maximum length is %d characters", item.limit),
			})
		}
	}

	return contact, nil
}

func friendlyContactSaveError(contactType string, err error) error {
	errorText := strings.ToLower(err.Error())
	if strings.Contains(errorText, "duplicate") || strings.Contains(errorText, "unique constraint") {
		return utils.NewConflictError("Contact code is already in use. Leave the code empty to generate it automatically.")
	}

	label := "contact"
	if contactType == models.ContactTypeVendor {
		label = "vendor"
	}
	return utils.NewInternalError(fmt.Sprintf("Unable to save %s. Please try again.", label), err)
}

// DeleteContact deletes a contact by ID
func (s *contactService) DeleteContact(id string) error {
	contactID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid contact ID: %v", err)
	}
	return s.repo.Delete(uint(contactID))
}

// GetContactsByType returns contacts filtered by type
func (s *contactService) GetContactsByType(contactType string) ([]models.Contact, error) {
	if !isValidContactType(contactType) {
		return nil, fmt.Errorf("invalid contact type: %s", contactType)
	}
	return s.repo.GetByType(contactType)
}

// ImportContacts imports multiple contacts
func (s *contactService) ImportContacts(contacts []models.Contact) error {
	for i, contact := range contacts {
		// Generate contact code if not provided
		if contact.Code == "" {
			code, err := s.generateContactCode(contact.Type)
			if err != nil {
				return fmt.Errorf("error generating code for contact %d: %v", i+1, err)
			}
			contacts[i].Code = code
		}

		// Validate contact type
		if !isValidContactType(contact.Type) {
			return fmt.Errorf("invalid contact type for contact %d: %s", i+1, contact.Type)
		}
	}

	return s.repo.BulkCreate(contacts)
}

// SearchContacts searches contacts by query
func (s *contactService) SearchContacts(query string) ([]models.Contact, error) {
	if query == "" {
		return []models.Contact{}, nil
	}
	return s.repo.Search(query)
}

// ExportContacts exports all contacts
func (s *contactService) ExportContacts() ([]models.Contact, error) {
	return s.repo.GetAll()
}

// AddContactAddress adds a new address to a contact
func (s *contactService) AddContactAddress(contactID string, address models.ContactAddress) (*models.ContactAddress, error) {
	contactIDUint, err := strconv.ParseUint(contactID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid contact ID: %v", err)
	}

	// Validate that contact exists
	_, err = s.repo.GetByID(uint(contactIDUint))
	if err != nil {
		return nil, fmt.Errorf("contact not found: %v", err)
	}

	address.ContactID = uint(contactIDUint)
	return s.repo.AddAddress(address)
}

// UpdateContactAddress updates an existing contact address
func (s *contactService) UpdateContactAddress(contactID string, addressID string, address models.ContactAddress) (*models.ContactAddress, error) {
	contactIDUint, err := strconv.ParseUint(contactID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid contact ID: %v", err)
	}

	addressIDUint, err := strconv.ParseUint(addressID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid address ID: %v", err)
	}

	// Validate that contact exists
	_, err = s.repo.GetByID(uint(contactIDUint))
	if err != nil {
		return nil, fmt.Errorf("contact not found: %v", err)
	}

	address.ID = uint(addressIDUint)
	address.ContactID = uint(contactIDUint)
	return s.repo.UpdateAddress(address)
}

// DeleteContactAddress deletes a contact address
func (s *contactService) DeleteContactAddress(contactID string, addressID string) error {
	contactIDUint, err := strconv.ParseUint(contactID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid contact ID: %v", err)
	}

	addressIDUint, err := strconv.ParseUint(addressID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid address ID: %v", err)
	}

	// Validate that contact exists
	_, err = s.repo.GetByID(uint(contactIDUint))
	if err != nil {
		return fmt.Errorf("contact not found: %v", err)
	}

	return s.repo.DeleteAddress(uint(addressIDUint))
}

// generateContactCode generates a unique contact code based on type
func (s *contactService) generateContactCode(contactType string) (string, error) {
	var prefix string
	switch contactType {
	case models.ContactTypeCustomer:
		prefix = "CUST"
	case models.ContactTypeVendor:
		prefix = "VEND"
	case models.ContactTypeEmployee:
		prefix = "EMP"
	default:
		return "", fmt.Errorf("invalid contact type: %s", contactType)
	}

	// Find the highest existing number for this type to avoid duplicates
	// This handles soft deletes and ensures uniqueness by checking ALL contacts
	maxNumber := 0
	allContacts, err := s.repo.GetAllIncludingDeleted()
	if err == nil {
		for _, c := range allContacts {
			if len(c.Code) > len(prefix)+1 && c.Code[:len(prefix)] == prefix {
				numberStr := c.Code[len(prefix)+1:] // Skip "PREFIX-"
				if number, parseErr := strconv.Atoi(numberStr); parseErr == nil {
					if number > maxNumber {
						maxNumber = number
					}
				}
			}
		}
	}

	nextNumber := maxNumber + 1
	for attempts := 0; attempts < 10; attempts++ {
		code := fmt.Sprintf("%s-%04d", prefix, nextNumber)
		exists, checkErr := s.repo.CodeExists(code)
		if checkErr != nil {
			return "", fmt.Errorf("error checking code existence: %v", checkErr)
		}
		if !exists {
			return code, nil
		}
		nextNumber++
	}
	return "", fmt.Errorf("unable to generate unique code for contact type %s after 10 attempts", contactType)
}

// isValidContactType validates contact type
func isValidContactType(contactType string) bool {
	validTypes := []string{
		models.ContactTypeCustomer,
		models.ContactTypeVendor,
		models.ContactTypeEmployee,
	}

	for _, validType := range validTypes {
		if contactType == validType {
			return true
		}
	}
	return false
}
