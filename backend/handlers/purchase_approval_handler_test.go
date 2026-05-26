package handlers

import (
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
)

func TestPurchaseFinalApprovalQueueIsFinanceManagerOnly(t *testing.T) {
	handler := &PurchaseApprovalHandler{}
	purchases := []models.Purchase{
		{
			ID:             1,
			Code:           "PO-FINAL-QUEUE",
			Status:         models.PurchaseStatusPendingFinalApproval,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    100000,
		},
	}

	financeManagerItems := handler.filterPurchasesByApprovalAuthority(purchases, models.RoleFinanceManager, 10)
	require.Len(t, financeManagerItems, 1)
	require.Equal(t, "PO-FINAL-QUEUE", financeManagerItems[0].Code)

	for _, role := range []string{models.RoleFinanceStaff, models.RoleAccounting, models.RoleDirector, models.RoleAdmin, models.RoleEmployee} {
		t.Run(role, func(t *testing.T) {
			items := handler.filterPurchasesByApprovalAuthority(purchases, role, 10)
			require.Empty(t, items)
		})
	}
}
