package repositories

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PurchaseRepository struct {
	db *gorm.DB
}

func NewPurchaseRepository(db *gorm.DB) *PurchaseRepository {
	return &PurchaseRepository{db: db}
}

func (r *PurchaseRepository) withPurchasePaymentPreloads(query *gorm.DB) *gorm.DB {
	if r.db.Migrator().HasTable(&models.PurchasePayment{}) {
		query = query.
			Preload("PurchasePayments").
			Preload("PurchasePayments.Payment").
			Preload("PurchasePayments.PurchasePaymentTerm").
			Preload("PurchasePayments.PaymentProofUploader").
			Preload("PurchasePayments.PaymentProofReviewer")
	}

	if r.db.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		query = query.Preload("PaymentTerms", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC, due_date ASC, id ASC")
		})
	}

	return query
}

// Component VAT was introduced after the original Purchase schema. Keep the
// lookup optional so lightweight test databases and older read-only replicas
// can still read historical purchases while the additive migration is rolling
// out.
func (r *PurchaseRepository) withPurchaseTaxAllocationPreloads(query *gorm.DB) *gorm.DB {
	if !r.db.Migrator().HasTable(&models.PurchaseTaxAllocation{}) {
		return query
	}

	return query.
		Preload("TaxAllocations", func(db *gorm.DB) *gorm.DB {
			return db.Order("purchase_item_id ASC, id ASC")
		}).
		Preload("TaxAllocations.PurchaseItem").
		Preload("TaxAllocations.Verifier")
}

func isDateOnlyFilter(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return trimmed, false
	}

	return parsed.Format("2006-01-02"), true
}

func (r *PurchaseRepository) applyPurchaseFilter(query *gorm.DB, filter models.PurchaseFilter) *gorm.DB {
	if filter.Status != "" {
		query = query.Where("purchases.status = ?", filter.Status)
	}

	if filter.VendorID != "" {
		query = query.Where("purchases.vendor_id = ?", filter.VendorID)
	}

	if filter.ProjectID != "" {
		query = query.Where("purchases.project_id = ?", filter.ProjectID)
	}

	if filter.UserID != 0 {
		query = query.Where("purchases.user_id = ?", filter.UserID)
	}

	if filter.StartDate != "" {
		if startDate, ok := isDateOnlyFilter(filter.StartDate); ok {
			query = query.Where("DATE(purchases.date) >= ?", startDate)
		} else {
			query = query.Where("purchases.date >= ?", strings.TrimSpace(filter.StartDate))
		}
	}

	if filter.EndDate != "" {
		if endDate, ok := isDateOnlyFilter(filter.EndDate); ok {
			query = query.Where("DATE(purchases.date) <= ?", endDate)
		} else {
			query = query.Where("purchases.date <= ?", strings.TrimSpace(filter.EndDate))
		}
	}

	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		query = query.Joins("LEFT JOIN contacts ON contacts.id = purchases.vendor_id").
			Where("purchases.code ILIKE ? OR purchases.notes ILIKE ? OR contacts.name ILIKE ? OR contacts.code ILIKE ?",
				like, like, like, like)
	}

	if filter.ApprovalStatus != "" {
		query = query.Where("purchases.approval_status = ?", filter.ApprovalStatus)
	}

	if filter.AccountingStatus != "" {
		switch strings.ToUpper(strings.TrimSpace(filter.AccountingStatus)) {
		case models.PurchaseAccountingStatusReadyForCOA:
			query = query.Where(
				"purchases.approval_status = ? AND purchases.status = ?",
				models.PurchaseApprovalApproved,
				models.PurchaseStatusPendingAccounting,
			)
		case models.PurchaseAccountingStatusApprovedByFinance, "FINANCE_APPROVED":
			query = query.Where(
				"purchases.approval_status = ? AND purchases.status IN ?",
				models.PurchaseApprovalApproved,
				[]string{
					models.PurchaseStatusPendingAccounting,
					models.PurchaseStatusPendingFinalApproval,
					models.PurchaseStatusApproved,
					models.PurchaseStatusCompleted,
					models.PurchaseStatusPaid,
				},
			)
		case models.PurchaseAccountingStatusPendingFinalApproval, "FINAL_APPROVAL":
			query = query.Where(
				"purchases.approval_status = ? AND purchases.status = ?",
				models.PurchaseApprovalApproved,
				models.PurchaseStatusPendingFinalApproval,
			)
		case models.PurchaseAccountingStatusCOACompleted, "COMPLETED_COA":
			query = query.Where(
				"purchases.approval_status = ? AND purchases.status IN ?",
				models.PurchaseApprovalApproved,
				[]string{
					models.PurchaseStatusPendingFinalApproval,
					models.PurchaseStatusApproved,
					models.PurchaseStatusCompleted,
					models.PurchaseStatusPaid,
				},
			)
		}
	}

	if filter.RequiresApproval != nil {
		query = query.Where("purchases.requires_approval = ?", *filter.RequiresApproval)
	}

	if filter.OutstandingOnly != nil && *filter.OutstandingOnly {
		query = query.Where("purchases.outstanding_amount > 0")
	}

	return query
}

// Purchase CRUD Operations

func (r *PurchaseRepository) Create(purchase *models.Purchase) (*models.Purchase, error) {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(purchase).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return r.FindByID(purchase.ID)
}

func (r *PurchaseRepository) FindByID(id uint) (*models.Purchase, error) {
	var purchase models.Purchase
	query := r.db.Preload("Vendor").
		Preload("User").
		Preload("PurchaseRequest").
		Preload("PurchaseRequest.Project").
		Preload("PurchaseRequest.Documents", "document_type = ?", models.PurchaseRequestDocumentRealization).
		Preload("PurchaseRequest.Documents.Uploader").
		Preload("Project").
		// Component-VAT allocations may temporarily reference an incoming item
		// index while a draft Purchase is being persisted. Keep the visible and
		// server-side item order deterministic across all database engines.
		Preload("PurchaseItems", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("PurchaseItems.Product").
		Preload("PurchaseItems.PurchaseRequestItem").
		Preload("PurchaseItems.ExpenseAccount").
		Preload("OtherCostAccount").
		Preload("ApprovalRequest.Workflow").
		Preload("ApprovalRequest.Requester").
		Preload("ApprovalRequest.ApprovalSteps.Step").
		Preload("ApprovalRequest.ApprovalSteps.Approver").
		Preload("Approver")
	query = r.withPurchasePaymentPreloads(query)
	query = r.withPurchaseTaxAllocationPreloads(query)
	err := query.First(&purchase, id).Error

	if err != nil {
		return nil, err
	}
	return &purchase, nil
}

func (r *PurchaseRepository) FindAll() ([]models.Purchase, error) {
	var purchases []models.Purchase
	query := r.db.Preload("Vendor").
		Preload("User").
		Preload("PurchaseRequest").
		Preload("PurchaseRequest.Project").
		Preload("PurchaseRequest.Documents", "document_type = ?", models.PurchaseRequestDocumentRealization).
		Preload("PurchaseRequest.Documents.Uploader").
		Preload("Project").
		Preload("PurchaseItems", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("OtherCostAccount").
		Preload("ApprovalRequest")
	query = r.withPurchasePaymentPreloads(query)
	query = r.withPurchaseTaxAllocationPreloads(query)
	err := query.Find(&purchases).Error

	return purchases, err
}

func (r *PurchaseRepository) FindWithFilter(filter models.PurchaseFilter) ([]models.Purchase, int64, error) {
	var purchases []models.Purchase
	var total int64

	query := r.db.Model(&models.Purchase{}).
		Preload("Vendor").
		Preload("User").
		Preload("PurchaseRequest").
		Preload("PurchaseRequest.Project").
		Preload("PurchaseRequest.Documents", "document_type = ?", models.PurchaseRequestDocumentRealization).
		Preload("PurchaseRequest.Documents.Uploader").
		Preload("Project").
		Preload("PurchaseItems", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("PurchaseItems.Product").
		Preload("PurchaseItems.PurchaseRequestItem").
		Preload("OtherCostAccount").
		// Load approval request with all nested data for role-based filtering
		Preload("ApprovalRequest").
		Preload("ApprovalRequest.Workflow").
		Preload("ApprovalRequest.Requester").
		Preload("ApprovalRequest.ApprovalSteps").
		Preload("ApprovalRequest.ApprovalSteps.Step").
		Preload("ApprovalRequest.ApprovalSteps.Approver").
		Preload("Approver")
	query = r.withPurchasePaymentPreloads(query)
	query = r.withPurchaseTaxAllocationPreloads(query)

	query = r.applyPurchaseFilter(query, filter)

	// Get total count
	query.Count(&total)

	// Apply pagination
	offset := (filter.Page - 1) * filter.Limit
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(filter.Limit).
		Find(&purchases).Error

	return purchases, total, err
}

func (r *PurchaseRepository) Update(purchase *models.Purchase) (*models.Purchase, error) {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Save(purchase).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return r.FindByID(purchase.ID)
}

// UpdateWithItemReplacement atomically replaces all active purchase items.
// GORM does not delete has-many rows when an association slice is cleared in
// memory, so the replacement must be explicit to avoid duplicate line items.
func (r *PurchaseRepository) UpdateWithItemReplacement(purchase *models.Purchase) (*models.Purchase, error) {
	if purchase == nil || purchase.ID == 0 {
		return nil, fmt.Errorf("purchase is required to replace items")
	}

	tx := r.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			tx.Rollback()
			panic(recovered)
		}
	}()

	// Persist the purchase header only. The existing item rows are managed below
	// so Save cannot append a second copy of the submitted items.
	if err := tx.Omit(clause.Associations).Save(purchase).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Where("purchase_id = ?", purchase.ID).Delete(&models.PurchaseItem{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	items := make([]models.PurchaseItem, len(purchase.PurchaseItems))
	copy(items, purchase.PurchaseItems)
	for i := range items {
		// Incoming update items represent the new current lines, never rows to
		// update in place. Reset persistence fields before creating them.
		items[i].ID = 0
		items[i].PurchaseID = purchase.ID
		items[i].CreatedAt = time.Time{}
		items[i].UpdatedAt = time.Time{}
		items[i].DeletedAt = gorm.DeletedAt{}
	}

	if len(items) > 0 {
		if err := tx.Omit(clause.Associations).Create(&items).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return r.FindByID(purchase.ID)
}

func (r *PurchaseRepository) Delete(id uint) error {
	return r.db.Delete(&models.Purchase{}, id).Error
}

// Purchase Item Operations

func (r *PurchaseRepository) CreateItem(item *models.PurchaseItem) error {
	return r.db.Create(item).Error
}

func (r *PurchaseRepository) UpdateItem(item *models.PurchaseItem) error {
	return r.db.Save(item).Error
}

func (r *PurchaseRepository) DeleteItem(id uint) error {
	return r.db.Delete(&models.PurchaseItem{}, id).Error
}

func (r *PurchaseRepository) FindItemsByPurchaseID(purchaseID uint) ([]models.PurchaseItem, error) {
	var items []models.PurchaseItem
	err := r.db.Preload("Product").
		Preload("ExpenseAccount").
		Where("purchase_id = ?", purchaseID).
		Find(&items).Error

	return items, err
}

// Purchase Document Operations

func (r *PurchaseRepository) CreateDocument(document *models.PurchaseDocument) error {
	return r.db.Create(document).Error
}

func (r *PurchaseRepository) FindDocumentsByPurchaseID(purchaseID uint) ([]models.PurchaseDocument, error) {
	var documents []models.PurchaseDocument
	err := r.db.Where("purchase_id = ?", purchaseID).
		Find(&documents).Error

	return documents, err
}

func (r *PurchaseRepository) FindDocumentByID(id uint) (*models.PurchaseDocument, error) {
	var document models.PurchaseDocument
	if err := r.db.First(&document, id).Error; err != nil {
		return nil, err
	}
	return &document, nil
}

func (r *PurchaseRepository) DeleteDocument(id uint) error {
	return r.db.Delete(&models.PurchaseDocument{}, id).Error
}

// Purchase Receipt Operations

func (r *PurchaseRepository) CreateReceipt(receipt *models.PurchaseReceipt) (*models.PurchaseReceipt, error) {
	if err := r.db.Create(receipt).Error; err != nil {
		return nil, err
	}

	return r.FindReceiptByID(receipt.ID)
}

func (r *PurchaseRepository) FindReceiptByID(id uint) (*models.PurchaseReceipt, error) {
	var receipt models.PurchaseReceipt
	err := r.db.Preload("Purchase.Vendor").
		Preload("Purchase").
		Preload("Receiver").
		Preload("ReceiptItems.PurchaseItem.Product").
		First(&receipt, id).Error

	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (r *PurchaseRepository) FindReceiptsByPurchaseID(purchaseID uint) ([]models.PurchaseReceipt, error) {
	var receipts []models.PurchaseReceipt
	err := r.db.Preload("Purchase.Vendor").
		Preload("Purchase").
		Preload("Receiver").
		Preload("ReceiptItems.PurchaseItem.Product").
		Where("purchase_id = ?", purchaseID).
		Order("created_at ASC").
		Find(&receipts).Error

	return receipts, err
}

// SumReceivedQtyByPurchaseItem returns total received quantity for a purchase item across all receipts
func (r *PurchaseRepository) SumReceivedQtyByPurchaseItem(purchaseItemID uint) (int, error) {
	var total int64
	err := r.db.Model(&models.PurchaseReceiptItem{}).
		Where("purchase_item_id = ?", purchaseItemID).
		Select("COALESCE(SUM(quantity_received), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

// AreAllItemsFullyReceived checks if all purchase items have been fully received
func (r *PurchaseRepository) AreAllItemsFullyReceived(purchaseID uint) (bool, error) {
	items, err := r.FindItemsByPurchaseID(purchaseID)
	if err != nil {
		return false, err
	}
	for _, it := range items {
		if strings.EqualFold(strings.TrimSpace(it.ItemType), models.PurchaseItemTypePrepaid) {
			continue
		}
		received, err := r.SumReceivedQtyByPurchaseItem(it.ID)
		if err != nil {
			return false, err
		}
		if received < int(it.Quantity) {
			return false, nil
		}
	}
	return true, nil
}

// FindCompletedReceiptsByPurchaseID gets only completed receipts for a purchase
func (r *PurchaseRepository) FindCompletedReceiptsByPurchaseID(purchaseID uint) ([]models.PurchaseReceipt, error) {
	var receipts []models.PurchaseReceipt
	err := r.db.Preload("Purchase.Vendor").
		Preload("Purchase").
		Preload("Receiver").
		Preload("ReceiptItems.PurchaseItem.Product").
		Where("purchase_id = ? AND status = ?", purchaseID, models.ReceiptStatusComplete).
		Find(&receipts).Error

	return receipts, err
}

func (r *PurchaseRepository) CreateReceiptItem(item *models.PurchaseReceiptItem) error {
	return r.db.Create(item).Error
}

// GetReceiptItems gets all receipt items for a specific receipt
func (r *PurchaseRepository) GetReceiptItems(receiptID uint) ([]models.PurchaseReceiptItem, error) {
	var items []models.PurchaseReceiptItem
	err := r.db.Preload("PurchaseItem").
		Preload("PurchaseItem.Product").
		Where("receipt_id = ?", receiptID).
		Find(&items).Error
	return items, err
}

// Statistics and Analytics

func (r *PurchaseRepository) GetPurchasesSummary(filter models.PurchaseFilter) (*models.PurchaseSummary, error) {
	var summary models.PurchaseSummary

	activePurchasesQuery := func() *gorm.DB {
		return r.applyPurchaseFilter(r.db.Model(&models.Purchase{}), filter).
			Where("purchases.status != ? AND purchases.approval_status != ?", models.PurchaseStatusCancelled, models.PurchaseApprovalRejected)
	}

	// Get basic counts and totals
	var totalCount int64
	var totalAmount float64
	var totalApprovedAmount float64
	var totalPaid float64
	var totalOutstanding float64

	activePurchasesQuery().Count(&totalCount)
	// Only include non-rejected purchases in total amount calculation
	activePurchasesQuery().Select("COALESCE(SUM(total_amount), 0)").Scan(&totalAmount)
	activePurchasesQuery().Select("COALESCE(SUM(paid_amount), 0)").Scan(&totalPaid)
	activePurchasesQuery().Select("COALESCE(SUM(outstanding_amount), 0)").Scan(&totalOutstanding)

	// Calculate approved purchases total
	query2 := r.applyPurchaseFilter(r.db.Model(&models.Purchase{}), filter)
	approvedAmountStatuses := []string{
		models.PurchaseStatusPendingAccounting,
		models.PurchaseStatusPendingFinalApproval,
		models.PurchaseStatusApproved,
		models.PurchaseStatusCompleted,
		models.PurchaseStatusPaid,
	}

	// Debug: Log the values being used in query
	fmt.Printf("[DEBUG] Approved query - Status values: %v, Approval status: %v\n", approvedAmountStatuses, models.PurchaseApprovalApproved)

	query2.Where("purchases.status IN (?) AND purchases.approval_status = ?", approvedAmountStatuses, models.PurchaseApprovalApproved).
		Select("COALESCE(SUM(total_amount), 0)").Scan(&totalApprovedAmount)

	// Debug: Log the result
	fmt.Printf("[DEBUG] Total approved amount calculated: %f\n", totalApprovedAmount)

	// Get status counts
	var statusCounts []struct {
		Status string
		Count  int64
	}
	r.applyPurchaseFilter(r.db.Model(&models.Purchase{}), filter).
		Select("purchases.status as status, COUNT(*) as count").
		Group("purchases.status").
		Find(&statusCounts)

	// Get approval status counts
	var approvalStatusCounts []struct {
		ApprovalStatus string
		Count          int64
	}
	r.applyPurchaseFilter(r.db.Model(&models.Purchase{}), filter).
		Select("purchases.approval_status as approval_status, COUNT(*) as count").
		Group("purchases.approval_status").
		Find(&approvalStatusCounts)

	summary.TotalPurchases = totalCount
	summary.TotalAmount = totalAmount
	summary.TotalApprovedAmount = totalApprovedAmount
	summary.TotalPaid = totalPaid
	summary.TotalOutstanding = totalOutstanding
	if totalCount > 0 {
		summary.AvgOrderValue = totalAmount / float64(totalCount)
	}
	summary.StatusCounts = make(map[string]int64)
	summary.ApprovalStatusCounts = make(map[string]int64)

	for _, sc := range statusCounts {
		summary.StatusCounts[sc.Status] = sc.Count
	}

	for _, asc := range approvalStatusCounts {
		summary.ApprovalStatusCounts[asc.ApprovalStatus] = asc.Count
	}

	return &summary, nil
}

func (r *PurchaseRepository) GetVendorPurchaseSummary(vendorID uint) (*models.VendorPurchaseSummary, error) {
	var summary models.VendorPurchaseSummary

	err := r.db.Model(&models.Purchase{}).
		Where("vendor_id = ?", vendorID).
		Select("COUNT(*) as total_orders, COALESCE(SUM(total_amount), 0) as total_amount").
		Scan(&summary).Error

	return &summary, err
}

// Count operations for code generation

func (r *PurchaseRepository) CountByMonth(year, month int) (int64, error) {
	var count int64
	err := r.db.Model(&models.Purchase{}).
		Where("EXTRACT(year FROM created_at) = ? AND EXTRACT(month FROM created_at) = ?",
			year, month).
		Count(&count).Error

	return count, err
}

// GetLastPurchaseNumberByMonth gets the last number used in purchase code for a specific month
func (r *PurchaseRepository) GetLastPurchaseNumberByMonth(year, month int) (int, error) {
	var purchase models.Purchase
	var lastNumber int

	// Query for purchases with codes matching the pattern for this year/month
	// Include deleted records since unique constraint applies to all records
	pattern := fmt.Sprintf("PO/%04d/%02d/%%", year, month)

	err := r.db.Unscoped().Model(&models.Purchase{}).
		Where("code LIKE ?", pattern).
		Order("code DESC").
		First(&purchase).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No purchases found for this month, return 0
			return 0, nil
		}
		return 0, err
	}

	// Extract the number from the code
	if purchase.Code != "" {
		// Parse the last 4 digits of the code
		_, err := fmt.Sscanf(purchase.Code, fmt.Sprintf("PO/%04d/%02d/%%04d", year, month), &lastNumber)
		if err != nil {
			// If parsing fails, try to get it from the last 4 characters
			if len(purchase.Code) >= 4 {
				lastPart := purchase.Code[len(purchase.Code)-4:]
				fmt.Sscanf(lastPart, "%d", &lastNumber)
			}
		}
	}

	return lastNumber, nil
}

func (r *PurchaseRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Purchase{}).
		Where("status = ?", status).
		Count(&count).Error

	return count, err
}

func (r *PurchaseRepository) CountPendingApproval() (int64, error) {
	var count int64
	err := r.db.Model(&models.Purchase{}).
		Where("approval_status = ?", models.PurchaseApprovalPending).
		Count(&count).Error

	return count, err
}

// Three-way matching operations

func (r *PurchaseRepository) GetPurchaseForMatching(id uint) (*models.PurchaseMatchingData, error) {
	var purchase models.Purchase
	err := r.db.Preload("PurchaseItems.Product").
		Preload("Vendor").
		First(&purchase, id).Error

	if err != nil {
		return nil, err
	}

	// Get receipts for this purchase
	var receipts []models.PurchaseReceipt
	err = r.db.Where("purchase_id = ?", id).
		Preload("ReceiptItems").
		Find(&receipts).Error
	if err != nil {
		return nil, err
	}

	// Get documents for this purchase
	var documents []models.PurchaseDocument
	err = r.db.Where("purchase_id = ?", id).Find(&documents).Error
	if err != nil {
		return nil, err
	}

	matching := &models.PurchaseMatchingData{
		Purchase:  purchase,
		Receipts:  receipts,
		Documents: documents,
	}

	return matching, nil
}

func (r *PurchaseRepository) UpdateMatchingStatus(purchaseID uint, status string) error {
	return r.db.Model(&models.Purchase{}).
		Where("id = ?", purchaseID).
		Update("matching_status", status).Error
}

// CodeExists checks if a purchase with the given code already exists
func (r *PurchaseRepository) CodeExists(code string) (bool, error) {
	var count int64
	// Include deleted records since unique constraint applies to all records
	err := r.db.Unscoped().Model(&models.Purchase{}).Where("code = ?", code).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateJournal creates journal entry
func (r *PurchaseRepository) CreateJournal(journal *models.Journal) error {
	return r.db.Create(journal).Error
}

// GetPurchaseItemByID gets purchase item by ID
func (r *PurchaseRepository) GetPurchaseItemByID(id uint) (*models.PurchaseItem, error) {
	var item models.PurchaseItem
	err := r.db.Preload("Product").
		Preload("ExpenseAccount").
		First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateReceipt updates receipt
func (r *PurchaseRepository) UpdateReceipt(receipt *models.PurchaseReceipt) (*models.PurchaseReceipt, error) {
	err := r.db.Save(receipt).Error
	if err != nil {
		return nil, err
	}
	return r.FindReceiptByID(receipt.ID)
}

// CountReceiptsByMonth counts receipts by month
func (r *PurchaseRepository) CountReceiptsByMonth(year, month int) (int64, error) {
	var count int64
	err := r.db.Model(&models.PurchaseReceipt{}).
		Where("EXTRACT(year FROM received_date) = ? AND EXTRACT(month FROM received_date) = ?",
			year, month).
		Count(&count).Error
	return count, err
}

// CountJournalsByMonth counts journals by month
func (r *PurchaseRepository) CountJournalsByMonth(year, month int) (int64, error) {
	var count int64
	err := r.db.Model(&models.Journal{}).
		Where("reference_type = ? AND EXTRACT(year FROM date) = ? AND EXTRACT(month FROM date) = ?",
			models.JournalRefTypePurchase, year, month).
		Count(&count).Error
	return count, err
}

// GetPurchaseSummary gets purchase summary
func (r *PurchaseRepository) GetPurchaseSummary(startDate, endDate string) (*models.PurchaseSummary, error) {
	return r.GetPurchasesSummary(models.PurchaseFilter{
		StartDate: startDate,
		EndDate:   endDate,
	})
}

// GetPayablesReport gets payables report
func (r *PurchaseRepository) GetPayablesReport() (*models.PayablesReportResponse, error) {
	// This is a placeholder implementation - you may need to adjust based on your models
	var payables []models.PayablesReportData

	err := r.db.Model(&models.Purchase{}).
		Select(`
			purchases.id as purchase_id,
			purchases.code as purchase_code,
			contacts.name as vendor_name,
			purchases.date,
			purchases.due_date,
			purchases.total_amount,
			purchases.paid_amount,
			purchases.outstanding_amount,
			purchases.status
		`).
		Joins("JOIN contacts ON contacts.id = purchases.vendor_id").
		Where("purchases.outstanding_amount > 0 AND purchases.status IN (?)",
			[]string{models.PurchaseStatusApproved, models.PurchaseStatusCompleted}).
		Order("purchases.due_date ASC").
		Scan(&payables).Error

	if err != nil {
		return nil, err
	}

	// Calculate totals
	var totalOutstanding float64
	for _, item := range payables {
		totalOutstanding += item.OutstandingAmount
	}

	return &models.PayablesReportResponse{
		TotalOutstanding: totalOutstanding,
		Payables:         payables,
	}, nil
}
