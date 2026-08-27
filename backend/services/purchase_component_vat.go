package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	purchaseComponentVATDefaultRate = 12.0
	purchaseVATVarianceTolerance    = 1.0 // IDR rounding tolerance
)

func normalizePurchaseVATTreatment(value string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", models.PurchaseVATTreatmentStandard:
		return models.PurchaseVATTreatmentStandard, nil
	case models.PurchaseVATTreatmentNone:
		return models.PurchaseVATTreatmentNone, nil
	case models.PurchaseVATTreatmentComponentDPPNilaiLain:
		return models.PurchaseVATTreatmentComponentDPPNilaiLain, nil
	default:
		return "", fmt.Errorf("perlakuan PPN purchase tidak dikenal: %s", value)
	}
}

func isComponentPurchaseVAT(purchase *models.Purchase) bool {
	return purchase != nil && strings.EqualFold(
		strings.TrimSpace(purchase.VATTreatment),
		models.PurchaseVATTreatmentComponentDPPNilaiLain,
	)
}

func roundPurchaseVATIDR(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	return math.Round(value)
}

func purchaseItemNetAmountBeforeVAT(purchase *models.Purchase, item *models.PurchaseItem) float64 {
	if item == nil {
		return 0
	}
	orderDiscountRate := 0.0
	if purchase != nil {
		orderDiscountRate = clampPercent(purchase.Discount) / 100
	}
	return roundPurchaseMoney(clampNonNegative(item.TotalPrice * (1 - orderDiscountRate)))
}

func purchaseItemForTaxAllocation(
	purchase *models.Purchase,
	allocation models.PurchaseTaxAllocation,
) (*models.PurchaseItem, error) {
	if purchase == nil {
		return nil, fmt.Errorf("purchase wajib diisi")
	}

	if allocation.PurchaseItemID != 0 {
		for index := range purchase.PurchaseItems {
			if purchase.PurchaseItems[index].ID == allocation.PurchaseItemID {
				return &purchase.PurchaseItems[index], nil
			}
		}
		return nil, fmt.Errorf("item dasar PPN %d tidak ditemukan pada purchase", allocation.PurchaseItemID)
	}

	if allocation.PurchaseItemIndex == nil {
		return nil, fmt.Errorf("item dasar PPN wajib dipilih")
	}
	index := *allocation.PurchaseItemIndex
	if index < 0 || index >= len(purchase.PurchaseItems) {
		return nil, fmt.Errorf("urutan item dasar PPN tidak valid")
	}
	return &purchase.PurchaseItems[index], nil
}

func taxAllocationReferenceLabel(index int) string {
	return fmt.Sprintf("detail PPN %d", index+1)
}

// normalizePurchaseTaxInvoiceReference gives allocation rows and uploaded
// evidence a stable comparison key without changing their human-facing format.
func normalizePurchaseTaxInvoiceReference(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// buildPurchaseTaxAllocations turns the user input into server-authoritative
// allocation data. Source values are never trusted from the browser. For the
// standard DPP Nilai Lain mechanism the server proposes 11/12 of the selected
// source, while invoice figures remain the final amount when an explained
// variance is supplied.
func buildPurchaseTaxAllocations(
	purchase *models.Purchase,
	requests []models.PurchaseTaxAllocationRequest,
) ([]models.PurchaseTaxAllocation, error) {
	if purchase == nil {
		return nil, fmt.Errorf("purchase wajib diisi")
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("minimal satu detail PPN berdasarkan komponen wajib diisi")
	}

	allocations := make([]models.PurchaseTaxAllocation, 0, len(requests))
	dppByItem := make(map[string]float64)
	sourceByItem := make(map[string]float64)
	for index, request := range requests {
		label := taxAllocationReferenceLabel(index)
		allocation := models.PurchaseTaxAllocation{
			TaxTreatment:      strings.ToUpper(strings.TrimSpace(request.TaxTreatment)),
			PurchaseItemIndex: request.PurchaseItemIndex,
			TaxInvoiceNumber:  normalizePurchaseTaxInvoiceReference(request.TaxInvoiceNumber),
			TaxInvoiceDate:    request.TaxInvoiceDate,
			Notes:             strings.TrimSpace(request.Notes),
		}
		if request.ID != nil {
			allocation.ID = *request.ID
		}
		if request.PurchaseItemID != nil {
			allocation.PurchaseItemID = *request.PurchaseItemID
		}
		if allocation.TaxTreatment == "" {
			allocation.TaxTreatment = models.PurchaseTaxTreatmentDPPNilaiLain
		}
		if allocation.TaxTreatment != models.PurchaseTaxTreatmentDPPNilaiLain {
			return nil, fmt.Errorf("%s memiliki perlakuan PPN yang belum didukung", label)
		}

		item, err := purchaseItemForTaxAllocation(purchase, allocation)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		if allocation.PurchaseItemID == 0 {
			allocation.PurchaseItemID = item.ID
		}
		allocation.SourceAmount = purchaseItemNetAmountBeforeVAT(purchase, item)
		if allocation.SourceAmount <= 0 {
			return nil, fmt.Errorf("%s: nilai item dasar PPN harus lebih dari nol", label)
		}

		rate := purchaseComponentVATDefaultRate
		if request.PPNRate != nil {
			rate = *request.PPNRate
		}
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || rate > 100 {
			return nil, fmt.Errorf("%s: tarif PPN harus antara 0 dan 100", label)
		}
		allocation.PPNRate = rate

		expectedDPP := roundPurchaseVATIDR(allocation.SourceAmount * 11 / 12)
		allocation.DPPAmount = expectedDPP
		if request.DPPAmount != nil {
			allocation.DPPAmount = roundPurchaseMoney(clampNonNegative(*request.DPPAmount))
		}
		if allocation.DPPAmount <= 0 {
			return nil, fmt.Errorf("%s: DPP harus lebih dari nol", label)
		}
		if allocation.DPPAmount > allocation.SourceAmount+purchaseVATVarianceTolerance {
			return nil, fmt.Errorf("%s: DPP tidak boleh melebihi nilai item dasar PPN", label)
		}

		expectedPPN := roundPurchaseVATIDR(allocation.DPPAmount * allocation.PPNRate / 100)
		allocation.PPNAmount = expectedPPN
		if request.PPNAmount != nil {
			allocation.PPNAmount = roundPurchaseMoney(clampNonNegative(*request.PPNAmount))
		}
		if allocation.PPNAmount < 0 {
			return nil, fmt.Errorf("%s: nilai PPN tidak valid", label)
		}

		dppVariance := math.Abs(allocation.DPPAmount - expectedDPP)
		ppnVariance := math.Abs(allocation.PPNAmount - expectedPPN)
		if (dppVariance > purchaseVATVarianceTolerance || ppnVariance > purchaseVATVarianceTolerance) && allocation.Notes == "" {
			return nil, fmt.Errorf("%s: isi catatan apabila DPP atau PPN faktur berbeda dari perhitungan sistem", label)
		}
		if allocation.TaxInvoiceNumber == "" {
			return nil, fmt.Errorf("%s: nomor faktur pajak wajib diisi", label)
		}
		if allocation.TaxInvoiceDate == nil || allocation.TaxInvoiceDate.IsZero() {
			return nil, fmt.Errorf("%s: tanggal faktur pajak wajib diisi", label)
		}
		// Conservative default: an omitted creditability flag cannot create an
		// input-VAT claim. The UI always submits this value explicitly.
		allocation.IsCreditable = request.IsCreditable != nil && *request.IsCreditable
		basisKey := fmt.Sprintf("item:%d", allocation.PurchaseItemID)
		if allocation.PurchaseItemID == 0 && allocation.PurchaseItemIndex != nil {
			basisKey = fmt.Sprintf("index:%d", *allocation.PurchaseItemIndex)
		}
		dppByItem[basisKey] += allocation.DPPAmount
		sourceByItem[basisKey] = allocation.SourceAmount
		if dppByItem[basisKey] > sourceByItem[basisKey]+purchaseVATVarianceTolerance {
			return nil, fmt.Errorf("%s: akumulasi DPP untuk item dasar PPN melebihi nilai item", label)
		}
		allocations = append(allocations, allocation)
	}

	return allocations, nil
}

func applyPurchaseVATTreatment(purchase *models.Purchase) error {
	if purchase == nil {
		return fmt.Errorf("purchase wajib diisi")
	}

	treatment, err := normalizePurchaseVATTreatment(purchase.VATTreatment)
	if err != nil {
		return err
	}
	purchase.VATTreatment = treatment

	switch treatment {
	case models.PurchaseVATTreatmentStandard:
		if len(purchase.TaxAllocations) > 0 {
			return fmt.Errorf("detail PPN berdasarkan komponen hanya dapat digunakan pada perlakuan PPN khusus")
		}
		return nil
	case models.PurchaseVATTreatmentNone:
		if len(purchase.TaxAllocations) > 0 {
			return fmt.Errorf("perlakuan tanpa PPN tidak dapat memiliki detail PPN")
		}
		purchase.PPNRate = 0
		purchase.PPNAmount = 0
		return nil
	case models.PurchaseVATTreatmentComponentDPPNilaiLain:
		if len(purchase.TaxAllocations) == 0 {
			return fmt.Errorf("detail PPN berdasarkan komponen wajib diisi")
		}

		totalPPN := 0.0
		dppByItem := make(map[uint]float64, len(purchase.TaxAllocations))
		sourceByItem := make(map[uint]float64, len(purchase.TaxAllocations))
		for index := range purchase.TaxAllocations {
			allocation := &purchase.TaxAllocations[index]
			item, err := purchaseItemForTaxAllocation(purchase, *allocation)
			if err != nil {
				return fmt.Errorf("%s: %w", taxAllocationReferenceLabel(index), err)
			}
			allocation.PurchaseItemID = item.ID
			allocation.SourceAmount = purchaseItemNetAmountBeforeVAT(purchase, item)
			if allocation.SourceAmount <= 0 || allocation.DPPAmount <= 0 || allocation.PPNAmount < 0 ||
				allocation.PPNRate <= 0 || allocation.PPNRate > 100 {
				return fmt.Errorf("%s: data DPP/PPN tidak valid", taxAllocationReferenceLabel(index))
			}
			if allocation.DPPAmount > allocation.SourceAmount+purchaseVATVarianceTolerance {
				return fmt.Errorf("%s: DPP tidak boleh melebihi nilai item dasar PPN", taxAllocationReferenceLabel(index))
			}
			if strings.TrimSpace(allocation.TaxInvoiceNumber) == "" ||
				allocation.TaxInvoiceDate == nil || allocation.TaxInvoiceDate.IsZero() {
				return fmt.Errorf("%s: nomor dan tanggal faktur pajak wajib diisi", taxAllocationReferenceLabel(index))
			}
			dppByItem[item.ID] += allocation.DPPAmount
			sourceByItem[item.ID] = allocation.SourceAmount
			if dppByItem[item.ID] > sourceByItem[item.ID]+purchaseVATVarianceTolerance {
				return fmt.Errorf("%s: akumulasi DPP untuk item dasar PPN melebihi nilai item", taxAllocationReferenceLabel(index))
			}
			totalPPN += allocation.PPNAmount
		}

		// A header rate would be misleading when multiple component allocations
		// can have different tax rates. The allocation rows are the source of truth.
		purchase.PPNRate = 0
		purchase.PPNAmount = roundPurchaseMoney(clampNonNegative(totalPPN))
		return nil
	}

	return nil
}

func purchaseCreditableVATAmount(purchase *models.Purchase) float64 {
	if purchase == nil {
		return 0
	}
	if !isComponentPurchaseVAT(purchase) {
		return clampNonNegative(purchase.PPNAmount)
	}

	total := 0.0
	for _, allocation := range purchase.TaxAllocations {
		if allocation.IsCreditable {
			total += allocation.PPNAmount
		}
	}
	return roundPurchaseMoney(clampNonNegative(total))
}

func purchaseNonCreditableVATByItem(purchase *models.Purchase) map[uint]float64 {
	allocations := make(map[uint]float64)
	if purchase == nil || !isComponentPurchaseVAT(purchase) {
		return allocations
	}
	for _, allocation := range purchase.TaxAllocations {
		if allocation.IsCreditable || allocation.PurchaseItemID == 0 || allocation.PPNAmount <= 0 {
			continue
		}
		allocations[allocation.PurchaseItemID] += allocation.PPNAmount
	}
	for itemID, amount := range allocations {
		allocations[itemID] = roundPurchaseMoney(clampNonNegative(amount))
	}
	return allocations
}

func resolvePurchaseTaxAllocationsForPersistence(
	purchase *models.Purchase,
	allocations []models.PurchaseTaxAllocation,
) ([]models.PurchaseTaxAllocation, error) {
	if purchase == nil || purchase.ID == 0 {
		return nil, fmt.Errorf("purchase wajib disimpan sebelum detail PPN dapat dibuat")
	}

	resolved := make([]models.PurchaseTaxAllocation, 0, len(allocations))
	for index, allocation := range allocations {
		item, err := purchaseItemForTaxAllocation(purchase, allocation)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", taxAllocationReferenceLabel(index), err)
		}
		allocation.ID = 0 // replace is append-only from the current draft state
		allocation.PurchaseID = purchase.ID
		allocation.PurchaseItemID = item.ID
		allocation.PurchaseItemIndex = nil
		allocation.Purchase = models.Purchase{}
		allocation.PurchaseItem = models.PurchaseItem{}
		allocation.Verifier = nil
		allocation.VerifiedBy = nil
		allocation.VerifiedAt = nil
		allocation.SourceAmount = purchaseItemNetAmountBeforeVAT(purchase, item)
		allocation.CreatedAt = time.Time{}
		allocation.UpdatedAt = time.Time{}
		allocation.DeletedAt = gorm.DeletedAt{}
		resolved = append(resolved, allocation)
	}
	return resolved, nil
}

// syncPurchaseTaxAllocations replaces only the active VAT allocation rows for
// a draft/pre-posting purchase. Header totals are calculated before this call,
// so the save never reinterprets a user-provided global PPN rate.
func (s *PurchaseService) syncPurchaseTaxAllocations(
	purchase *models.Purchase,
	allocations []models.PurchaseTaxAllocation,
) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.syncPurchaseTaxAllocationsInTransaction(tx, purchase, allocations)
	})
}

// syncPurchaseTaxAllocationsInTransaction is the transactional variant used by
// the Finance revision workflow. Keeping the header, its replacement items,
// and its component VAT rows in the same transaction prevents a partially
// saved Purchase from ever being approved or audited with mismatched VAT.
func (s *PurchaseService) syncPurchaseTaxAllocationsInTransaction(
	tx *gorm.DB,
	purchase *models.Purchase,
	allocations []models.PurchaseTaxAllocation,
) error {
	if purchase == nil {
		return fmt.Errorf("purchase wajib diisi")
	}
	if tx == nil {
		return fmt.Errorf("transaksi database untuk detail PPN tidak tersedia")
	}
	if !tx.Migrator().HasTable(&models.PurchaseTaxAllocation{}) {
		if isComponentPurchaseVAT(purchase) {
			return fmt.Errorf("migrasi detail PPN purchase belum tersedia")
		}
		return nil
	}

	if !isComponentPurchaseVAT(purchase) {
		allocations = nil
	}
	if isComponentPurchaseVAT(purchase) && len(allocations) == 0 {
		return fmt.Errorf("detail PPN berdasarkan komponen wajib diisi")
	}

	resolved, err := resolvePurchaseTaxAllocationsForPersistence(purchase, allocations)
	if err != nil {
		return err
	}

	if err := tx.Where("purchase_id = ?", purchase.ID).Delete(&models.PurchaseTaxAllocation{}).Error; err != nil {
		return fmt.Errorf("gagal menghapus detail PPN sebelumnya: %w", err)
	}
	if len(resolved) == 0 {
		return nil
	}
	if err := tx.Create(&resolved).Error; err != nil {
		return fmt.Errorf("gagal menyimpan detail PPN purchase: %w", err)
	}
	return nil
}

func (s *PurchaseService) validatePurchaseComponentVATDocuments(purchase *models.Purchase) error {
	if !isComponentPurchaseVAT(purchase) {
		return nil
	}
	if len(purchase.TaxAllocations) == 0 {
		return fmt.Errorf("detail PPN berdasarkan komponen wajib diisi")
	}

	invoiceNumbers := make(map[string]struct{}, len(purchase.TaxAllocations))
	for index, allocation := range purchase.TaxAllocations {
		number := normalizePurchaseTaxInvoiceReference(allocation.TaxInvoiceNumber)
		if number == "" || allocation.TaxInvoiceDate == nil || allocation.TaxInvoiceDate.IsZero() {
			return fmt.Errorf("%s belum memiliki nomor atau tanggal faktur pajak", taxAllocationReferenceLabel(index))
		}
		invoiceNumbers[number] = struct{}{}
	}

	documents, err := s.purchaseRepo.FindDocumentsByPurchaseID(purchase.ID)
	if err != nil {
		return fmt.Errorf("gagal memeriksa lampiran faktur pajak: %w", err)
	}
	taxInvoiceDocumentReferences := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		if strings.EqualFold(strings.TrimSpace(document.DocumentType), models.PurchaseDocumentTaxInvoice) {
			if reference := normalizePurchaseTaxInvoiceReference(document.ReferenceNumber); reference != "" {
				taxInvoiceDocumentReferences[reference] = struct{}{}
			}
		}
	}
	missingReferences := make([]string, 0)
	for invoiceNumber := range invoiceNumbers {
		if _, exists := taxInvoiceDocumentReferences[invoiceNumber]; !exists {
			missingReferences = append(missingReferences, invoiceNumber)
		}
	}
	if len(missingReferences) > 0 {
		return fmt.Errorf(
			"upload faktur pajak yang terhubung untuk nomor: %s sebelum purchase dapat diteruskan",
			strings.Join(missingReferences, ", "),
		)
	}
	return nil
}

func (s *PurchaseService) verifyPurchaseTaxAllocations(
	tx *gorm.DB,
	purchase *models.Purchase,
	userID uint,
) error {
	if purchase == nil || !isComponentPurchaseVAT(purchase) {
		return nil
	}
	if err := s.validatePurchaseComponentVATDocuments(purchase); err != nil {
		return err
	}
	now := time.Now()
	return tx.Model(&models.PurchaseTaxAllocation{}).
		Where("purchase_id = ? AND deleted_at IS NULL", purchase.ID).
		Updates(map[string]interface{}{
			"verified_by": userID,
			"verified_at": now,
			"updated_at":  now,
		}).Error
}
