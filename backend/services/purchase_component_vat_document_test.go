package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPurchaseComponentVATDocumentTestService(t *testing.T) (*PurchaseService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.PurchaseTaxAllocation{},
		&models.PurchaseDocument{},
	); err != nil {
		t.Fatalf("migrate component VAT document test database: %v", err)
	}
	return &PurchaseService{
		db:           db,
		purchaseRepo: repositories.NewPurchaseRepository(db),
	}, db
}

func TestPurchaseComponentVATRequiresDocumentForExactInvoiceReference(t *testing.T) {
	service, db := newPurchaseComponentVATDocumentTestService(t)
	fakturDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:         "PO-COMPONENT-DOC-1",
		Date:         fakturDate,
		Status:       models.PurchaseStatusDraft,
		VATTreatment: models.PurchaseVATTreatmentComponentDPPNilaiLain,
	}
	if err := db.Create(&purchase).Error; err != nil {
		t.Fatalf("create purchase: %v", err)
	}
	item := models.PurchaseItem{PurchaseID: purchase.ID, Quantity: 1, UnitPrice: 31304, TotalPrice: 31304}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create purchase item: %v", err)
	}
	allocation := models.PurchaseTaxAllocation{
		PurchaseID:       purchase.ID,
		PurchaseItemID:   item.ID,
		TaxTreatment:     models.PurchaseTaxTreatmentDPPNilaiLain,
		SourceAmount:     31304,
		DPPAmount:        28695,
		PPNRate:          12,
		PPNAmount:        3443,
		IsCreditable:     true,
		TaxInvoiceNumber: "010.001-26.00000001",
		TaxInvoiceDate:   &fakturDate,
	}
	if err := db.Create(&allocation).Error; err != nil {
		t.Fatalf("create tax allocation: %v", err)
	}
	purchase.TaxAllocations = []models.PurchaseTaxAllocation{allocation}

	wrongDocument := models.PurchaseDocument{
		PurchaseID:      purchase.ID,
		DocumentType:    models.PurchaseDocumentTaxInvoice,
		ReferenceNumber: "010.001-26.00000002",
		FileName:        "wrong.pdf",
		FilePath:        "/uploads/purchases/wrong.pdf",
		UploadedBy:      1,
	}
	if err := db.Create(&wrongDocument).Error; err != nil {
		t.Fatalf("create wrong tax invoice document: %v", err)
	}
	if err := service.validatePurchaseComponentVATDocuments(&purchase); err == nil || !strings.Contains(err.Error(), allocation.TaxInvoiceNumber) {
		t.Fatalf("expected exact invoice-reference validation error, got %v", err)
	}

	matchingDocument := models.PurchaseDocument{
		PurchaseID:      purchase.ID,
		DocumentType:    models.PurchaseDocumentTaxInvoice,
		ReferenceNumber: "010.001-26.00000001",
		FileName:        "matching.pdf",
		FilePath:        "/uploads/purchases/matching.pdf",
		UploadedBy:      1,
	}
	if err := db.Create(&matchingDocument).Error; err != nil {
		t.Fatalf("create matching tax invoice document: %v", err)
	}
	if err := service.validatePurchaseComponentVATDocuments(&purchase); err != nil {
		t.Fatalf("expected matching tax invoice document to validate: %v", err)
	}

	verifiedAt := time.Now().UTC()
	if err := db.Model(&models.PurchaseTaxAllocation{}).
		Where("id = ?", allocation.ID).
		Update("verified_at", verifiedAt).Error; err != nil {
		t.Fatalf("verify tax allocation: %v", err)
	}
	if err := service.DeleteDocument(matchingDocument.ID); err == nil {
		t.Fatal("expected verified tax invoice document deletion to be blocked")
	}
}

func TestCreatePurchaseWithTaxAllocationsMapsIncomingIndexToPersistedItem(t *testing.T) {
	service, _ := newPurchaseComponentVATDocumentTestService(t)
	invoiceDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	itemIndex := 1
	purchase := &models.Purchase{
		Code:         "PO-COMPONENT-INDEX-1",
		Date:         invoiceDate,
		Status:       models.PurchaseStatusDraft,
		VATTreatment: models.PurchaseVATTreatmentComponentDPPNilaiLain,
		PPNAmount:    3443,
		TotalAmount:  660819,
		PurchaseItems: []models.PurchaseItem{
			{Quantity: 1, UnitPrice: 607837, TotalPrice: 607837, Description: "Biaya listrik"},
			{Quantity: 1, UnitPrice: 31304, TotalPrice: 31304, Description: "Surcharge"},
		},
	}
	allocation := models.PurchaseTaxAllocation{
		TaxTreatment:      models.PurchaseTaxTreatmentDPPNilaiLain,
		PurchaseItemIndex: &itemIndex,
		DPPAmount:         28695,
		PPNRate:           12,
		PPNAmount:         3443,
		IsCreditable:      true,
		TaxInvoiceNumber:  "010.001-26.00000003",
		TaxInvoiceDate:    &invoiceDate,
	}

	created, err := service.createPurchaseWithTaxAllocations(purchase, []models.PurchaseTaxAllocation{allocation})
	if err != nil {
		t.Fatalf("create component VAT purchase: %v", err)
	}
	if len(created.TaxAllocations) != 1 || created.TaxAllocations[0].PurchaseItem.Description != "Surcharge" {
		t.Fatalf("component VAT was linked to the wrong persisted item: %#v", created.TaxAllocations)
	}
}

func TestCreatePurchaseWithTaxAllocationsRollsBackWhenAllocationIsInvalid(t *testing.T) {
	service, db := newPurchaseComponentVATDocumentTestService(t)
	invoiceDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	invalidIndex := 4
	purchase := &models.Purchase{
		Code:         "PO-COMPONENT-ROLLBACK-1",
		Date:         invoiceDate,
		Status:       models.PurchaseStatusDraft,
		VATTreatment: models.PurchaseVATTreatmentComponentDPPNilaiLain,
		PurchaseItems: []models.PurchaseItem{{
			Quantity: 1, UnitPrice: 31304, TotalPrice: 31304, Description: "Surcharge",
		}},
	}
	err := func() error {
		_, createErr := service.createPurchaseWithTaxAllocations(purchase, []models.PurchaseTaxAllocation{{
			TaxTreatment:      models.PurchaseTaxTreatmentDPPNilaiLain,
			PurchaseItemIndex: &invalidIndex,
			DPPAmount:         28695,
			PPNRate:           12,
			PPNAmount:         3443,
			TaxInvoiceNumber:  "010.001-26.00000004",
			TaxInvoiceDate:    &invoiceDate,
		}})
		return createErr
	}()
	if err == nil {
		t.Fatal("expected invalid allocation to fail")
	}

	var purchaseCount int64
	if err := db.Model(&models.Purchase{}).
		Where("code = ?", "PO-COMPONENT-ROLLBACK-1").
		Count(&purchaseCount).Error; err != nil {
		t.Fatalf("count rolled-back purchase: %v", err)
	}
	if purchaseCount != 0 {
		t.Fatalf("expected failed component VAT purchase to roll back, found %d row(s)", purchaseCount)
	}
}
