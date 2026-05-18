package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildSaleItemFulfillmentSnapshotWithRemainingStockTracksCumulativeDemand(t *testing.T) {
	product := &models.Product{
		ID:    1,
		Code:  "PRD-001",
		Name:  "Produk A",
		Unit:  "pcs",
		Stock: 10,
	}
	remainingProductStock := make(map[uint]int)

	firstSnapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(product, 8, remainingProductStock)
	require.Equal(t, 8, firstSnapshot.availableQuantity)
	require.Equal(t, 0, firstSnapshot.backorderQuantity)
	require.Equal(t, models.SaleFulfillmentStatusInStock, firstSnapshot.fulfillmentStatus)
	require.Equal(t, 2, remainingProductStock[product.ID])

	secondSnapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(product, 5, remainingProductStock)
	require.Equal(t, 2, secondSnapshot.availableQuantity)
	require.Equal(t, 3, secondSnapshot.backorderQuantity)
	require.Equal(t, models.SaleFulfillmentStatusPartialBackorder, secondSnapshot.fulfillmentStatus)
	require.Equal(t, 0, remainingProductStock[product.ID])
}

func TestEstimatedUnitCostOverrideDrivesBackorderCOGS(t *testing.T) {
	product := &models.Product{
		ID:            1,
		Code:          "PRD-BO",
		Name:          "Produk Backorder",
		Unit:          "pcs",
		Stock:         2,
		CostPrice:     100,
		PurchasePrice: 100,
	}
	remainingProductStock := map[uint]int{product.ID: product.Stock}

	snapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(product, 5, remainingProductStock)
	estimatedUnitCost := 125.0
	applySaleEstimatedUnitCostOverride(&snapshot, 5, &estimatedUnitCost)

	require.Equal(t, 2, snapshot.availableQuantity)
	require.Equal(t, 3, snapshot.backorderQuantity)
	require.Equal(t, 125.0, snapshot.estimatedUnitCost)
	require.Equal(t, 625.0, snapshot.estimatedCOGSAmount)
	require.Equal(t, 250.0, snapshot.actualCOGSAmount)
	require.Equal(t, models.SaleFulfillmentStatusPartialBackorder, snapshot.fulfillmentStatus)
}

func TestValidateStockForCreateUsesCumulativeStockPerProduct(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Product{}))
	require.NoError(t, db.Create(&models.Product{
		Code:     "PRD-001",
		Name:     "Produk A",
		Unit:     "pcs",
		Stock:    10,
		IsActive: true,
	}).Error)

	service := &SalesServiceV2{db: db}

	result, err := service.ValidateStockForCreate(models.StockValidationRequest{
		Items: []models.SaleItemRequest{
			{ProductID: 1, Quantity: 8},
			{ProductID: 1, Quantity: 5},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Items, 2)

	require.True(t, result.Items[0].IsSufficient)
	require.Equal(t, 10, result.Items[0].AvailableQty)

	require.False(t, result.Items[1].IsSufficient)
	require.Equal(t, 2, result.Items[1].AvailableQty)
	require.True(t, result.HasInsufficient)
}

func TestSettleBackordersForApprovedPurchaseMarksSaleItemFulfilled(t *testing.T) {
	db, service, purchase, productID := setupBackorderSettlementTest(t, models.SaleStatusPaid)

	affectedProductIDs, err := service.settleBackordersForApprovedPurchase(&purchase)
	require.NoError(t, err)
	require.Contains(t, affectedProductIDs, productID)

	assertBackorderSettled(t, db, productID)
}

func TestSettleBackordersForApprovedPurchaseIncludesPartiallyPaidSales(t *testing.T) {
	db, service, purchase, productID := setupBackorderSettlementTest(t, models.SaleStatusPartiallyPaid)

	_, err := service.settleBackordersForApprovedPurchase(&purchase)
	require.NoError(t, err)

	assertBackorderSettled(t, db, productID)
}

func TestSalesBackorderEstimatedHPPFlowFromInvoiceToPurchaseSettlement(t *testing.T) {
	resetSettingsCacheForTest()
	defer resetSettingsCacheForTest()

	db, err := gorm.Open(sqlite.Open(salesFulfillmentTestDSN(t)), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.Account{},
		&models.AccountingPeriod{},
		&models.Settings{},
		&models.Contact{},
		&models.User{},
		&models.Product{},
		&models.CashBank{},
		&models.Sale{},
		&models.SaleItem{},
		&models.SalePaymentSchedule{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.SaleBackorderAllocation{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.TaxAccountSettings{},
	))
	seedBackorderFlowAccounts(t, db)
	seedBackorderFlowMasterData(t, db)

	settingsService := NewSettingsService(db)
	salesJournalService := NewSalesJournalServiceSSOT(db, NewCOAService(db), settingsService)
	salesService := NewSalesServiceV2(
		db,
		nil,
		nil,
		salesJournalService,
		NewStockService(db),
		nil,
		nil,
		settingsService,
		nil,
	)

	estimatedUnitCost := 125.0
	ppnRate := 0.0
	sale, err := salesService.CreateSale(models.SaleCreateRequest{
		CustomerID:        1,
		Type:              "INVOICE",
		Date:              time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC),
		PPNPercent:        &ppnRate,
		PaymentMethodType: "CREDIT",
		Items: []models.SaleItemRequest{
			{
				ProductID:         1,
				Quantity:          5,
				UnitPrice:         200,
				EstimatedUnitCost: &estimatedUnitCost,
				RevenueAccountID:  3,
			},
		},
	}, 1)
	require.NoError(t, err)
	require.Len(t, sale.SaleItems, 1)
	require.Equal(t, 2, sale.SaleItems[0].AvailableQuantity)
	require.Equal(t, 3, sale.SaleItems[0].BackorderQuantity)
	require.Equal(t, models.SaleFulfillmentStatusPartialBackorder, sale.SaleItems[0].FulfillmentStatus)

	invoicedSale, err := salesService.CreateInvoice(sale.ID, 1)
	require.NoError(t, err)
	require.Equal(t, models.SaleStatusInvoiced, invoicedSale.Status)

	var productAfterInvoice models.Product
	require.NoError(t, db.First(&productAfterInvoice, 1).Error)
	require.Equal(t, 0, productAfterInvoice.Stock)

	saleJournal := requireSingleJournal(t, db, "SALE", uint64(invoicedSale.ID), "")
	require.Equal(t, models.SSOTStatusPosted, saleJournal.Status)
	require.InDelta(t, 1625, saleJournal.TotalDebit.InexactFloat64(), 0.01)
	require.InDelta(t, 1625, saleJournal.TotalCredit.InexactFloat64(), 0.01)
	assertJournalLineTotals(t, db, saleJournal.ID, map[string][2]float64{
		"1102-001": {1000, 0},
		"4101-001": {0, 1000},
		"5101":     {625, 0},
		"1103-001": {0, 250},
		"2103-009": {0, 375},
	})

	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", 1).Update("stock", 3).Error)
	purchaseItemProductID := uint(1)
	purchase := models.Purchase{
		ID:          1,
		Code:        "PO-BO-001",
		VendorID:    2,
		UserID:      1,
		Date:        time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC),
		Status:      models.PurchaseStatusApproved,
		FreightCost: 30,
	}
	require.NoError(t, db.Create(&purchase).Error)
	purchaseItem := models.PurchaseItem{
		ID:         1,
		PurchaseID: purchase.ID,
		ProductID:  &purchaseItemProductID,
		Quantity:   3,
		UnitPrice:  130,
		TotalPrice: 390,
	}
	require.NoError(t, db.Create(&purchaseItem).Error)
	purchase.PurchaseItems = []models.PurchaseItem{purchaseItem}

	purchaseService := &PurchaseService{
		db:                    db,
		unifiedJournalService: NewUnifiedJournalService(db),
	}
	affectedProductIDs, err := purchaseService.settleBackordersForApprovedPurchase(&purchase)
	require.NoError(t, err)
	require.Contains(t, affectedProductIDs, uint(1))

	var settledSaleItem models.SaleItem
	require.NoError(t, db.First(&settledSaleItem, sale.SaleItems[0].ID).Error)
	require.Equal(t, 0, settledSaleItem.BackorderQuantity)
	require.Equal(t, 5, settledSaleItem.FulfilledQuantity)
	require.Equal(t, models.SaleFulfillmentStatusFulfilled, settledSaleItem.FulfillmentStatus)
	require.InDelta(t, 625, settledSaleItem.EstimatedCOGSAmount, 0.01)
	require.InDelta(t, 670, settledSaleItem.ActualCOGSAmount, 0.01)
	require.InDelta(t, 45, settledSaleItem.COGSVarianceAmount, 0.01)

	var productAfterSettlement models.Product
	require.NoError(t, db.First(&productAfterSettlement, 1).Error)
	require.Equal(t, 0, productAfterSettlement.Stock)

	settlementJournal := requireSingleJournal(t, db, models.SSOTSourceTypeAdjustment, uint64(purchase.ID), salesBackorderSettlementNotes)
	require.Equal(t, models.SSOTStatusPosted, settlementJournal.Status)
	require.InDelta(t, 420, settlementJournal.TotalDebit.InexactFloat64(), 0.01)
	require.InDelta(t, 420, settlementJournal.TotalCredit.InexactFloat64(), 0.01)
	assertJournalLineTotals(t, db, settlementJournal.ID, map[string][2]float64{
		"2103-009": {375, 0},
		"1103-001": {0, 420},
		"5101":     {45, 0},
	})

	var allocation models.SaleBackorderAllocation
	require.NoError(t, db.First(&allocation).Error)
	require.Equal(t, 3, allocation.Quantity)
	require.InDelta(t, 125, allocation.EstimatedUnitCost, 0.01)
	require.InDelta(t, 140, allocation.ActualUnitCost, 0.01)
	require.InDelta(t, 375, allocation.EstimatedAmount, 0.01)
	require.InDelta(t, 420, allocation.ActualAmount, 0.01)
	require.InDelta(t, 45, allocation.VarianceAmount, 0.01)
}

func setupBackorderSettlementTest(t *testing.T, saleStatus string) (*gorm.DB, *PurchaseService, models.Purchase, uint) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(salesFulfillmentTestDSN(t)), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.Account{},
		&models.AccountingPeriod{},
		&models.Product{},
		&models.Sale{},
		&models.SaleItem{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.SaleBackorderAllocation{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.TaxAccountSettings{},
	))

	require.NoError(t, db.Create(&[]models.Account{
		{
			ID:       1,
			Code:     "1103-001",
			Name:     "PERSEDIAAN BARANG DAGANG",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			IsActive: true,
		},
		{
			ID:       2,
			Code:     "5101",
			Name:     "HARGA POKOK PENJUALAN",
			Type:     models.AccountTypeExpense,
			Category: models.CategoryCostOfGoodsSold,
			IsActive: true,
		},
		{
			ID:       3,
			Code:     salesBackorderClearingAccountCode,
			Name:     salesBackorderClearingAccountName,
			Type:     models.AccountTypeLiability,
			Category: models.CategoryCurrentLiability,
			IsActive: true,
		},
	}).Error)
	seedBackorderFlowTaxAccountSettings(t, db, 1, 2)

	now := time.Now()
	product := models.Product{
		ID:            1,
		Code:          "PRD-BO",
		Name:          "Produk Backorder",
		Unit:          "pcs",
		Stock:         10,
		PurchasePrice: 100,
		CostPrice:     100,
		IsActive:      true,
	}
	require.NoError(t, db.Create(&product).Error)

	sale := models.Sale{
		ID:                1,
		Code:              "SO-BO",
		CustomerID:        1,
		UserID:            1,
		Status:            saleStatus,
		Date:              now,
		TotalAmount:       1_000,
		PaidAmount:        1_000,
		OutstandingAmount: 0,
	}
	require.NoError(t, db.Create(&sale).Error)
	require.NoError(t, db.Create(&models.SaleItem{
		ID:                  1,
		SaleID:              sale.ID,
		ProductID:           product.ID,
		Quantity:            10,
		UnitPrice:           100,
		LineTotal:           1_000,
		AvailableQuantity:   0,
		FulfilledQuantity:   0,
		BackorderQuantity:   10,
		EstimatedUnitCost:   90,
		EstimatedCOGSAmount: 900,
		ActualCOGSAmount:    0,
		FulfillmentStatus:   models.SaleFulfillmentStatusBackordered,
	}).Error)

	productID := product.ID
	purchase := models.Purchase{
		ID:       1,
		Code:     "PO-BO",
		VendorID: 1,
		UserID:   1,
		Status:   models.PurchaseStatusApproved,
		Date:     now,
	}
	require.NoError(t, db.Create(&purchase).Error)

	purchaseItem := models.PurchaseItem{
		ID:         1,
		PurchaseID: purchase.ID,
		ProductID:  &productID,
		Quantity:   10,
		UnitPrice:  100,
		TotalPrice: 1_000,
	}
	require.NoError(t, db.Create(&purchaseItem).Error)
	purchase.PurchaseItems = []models.PurchaseItem{purchaseItem}

	service := &PurchaseService{
		db:                    db,
		unifiedJournalService: NewUnifiedJournalService(db),
	}

	return db, service, purchase, productID
}

func salesFulfillmentTestDSN(t *testing.T) string {
	t.Helper()
	return "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
}

func resetSettingsCacheForTest() {
	settingsCacheMutex.Lock()
	defer settingsCacheMutex.Unlock()
	settingsCache = nil
	settingsCacheAt = time.Time{}
}

func seedBackorderFlowAccounts(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Create(&[]models.Account{
		{
			ID:       1,
			Code:     "1102-001",
			Name:     "PIUTANG USAHA",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			IsActive: true,
		},
		{
			ID:       2,
			Code:     "1103-001",
			Name:     "PERSEDIAAN BARANG DAGANG",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			IsActive: true,
		},
		{
			ID:       3,
			Code:     "4101-001",
			Name:     "PENDAPATAN PENJUALAN",
			Type:     models.AccountTypeRevenue,
			Category: models.CategorySalesRevenue,
			IsActive: true,
		},
		{
			ID:       4,
			Code:     "5101",
			Name:     "HARGA POKOK PENJUALAN",
			Type:     models.AccountTypeExpense,
			Category: models.CategoryCostOfGoodsSold,
			IsActive: true,
		},
		{
			ID:       5,
			Code:     salesBackorderClearingAccountCode,
			Name:     salesBackorderClearingAccountName,
			Type:     models.AccountTypeLiability,
			Category: models.CategoryCurrentLiability,
			IsActive: true,
		},
	}).Error)
	seedBackorderFlowTaxAccountSettings(t, db, 2, 4)
}

func seedBackorderFlowMasterData(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Create(&models.Settings{
		CompanyName:        "Test Trading Company",
		CompanyType:        models.CompanyTypeTrading,
		Currency:           "IDR",
		SalesPrefix:        "SOA",
		SalesNextNumber:    1,
		InvoicePrefix:      "INV",
		InvoiceNextNumber:  1,
		PurchasePrefix:     "PO",
		PurchaseNextNumber: 1,
	}).Error)
	require.NoError(t, db.Create(&models.User{
		ID:        1,
		Username:  "tester",
		Email:     "tester@example.test",
		Password:  "secret",
		Role:      "admin",
		FirstName: "Test",
		IsActive:  true,
	}).Error)
	require.NoError(t, db.Create(&[]models.Contact{
		{
			ID:       1,
			Code:     "CUST-001",
			Name:     "PT Customer Test",
			Type:     models.ContactTypeCustomer,
			IsActive: true,
		},
		{
			ID:       2,
			Code:     "VEND-001",
			Name:     "PT Vendor Test",
			Type:     models.ContactTypeVendor,
			IsActive: true,
		},
	}).Error)
	require.NoError(t, db.Create(&models.Product{
		ID:            1,
		Code:          "PRD-BO",
		Name:          "Produk Backorder",
		Unit:          "pcs",
		Stock:         2,
		PurchasePrice: 100,
		CostPrice:     100,
		SalePrice:     200,
		IsActive:      true,
	}).Error)
}

func seedBackorderFlowTaxAccountSettings(t *testing.T, db *gorm.DB, inventoryAccountID uint, cogsAccountID uint) {
	t.Helper()

	require.NoError(t, db.Create(&models.TaxAccountSettings{
		SalesReceivableAccountID:  1,
		SalesCashAccountID:        1,
		SalesBankAccountID:        1,
		SalesRevenueAccountID:     1,
		SalesOutputVATAccountID:   1,
		PurchasePayableAccountID:  1,
		PurchaseCashAccountID:     1,
		PurchaseBankAccountID:     1,
		PurchaseInputVATAccountID: 1,
		PurchaseExpenseAccountID:  cogsAccountID,
		InventoryAccountID:        &inventoryAccountID,
		COGSAccountID:             &cogsAccountID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 1,
	}).Error)
}

func requireSingleJournal(t *testing.T, db *gorm.DB, sourceType string, sourceID uint64, notes string) models.SSOTJournalEntry {
	t.Helper()

	query := db.Where("source_type = ? AND source_id = ?", sourceType, sourceID)
	if notes != "" {
		query = query.Where("notes = ?", notes)
	}

	var count int64
	require.NoError(t, query.Model(&models.SSOTJournalEntry{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	var journal models.SSOTJournalEntry
	require.NoError(t, query.First(&journal).Error)
	return journal
}

func assertJournalLineTotals(t *testing.T, db *gorm.DB, journalID uint64, expected map[string][2]float64) {
	t.Helper()

	type row struct {
		Code   string
		Debit  float64
		Credit float64
	}

	var rows []row
	require.NoError(t, db.Table("unified_journal_lines").
		Select("accounts.code AS code, unified_journal_lines.debit_amount AS debit, unified_journal_lines.credit_amount AS credit").
		Joins("JOIN accounts ON accounts.id = unified_journal_lines.account_id").
		Where("unified_journal_lines.journal_id = ?", journalID).
		Scan(&rows).Error)

	actual := make(map[string][2]float64, len(rows))
	for _, row := range rows {
		totals := actual[row.Code]
		totals[0] += row.Debit
		totals[1] += row.Credit
		actual[row.Code] = totals
	}

	require.Len(t, actual, len(expected))
	for code, expectedTotals := range expected {
		actualTotals, exists := actual[code]
		require.Truef(t, exists, "expected journal line for account %s", code)
		require.InDeltaf(t, expectedTotals[0], actualTotals[0], 0.01, "debit mismatch for account %s", code)
		require.InDeltaf(t, expectedTotals[1], actualTotals[1], 0.01, "credit mismatch for account %s", code)
	}
}

func assertBackorderSettled(t *testing.T, db *gorm.DB, productID uint) {
	t.Helper()

	var saleItem models.SaleItem
	require.NoError(t, db.First(&saleItem, 1).Error)
	require.Equal(t, 0, saleItem.BackorderQuantity)
	require.Equal(t, 10, saleItem.FulfilledQuantity)
	require.Equal(t, models.SaleFulfillmentStatusFulfilled, saleItem.FulfillmentStatus)

	var product models.Product
	require.NoError(t, db.First(&product, productID).Error)
	require.Equal(t, 0, product.Stock)

	var allocation models.SaleBackorderAllocation
	require.NoError(t, db.First(&allocation).Error)
	require.Equal(t, 10, allocation.Quantity)
	require.Equal(t, models.SaleBackorderAllocationStatusApplied, allocation.Status)

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).
		Where("notes = ?", salesBackorderSettlementNotes).
		Count(&journalCount).Error)
	require.EqualValues(t, 1, journalCount)

	var journal models.SSOTJournalEntry
	require.NoError(t, db.Preload("Lines").Where("notes = ?", salesBackorderSettlementNotes).First(&journal).Error)
	require.Len(t, journal.Lines, 3)

	var debitTotal float64
	var creditTotal float64
	for _, line := range journal.Lines {
		debitTotal += line.DebitAmount.InexactFloat64()
		creditTotal += line.CreditAmount.InexactFloat64()
	}
	require.Equal(t, 1000.0, debitTotal)
	require.Equal(t, 1000.0, creditTotal)
}
