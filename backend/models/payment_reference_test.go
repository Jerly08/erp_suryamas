package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpenseLiabilityOverpaymentReferenceHelpers(t *testing.T) {
	marked := MarkExpenseLiabilityOverpaymentReference("Bukti setor PPh 22")

	require.True(t, HasExpenseLiabilityOverpaymentReference(marked))
	require.Equal(t, "Bukti setor PPh 22", StripInternalPaymentReference(marked))
	require.Equal(t, "Bukti setor PPh 22", ClearExpenseLiabilityOverpaymentReference(marked))
}

func TestStripInternalPaymentReferenceRemovesStackedPrefixes(t *testing.T) {
	reference := MarkPendingDraftAllocationsReference(MarkExpenseLiabilityOverpaymentReference("REF-001"))

	require.Equal(t, "REF-001", StripInternalPaymentReference(reference))
}
