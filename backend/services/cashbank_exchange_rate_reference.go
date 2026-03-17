package services

import (
	"app-sistem-akuntansi/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"
)

type ReferenceExchangeRate struct {
	BaseCurrency   string    `json:"base_currency"`
	TargetCurrency string    `json:"target_currency"`
	ExchangeRate   float64   `json:"exchange_rate"`
	Source         string    `json:"source"`
	Provider       string    `json:"provider"`
	FetchedAt      time.Time `json:"fetched_at"`
	CachedUntil    time.Time `json:"cached_until"`
	Message        string    `json:"message,omitempty"`
}

type exchangeRateAPIResponse struct {
	Base               string             `json:"base"`
	Date               string             `json:"date"`
	Rates              map[string]float64 `json:"rates"`
	TimeLastUpdateUTC  string             `json:"time_last_update_utc"`
	TimeLastUpdatedUTC string             `json:"time_last_updated_utc"`
	TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
}

type cachedReferenceRate struct {
	data      *ReferenceExchangeRate
	expiresAt time.Time
}

var referenceRateCache = struct {
	mu    sync.RWMutex
	items map[string]cachedReferenceRate
}{
	items: map[string]cachedReferenceRate{},
}

func normalizeRateCurrency(currency string) string {
	normalized := strings.ToUpper(strings.TrimSpace(currency))
	if normalized == "" {
		return "IDR"
	}
	return normalized
}

func cacheReferenceRate(key string, rate *ReferenceExchangeRate) {
	referenceRateCache.mu.Lock()
	defer referenceRateCache.mu.Unlock()

	referenceRateCache.items[key] = cachedReferenceRate{
		data:      rate,
		expiresAt: rate.CachedUntil,
	}
}

func getCachedReferenceRate(key string, allowStale bool) *ReferenceExchangeRate {
	referenceRateCache.mu.RLock()
	defer referenceRateCache.mu.RUnlock()

	item, exists := referenceRateCache.items[key]
	if !exists || item.data == nil {
		return nil
	}
	if !allowStale && time.Now().After(item.expiresAt) {
		return nil
	}

	cloned := *item.data
	return &cloned
}

func deriveProviderName(rawURL string) string {
	if rawURL == "" {
		return "manual"
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}

	return parsed.Host
}

func parseProviderFetchedAt(response exchangeRateAPIResponse) time.Time {
	if response.TimeLastUpdateUnix > 0 {
		return time.Unix(response.TimeLastUpdateUnix, 0).UTC()
	}

	for _, candidate := range []string{response.TimeLastUpdatedUTC, response.TimeLastUpdateUTC} {
		if candidate == "" {
			continue
		}

		if parsed, err := time.Parse(time.RFC1123, candidate); err == nil {
			return parsed.UTC()
		}
		if parsed, err := time.Parse(time.RFC1123Z, candidate); err == nil {
			return parsed.UTC()
		}
	}

	return time.Now().UTC()
}

func fetchReferenceRateFromProvider(providerBase, base, target string) (*ReferenceExchangeRate, error) {
	requestURL := strings.TrimRight(providerBase, "/") + "/" + neturl.PathEscape(base)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create exchange rate request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange rate reference: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("exchange rate API returned status %d", resp.StatusCode)
	}

	var payload exchangeRateAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode exchange rate response: %w", err)
	}

	rate := payload.Rates[target]
	if rate <= 0 {
		return nil, fmt.Errorf("exchange rate for %s/%s not available", base, target)
	}

	fetchedAt := parseProviderFetchedAt(payload)
	return &ReferenceExchangeRate{
		BaseCurrency:   base,
		TargetCurrency: target,
		ExchangeRate:   rate,
		Source:         "API_REFERENCE",
		Provider:       deriveProviderName(providerBase),
		FetchedAt:      fetchedAt,
		CachedUntil:    time.Now().UTC().Add(15 * time.Minute),
		Message:        "Reference rate only. User can override before posting.",
	}, nil
}

func (s *CashBankService) GetReferenceExchangeRate(baseCurrency, targetCurrency string) (*ReferenceExchangeRate, error) {
	base := normalizeRateCurrency(baseCurrency)
	target := normalizeRateCurrency(targetCurrency)

	if base == target {
		now := time.Now().UTC()
		return &ReferenceExchangeRate{
			BaseCurrency:   base,
			TargetCurrency: target,
			ExchangeRate:   1,
			Source:         "SYSTEM",
			Provider:       "internal",
			FetchedAt:      now,
			CachedUntil:    now.Add(15 * time.Minute),
			Message:        "Base and target currency are the same. This is a reference-only value.",
		}, nil
	}

	cacheKey := fmt.Sprintf("%s:%s", base, target)
	if cached := getCachedReferenceRate(cacheKey, false); cached != nil {
		return cached, nil
	}

	cfg := config.GetAccountingConfig()
	apiBase := ""
	if cfg != nil {
		apiBase = strings.TrimSpace(cfg.CurrencySettings.ExchangeRateAPI)
	}
	if apiBase == "" {
		return nil, fmt.Errorf("exchange rate API is not configured")
	}

	providers := []string{
		apiBase,
		"https://open.er-api.com/v6/latest",
		"https://api.exchangerate-api.com/v4/latest",
	}
	seen := map[string]bool{}
	var lastErr error

	for _, provider := range providers {
		normalizedProvider := strings.TrimSpace(provider)
		if normalizedProvider == "" || seen[normalizedProvider] {
			continue
		}
		seen[normalizedProvider] = true

		reference, err := fetchReferenceRateFromProvider(normalizedProvider, base, target)
		if err != nil {
			lastErr = err
			continue
		}

		cacheReferenceRate(cacheKey, reference)
		return reference, nil
	}

	if staleCached := getCachedReferenceRate(cacheKey, true); staleCached != nil {
		staleCached.Source = "API_CACHE"
		staleCached.Message = "Using cached reference rate because live providers are currently unavailable."
		staleCached.CachedUntil = time.Now().UTC().Add(15 * time.Minute)
		return staleCached, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("unable to retrieve exchange rate reference")
}
