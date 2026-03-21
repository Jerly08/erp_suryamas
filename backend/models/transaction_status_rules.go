package models

import (
	"fmt"
	"strings"
)

const (
	ReportSettlementStatusUnpaid  = "UNPAID"
	ReportSettlementStatusPartial = "PARTIAL"
)

var (
	saleOperationalReportStatuses = []string{
		SaleStatusInvoiced,
		SaleStatusOverdue,
		SaleStatusPaid,
	}
	salePaymentEligibleStatuses = []string{
		SaleStatusInvoiced,
		SaleStatusOverdue,
	}
	purchaseOperationalReportStatuses = []string{
		PurchaseStatusApproved,
		PurchaseStatusCompleted,
		PurchaseStatusPaid,
	}
	purchasePaymentEligibleStatuses = []string{
		PurchaseStatusApproved,
		PurchaseStatusCompleted,
		PurchaseStatusPaid,
	}
	paymentEffectiveStatuses = []string{
		PaymentStatusCompleted,
	}
)

func SaleOperationalReportStatuses() []string {
	return cloneStatusSlice(saleOperationalReportStatuses)
}

func IsSaleOperationallyReportable(status string) bool {
	switch normalizeLifecycleStatus(status) {
	case SaleStatusInvoiced, SaleStatusOverdue, SaleStatusPaid:
		return true
	default:
		return false
	}
}

func IsSalePaymentEligible(status string) bool {
	switch normalizeLifecycleStatus(status) {
	case SaleStatusInvoiced, SaleStatusOverdue:
		return true
	default:
		return false
	}
}

func SalePaymentEligibleStatuses() []string {
	return cloneStatusSlice(salePaymentEligibleStatuses)
}

func SaleOperationalReportWhereClause(alias string) string {
	return fmt.Sprintf(
		"%s IN ('%s', '%s', '%s')",
		qualifiedColumn(alias, "status"),
		SaleStatusInvoiced,
		SaleStatusOverdue,
		SaleStatusPaid,
	)
}

func PurchaseOperationalReportStatuses() []string {
	return cloneStatusSlice(purchaseOperationalReportStatuses)
}

func IsPurchaseOperationallyReportable(status, approvalStatus string) bool {
	if normalizeLifecycleStatus(status) == PurchaseStatusCancelled {
		return false
	}

	if normalizeLifecycleStatus(approvalStatus) == PurchaseApprovalApproved {
		return true
	}

	switch normalizeLifecycleStatus(status) {
	case PurchaseStatusApproved, PurchaseStatusCompleted, PurchaseStatusPaid:
		return true
	default:
		return false
	}
}

func PurchaseOperationalReportWhereClause(alias string) string {
	return fmt.Sprintf(
		"(%s <> '%s' AND (%s IN ('%s', '%s', '%s') OR %s = '%s'))",
		qualifiedColumn(alias, "status"),
		PurchaseStatusCancelled,
		qualifiedColumn(alias, "status"),
		PurchaseStatusApproved,
		PurchaseStatusCompleted,
		PurchaseStatusPaid,
		qualifiedColumn(alias, "approval_status"),
		PurchaseApprovalApproved,
	)
}

func IsCreditPurchaseMethod(method string) bool {
	return normalizeLifecycleStatus(method) == PurchasePaymentCredit
}

func IsPurchasePaymentEligible(status, paymentMethod string, outstandingAmount float64) bool {
	if !IsCreditPurchaseMethod(paymentMethod) || outstandingAmount <= 0.01 {
		return false
	}

	switch normalizeLifecycleStatus(status) {
	case PurchaseStatusApproved, PurchaseStatusCompleted, PurchaseStatusPaid:
		return true
	default:
		return false
	}
}

func PurchasePaymentEligibleStatuses() []string {
	return cloneStatusSlice(purchasePaymentEligibleStatuses)
}

func NormalizePurchaseSettlementStatus(status string, paidAmount, outstandingAmount float64) string {
	if normalizeLifecycleStatus(status) == PurchaseStatusCancelled {
		return PurchaseStatusCancelled
	}

	switch {
	case outstandingAmount <= 0.01:
		return PurchaseStatusPaid
	case paidAmount > 0.01:
		return ReportSettlementStatusPartial
	default:
		return ReportSettlementStatusUnpaid
	}
}

func PaymentEffectiveStatuses() []string {
	return cloneStatusSlice(paymentEffectiveStatuses)
}

func IsPaymentEffective(status string) bool {
	return normalizeLifecycleStatus(status) == PaymentStatusCompleted
}

func PaymentEffectiveWhereClause(alias string) string {
	return fmt.Sprintf("%s = '%s'", qualifiedColumn(alias, "status"), PaymentStatusCompleted)
}

func cloneStatusSlice(statuses []string) []string {
	copied := make([]string, len(statuses))
	copy(copied, statuses)
	return copied
}

func normalizeLifecycleStatus(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func qualifiedColumn(alias, column string) string {
	trimmedAlias := strings.TrimSpace(alias)
	if trimmedAlias == "" {
		return column
	}
	return trimmedAlias + "." + column
}
