package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"
	"github.com/stretchr/testify/require"
)

func TestPrepareContactVendorAllowsOptionalFieldsToBeEmpty(t *testing.T) {
	contact, err := prepareContact(models.Contact{
		Name:  "  Vendor Contoh  ",
		Type:  " vendor ",
		Email: " vendor@example.com ",
	})

	require.NoError(t, err)
	require.Equal(t, "Vendor Contoh", contact.Name)
	require.Equal(t, models.ContactTypeVendor, contact.Type)
	require.Equal(t, "vendor@example.com", contact.Email)
	require.Empty(t, contact.Phone)
	require.Empty(t, contact.Mobile)
	require.Empty(t, contact.PICName)
	require.Empty(t, contact.ExternalID)
	require.Empty(t, contact.Address)
	require.Empty(t, contact.Notes)
}

func TestPrepareContactVendorAcceptsOptionalPlaceholderValues(t *testing.T) {
	contact, err := prepareContact(models.Contact{
		Name:       "Vendor Contoh",
		Type:       models.ContactTypeVendor,
		Email:      "vendor@example.com",
		Mobile:     "-",
		ExternalID: "-",
		Address:    "-",
	})

	require.NoError(t, err)
	require.Equal(t, "-", contact.Mobile)
	require.Equal(t, "-", contact.ExternalID)
	require.Equal(t, "-", contact.Address)
}

func TestPrepareContactVendorValidatesOnlyStarredFields(t *testing.T) {
	tests := []struct {
		name    string
		contact models.Contact
		field   string
	}{
		{
			name:    "missing vendor name",
			contact: models.Contact{Type: models.ContactTypeVendor, Email: "vendor@example.com"},
			field:   "name",
		},
		{
			name:    "missing vendor email",
			contact: models.Contact{Type: models.ContactTypeVendor, Name: "Vendor Contoh"},
			field:   "email",
		},
		{
			name:    "invalid vendor email",
			contact: models.Contact{Type: models.ContactTypeVendor, Name: "Vendor Contoh", Email: "bukan-email"},
			field:   "email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := prepareContact(tt.contact)
			require.Error(t, err)

			appErr := utils.GetAppError(err)
			require.NotNil(t, appErr)
			require.Equal(t, utils.ErrorTypeValidation, appErr.Type)
			require.Contains(t, appErr.Details, tt.field)
		})
	}
}
