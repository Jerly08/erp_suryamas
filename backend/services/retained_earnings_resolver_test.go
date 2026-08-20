package services

import (
	"fmt"
	"strings"
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newRetainedEarningsResolverTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Account{}); err != nil {
		t.Fatalf("migrate accounts: %v", err)
	}
	return db
}

func createRetainedEarningsTestAccounts(t *testing.T, db *gorm.DB, accounts ...models.Account) {
	t.Helper()
	for index := range accounts {
		if err := db.Create(&accounts[index]).Error; err != nil {
			t.Fatalf("create account %s: %v", accounts[index].Code, err)
		}
	}
}

func postingEquityAccount(id uint, code, name, category string) models.Account {
	return models.Account{
		ID:       id,
		Code:     code,
		Name:     name,
		Type:     models.AccountTypeEquity,
		Category: category,
		IsActive: true,
		IsHeader: false,
	}
}

func TestResolveRetainedEarningsAccountPrefersSemanticCategory(t *testing.T) {
	db := newRetainedEarningsResolverTestDB(t)
	createRetainedEarningsTestAccounts(t, db,
		postingEquityAccount(10, "3102", "LABA (RUGI) DITAHAN", models.CategoryRetainedEarnings),
		postingEquityAccount(11, "3103", "LABA (RUGI) TAHUN BERJALAN", models.CategoryEquity),
	)

	account, source, err := resolveRetainedEarningsAccount(db, 9999)
	if err != nil {
		t.Fatalf("resolve retained earnings: %v", err)
	}
	if account.Code != "3102" || source != retainedEarningsFromCategory {
		t.Fatalf("expected category account 3102, got %s from %s", account.Code, source)
	}
}

func TestResolveRetainedEarningsAccountFindsLegacyNamedAccountInsteadOfPrive(t *testing.T) {
	db := newRetainedEarningsResolverTestDB(t)
	createRetainedEarningsTestAccounts(t, db,
		postingEquityAccount(20, "3102", "OTHER PRIVE", models.CategoryEquity),
		postingEquityAccount(21, "3103", "LABA (RUGI) DITAHAN", models.CategoryEquity),
		postingEquityAccount(22, "3104", "LABA (RUGI) TAHUN BERJALAN", models.CategoryEquity),
	)

	// A stale config reference must not redirect closing to Other Prive while
	// the COA itself clearly identifies the retained earnings account.
	account, source, err := resolveRetainedEarningsAccount(db, 3102)
	if err != nil {
		t.Fatalf("resolve retained earnings: %v", err)
	}
	if account.Code != "3103" || source != retainedEarningsFromName {
		t.Fatalf("expected named account 3103, got %s from %s", account.Code, source)
	}
}

func TestResolveRetainedEarningsAccountRejectsNonSemanticConfiguredFallback(t *testing.T) {
	db := newRetainedEarningsResolverTestDB(t)
	createRetainedEarningsTestAccounts(t, db,
		postingEquityAccount(77, "3999", "AKUMULASI HASIL USAHA", models.CategoryEquity),
	)

	_, _, err := resolveRetainedEarningsAccount(db, 77)
	if err == nil {
		t.Fatal("expected non-semantic configured account to be rejected")
	}
	if !strings.Contains(err.Error(), "retained earnings account not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRetainedEarningsAccountRejectsAmbiguousSemanticMatches(t *testing.T) {
	db := newRetainedEarningsResolverTestDB(t)
	createRetainedEarningsTestAccounts(t, db,
		postingEquityAccount(31, "3102", "LABA DITAHAN", models.CategoryEquity),
		postingEquityAccount(32, "3201", "RETAINED EARNINGS", models.CategoryEquity),
	)

	_, _, err := resolveRetainedEarningsAccount(db, 0)
	if err == nil {
		t.Fatal("expected ambiguous retained earnings accounts to be rejected")
	}
	if !strings.Contains(err.Error(), "multiple active posting equity accounts") {
		t.Fatalf("unexpected ambiguity error: %v", err)
	}
}

func TestResolveRetainedEarningsAccountConfiguredCodeDisambiguates(t *testing.T) {
	db := newRetainedEarningsResolverTestDB(t)
	createRetainedEarningsTestAccounts(t, db,
		// The legacy config convention stores COA codes. This account ID deliberately
		// collides with that code to verify that the code wins over a primary-key match.
		postingEquityAccount(3201, "3102", "LABA DITAHAN", models.CategoryEquity),
		postingEquityAccount(42, "3201", "RETAINED EARNINGS", models.CategoryEquity),
	)

	account, source, err := resolveRetainedEarningsAccount(db, 3201)
	if err != nil {
		t.Fatalf("resolve retained earnings: %v", err)
	}
	if account.Code != "3201" || source != retainedEarningsFromName {
		t.Fatalf("expected configured semantic account 3201, got %s from %s", account.Code, source)
	}
}
