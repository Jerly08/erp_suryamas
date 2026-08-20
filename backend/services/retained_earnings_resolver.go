package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

type retainedEarningsResolutionSource string

const (
	retainedEarningsFromCategory retainedEarningsResolutionSource = "category"
	retainedEarningsFromName     retainedEarningsResolutionSource = "name"
)

var retainedEarningsNames = map[string]struct{}{
	"LABA DITAHAN":      {},
	"LABA RUGI DITAHAN": {},
	"RETAINED EARNINGS": {},
}

// getRetainedEarningsAccount resolves the posting account used by period closing.
//
// Account codes are tenant data: older tenants can use a different code from the
// standard COA. The configured mapping may only disambiguate accounts that have
// already been identified semantically; it can never nominate an unrelated equity
// account. This prevents a stale global config from sending a closing entry to
// prive or another equity account.
func (s *UnifiedPeriodClosingService) getRetainedEarningsAccount(db *gorm.DB) (models.Account, error) {
	var configuredReference uint
	if accountingConfig := config.GetAccountingConfig(); accountingConfig != nil {
		configuredReference = accountingConfig.DefaultAccounts.RetainedEarnings
	}

	account, source, err := resolveRetainedEarningsAccount(db, configuredReference)
	if err != nil {
		return models.Account{}, err
	}

	log.Printf("[UNIFIED CLOSING] Retained earnings account resolved from %s: %s - %s (ID: %d)",
		source, account.Code, account.Name, account.ID)
	return account, nil
}

// resolveRetainedEarningsAccount returns an active posting equity account in this
// order: the RETAINED_EARNINGS category, then a canonical retained-earnings name.
// When a semantic criterion matches more than one account, the configured account
// reference may disambiguate it. The config historically contains both account IDs
// and account codes, so both forms are supported for backward compatibility.
func resolveRetainedEarningsAccount(db *gorm.DB, configuredReference uint) (models.Account, retainedEarningsResolutionSource, error) {
	var eligibleAccounts []models.Account
	if err := db.
		Where("UPPER(TRIM(type)) IN ?", []string{models.AccountTypeEquity, "EKUITAS"}).
		Where("is_active = ? AND is_header = ?", true, false).
		Order("code ASC, id ASC").
		Find(&eligibleAccounts).Error; err != nil {
		return models.Account{}, "", fmt.Errorf("failed to query retained earnings candidates: %w", err)
	}

	categoryCandidates := make([]models.Account, 0, 1)
	nameCandidates := make([]models.Account, 0, 1)
	for _, account := range eligibleAccounts {
		if strings.EqualFold(strings.TrimSpace(account.Category), models.CategoryRetainedEarnings) {
			categoryCandidates = append(categoryCandidates, account)
		}
		if isRetainedEarningsName(account.Name) {
			nameCandidates = append(nameCandidates, account)
		}
	}

	if account, found, err := selectRetainedEarningsCandidate(categoryCandidates, configuredReference, "RETAINED_EARNINGS category"); err != nil {
		return models.Account{}, "", err
	} else if found {
		return account, retainedEarningsFromCategory, nil
	}

	if account, found, err := selectRetainedEarningsCandidate(nameCandidates, configuredReference, "retained earnings name"); err != nil {
		return models.Account{}, "", err
	} else if found {
		return account, retainedEarningsFromName, nil
	}

	return models.Account{}, "", fmt.Errorf(
		"retained earnings account not found: assign category %s or use the name Laba (Rugi) Ditahan / Retained Earnings on one active posting EQUITY account",
		models.CategoryRetainedEarnings,
	)
}

func selectRetainedEarningsCandidate(candidates []models.Account, configuredReference uint, criterion string) (models.Account, bool, error) {
	switch len(candidates) {
	case 0:
		return models.Account{}, false, nil
	case 1:
		return candidates[0], true, nil
	}

	if account, found := configuredRetainedEarningsCandidate(candidates, configuredReference); found {
		return account, true, nil
	}

	return models.Account{}, false, fmt.Errorf(
		"multiple active posting equity accounts match %s (%s); set the retained earnings account mapping explicitly",
		criterion,
		formatAccountCandidates(candidates),
	)
}

func configuredRetainedEarningsCandidate(accounts []models.Account, configuredReference uint) (models.Account, bool) {
	if configuredReference == 0 {
		return models.Account{}, false
	}

	configuredCode := strconv.FormatUint(uint64(configuredReference), 10)
	for _, account := range accounts {
		// The shipped config has historically stored a COA code in this field,
		// even though its name says account ID. Prefer that legacy convention
		// before treating the value as a database primary key.
		if strings.TrimSpace(account.Code) == configuredCode {
			return account, true
		}
	}
	for _, account := range accounts {
		if account.ID == configuredReference {
			return account, true
		}
	}

	return models.Account{}, false
}

func isRetainedEarningsName(name string) bool {
	_, found := retainedEarningsNames[normalizeRetainedEarningsName(name)]
	return found
}

func normalizeRetainedEarningsName(name string) string {
	var normalized strings.Builder
	for _, character := range strings.ToUpper(strings.TrimSpace(name)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			normalized.WriteRune(character)
		} else {
			normalized.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(normalized.String()), " ")
}

func formatAccountCandidates(accounts []models.Account) string {
	formatted := make([]string, 0, len(accounts))
	for _, account := range accounts {
		formatted = append(formatted, fmt.Sprintf("%s - %s", account.Code, account.Name))
	}
	return strings.Join(formatted, ", ")
}
