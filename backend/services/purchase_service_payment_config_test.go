package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizePurchasePaymentConfigurationUsesDefaultCreditAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Account{}))
	require.NoError(t, db.Create(&models.Account{
		ID:       1,
		Code:     "2101-001",
		Name:     "UTANG USAHA - LOKAL",
		Type:     models.AccountTypeLiability,
		IsActive: true,
		IsHeader: false,
	}).Error)

	service := &PurchaseService{db: db}

	bankAccountID, creditAccountID, err := service.normalizePurchasePaymentConfiguration(
		models.PurchasePaymentCredit,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Nil(t, bankAccountID)
	require.NotNil(t, creditAccountID)
	require.EqualValues(t, 1, *creditAccountID)
}

func TestNormalizePurchasePaymentConfigurationMapsHeaderCreditAccountToDetail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Account{}))
	require.NoError(t, db.Create(&models.Account{
		ID:       10,
		Code:     "2101",
		Name:     "UTANG USAHA",
		Type:     models.AccountTypeLiability,
		IsActive: true,
		IsHeader: true,
	}).Error)
	require.NoError(t, db.Create(&models.Account{
		ID:       11,
		Code:     "2101-001",
		Name:     "UTANG USAHA - LOKAL",
		Type:     models.AccountTypeLiability,
		ParentID: uintPtr(10),
		IsActive: true,
		IsHeader: false,
	}).Error)

	service := &PurchaseService{db: db}
	selectedID := uint(10)

	_, creditAccountID, err := service.normalizePurchasePaymentConfiguration(
		models.PurchasePaymentCredit,
		nil,
		&selectedID,
	)
	require.NoError(t, err)
	require.NotNil(t, creditAccountID)
	require.EqualValues(t, 11, *creditAccountID)
}

func TestValidateLinkedPurchaseRequestSelectionRejectsRequestAlreadyLinkedToPurchase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Product{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
	))

	user := models.User{
		Username: "purchase-user",
		Email:    "purchase-user@example.com",
		Password: "hashed",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code:     "VEN-PR-LINK",
		Name:     "Vendor PR Link",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	product := models.Product{
		Code:     "PRD-PR-LINK",
		Name:     "Product PR Link",
		Unit:     "pcs",
		IsActive: true,
	}
	require.NoError(t, db.Create(&product).Error)

	pr := models.PurchaseRequest{
		PRNumber:             "PR-LINKED",
		RequesterID:          user.ID,
		Status:               models.PRStatusApproved,
		CurrentApprovalStage: models.PRApprovalStageFinalized,
		Items: []models.PurchaseRequestItem{
			{
				ProductID:           uintPtr(product.ID),
				ItemName:            product.Name,
				Quantity:            1,
				EstimatedPrice:      1000,
				TotalEstimatedPrice: 1000,
				ApprovalStatus:      models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&pr).Error)

	existingPurchase := models.Purchase{
		Code:              "PO-PR-LINKED",
		VendorID:          vendor.ID,
		UserID:            user.ID,
		Status:            models.PurchaseStatusDraft,
		PurchaseRequestID: &pr.ID,
	}
	require.NoError(t, db.Create(&existingPurchase).Error)

	service := &PurchaseService{db: db}
	purchaseItemID := pr.Items[0].ID
	productID := product.ID
	newPurchase := &models.Purchase{
		PurchaseRequestID: &pr.ID,
		PurchaseItems: []models.PurchaseItem{
			{
				ProductID:             &productID,
				PurchaseRequestItemID: &purchaseItemID,
				Quantity:              1,
				UnitPrice:             1000,
				TotalPrice:            1000,
			},
		},
	}

	_, _, err = service.validateLinkedPurchaseRequestSelection(newPurchase, false)
	require.ErrorContains(t, err, "already linked to purchase PO-PR-LINKED")

	existingPurchase.PurchaseItems = newPurchase.PurchaseItems
	_, _, err = service.validateLinkedPurchaseRequestSelection(&existingPurchase, true)
	require.NoError(t, err)
}

func TestValidateLinkedPurchaseRequestSelectionRejectsRequestUsedByPurchaseItem(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Product{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
	))

	user := models.User{
		Username: "purchase-item-user",
		Email:    "purchase-item-user@example.com",
		Password: "hashed",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code:     "VEN-PR-ITEM-LINK",
		Name:     "Vendor PR Item Link",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	product := models.Product{
		Code:     "PRD-PR-ITEM-LINK",
		Name:     "Product PR Item Link",
		Unit:     "pcs",
		IsActive: true,
	}
	require.NoError(t, db.Create(&product).Error)

	pr := models.PurchaseRequest{
		PRNumber:             "PR-ITEM-LINKED",
		RequesterID:          user.ID,
		Status:               models.PRStatusApproved,
		CurrentApprovalStage: models.PRApprovalStageFinalized,
		Items: []models.PurchaseRequestItem{
			{
				ProductID:           uintPtr(product.ID),
				ItemName:            product.Name,
				Quantity:            1,
				EstimatedPrice:      1000,
				TotalEstimatedPrice: 1000,
				ApprovalStatus:      models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&pr).Error)

	existingPurchase := models.Purchase{
		Code:     "PO-PR-ITEM-LINKED",
		VendorID: vendor.ID,
		UserID:   user.ID,
		Status:   models.PurchaseStatusDraft,
	}
	require.NoError(t, db.Create(&existingPurchase).Error)

	requestItemID := pr.Items[0].ID
	productID := product.ID
	require.NoError(t, db.Create(&models.PurchaseItem{
		PurchaseID:            existingPurchase.ID,
		ProductID:             &productID,
		PurchaseRequestItemID: &requestItemID,
		Quantity:              1,
		UnitPrice:             1000,
		TotalPrice:            1000,
	}).Error)

	service := &PurchaseService{db: db}
	newPurchase := &models.Purchase{
		PurchaseRequestID: &pr.ID,
		PurchaseItems: []models.PurchaseItem{
			{
				ProductID:             &productID,
				PurchaseRequestItemID: &requestItemID,
				Quantity:              1,
				UnitPrice:             1000,
				TotalPrice:            1000,
			},
		},
	}

	_, _, err = service.validateLinkedPurchaseRequestSelection(newPurchase, false)
	require.ErrorContains(t, err, "already linked to purchase PO-PR-ITEM-LINKED")
}

func TestValidateLinkedPurchaseRequestSelectionRejectsOrphanRequestItemLink(t *testing.T) {
	service := &PurchaseService{}
	purchaseRequestItemID := uint(99)
	productID := uint(1)
	purchase := &models.Purchase{
		PurchaseItems: []models.PurchaseItem{
			{
				ProductID:             &productID,
				PurchaseRequestItemID: &purchaseRequestItemID,
				Quantity:              1,
				UnitPrice:             1000,
				TotalPrice:            1000,
			},
		},
	}

	_, _, err := service.validateLinkedPurchaseRequestSelection(purchase, false)
	require.ErrorContains(t, err, "purchase_request_id is required")
}

func uintPtr(value uint) *uint {
	return &value
}
