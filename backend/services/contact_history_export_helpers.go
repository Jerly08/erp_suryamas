package services

import (
	"encoding/json"
	"strings"
)

type customerHistoryExportView struct {
	CustomerName          string
	CustomerCode          string
	StartDate             string
	EndDate               string
	SalesTransactions     []map[string]interface{}
	PaymentTransactions   []map[string]interface{}
	TotalActivities       int
	SalesTotalAmount      float64
	SalesTotalPaid        float64
	SalesTotalOutstanding float64
	PaymentTotalAmount    float64
	PaymentProgress       float64
}

type vendorHistoryExportView struct {
	VendorName               string
	VendorCode               string
	StartDate                string
	EndDate                  string
	PurchaseTransactions     []map[string]interface{}
	PaymentTransactions      []map[string]interface{}
	TotalActivities          int
	PurchaseTotalAmount      float64
	PurchaseTotalPaid        float64
	PurchaseTotalOutstanding float64
	PaymentTotalAmount       float64
	PaymentProgress          float64
}

func buildCustomerHistoryExportView(historyData interface{}) (customerHistoryExportView, error) {
	var dataMap map[string]interface{}
	payload, err := json.Marshal(historyData)
	if err != nil {
		return customerHistoryExportView{}, err
	}
	if err := json.Unmarshal(payload, &dataMap); err != nil {
		return customerHistoryExportView{}, err
	}

	view := customerHistoryExportView{
		CustomerName: "Unknown Customer",
		StartDate:    getString(dataMap, "start_date"),
		EndDate:      getString(dataMap, "end_date"),
	}

	if customerMap, ok := dataMap["customer"].(map[string]interface{}); ok && customerMap != nil {
		if name := getString(customerMap, "name"); name != "" {
			view.CustomerName = name
		}
		view.CustomerCode = getString(customerMap, "code")
	}

	if transactions, exists := dataMap["transactions"]; exists {
		if txSlice, ok := transactions.([]interface{}); ok {
			for _, tx := range txSlice {
				txMap, ok := tx.(map[string]interface{})
				if !ok {
					continue
				}

				switch strings.ToUpper(getString(txMap, "transaction_type")) {
				case "SALE":
					view.SalesTransactions = append(view.SalesTransactions, txMap)
					view.SalesTotalAmount += getNumFrom(txMap["amount"])
					view.SalesTotalPaid += getNumFrom(txMap["paid_amount"])
					view.SalesTotalOutstanding += getNumFrom(txMap["outstanding"])
				case "PAYMENT":
					view.PaymentTransactions = append(view.PaymentTransactions, txMap)
					view.PaymentTotalAmount += getNumFrom(txMap["amount"])
				}
			}
		}
	}

	view.TotalActivities = len(view.SalesTransactions) + len(view.PaymentTransactions)
	if view.SalesTotalAmount > 0 {
		view.PaymentProgress = (view.SalesTotalPaid / view.SalesTotalAmount) * 100
	}

	return view, nil
}

func buildVendorHistoryExportView(historyData interface{}) (vendorHistoryExportView, error) {
	var dataMap map[string]interface{}
	payload, err := json.Marshal(historyData)
	if err != nil {
		return vendorHistoryExportView{}, err
	}
	if err := json.Unmarshal(payload, &dataMap); err != nil {
		return vendorHistoryExportView{}, err
	}

	view := vendorHistoryExportView{
		VendorName: "Unknown Vendor",
		StartDate:  getString(dataMap, "start_date"),
		EndDate:    getString(dataMap, "end_date"),
	}

	if vendorMap, ok := dataMap["vendor"].(map[string]interface{}); ok && vendorMap != nil {
		if name := getString(vendorMap, "name"); name != "" {
			view.VendorName = name
		}
		view.VendorCode = getString(vendorMap, "code")
	}

	if transactions, exists := dataMap["transactions"]; exists {
		if txSlice, ok := transactions.([]interface{}); ok {
			for _, tx := range txSlice {
				txMap, ok := tx.(map[string]interface{})
				if !ok {
					continue
				}

				switch strings.ToUpper(getString(txMap, "transaction_type")) {
				case "PURCHASE":
					view.PurchaseTransactions = append(view.PurchaseTransactions, txMap)
					view.PurchaseTotalAmount += getNumFrom(txMap["amount"])
					view.PurchaseTotalPaid += getNumFrom(txMap["paid_amount"])
					view.PurchaseTotalOutstanding += getNumFrom(txMap["outstanding"])
				case "PAYMENT":
					view.PaymentTransactions = append(view.PaymentTransactions, txMap)
					view.PaymentTotalAmount += getNumFrom(txMap["amount"])
				}
			}
		}
	}

	view.TotalActivities = len(view.PurchaseTransactions) + len(view.PaymentTransactions)
	if view.PurchaseTotalAmount > 0 {
		view.PaymentProgress = (view.PurchaseTotalPaid / view.PurchaseTotalAmount) * 100
	}

	return view, nil
}

func exportHistoryDate(raw string) string {
	if len(raw) > 10 {
		return raw[:10]
	}
	return raw
}

func exportHistoryTrim(value string, max int) string {
	if max <= 3 || len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}
