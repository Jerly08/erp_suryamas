package services

import (
	"app-sistem-akuntansi/models"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// InventoryCOGSService handles Cost of Goods Sold journal entries
// This service creates COGS journal entries when items are sold
type InventoryCOGSService struct {
	db               *gorm.DB
	coaService       *COAService
	taxAccountHelper *TaxAccountHelper
}

// NewInventoryCOGSService creates a new COGS service instance
func NewInventoryCOGSService(db *gorm.DB, coaService *COAService) *InventoryCOGSService {
	return &InventoryCOGSService{
		db:               db,
		coaService:       coaService,
		taxAccountHelper: NewTaxAccountHelper(db),
	}
}

// RecordCOGSForSale creates COGS journal entries for a sale transaction
// This is the key method that ensures purchases are reflected in P&L as expenses
func (s *InventoryCOGSService) RecordCOGSForSale(sale *models.Sale, tx *gorm.DB) error {
	log.Printf("📊 [COGS] Recording COGS for Sale #%d", sale.ID)

	// Determine database to use
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Check if COGS journal already exists (check by Notes = 'COGS')
	var existingCount int64
	saleIDCheck := uint64(sale.ID)
	if err := dbToUse.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ? AND notes = ?",
			"SALE", &saleIDCheck, "COGS").
		Count(&existingCount).Error; err == nil && existingCount > 0 {
		log.Printf("⚠️ [COGS] COGS journal already exists for Sale #%d, skipping", sale.ID)
		return nil
	}

	// Load sale items with product details
	if err := dbToUse.Preload("SaleItems.Product").First(&sale, sale.ID).Error; err != nil {
		return fmt.Errorf("failed to load sale items: %v", err)
	}

	// Get COGS and Inventory accounts from Tax Account Settings (configurable)
	cogsAccount, err := s.taxAccountHelper.GetCOGSAccount(dbToUse)
	if err != nil {
		// Fallback to direct code lookup
		cogsAccount, err = s.getAccountByCode(dbToUse, "5101")
		if err != nil {
			return fmt.Errorf("COGS account not found: %v", err)
		}
	}

	inventoryAccount, err := s.taxAccountHelper.GetInventoryAccount(dbToUse)
	if err != nil {
		// Fallback to direct code lookup
		inventoryAccount, err = s.getAccountByCode(dbToUse, "1103-001")
		if err != nil {
			return fmt.Errorf("Inventory account not found: %v", err)
		}
	}

	// Calculate total COGS from sale items
	var totalCOGS decimal.Decimal
	var journalLines []models.SSOTJournalLine

	for _, item := range sale.SaleItems {
		// Skip if no product loaded (check if Product.ID is 0)
		if item.Product.ID == 0 {
			log.Printf("⚠️ [COGS] Sale item #%d has no product, skipping", item.ID)
			continue
		}

		// Calculate COGS: Quantity * Cost Price
		itemCOGS := decimal.NewFromFloat(float64(item.Quantity)).Mul(
			decimal.NewFromFloat(item.Product.CostPrice),
		)

		if itemCOGS.IsZero() {
			log.Printf("⚠️ [COGS] Product '%s' has zero cost price, skipping", item.Product.Name)
			continue
		}

		totalCOGS = totalCOGS.Add(itemCOGS)

		log.Printf("   [COGS] Item: %s | Qty: %d | Cost: Rp %.2f | COGS: Rp %.2f",
			item.Product.Name, item.Quantity, item.Product.CostPrice, itemCOGS.InexactFloat64())
	}

	// If no COGS calculated, skip
	if totalCOGS.IsZero() {
		log.Printf("⚠️ [COGS] No COGS calculated for Sale #%d, skipping journal entry", sale.ID)
		return nil
	}

	// Prepare source ID as pointer to uint64
	saleIDUint64 := uint64(sale.ID)

	// Create journal entry
	// Insert as DRAFT first to avoid trigger validation before lines are created
	journalEntry := models.SSOTJournalEntry{
		EntryDate:   sale.Date, // Use Date instead of SaleDate
		Reference:   fmt.Sprintf("COGS-%s", sale.InvoiceNumber),
		Description: fmt.Sprintf("Cost of Goods Sold - %s", sale.InvoiceNumber),
		SourceType:  "SALE",
		SourceID:    &saleIDUint64,
		SourceCode:  sale.Code,
		Notes:       "COGS", // Use Notes field to mark as COGS entry
		Status:      "DRAFT",
		TotalDebit:  totalCOGS,
		TotalCredit: totalCOGS,
		CreatedBy:   uint64(sale.UserID), // Convert uint to uint64
		CreatedAt:   time.Now(),
	}

	if err := dbToUse.Create(&journalEntry).Error; err != nil {
		return fmt.Errorf("failed to create COGS journal entry: %v", err)
	}

	// Create journal lines
	// DEBIT: 5101 Harga Pokok Penjualan (COGS - Expense)
	journalLines = append(journalLines, models.SSOTJournalLine{
		JournalID:    journalEntry.ID,
		AccountID:    uint64(cogsAccount.ID),
		LineNumber:   1,
		DebitAmount:  totalCOGS,
		CreditAmount: decimal.Zero,
		Description:  fmt.Sprintf("COGS - %s", sale.InvoiceNumber),
	})

	// CREDIT: 1103-001 Persediaan Barang (Inventory - Asset)
	journalLines = append(journalLines, models.SSOTJournalLine{
		JournalID:    journalEntry.ID,
		AccountID:    uint64(inventoryAccount.ID),
		LineNumber:   2,
		DebitAmount:  decimal.Zero,
		CreditAmount: totalCOGS,
		Description:  fmt.Sprintf("Pengurangan Inventory - %s", sale.InvoiceNumber),
	})

	// Save journal lines
	if err := dbToUse.Create(&journalLines).Error; err != nil {
		// Rollback journal entry if lines fail
		dbToUse.Delete(&journalEntry)
		return fmt.Errorf("failed to create COGS journal lines: %v", err)
	}

	// Now update status to POSTED after lines are created
	now := time.Now()
	postedBy := uint64(sale.UserID)
	if err := dbToUse.Model(&journalEntry).Updates(map[string]interface{}{
		"status":    "POSTED",
		"posted_at": &now,
		"posted_by": &postedBy,
	}).Error; err != nil {
		return fmt.Errorf("failed to post COGS journal entry: %v", err)
	}
	journalEntry.Status = "POSTED" // Update in-memory object

	// ✅ Update account balances for COA tree view
	for _, line := range journalLines {
		if err := s.updateAccountBalance(dbToUse, line.AccountID, line.DebitAmount, line.CreditAmount); err != nil {
			log.Printf("⚠️ [COGS] Warning: Failed to update account balance for account %d: %v", line.AccountID, err)
			// Continue - don't fail transaction
		}
	}

	log.Printf("✅ [COGS] Successfully recorded COGS for Sale #%d: Rp %.2f",
		sale.ID, totalCOGS.InexactFloat64())

	return nil
}

// RecordCOGSForMultipleSales records COGS for multiple sales (batch processing)
func (s *InventoryCOGSService) RecordCOGSForMultipleSales(saleIDs []uint) (int, error) {
	successCount := 0

	for _, saleID := range saleIDs {
		var sale models.Sale
		if err := s.db.First(&sale, saleID).Error; err != nil {
			log.Printf("❌ [COGS] Failed to load sale #%d: %v", saleID, err)
			continue
		}

		if err := s.RecordCOGSForSale(&sale, nil); err != nil {
			log.Printf("❌ [COGS] Failed to record COGS for sale #%d: %v", saleID, err)
			continue
		}

		successCount++
	}

	log.Printf("✅ [COGS] Successfully recorded COGS for %d/%d sales", successCount, len(saleIDs))
	return successCount, nil
}

// BackfillCOGSForExistingSales backfills COGS entries for sales that don't have them yet
func (s *InventoryCOGSService) BackfillCOGSForExistingSales(ctx context.Context, startDate, endDate time.Time) (int, error) {
	log.Printf("🔄 [COGS] Backfilling COGS for sales between %s and %s",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Find sales that don't have COGS journal entries
	var sales []models.Sale
	query := s.db.Where("date >= ? AND date <= ?", startDate, endDate).
		Where("status IN ('INVOICED', 'PAID')").
		Where("NOT EXISTS (SELECT 1 FROM unified_journal_ledger WHERE source_type = 'SALE' AND source_id = sales.id AND notes = 'COGS')")

	if err := query.Find(&sales).Error; err != nil {
		return 0, fmt.Errorf("failed to query sales: %v", err)
	}

	log.Printf("📊 [COGS] Found %d sales without COGS entries", len(sales))

	successCount := 0
	for _, sale := range sales {
		// Check context cancellation
		select {
		case <-ctx.Done():
			log.Printf("⚠️ [COGS] Backfill cancelled by context")
			return successCount, ctx.Err()
		default:
		}

		if err := s.RecordCOGSForSale(&sale, nil); err != nil {
			log.Printf("❌ [COGS] Failed to backfill COGS for sale #%d: %v", sale.ID, err)
			continue
		}

		successCount++
	}

	log.Printf("✅ [COGS] Backfill complete: %d/%d sales processed", successCount, len(sales))
	return successCount, nil
}

// GetCOGSSummary returns COGS summary for a period
func (s *InventoryCOGSService) GetCOGSSummary(startDate, endDate time.Time) (map[string]interface{}, error) {
	var result struct {
		TotalCOGS      decimal.Decimal
		TotalSales     int64
		TotalRevenue   decimal.Decimal
		AvgCOGS        decimal.Decimal
		COGSPercentage decimal.Decimal
	}

	// Resolve COGS account code dynamically from Tax Account Settings
	cogsAccountCode := "5101" // default fallback
	cogsAccount, err := s.taxAccountHelper.GetCOGSAccount(s.db)
	if err == nil {
		cogsAccountCode = cogsAccount.Code
	} else {
		log.Printf("⚠️ [COGS Summary] Could not resolve COGS account from settings, using default code '5101'")
	}

	// Get total COGS from journal entries
	var totalCOGS decimal.Decimal
	if err := s.db.Raw(`
		SELECT COALESCE(SUM(ujl.debit_amount), 0) as total_cogs
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		JOIN accounts a ON a.id = ujl.account_id
		WHERE uje.entry_date >= ? AND uje.entry_date <= ?
		  AND uje.status = 'POSTED'
		  AND uje.notes = 'COGS'
		  AND a.code = ?
	`, startDate, endDate, cogsAccountCode).Scan(&totalCOGS).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate total COGS: %v", err)
	}

	// Get sales count and revenue
	var totalSales int64
	var totalRevenue decimal.Decimal
	if err := s.db.Model(&models.Sale{}).
		Where("date >= ? AND date <= ?", startDate, endDate).
		Where("status IN ('INVOICED', 'PAID')").
		Count(&totalSales).Error; err != nil {
		return nil, fmt.Errorf("failed to count sales: %v", err)
	}

	if err := s.db.Raw(`
		SELECT COALESCE(SUM(total_amount), 0) as total_revenue
		FROM sales
		WHERE date >= ? AND date <= ?
		  AND status IN ('INVOICED', 'PAID')
	`, startDate, endDate).Scan(&totalRevenue).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate total revenue: %v", err)
	}

	result.TotalCOGS = totalCOGS
	result.TotalSales = totalSales
	result.TotalRevenue = totalRevenue

	if totalSales > 0 {
		result.AvgCOGS = totalCOGS.Div(decimal.NewFromInt(totalSales))
	}

	if totalRevenue.GreaterThan(decimal.Zero) {
		result.COGSPercentage = totalCOGS.Div(totalRevenue).Mul(decimal.NewFromInt(100))
	}

	return map[string]interface{}{
		"total_cogs":      result.TotalCOGS.InexactFloat64(),
		"total_sales":     result.TotalSales,
		"total_revenue":   result.TotalRevenue.InexactFloat64(),
		"avg_cogs":        result.AvgCOGS.InexactFloat64(),
		"cogs_percentage": result.COGSPercentage.InexactFloat64(),
	}, nil
}

// Helper: Get account by code
func (s *InventoryCOGSService) getAccountByCode(db *gorm.DB, code string) (*models.Account, error) {
	var account models.Account
	if err := db.Where("code = ?", code).First(&account).Error; err != nil {
		return nil, fmt.Errorf("account code %s not found: %v", code, err)
	}
	return &account, nil
}
