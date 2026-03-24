package services

import (
	"strings"
	"testing"

	"app-sistem-akuntansi/models"
)

func TestGenerateReversalEntryNumberFitsColumnLimit(t *testing.T) {
	service := NewJournalReversalService(nil)
	original := &models.SSOTJournalEntry{
		ID:          123,
		EntryNumber: "PURCHASE-1-1774103292-VERY-LONG-ENTRY-NUMBER-FOR-TESTING",
	}

	reversalNumber := service.generateReversalEntryNumber(original)

	if len(reversalNumber) > 50 {
		t.Fatalf("reversal entry number exceeds column limit: %d > 50 (%s)", len(reversalNumber), reversalNumber)
	}
	if !strings.HasPrefix(reversalNumber, "REV-123-") {
		t.Fatalf("unexpected reversal entry number format: %s", reversalNumber)
	}
}
