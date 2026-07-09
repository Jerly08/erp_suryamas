package services

import (
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
