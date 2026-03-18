package repositories_test

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupJournalTestDB() *gorm.DB {
	// Use test database URL or default test database
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres:postgres@localhost/sistem_akuntansi_test?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(testDBURL), &gorm.Config{})
	if err != nil {
		// If test database is not available, skip the test
		return nil
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.Account{},
		&models.User{},
	)
	if err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	// Create test accounts
	accounts := []models.Account{
		{ID: 1, Code: "1101", Name: "Kas", Type: models.AccountTypeAsset, IsActive: true, Balance: 0},
		{ID: 2, Code: "2001", Name: "Hutang Usaha", Type: models.AccountTypeLiability, IsActive: true, Balance: 0},
		{ID: 3, Code: "3001", Name: "Modal Usaha", Type: models.AccountTypeEquity, IsActive: true, Balance: 0},
		{ID: 4, Code: "4001", Name: "Pendapatan", Type: models.AccountTypeRevenue, IsActive: true, Balance: 0},
		{ID: 5, Code: "5001", Name: "Beban Gaji", Type: models.AccountTypeExpense, IsActive: true, Balance: 0},
		{ID: 6, Code: "1000", Name: "Aktiva", Type: models.AccountTypeAsset, IsActive: true, IsHeader: true, Balance: 0},
	}

	for _, account := range accounts {
		db.Create(&account)
	}

	// Create test user
	user := models.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	db.Create(&user)

	return db
}

func requireJournalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupJournalTestDB()
	if db == nil {
		t.Skip("test database not available, skipping test")
	}

	return db
}

func TestJournalEntryRepo_Create(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	tests := []struct {
		name    string
		req     *models.JournalEntryCreateRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid balanced journal entry",
			req: &models.JournalEntryCreateRequest{
				Description: "Test transaction",
				Reference:   "REF001",
				EntryDate:   time.Now(),
				UserID:      1,
				TotalDebit:  1000.00,
				TotalCredit: 1000.00,
			},
			wantErr: false,
		},
		{
			name: "Unbalanced journal entry",
			req: &models.JournalEntryCreateRequest{
				Description: "Unbalanced transaction",
				Reference:   "REF002",
				EntryDate:   time.Now(),
				UserID:      1,
				TotalDebit:  1000.00,
				TotalCredit: 500.00,
			},
			wantErr: true,
			errMsg:  "not balanced",
		},
		{
			name: "Zero amount journal entry",
			req: &models.JournalEntryCreateRequest{
				Description: "Zero transaction",
				Reference:   "REF003",
				EntryDate:   time.Now(),
				UserID:      1,
				TotalDebit:  0.00,
				TotalCredit: 0.00,
			},
			wantErr: true,
			errMsg:  "not balanced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.Create(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.req.Description, result.Description)
				assert.Equal(t, models.JournalStatusDraft, result.Status)
				assert.True(t, result.IsBalanced)
			}
		})
	}
}

func TestJournalEntryRepo_Update(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create a journal entry first
	req := &models.JournalEntryCreateRequest{
		Description: "Original description",
		Reference:   "UPD001",
		EntryDate:   time.Now(),
		UserID:      1,
		TotalDebit:  1000.00,
		TotalCredit: 1000.00,
	}

	entry, err := repo.Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, entry)

	// Test update with journal lines
	newDesc := "Updated description"
	newNotes := "Updated notes"
	updateReq := &models.JournalEntryUpdateRequest{
		Description: &newDesc,
		Notes:       &newNotes,
		JournalLines: []models.JournalLineCreateRequest{
			{
				AccountID:    1,
				Description:  "Cash received",
				DebitAmount:  1000.00,
				CreditAmount: 0,
			},
			{
				AccountID:    4,
				Description:  "Revenue earned",
				DebitAmount:  0,
				CreditAmount: 1000.00,
			},
		},
	}

	updatedEntry, err := repo.Update(ctx, entry.ID, updateReq)
	assert.NoError(t, err)
	assert.NotNil(t, updatedEntry)
	assert.Equal(t, newDesc, updatedEntry.Description)
	assert.Equal(t, newNotes, updatedEntry.Notes)
	assert.Equal(t, 1000.00, updatedEntry.TotalDebit)
	assert.Equal(t, 1000.00, updatedEntry.TotalCredit)
	assert.True(t, updatedEntry.IsBalanced)
}

func TestJournalEntryRepo_PostJournalEntry(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create a balanced journal entry with primary account
	accountID := uint(1)
	req := &models.JournalEntryCreateRequest{
		Description: "Test transaction for posting",
		Reference:   "POST001",
		EntryDate:   time.Now(),
		UserID:      1,
		AccountID:   &accountID,
		TotalDebit:  1000.00,
		TotalCredit: 1000.00,
	}

	entry, err := repo.Create(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, models.JournalStatusDraft, entry.Status)

	// Check initial account balance
	var cashAccount models.Account
	db.First(&cashAccount, 1)
	initialBalance := cashAccount.Balance

	// Post the journal entry
	err = repo.PostJournalEntry(ctx, entry.ID, 1)
	assert.NoError(t, err)

	// Verify status changed
	postedEntry, err := repo.FindByID(ctx, entry.ID)
	assert.NoError(t, err)
	assert.Equal(t, models.JournalStatusPosted, postedEntry.Status)
	assert.NotNil(t, postedEntry.PostedBy)
	assert.NotNil(t, postedEntry.PostingDate)

	// Verify account balance updated (debit - credit = 1000 - 1000 = 0, so no change)
	db.First(&cashAccount, 1)
	assert.Equal(t, initialBalance, cashAccount.Balance)
}

func TestJournalEntryRepo_ReverseJournalEntry(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create and post a journal entry with journal lines
	req := &models.JournalEntryCreateRequest{
		Description: "Original transaction",
		Reference:   "REV001",
		EntryDate:   time.Now(),
		UserID:      1,
		TotalDebit:  500.00,
		TotalCredit: 500.00,
	}

	originalEntry, err := repo.Create(ctx, req)
	assert.NoError(t, err)

	// Add journal lines via update
	updateReq := &models.JournalEntryUpdateRequest{
		JournalLines: []models.JournalLineCreateRequest{
			{
				AccountID:    1,
				Description:  "Cash received",
				DebitAmount:  500.00,
				CreditAmount: 0,
			},
			{
				AccountID:    4,
				Description:  "Revenue earned",
				DebitAmount:  0,
				CreditAmount: 500.00,
			},
		},
	}

	originalEntry, err = repo.Update(ctx, originalEntry.ID, updateReq)
	assert.NoError(t, err)

	// Post the entry
	err = repo.PostJournalEntry(ctx, originalEntry.ID, 1)
	assert.NoError(t, err)

	// Test reversal
	reversalEntry, err := repo.ReverseJournalEntry(ctx, originalEntry.ID, 1, "Error correction")
	assert.NoError(t, err)
	assert.NotNil(t, reversalEntry)

	// Verify reversal entry
	assert.Equal(t, models.JournalStatusPosted, reversalEntry.Status)
	assert.Contains(t, reversalEntry.Description, "REVERSAL")
	assert.Contains(t, reversalEntry.Reference, "REV-")
	assert.Equal(t, originalEntry.TotalDebit, reversalEntry.TotalCredit) // Amounts swapped
	assert.Equal(t, originalEntry.TotalCredit, reversalEntry.TotalDebit)

	// Verify original entry marked as reversed
	updatedOriginal, err := repo.FindByID(ctx, originalEntry.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updatedOriginal.ReversalID)
	assert.Equal(t, reversalEntry.ID, *updatedOriginal.ReversalID)

	// Test that already reversed entry cannot be reversed again
	_, err = repo.ReverseJournalEntry(ctx, originalEntry.ID, 1, "Another reversal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already been reversed")
}

func TestJournalEntryRepo_GetSummary(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create multiple journal entries
	entries := []struct {
		desc      string
		ref       string
		refType   string
		debit     float64
		credit    float64
		shouldPost bool
	}{
		{"Sale 1", "SALE001", models.JournalRefSale, 1000, 1000, true},
		{"Purchase 1", "PUR001", models.JournalRefPurchase, 500, 500, true},
		{"Manual 1", "MAN001", "", 300, 300, false}, // Keep as draft
	}

	for _, e := range entries {
		req := &models.JournalEntryCreateRequest{
			Description:   e.desc,
			Reference:     e.ref,
			ReferenceType: e.refType,
			EntryDate:     time.Now(),
			UserID:        1,
			TotalDebit:    e.debit,
			TotalCredit:   e.credit,
		}

		entry, err := repo.Create(ctx, req)
		assert.NoError(t, err)

		if e.shouldPost {
			err = repo.PostJournalEntry(ctx, entry.ID, 1)
			assert.NoError(t, err)
		}
	}

	// Get summary
	summary, err := repo.GetSummary(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, summary)

	// Verify summary data
	assert.Equal(t, int64(3), summary.TotalEntries)
	assert.Equal(t, 1800.0, summary.TotalDebit)   // 1000 + 500 + 300
	assert.Equal(t, 1800.0, summary.TotalCredit)  // 1000 + 500 + 300
	assert.Equal(t, int64(3), summary.BalancedEntries) // All should be balanced

	// Check status counts
	assert.Equal(t, int64(2), summary.StatusCounts[models.JournalStatusPosted])
	assert.Equal(t, int64(1), summary.StatusCounts[models.JournalStatusDraft])

	// Check type counts
	assert.Equal(t, int64(1), summary.TypeCounts[models.JournalRefSale])
	assert.Equal(t, int64(1), summary.TypeCounts[models.JournalRefPurchase])
}

func TestJournalEntryRepo_FindByReferenceID(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create journal entry with reference
	refID := uint(123)
	req := &models.JournalEntryCreateRequest{
		Description:   "Sale transaction",
		Reference:     "SALE001",
		ReferenceType: models.JournalRefSale,
		ReferenceID:   &refID,
		EntryDate:     time.Now(),
		UserID:        1,
		TotalDebit:    1000.00,
		TotalCredit:   1000.00,
	}

	entry, err := repo.Create(ctx, req)
	assert.NoError(t, err)

	// Find by reference
	foundEntry, err := repo.FindByReferenceID(ctx, models.JournalRefSale, 123)
	assert.NoError(t, err)
	assert.NotNil(t, foundEntry)
	assert.Equal(t, entry.ID, foundEntry.ID)
	assert.Equal(t, models.JournalRefSale, foundEntry.ReferenceType)

	// Find non-existing reference
	notFound, err := repo.FindByReferenceID(ctx, models.JournalRefPurchase, 999)
	assert.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestJournalEntryRepo_Delete(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create a journal entry
	req := &models.JournalEntryCreateRequest{
		Description: "Test delete",
		Reference:   "DEL001",
		EntryDate:   time.Now(),
		UserID:      1,
		TotalDebit:  1000.00,
		TotalCredit: 1000.00,
	}

	entry, err := repo.Create(ctx, req)
	assert.NoError(t, err)

	// Should be able to delete draft entry
	err = repo.Delete(ctx, entry.ID)
	assert.NoError(t, err)

	// Verify entry is deleted
	_, err = repo.FindByID(ctx, entry.ID)
	assert.Error(t, err)
}

func TestJournalEntryRepo_FindAll(t *testing.T) {
	db := requireJournalTestDB(t)
	repo := repositories.NewJournalEntryRepository(db)

	ctx := context.Background()

	// Create test entries
	for i := 1; i <= 5; i++ {
		req := &models.JournalEntryCreateRequest{
			Description: fmt.Sprintf("Test entry %d", i),
			Reference:   fmt.Sprintf("TEST%03d", i),
			EntryDate:   time.Now(),
			UserID:      1,
			TotalDebit:  float64(i * 100),
			TotalCredit: float64(i * 100),
		}

		_, err := repo.Create(ctx, req)
		assert.NoError(t, err)
	}

	// Test find all with pagination
	filter := &models.JournalEntryFilter{
		Page:  1,
		Limit: 3,
	}

	entries, total, err := repo.FindAll(ctx, filter)
	assert.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, int64(5), total)

	// Test with search filter
	filter.Search = "Test entry 1"
	entries, total, err = repo.FindAll(ctx, filter)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, int64(1), total)
}
