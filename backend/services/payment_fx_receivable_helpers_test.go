package services

import (
	"app-sistem-akuntansi/models"
	"testing"
)

func TestCalculateSaleReceivableSettlementBookAmount_IDRReceivableUsesNominalBookValue(t *testing.T) {
	sale := &models.Sale{
		ID:           1,
		Currency:     "IDR",
		ExchangeRate: 1,
	}

	bookAmount, nativeAmount, err := calculateSaleReceivableSettlementBookAmount(sale, 2775000, 16969)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bookAmount != 2775000 {
		t.Fatalf("expected IDR book amount to stay nominal, got %.2f", bookAmount)
	}
	if nativeAmount != 1 {
		t.Fatalf("expected native indicator 1 for non-USD receivable, got %.2f", nativeAmount)
	}
}

func TestReceivableRealizedFXScenarios(t *testing.T) {
	tests := []struct {
		name               string
		sale               *models.Sale
		settledAmount      float64
		settlementRate     float64
		bankBookAmountIDR  float64
		expectedBookAmount float64
		expectedFXAmount   float64
		expectedFXType     string
	}{
		{
			name: "IDR invoice to IDR bank stays neutral",
			sale: &models.Sale{
				ID:           1,
				Currency:     "IDR",
				ExchangeRate: 1,
			},
			settledAmount:      2775000,
			settlementRate:     1,
			bankBookAmountIDR:  2775000,
			expectedBookAmount: 2775000,
			expectedFXAmount:   0,
			expectedFXType:     "",
		},
		{
			name: "IDR invoice to USD bank stays neutral when book amount equals settlement",
			sale: &models.Sale{
				ID:           2,
				Currency:     "IDR",
				ExchangeRate: 1,
			},
			settledAmount:      2775000,
			settlementRate:     16969,
			bankBookAmountIDR:  2775000,
			expectedBookAmount: 2775000,
			expectedFXAmount:   0,
			expectedFXType:     "",
		},
		{
			name: "USD invoice to IDR bank with stronger settlement rate creates gain",
			sale: &models.Sale{
				ID:           3,
				Currency:     "USD",
				ExchangeRate: 16000,
			},
			settledAmount:      1700000,
			settlementRate:     17000,
			bankBookAmountIDR:  1700000,
			expectedBookAmount: 1600000,
			expectedFXAmount:   100000,
			expectedFXType:     "GAIN",
		},
		{
			name: "USD invoice to USD bank with weaker settlement rate creates loss",
			sale: &models.Sale{
				ID:           4,
				Currency:     "USD",
				ExchangeRate: 16000,
			},
			settledAmount:      1500000,
			settlementRate:     15000,
			bankBookAmountIDR:  1500000,
			expectedBookAmount: 1600000,
			expectedFXAmount:   100000,
			expectedFXType:     "LOSS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bookAmount, _, err := calculateSaleReceivableSettlementBookAmount(test.sale, test.settledAmount, test.settlementRate)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if bookAmount != test.expectedBookAmount {
				t.Fatalf("expected historical settlement book amount %.2f, got %.2f", test.expectedBookAmount, bookAmount)
			}

			fxAmount, fxType := classifyReceivableRealizedFX(test.bankBookAmountIDR, bookAmount)
			if fxAmount != test.expectedFXAmount || fxType != test.expectedFXType {
				t.Fatalf("expected FX %s %.2f, got %s %.2f", test.expectedFXType, test.expectedFXAmount, fxType, fxAmount)
			}
		})
	}
}

func TestResolveSaleSettlementStatus_PartiallyPaid(t *testing.T) {
	status := models.ResolveSaleSettlementStatus(models.SaleStatusInvoiced, 1500000, 4000000)
	if status != models.SaleStatusPartiallyPaid {
		t.Fatalf("expected settlement status %s, got %s", models.SaleStatusPartiallyPaid, status)
	}
}
