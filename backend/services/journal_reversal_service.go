package services

import (
	"context"
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JournalReversalService handles journal entry reversals
// This is a TEMPLATE implementation - customize as needed
type JournalReversalService struct {
	db *gorm.DB
}

// NewJournalReversalService creates a new journal reversal service
func NewJournalReversalService(db *gorm.DB) *JournalReversalService {
	return &JournalReversalService{
		db: db,
	}
}

// ReversalRequest represents a request to reverse a journal entry
type ReversalRequest struct {
	JournalID    uint64    `json:"journal_id" binding:"required"`
	Reason       string    `json:"reason" binding:"required,min=10"`
	ReversalDate time.Time `json:"reversal_date" binding:"required"`
	UserID       uint64    `json:"user_id" binding:"required"`
	Notes        string    `json:"notes"`
}

// ReversalResponse represents the result of a reversal operation
type ReversalResponse struct {
	OriginalJournalID uint64                   `json:"original_journal_id"`
	ReversalJournalID uint64                   `json:"reversal_journal_id"`
	OriginalEntry     *models.SSOTJournalEntry `json:"original_entry"`
	ReversalEntry     *models.SSOTJournalEntry `json:"reversal_entry"`
	Message           string                   `json:"message"`
}

// ReverseJournalEntry reverses a posted journal entry by creating an offsetting entry
func (s *JournalReversalService) ReverseJournalEntry(ctx context.Context, req ReversalRequest) (*ReversalResponse, error) {
	fmt.Printf("🔄 Starting journal reversal for Journal ID: %d\n", req.JournalID)

	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %v", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("❌ Panic recovered during reversal: %v\n", r)
		}
	}()

	// 1. Get original journal entry
	originalEntry, err := s.getJournalEntry(tx, req.JournalID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get original journal: %v", err)
	}

	// 2. Validate reversal is allowed
	if err := s.validateReversal(originalEntry); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("reversal validation failed: %v", err)
	}

	// 3. Get original journal lines
	originalLines, err := s.getJournalLines(tx, req.JournalID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get journal lines: %v", err)
	}

	// 4. Create reversal entry
	reversalEntry, err := s.createReversalEntry(tx, originalEntry, originalLines, req)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create reversal entry: %v", err)
	}

	// 5. Update original entry to mark as reversed
	if err := s.markOriginalAsReversed(tx, originalEntry, reversalEntry.ID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to mark original as reversed: %v", err)
	}

	// 6. Create event log
	if err := s.logReversalEvent(tx, originalEntry.ID, reversalEntry.ID, req); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to log reversal event: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit reversal: %v", err)
	}

	fmt.Printf("✅ Journal reversal completed: Original=%d, Reversal=%d\n",
		originalEntry.ID, reversalEntry.ID)

	return &ReversalResponse{
		OriginalJournalID: originalEntry.ID,
		ReversalJournalID: reversalEntry.ID,
		OriginalEntry:     originalEntry,
		ReversalEntry:     reversalEntry,
		Message:           "Journal entry reversed successfully",
	}, nil
}

// getJournalEntry retrieves a journal entry by ID
func (s *JournalReversalService) getJournalEntry(tx *gorm.DB, journalID uint64) (*models.SSOTJournalEntry, error) {
	var entry models.SSOTJournalEntry

	err := tx.Where("id = ? AND deleted_at IS NULL", journalID).
		First(&entry).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("journal entry not found: %d", journalID)
		}
		return nil, err
	}

	return &entry, nil
}

// getJournalLines retrieves all lines for a journal entry
func (s *JournalReversalService) getJournalLines(tx *gorm.DB, journalID uint64) ([]models.SSOTJournalLine, error) {
	var lines []models.SSOTJournalLine

	err := tx.Where("journal_id = ?", journalID).
		Order("line_number ASC").
		Find(&lines).Error

	if err != nil {
		return nil, err
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("no lines found for journal %d", journalID)
	}

	return lines, nil
}

// validateReversal checks if a journal entry can be reversed
func (s *JournalReversalService) validateReversal(entry *models.SSOTJournalEntry) error {
	// Check if entry is posted
	if entry.Status != "POSTED" {
		return fmt.Errorf("can only reverse POSTED entries, current status: %s", entry.Status)
	}

	// Check if already reversed
	if entry.ReversedBy != nil {
		return fmt.Errorf("journal entry already reversed by entry %d", *entry.ReversedBy)
	}

	// Check if entry is balanced
	if !entry.IsBalanced {
		return fmt.Errorf("cannot reverse unbalanced journal entry")
	}

	// Additional business rules can be added here
	// For example: check if source transaction can be reversed

	return nil
}

// createReversalEntry creates the offsetting journal entry
func (s *JournalReversalService) createReversalEntry(
	tx *gorm.DB,
	original *models.SSOTJournalEntry,
	originalLines []models.SSOTJournalLine,
	req ReversalRequest,
) (*models.SSOTJournalEntry, error) {
	effectiveReversalDate := req.ReversalDate
	if effectiveReversalDate.Before(original.EntryDate) {
		effectiveReversalDate = original.EntryDate
	}

	// Create reversal entry header
	reversalEntry := &models.SSOTJournalEntry{
		EntryNumber: s.generateReversalEntryNumber(original),
		SourceType:  "REVERSAL",
		SourceID:    &original.ID,
		SourceCode:  truncateString(fmt.Sprintf("REV-%s", original.SourceCode), 100),

		EntryDate:   effectiveReversalDate,
		Description: fmt.Sprintf("REVERSAL: %s", original.Description),
		Reference:   truncateString(fmt.Sprintf("Reversal of %s", original.EntryNumber), 200),
		Notes:       fmt.Sprintf("Reason: %s\n%s", req.Reason, req.Notes),

		TotalDebit:  original.TotalCredit, // Flip debit/credit
		TotalCredit: original.TotalDebit,  // Flip debit/credit

		Status:          "POSTED",
		IsBalanced:      true,
		IsAutoGenerated: false,

		PostedAt: &req.ReversalDate,
		PostedBy: &req.UserID,

		ReversedFrom: &original.ID, // Link to original

		CreatedBy: req.UserID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save reversal entry
	if err := tx.Create(reversalEntry).Error; err != nil {
		return nil, fmt.Errorf("failed to create reversal entry: %v", err)
	}

	// Create reversal lines (flip debit/credit)
	reversalLines := make([]models.SSOTJournalLine, 0, len(originalLines))
	for i, originalLine := range originalLines {
		reversalLine := models.SSOTJournalLine{
			JournalID:   reversalEntry.ID,
			AccountID:   originalLine.AccountID,
			LineNumber:  i + 1,
			Description: fmt.Sprintf("Reversal: %s", originalLine.Description),

			// FLIP debit and credit
			DebitAmount:  originalLine.CreditAmount,
			CreditAmount: originalLine.DebitAmount,

			Quantity:  originalLine.Quantity,
			UnitPrice: originalLine.UnitPrice,

			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		reversalLines = append(reversalLines, reversalLine)
	}

	// Save reversal lines
	if err := tx.Create(&reversalLines).Error; err != nil {
		return nil, fmt.Errorf("failed to create reversal lines: %v", err)
	}

	// Verify reversal entry is balanced
	if !reversalEntry.TotalDebit.Equal(reversalEntry.TotalCredit) {
		return nil, fmt.Errorf("reversal entry is not balanced: debit=%s credit=%s",
			reversalEntry.TotalDebit.String(), reversalEntry.TotalCredit.String())
	}

	reversalEntry.Lines = reversalLines
	return reversalEntry, nil
}

// markOriginalAsReversed updates the original entry to indicate it has been reversed
func (s *JournalReversalService) markOriginalAsReversed(
	tx *gorm.DB,
	original *models.SSOTJournalEntry,
	reversalID uint64,
) error {
	return tx.Model(&models.SSOTJournalEntry{}).
		Where("id = ?", original.ID).
		Updates(map[string]interface{}{
			"reversed_by": reversalID,
			"updated_at":  time.Now(),
		}).Error
}

// logReversalEvent creates an audit log for the reversal
func (s *JournalReversalService) logReversalEvent(
	tx *gorm.DB,
	originalID uint64,
	reversalID uint64,
	req ReversalRequest,
) error {
	eventLog := &models.SSOTJournalEventLog{
		EventUUID: generateUUID(),
		JournalID: &originalID,
		EventType: "JOURNAL_REVERSED",
		EventData: map[string]interface{}{
			"original_journal_id": originalID,
			"reversal_journal_id": reversalID,
			"reason":              req.Reason,
			"notes":               req.Notes,
			"reversal_date":       req.ReversalDate,
		},
		UserID:         &req.UserID,
		EventTimestamp: time.Now(),
		IPAddress:      "0.0.0.0",
		UserAgent:      "journal-reversal-service",
	}

	return tx.Create(eventLog).Error
}

// generateReversalEntryNumber generates a unique entry number for reversal.
// The value must always fit unified_journal_ledger.entry_number (varchar(50)).
func (s *JournalReversalService) generateReversalEntryNumber(original *models.SSOTJournalEntry) string {
	timestamp := time.Now().Format("20060102150405")
	if original == nil {
		return truncateString(fmt.Sprintf("REV-%s", timestamp), 50)
	}

	return truncateString(fmt.Sprintf("REV-%d-%s", original.ID, timestamp), 50)
}

func generateUUID() string {
	return uuid.NewString()
}

func truncateString(value string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

// GetReversalHistory returns all reversals for a journal entry
func (s *JournalReversalService) GetReversalHistory(journalID uint64) ([]models.SSOTJournalEntry, error) {
	var reversals []models.SSOTJournalEntry

	err := s.db.Where("reversed_from = ? AND deleted_at IS NULL", journalID).
		Order("created_at DESC").
		Find(&reversals).Error

	if err != nil {
		return nil, err
	}

	return reversals, nil
}

// GetOriginalEntry returns the original entry that was reversed (if this is a reversal)
func (s *JournalReversalService) GetOriginalEntry(reversalID uint64) (*models.SSOTJournalEntry, error) {
	var reversal models.SSOTJournalEntry

	err := s.db.Where("id = ? AND deleted_at IS NULL", reversalID).
		First(&reversal).Error

	if err != nil {
		return nil, err
	}

	if reversal.ReversedFrom == nil {
		return nil, fmt.Errorf("entry %d is not a reversal", reversalID)
	}

	var original models.SSOTJournalEntry
	err = s.db.Where("id = ? AND deleted_at IS NULL", *reversal.ReversedFrom).
		First(&original).Error

	if err != nil {
		return nil, err
	}

	return &original, nil
}

// CanReverse checks if a journal entry can be reversed
func (s *JournalReversalService) CanReverse(journalID uint64) (bool, string, error) {
	entry, err := s.getJournalEntry(s.db, journalID)
	if err != nil {
		return false, "", err
	}

	if err := s.validateReversal(entry); err != nil {
		return false, err.Error(), nil
	}

	return true, "Journal entry can be reversed", nil
}
