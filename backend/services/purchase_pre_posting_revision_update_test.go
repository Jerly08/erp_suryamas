package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
)

func openPurchaseForScopedPrePostingRevision(
	t *testing.T,
	fixture *purchaseWorkflowFixture,
	purchaseID uint,
	revisionTypes []string,
) *models.PurchasePrePostingRevision {
	t.Helper()
	require.NoError(t, fixture.db.Model(&models.Purchase{}).
		Where("id = ?", purchaseID).
		Updates(map[string]interface{}{
			"status":          models.PurchaseStatusPending,
			"approval_status": models.PurchaseApprovalNotStarted,
		}).Error)

	revision, err := fixture.service.RequestPurchasePrePostingRevision(
		purchaseID,
		models.PurchasePrePostingRevisionCreateRequest{
			RevisionTypes: revisionTypes,
			Reason:        "Perlu koreksi data purchase sebelum posting",
		},
		fixture.financeStaff.ID,
		fixture.financeStaff.Role,
	)
	require.NoError(t, err)
	require.Equal(t, models.PurchasePrePostingRevisionStatusOpenForFinance, revision.Status)
	return revision
}

func TestPurchasePrePostingRevisionLegacyVendorProjectAuditIsAppendOnly(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, originalItem := fixture.createDraftPurchase(t, "PO-REV-AUDIT", 125000)
	revision := openPurchaseForScopedPrePostingRevision(
		t,
		fixture,
		purchase.ID,
		[]string{"VENDOR_OR_PROJECT"},
	)

	newVendor := models.Contact{
		Code:     "VEND-REV-AUDIT",
		Name:     "Vendor Revisi Audit",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	projectA := models.Project{
		Code:   "PRJ-REV-A",
		Name:   "Proyek Revisi A",
		Status: models.ProjectStatusActive,
	}
	projectB := models.Project{
		Code:   "PRJ-REV-B",
		Name:   "Proyek Revisi B",
		Status: models.ProjectStatusActive,
	}
	require.NoError(t, fixture.db.Create(&newVendor).Error)
	require.NoError(t, fixture.db.Create(&projectA).Error)
	require.NoError(t, fixture.db.Create(&projectB).Error)

	updated, err := fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{
			VendorID:  &newVendor.ID,
			ProjectID: &projectA.ID,
		},
		fixture.financeStaff.ID,
	)
	require.NoError(t, err)
	require.Equal(t, newVendor.ID, updated.VendorID)
	require.NotNil(t, updated.ProjectID)
	require.Equal(t, projectA.ID, *updated.ProjectID)

	updated, err = fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{ProjectID: &projectB.ID},
		fixture.financeStaff.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, updated.ProjectID)
	require.Equal(t, projectB.ID, *updated.ProjectID)

	var activeItems []models.PurchaseItem
	require.NoError(t, fixture.db.Where("purchase_id = ?", purchase.ID).Order("id ASC").Find(&activeItems).Error)
	require.Len(t, activeItems, 1)
	require.Equal(t, originalItem.ID, activeItems[0].ID, "sparse header updates must not replace items")

	var changes []models.PurchasePrePostingRevisionChange
	require.NoError(t, fixture.db.
		Where("purchase_pre_posting_revision_id = ?", revision.ID).
		Order("id ASC").
		Find(&changes).Error)
	require.Len(t, changes, 2, "each save must append a separate audit event")
	require.Equal(t, fixture.financeStaff.ID, changes[0].ChangedBy)
	require.Contains(t, changes[0].ChangedFields, "vendor")
	require.Contains(t, changes[0].ChangedFields, "project")
	require.Contains(t, changes[0].BeforeSnapshot, fixture.vendor.Name)
	require.Contains(t, changes[0].AfterSnapshot, newVendor.Name)
	require.Contains(t, changes[0].AfterSnapshot, projectA.Code)
	require.Contains(t, changes[1].BeforeSnapshot, projectA.Code)
	require.Contains(t, changes[1].AfterSnapshot, projectB.Code)
}

func TestPurchasePrePostingRevisionSeparatesVendorAndProjectScopes(t *testing.T) {
	t.Run("vendor does not authorize project", func(t *testing.T) {
		fixture := newPurchaseWorkflowFixture(t)
		purchase, _ := fixture.createDraftPurchase(t, "PO-REV-VENDOR-SCOPE", 125000)
		openPurchaseForScopedPrePostingRevision(t, fixture, purchase.ID, []string{"VENDOR"})

		newVendor := models.Contact{
			Code:     "VEND-REV-SCOPE",
			Name:     "Vendor Scope",
			Type:     models.ContactTypeVendor,
			IsActive: true,
		}
		project := models.Project{Code: "PRJ-REV-SCOPE", Name: "Proyek Scope", Status: models.ProjectStatusActive}
		require.NoError(t, fixture.db.Create(&newVendor).Error)
		require.NoError(t, fixture.db.Create(&project).Error)

		_, err := fixture.service.UpdatePurchase(
			purchase.ID,
			models.PurchaseUpdateRequest{VendorID: &newVendor.ID},
			fixture.financeStaff.ID,
		)
		require.NoError(t, err)

		_, err = fixture.service.UpdatePurchase(
			purchase.ID,
			models.PurchaseUpdateRequest{ProjectID: &project.ID},
			fixture.financeStaff.ID,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proyek")
	})

	t.Run("standalone project can be cleared but linked project is read only", func(t *testing.T) {
		fixture := newPurchaseWorkflowFixture(t)
		projectA := models.Project{Code: "PRJ-REV-CLEAR-A", Name: "Proyek Clear A", Status: models.ProjectStatusActive}
		projectB := models.Project{Code: "PRJ-REV-CLEAR-B", Name: "Proyek Clear B", Status: models.ProjectStatusActive}
		require.NoError(t, fixture.db.Create(&projectA).Error)
		require.NoError(t, fixture.db.Create(&projectB).Error)

		standalone, _ := fixture.createDraftPurchase(t, "PO-REV-CLEAR", 125000)
		require.NoError(t, fixture.db.Model(&models.Purchase{}).
			Where("id = ?", standalone.ID).
			Update("project_id", projectA.ID).Error)
		openPurchaseForScopedPrePostingRevision(t, fixture, standalone.ID, []string{"PROJECT"})

		cleared, err := fixture.service.UpdatePurchase(
			standalone.ID,
			models.PurchaseUpdateRequest{ClearProject: true},
			fixture.financeStaff.ID,
		)
		require.NoError(t, err)
		require.Nil(t, cleared.ProjectID)

		linked, _ := fixture.createDraftPurchase(t, "PO-REV-LINKED-PROJECT", 125000)
		purchaseRequest := models.PurchaseRequest{
			PRNumber:    "PR-REV-LINKED-PROJECT",
			RequesterID: fixture.staff.ID,
			ProjectID:   &projectA.ID,
			Status:      models.PRStatusApproved,
		}
		require.NoError(t, fixture.db.Create(&purchaseRequest).Error)
		require.NoError(t, fixture.db.Model(&models.Purchase{}).
			Where("id = ?", linked.ID).
			Updates(map[string]interface{}{
				"purchase_request_id": purchaseRequest.ID,
				"project_id":          projectA.ID,
			}).Error)
		openPurchaseForScopedPrePostingRevision(t, fixture, linked.ID, []string{"PROJECT"})

		_, err = fixture.service.UpdatePurchase(
			linked.ID,
			models.PurchaseUpdateRequest{ProjectID: &projectB.ID},
			fixture.financeStaff.ID,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "terhubung ke PR")
	})
}

func TestPurchasePrePostingRevisionRejectsFieldsOutsideApprovedScope(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, _ := fixture.createDraftPurchase(t, "PO-REV-SCOPE-BYPASS", 125000)
	openPurchaseForScopedPrePostingRevision(t, fixture, purchase.ID, []string{"VENDOR"})

	notes := "Mencoba mengubah catatan di luar jenis revisi vendor"
	_, err := fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{Notes: &notes},
		fixture.financeStaff.ID,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "catatan lain")

	ppnRate := 11.0
	ppnRatePointer := &ppnRate
	_, err = fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{PPNRate: &ppnRatePointer},
		fixture.financeStaff.ID,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pajak atau biaya")
}

func TestPurchasePrePostingRevisionTreatsTermRecalculationAsTaxCostConsequence(t *testing.T) {
	vatTreatment := models.PurchaseVATTreatmentComponentDPPNilaiLain
	itemID := uint(1)
	dppAmount := 28695.0
	ppnRate := 12.0
	ppnAmount := 3443.0
	isCreditable := true
	request := models.PurchaseUpdateRequest{
		VATTreatment: &vatTreatment,
		TaxAllocations: []models.PurchaseTaxAllocationRequest{{
			PurchaseItemID:   &itemID,
			DPPAmount:        &dppAmount,
			PPNRate:          &ppnRate,
			PPNAmount:        &ppnAmount,
			IsCreditable:     &isCreditable,
			TaxInvoiceNumber: "010.001-26.00000001",
		}},
		PaymentTerms: []models.PurchasePaymentTermRequest{{
			Sequence: 1,
			Amount:   660819,
		}},
	}

	scope := purchasePrePostingRevisionScopeFromRequest(request)
	require.True(t, scope.TaxAndCost)
	require.False(t, scope.PaymentConfiguration)

	revision := &models.PurchasePrePostingRevision{
		RevisionTypes: `["TAX_AND_COST"]`,
	}
	changed, err := validatePurchasePrePostingRevisionUpdateScope(revision, request)
	require.NoError(t, err)
	require.Equal(t, []string{"tax_and_cost"}, changed)

	standaloneTermRequest := models.PurchaseUpdateRequest{
		PaymentTerms: request.PaymentTerms,
	}
	standaloneScope := purchasePrePostingRevisionScopeFromRequest(standaloneTermRequest)
	require.False(t, standaloneScope.TaxAndCost)
	require.True(t, standaloneScope.PaymentConfiguration)
}

func TestPurchasePendingFinanceRevisionWithoutOpenRevisionRejectsUpdate(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, _ := fixture.createDraftPurchase(t, "PO-REV-NO-ACTIVE-REVISION", 125000)
	require.NoError(t, fixture.db.Model(&models.Purchase{}).
		Where("id = ?", purchase.ID).
		Update("status", models.PurchaseStatusPendingFinanceRevision).Error)

	notes := "Tidak boleh lolos tanpa revisi Finance Staff yang aktif"
	_, err := fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{Notes: &notes},
		fixture.financeStaff.ID,
	)
	require.EqualError(t, err, "purchase tidak memiliki revisi Finance Staff aktif")
}

func TestPurchasePrePostingRevisionItemReplacementReleasesRemovedPRReservations(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, _ := fixture.createDraftPurchase(t, "PO-REV-RELEASE-PR-LINK", 125000)
	purchaseRequest := models.PurchaseRequest{
		PRNumber:    "PR-REV-RELEASE-PR-LINK",
		RequesterID: fixture.staff.ID,
		Status:      models.PRStatusApproved,
	}
	require.NoError(t, fixture.db.Create(&purchaseRequest).Error)

	now := time.Now().UTC()
	requestItem := models.PurchaseRequestItem{
		PurchaseRequestID:   purchaseRequest.ID,
		ItemName:            "Item PR yang dilepas",
		Quantity:            1,
		EstimatedPrice:      125000,
		TotalEstimatedPrice: 125000,
		ApprovalStatus:      models.PRItemDecisionApproved,
		PurchaseID:          &purchase.ID,
		PurchasedAt:         &now,
	}
	require.NoError(t, fixture.db.Create(&requestItem).Error)

	// A replacement that contains no PurchaseRequestItemID must release an
	// existing reservation instead of leaving the PR line permanently linked.
	purchase.PurchaseRequestID = &purchaseRequest.ID
	purchase.PurchaseItems = nil
	require.NoError(t, fixture.service.syncLinkedPurchaseRequestItems(purchase))

	var savedRequestItem models.PurchaseRequestItem
	require.NoError(t, fixture.db.First(&savedRequestItem, requestItem.ID).Error)
	require.Nil(t, savedRequestItem.PurchaseID)
	require.Nil(t, savedRequestItem.PurchasedAt)
}

func TestPurchasePrePostingRevisionVendorChangeRequiresProofConfirmation(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, _ := fixture.createDraftPurchase(t, "PO-REV-PROOF", 125000)
	openPurchaseForScopedPrePostingRevision(t, fixture, purchase.ID, []string{"VENDOR"})

	now := time.Now().UTC()
	provisionalPayment := models.PurchasePayment{
		PurchaseID:               purchase.ID,
		UserID:                   fixture.financeStaff.ID,
		PaymentProofFileName:     "bukti-transfer.pdf",
		PaymentProofFilePath:     "uploads/purchases/bukti-transfer.pdf",
		PaymentProofUploadedAt:   &now,
		PaymentProofReviewStatus: models.PurchasePaymentProofPending,
	}
	newVendor := models.Contact{
		Code:     "VEND-REV-PROOF",
		Name:     "Vendor Dengan Bukti Provisional",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, fixture.db.Create(&provisionalPayment).Error)
	require.NoError(t, fixture.db.Create(&newVendor).Error)

	_, err := fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{VendorID: &newVendor.ID},
		fixture.financeStaff.ID,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "confirm_provisional_payment_proof=true")

	updated, err := fixture.service.UpdatePurchase(
		purchase.ID,
		models.PurchaseUpdateRequest{
			VendorID:                       &newVendor.ID,
			ConfirmProvisionalPaymentProof: true,
		},
		fixture.financeStaff.ID,
	)
	require.NoError(t, err)
	require.Equal(t, newVendor.ID, updated.VendorID)
}

func TestPurchasePrePostingRevisionRejectsInactiveVendorAndProject(t *testing.T) {
	t.Run("inactive or non-vendor contact", func(t *testing.T) {
		fixture := newPurchaseWorkflowFixture(t)
		purchase, _ := fixture.createDraftPurchase(t, "PO-REV-INACTIVE-VENDOR", 125000)
		openPurchaseForScopedPrePostingRevision(t, fixture, purchase.ID, []string{"VENDOR"})

		inactiveVendor := models.Contact{
			Code:     "VEND-REV-INACTIVE",
			Name:     "Vendor Tidak Aktif",
			Type:     models.ContactTypeVendor,
			IsActive: true,
		}
		customer := models.Contact{
			Code:     "CUST-REV-NONVENDOR",
			Name:     "Bukan Vendor",
			Type:     models.ContactTypeCustomer,
			IsActive: true,
		}
		require.NoError(t, fixture.db.Create(&inactiveVendor).Error)
		require.NoError(t, fixture.db.Model(&inactiveVendor).Update("is_active", false).Error)
		require.NoError(t, fixture.db.Create(&customer).Error)

		_, err := fixture.service.UpdatePurchase(
			purchase.ID,
			models.PurchaseUpdateRequest{VendorID: &inactiveVendor.ID},
			fixture.financeStaff.ID,
		)
		require.EqualError(t, err, "vendor yang dipilih tidak aktif")

		_, err = fixture.service.UpdatePurchase(
			purchase.ID,
			models.PurchaseUpdateRequest{VendorID: &customer.ID},
			fixture.financeStaff.ID,
		)
		require.EqualError(t, err, "kontak yang dipilih bukan vendor")
	})

	t.Run("inactive project", func(t *testing.T) {
		fixture := newPurchaseWorkflowFixture(t)
		purchase, _ := fixture.createDraftPurchase(t, "PO-REV-INACTIVE-PROJECT", 125000)
		openPurchaseForScopedPrePostingRevision(t, fixture, purchase.ID, []string{"PROJECT"})

		inactiveProject := models.Project{
			Code:   "PRJ-REV-INACTIVE",
			Name:   "Proyek Ditahan",
			Status: models.ProjectStatusOnHold,
		}
		require.NoError(t, fixture.db.Create(&inactiveProject).Error)

		_, err := fixture.service.UpdatePurchase(
			purchase.ID,
			models.PurchaseUpdateRequest{ProjectID: &inactiveProject.ID},
			fixture.financeStaff.ID,
		)
		require.EqualError(t, err, "proyek yang dipilih tidak aktif")
	})
}
