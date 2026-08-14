package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type purchaseVendorContactServiceStub struct {
	services.ContactService
	created models.Contact
}

func (stub *purchaseVendorContactServiceStub) CreateContact(contact models.Contact) (*models.Contact, error) {
	stub.created = contact
	contact.ID = 42
	contact.Code = "VEND-0042"
	return &contact, nil
}

func TestCreateVendorForPurchaseForcesVendorTypeAndAllowsOptionalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &purchaseVendorContactServiceStub{}
	controller := NewContactController(stub)
	router := gin.New()
	router.POST("/purchases/vendors", controller.CreateVendorForPurchase)

	body := []byte(`{
		"name":"Vendor Purchase",
		"email":"vendor@example.com",
		"type":"CUSTOMER",
		"phone":"",
		"mobile":"",
		"pic_name":"",
		"external_id":"",
		"address":"",
		"notes":""
	}`)
	request := httptest.NewRequest(http.MethodPost, "/purchases/vendors", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusCreated, response.Code)
	require.Equal(t, models.ContactTypeVendor, stub.created.Type)
	require.True(t, stub.created.IsActive)
	require.Empty(t, stub.created.Phone)
	require.Empty(t, stub.created.Mobile)
	require.Empty(t, stub.created.PICName)
	require.Empty(t, stub.created.ExternalID)
	require.Empty(t, stub.created.Address)
	require.Empty(t, stub.created.Notes)
}

type purchaseVendorListContactServiceStub struct {
	services.ContactService
	contacts []models.Contact
}

func (stub *purchaseVendorListContactServiceStub) GetContactsByType(string) ([]models.Contact, error) {
	return stub.contacts, nil
}

func TestGetVendorsForPurchaseReturnsOnlyActiveVendors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &purchaseVendorListContactServiceStub{contacts: []models.Contact{
		{ID: 14, Code: "VAND322797674", Name: "Andhika Prasetyo", Type: models.ContactTypeVendor, IsActive: true},
		{ID: 16, Code: "VAND310274807", Name: "Andhika Prasetyo", Type: models.ContactTypeVendor, IsActive: false},
	}}
	controller := NewContactController(stub)
	router := gin.New()
	router.GET("/purchases/vendors", controller.GetVendorsForPurchase)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/purchases/vendors", nil))

	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Data []models.Contact `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, []models.Contact{stub.contacts[0]}, body.Data)
}
