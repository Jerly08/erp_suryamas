package services

import (
	"fmt"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// ensurePurchasePaymentLink keeps the operational payment and purchase detail
// in one auditable chain. It never changes the financial allocation; it only
// mirrors an existing allocation for display and payment-proof attachment.
func ensurePurchasePaymentLink(
	tx *gorm.DB,
	payment *models.Payment,
	purchaseID uint,
	fallbackAmount float64,
) (*models.PurchasePayment, error) {
	if tx == nil {
		return nil, fmt.Errorf("purchase payment link requires a database transaction")
	}
	if payment == nil || payment.ID == 0 || purchaseID == 0 {
		return nil, fmt.Errorf("purchase payment link requires a payment and purchase")
	}

	if !tx.Migrator().HasTable(&models.PurchasePayment{}) {
		return nil, fmt.Errorf("purchase payment tracking is not available")
	}

	var allocationTotal float64
	if err := tx.Model(&models.PaymentAllocation{}).
		Where("payment_id = ? AND bill_id = ?", payment.ID, purchaseID).
		Select("COALESCE(SUM(allocated_amount), 0)").
		Scan(&allocationTotal).Error; err != nil {
		return nil, err
	}
	if allocationTotal <= 0 {
		allocationTotal = fallbackAmount
	}
	if allocationTotal <= 0 {
		return nil, fmt.Errorf("purchase payment allocation amount is required")
	}

	// A link can only name one term. Keep it null for legitimate multi-term
	// allocations instead of assigning the proof to the wrong term.
	var termIDs []uint
	if err := tx.Model(&models.PaymentAllocation{}).
		Where("payment_id = ? AND bill_id = ? AND purchase_payment_term_id IS NOT NULL", payment.ID, purchaseID).
		Distinct("purchase_payment_term_id").
		Pluck("purchase_payment_term_id", &termIDs).Error; err != nil {
		return nil, err
	}
	var paymentTermID *uint
	if len(termIDs) == 1 {
		paymentTermID = &termIDs[0]
	}

	var link models.PurchasePayment
	err := tx.Where("purchase_id = ? AND payment_id = ? AND deleted_at IS NULL", purchaseID, payment.ID).
		First(&link).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	linkValues := map[string]interface{}{
		"payment_number":           payment.Code,
		"date":                     payment.Date,
		"amount":                   allocationTotal,
		"method":                   payment.Method,
		"reference":                payment.Reference,
		"notes":                    payment.Notes,
		"cash_bank_id":             payment.CashBankID,
		"user_id":                  payment.UserID,
		"payment_id":               payment.ID,
		"purchase_payment_term_id": paymentTermID,
	}

	if link.ID != 0 {
		if err := tx.Model(&link).Updates(linkValues).Error; err != nil {
			return nil, err
		}
		if err := tx.First(&link, link.ID).Error; err != nil {
			return nil, err
		}
		return &link, nil
	}

	paymentID := payment.ID
	link = models.PurchasePayment{
		PurchaseID:            purchaseID,
		PaymentNumber:         payment.Code,
		Date:                  payment.Date,
		Amount:                allocationTotal,
		Method:                payment.Method,
		Reference:             payment.Reference,
		Notes:                 payment.Notes,
		CashBankID:            payment.CashBankID,
		UserID:                payment.UserID,
		PaymentID:             &paymentID,
		PurchasePaymentTermID: paymentTermID,
	}
	if err := tx.Create(&link).Error; err != nil {
		return nil, err
	}

	return &link, nil
}
