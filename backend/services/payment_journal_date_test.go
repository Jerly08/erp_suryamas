package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPaymentJournalEntryDateUsesJakartaBusinessDay(t *testing.T) {
	paymentDate := time.Date(2026, time.May, 10, 17, 0, 0, 0, time.UTC)

	entryDate := paymentJournalEntryDate(paymentDate)

	require.Equal(t, time.Date(2026, time.May, 11, 0, 0, 0, 0, time.UTC), entryDate)
}

func TestPaymentJournalEntryDateKeepsSameJakartaCalendarDay(t *testing.T) {
	paymentDate := time.Date(2026, time.May, 11, 0, 0, 0, 0, time.UTC)

	entryDate := paymentJournalEntryDate(paymentDate)

	require.Equal(t, time.Date(2026, time.May, 11, 0, 0, 0, 0, time.UTC), entryDate)
}
