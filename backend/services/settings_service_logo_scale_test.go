package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSettingsCompanyLogoScale(t *testing.T) {
	service := &SettingsService{}

	for _, value := range []float64{50, 100, 180} {
		updates := map[string]interface{}{"company_logo_scale": value}
		require.NoError(t, service.validateSettings(updates))
		require.Equal(t, int(value), updates["company_logo_scale"])
	}

	for _, value := range []interface{}{49.0, 181.0, 100.5, "large"} {
		updates := map[string]interface{}{"company_logo_scale": value}
		require.Error(t, service.validateSettings(updates))
	}
}

func TestValidateSettingsPoweredByText(t *testing.T) {
	service := &SettingsService{}

	updates := map[string]interface{}{"powered_by_text": "  Powered by Kode Inovasi Semesta  "}
	require.NoError(t, service.validateSettings(updates))
	require.Equal(t, "Powered by Kode Inovasi Semesta", updates["powered_by_text"])

	for _, value := range []interface{}{"", strings.Repeat("a", 161), 42} {
		updates := map[string]interface{}{"powered_by_text": value}
		require.Error(t, service.validateSettings(updates))
	}
}
