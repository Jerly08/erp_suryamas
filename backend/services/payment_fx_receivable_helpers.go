package services

import (
	"fmt"
	"strings"

	"app-sistem-akuntansi/models"
)

func calculateSaleReceivableSettlementBookAmount(sale *models.Sale, settledAmount, settlementExchangeRate float64) (float64, float64, error) {
	if sale == nil {
		return 0, 0, fmt.Errorf("sale is required")
	}
	if settledAmount < 0 {
		return 0, 0, fmt.Errorf("settled amount cannot be negative")
	}

	normalizedCurrency := strings.ToUpper(strings.TrimSpace(sale.Currency))
	if normalizedCurrency != "USD" {
		return settledAmount, 1, nil
	}
	if settledAmount == 0 {
		return 0, 0, nil
	}
	if settlementExchangeRate <= 0 {
		return 0, 0, fmt.Errorf("exchange rate is required to settle USD receivable for sale %d", sale.ID)
	}

	historicalRate := sale.ExchangeRate
	if historicalRate <= 0 {
		historicalRate = 1
	}

	settledNativeAmount := settledAmount / settlementExchangeRate
	return settledNativeAmount * historicalRate, settledNativeAmount, nil
}

func deriveReceivableSettlementBookAmount(payment *models.Payment) float64 {
	if payment == nil {
		return 0
	}

	bankBookAmount := payment.BookAmountIDR
	if bankBookAmount == 0 {
		bankBookAmount = payment.Amount
	}

	switch strings.ToUpper(strings.TrimSpace(payment.ForexGainLossType)) {
	case "GAIN":
		return bankBookAmount - payment.ForexGainLossAmount
	case "LOSS":
		return bankBookAmount + payment.ForexGainLossAmount
	default:
		return bankBookAmount
	}
}
