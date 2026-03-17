package database

import (
	"testing"

	"app-sistem-akuntansi/models"
)

func TestRequireExplicitTemplateSyncApprovalRequiresEnv(t *testing.T) {
	t.Setenv(destructiveCOATemplateSyncEnv, "")

	err := requireExplicitTemplateSyncApproval()
	if err == nil {
		t.Fatal("expected destructive template sync approval to be required")
	}
}

func TestRequireExplicitTemplateSyncApprovalAllowsApprovedEnv(t *testing.T) {
	t.Setenv(destructiveCOATemplateSyncEnv, destructiveCOATemplateSyncApproval)

	if err := requireExplicitTemplateSyncApproval(); err != nil {
		t.Fatalf("expected approval guard to pass, got %v", err)
	}
}

func TestBuildExistingAccountUpdatesSkipsSafeMode(t *testing.T) {
	existing := models.Account{
		Code:        "1101",
		Name:        "Kas Existing",
		IsActive:    false,
		Balance:     12345.67,
		Description: "existing account",
	}
	seed := models.Account{
		Code:        "1101",
		Name:        "Kas Template Baru",
		IsActive:    true,
		Balance:     0,
		Description: "template account",
	}

	updates, shouldUpdate := buildExistingAccountUpdates(existing, seed, "KAS TEMPLATE BARU", AccountSeedOptions{})
	if shouldUpdate {
		t.Fatal("expected safe seed mode to skip updates for existing accounts")
	}
	if updates != nil {
		t.Fatalf("expected no updates in safe seed mode, got %#v", updates)
	}
}

func TestBuildExistingAccountUpdatesRespectsSystemCriticalProtection(t *testing.T) {
	existing := models.Account{
		Code:             "1101",
		Name:             "Kas Existing",
		IsActive:         false,
		IsSystemCritical: true,
	}
	seed := models.Account{
		Code:        "1101",
		Name:        "Kas Template Baru",
		Type:        models.AccountTypeAsset,
		Category:    models.CategoryCurrentAsset,
		IsActive:    true,
		IsHeader:    true,
		Level:       1,
		Description: "template account",
	}

	updates, shouldUpdate := buildExistingAccountUpdates(existing, seed, "KAS TEMPLATE BARU", AccountSeedOptions{
		UpdateExistingTemplateAccounts: true,
	})
	if !shouldUpdate {
		t.Fatal("expected template sync mode to build update payload")
	}
	if _, exists := updates["is_active"]; exists {
		t.Fatal("expected system critical account to keep existing is_active")
	}
	if _, exists := updates["type"]; exists {
		t.Fatal("expected system critical account to keep existing type")
	}
	if _, exists := updates["category"]; exists {
		t.Fatal("expected system critical account to keep existing category")
	}
	if updates["name"] != "KAS TEMPLATE BARU" {
		t.Fatalf("expected normalized name update, got %#v", updates["name"])
	}
}
