package services

import (
	"bytes"
	"fmt"
	"time"

	"app-sistem-akuntansi/utils"
)

// GenerateCustomerHistoryCSV generates CSV for customer transaction history
func (p *PDFService) GenerateCustomerHistoryCSV(historyData interface{}) ([]byte, error) {
	view, err := buildCustomerHistoryExportView(historyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse customer history data: %v", err)
	}

	var buf bytes.Buffer
	w := utils.NewSanitizedCSVWriter(&buf)

	// Header
	w.Write([]string{"CUSTOMER TRANSACTION HISTORY"})
	w.Write([]string{"Customer:", fmt.Sprintf("%s (%s)", view.CustomerName, view.CustomerCode)})
	w.Write([]string{"Period:", fmt.Sprintf("%s to %s", view.StartDate, view.EndDate)})
	w.Write([]string{"Generated:", time.Now().Format("2006-01-02 15:04")})
	w.Write([]string{})

	// Summary
	w.Write([]string{"SUMMARY"})
	w.Write([]string{"Total Activities:", fmt.Sprintf("%d", view.TotalActivities)})
	w.Write([]string{"Total Sales:", fmt.Sprintf("%.2f", view.SalesTotalAmount)})
	w.Write([]string{"Total Payments:", fmt.Sprintf("%.2f", view.PaymentTotalAmount)})
	w.Write([]string{"Outstanding:", fmt.Sprintf("%.2f", view.SalesTotalOutstanding)})
	w.Write([]string{})

	if len(view.SalesTransactions) > 0 {
		w.Write([]string{"SALES DOCUMENTS"})
		w.Write([]string{"Date", "Code", "Description", "Amount", "Paid Amount", "Outstanding", "Status"})
		for _, tx := range view.SalesTransactions {
			w.Write([]string{
				exportHistoryDate(getString(tx, "date")),
				getString(tx, "transaction_code"),
				getString(tx, "description"),
				fmt.Sprintf("%.2f", getNumFrom(tx["amount"])),
				fmt.Sprintf("%.2f", getNumFrom(tx["paid_amount"])),
				fmt.Sprintf("%.2f", getNumFrom(tx["outstanding"])),
				getString(tx, "status"),
			})
		}
		w.Write([]string{"", "", "TOTAL SALES", fmt.Sprintf("%.2f", view.SalesTotalAmount), fmt.Sprintf("%.2f", view.SalesTotalPaid), fmt.Sprintf("%.2f", view.SalesTotalOutstanding), ""})
		w.Write([]string{})
	}

	if len(view.PaymentTransactions) > 0 {
		w.Write([]string{"PAYMENTS"})
		w.Write([]string{"Date", "Code", "Description", "Reference", "Amount", "Status"})
		for _, tx := range view.PaymentTransactions {
			w.Write([]string{
				exportHistoryDate(getString(tx, "date")),
				getString(tx, "transaction_code"),
				getString(tx, "description"),
				getString(tx, "reference"),
				fmt.Sprintf("%.2f", getNumFrom(tx["amount"])),
				getString(tx, "status"),
			})
		}
		w.Write([]string{"", "", "", "TOTAL PAYMENTS", fmt.Sprintf("%.2f", view.PaymentTotalAmount), ""})
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("failed to write customer history CSV: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateVendorHistoryCSV generates CSV for vendor transaction history
func (p *PDFService) GenerateVendorHistoryCSV(historyData interface{}) ([]byte, error) {
	view, err := buildVendorHistoryExportView(historyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vendor history data: %v", err)
	}

	var buf bytes.Buffer
	w := utils.NewSanitizedCSVWriter(&buf)

	// Header
	w.Write([]string{"VENDOR TRANSACTION HISTORY"})
	w.Write([]string{"Vendor:", fmt.Sprintf("%s (%s)", view.VendorName, view.VendorCode)})
	w.Write([]string{"Period:", fmt.Sprintf("%s to %s", view.StartDate, view.EndDate)})
	w.Write([]string{"Generated:", time.Now().Format("2006-01-02 15:04")})
	w.Write([]string{})

	// Summary
	w.Write([]string{"SUMMARY"})
	w.Write([]string{"Total Activities:", fmt.Sprintf("%d", view.TotalActivities)})
	w.Write([]string{"Total Purchases:", fmt.Sprintf("%.2f", view.PurchaseTotalAmount)})
	w.Write([]string{"Total Payments:", fmt.Sprintf("%.2f", view.PaymentTotalAmount)})
	w.Write([]string{"Outstanding:", fmt.Sprintf("%.2f", view.PurchaseTotalOutstanding)})
	w.Write([]string{})

	if len(view.PurchaseTransactions) > 0 {
		w.Write([]string{"PURCHASE DOCUMENTS"})
		w.Write([]string{"Date", "Code", "Description", "Amount", "Paid Amount", "Outstanding", "Status"})
		for _, tx := range view.PurchaseTransactions {
			w.Write([]string{
				exportHistoryDate(getString(tx, "date")),
				getString(tx, "transaction_code"),
				getString(tx, "description"),
				fmt.Sprintf("%.2f", getNumFrom(tx["amount"])),
				fmt.Sprintf("%.2f", getNumFrom(tx["paid_amount"])),
				fmt.Sprintf("%.2f", getNumFrom(tx["outstanding"])),
				getString(tx, "status"),
			})
		}
		w.Write([]string{"", "", "TOTAL PURCHASES", fmt.Sprintf("%.2f", view.PurchaseTotalAmount), fmt.Sprintf("%.2f", view.PurchaseTotalPaid), fmt.Sprintf("%.2f", view.PurchaseTotalOutstanding), ""})
		w.Write([]string{})
	}

	if len(view.PaymentTransactions) > 0 {
		w.Write([]string{"PAYMENTS"})
		w.Write([]string{"Date", "Code", "Description", "Reference", "Amount", "Status"})
		for _, tx := range view.PaymentTransactions {
			w.Write([]string{
				exportHistoryDate(getString(tx, "date")),
				getString(tx, "transaction_code"),
				getString(tx, "description"),
				getString(tx, "reference"),
				fmt.Sprintf("%.2f", getNumFrom(tx["amount"])),
				getString(tx, "status"),
			})
		}
		w.Write([]string{"", "", "", "TOTAL PAYMENTS", fmt.Sprintf("%.2f", view.PaymentTotalAmount), ""})
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("failed to write vendor history CSV: %v", err)
	}
	return buf.Bytes(), nil
}
