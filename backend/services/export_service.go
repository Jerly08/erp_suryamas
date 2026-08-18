package services

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/utils"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ExportService interface {
	ExportAccountsPDF(ctx context.Context, userID uint) ([]byte, error)
	ExportAccountsExcel(ctx context.Context, userID uint) ([]byte, error)
}

type ExportServiceImpl struct {
	accountRepo repositories.AccountRepository
	db          *gorm.DB
}

func NewExportService(accountRepo repositories.AccountRepository, db *gorm.DB) ExportService {
	return &ExportServiceImpl{
		accountRepo: accountRepo,
		db:          db,
	}
}

// ExportAccountsPDF exports accounts to PDF format with localization
func (s *ExportServiceImpl) ExportAccountsPDF(ctx context.Context, userID uint) ([]byte, error) {
	// Get user language preference
	language := utils.GetUserLanguageFromDB(s.db, userID)

	// Get all accounts from database
	accounts, err := s.accountRepo.FindAll(ctx)
	if err != nil {
		return nil, utils.NewInternalError("Failed to fetch accounts", err)
	}
	utils.SanitizeExportData(accounts)
	sortAccountsForExport(accounts)

	// Create new PDF document
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Set font
	pdf.SetFont("Arial", "B", 16)

	// Title with localization
	pdf.Cell(190, 10, utils.T("chart_of_accounts", language))
	pdf.Ln(15)

	// Company info (you can get this from database)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(190, 5, fmt.Sprintf("%s: %s", utils.T("generated_on", language), time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(10)

	// Table headers with localization
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(30, 8, utils.T("account_code", language), "1", 0, "C", true, 0, "")
	pdf.CellFormat(60, 8, utils.T("account_name", language), "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, utils.T("account_type", language), "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, 8, utils.T("balance", language), "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, utils.T("status", language), "1", 0, "C", true, 0, "")
	pdf.Ln(8)

	// Table data
	pdf.SetFont("Arial", "", 9)
	pdf.SetFillColor(255, 255, 255)

	for _, account := range accounts {
		// Check if we need a new page
		if pdf.GetY() > 270 {
			pdf.AddPage()
			// Re-add headers with localization
			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(220, 220, 220)
			pdf.CellFormat(30, 8, utils.T("account_code", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(60, 8, utils.T("account_name", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(25, 8, utils.T("account_type", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(35, 8, utils.T("balance", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(30, 8, utils.T("status", language), "1", 0, "C", true, 0, "")
			pdf.Ln(8)
			pdf.SetFont("Arial", "", 9)
			pdf.SetFillColor(255, 255, 255)
		}

		// Account data with localized status
		status := utils.T("active", language)
		if !account.IsActive {
			status = utils.T("inactive", language)
		}

		balance := fmt.Sprintf("%.2f", account.Balance)

		pdf.CellFormat(30, 6, account.Code, "1", 0, "L", false, 0, "")
		pdf.CellFormat(60, 6, account.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, account.Type, "1", 0, "C", false, 0, "")
		pdf.CellFormat(35, 6, balance, "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 6, status, "1", 0, "C", false, 0, "")
		pdf.Ln(6)
	}

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, utils.NewInternalError("Failed to generate PDF", err)
	}

	return buf.Bytes(), nil
}

// ExportAccountsExcel exports accounts to Excel format with localization
func (s *ExportServiceImpl) ExportAccountsExcel(ctx context.Context, userID uint) ([]byte, error) {
	// Get user language preference
	language := utils.GetUserLanguageFromDB(s.db, userID)

	// Get all accounts from database
	accounts, err := s.accountRepo.FindAll(ctx)
	if err != nil {
		return nil, utils.NewInternalError("Failed to fetch accounts", err)
	}
	utils.SanitizeExportData(accounts)
	sortAccountsForExport(accounts)

	// Create new Excel file
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			// Log the error but don't fail the export
			// In a production environment, you would use a proper logger here
		}
	}()

	sheetName := "Chart of Accounts"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, utils.NewInternalError("Failed to create Excel sheet", err)
	}

	// Set active sheet
	f.SetActiveSheet(index)

	// Set title with localization
	f.SetCellValue(sheetName, "A1", utils.T("chart_of_accounts", language))
	f.SetCellValue(sheetName, "A2", fmt.Sprintf("%s: %s", utils.T("generated_on", language), time.Now().Format("2006-01-02 15:04:05")))

	// Headers with localization
	headers := utils.GetCSVHeaders("chart_of_accounts", language)
	for i, header := range headers {
		cell := string(rune('A'+i)) + "4"
		f.SetCellValue(sheetName, cell, header)
	}

	// Style for headers
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D3D3D3"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, utils.NewInternalError("Failed to create Excel style", err)
	}

	// Apply style to headers
	f.SetCellStyle(sheetName, "A4", "H4", headerStyle)

	// Data style
	dataStyle, err := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, utils.NewInternalError("Failed to create data style", err)
	}

	// Fill data
	for i, account := range accounts {
		row := i + 5 // Start from row 5 (after headers)

		status := utils.T("active", language)
		if !account.IsActive {
			status = utils.T("inactive", language)
		}

		f.SetCellValue(sheetName, "A"+strconv.Itoa(row), account.Code)
		f.SetCellValue(sheetName, "B"+strconv.Itoa(row), account.Name)
		f.SetCellValue(sheetName, "C"+strconv.Itoa(row), account.Type)
		f.SetCellValue(sheetName, "D"+strconv.Itoa(row), account.Category)
		f.SetCellValue(sheetName, "E"+strconv.Itoa(row), account.Balance)
		f.SetCellValue(sheetName, "F"+strconv.Itoa(row), status)
		f.SetCellValue(sheetName, "G"+strconv.Itoa(row), account.Description)
		f.SetCellValue(sheetName, "H"+strconv.Itoa(row), account.CreatedAt.Format("2006-01-02"))

		// Apply style to data row
		cellRange := "A" + strconv.Itoa(row) + ":H" + strconv.Itoa(row)
		f.SetCellStyle(sheetName, "A"+strconv.Itoa(row), "H"+strconv.Itoa(row), dataStyle)
		_ = cellRange
	}

	// Auto-fit columns
	cols := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	for _, col := range cols {
		f.SetColWidth(sheetName, col, col, 15)
	}

	// Make name column wider
	f.SetColWidth(sheetName, "B", "B", 25)
	f.SetColWidth(sheetName, "G", "G", 30) // Description column

	// Delete default Sheet1 if it exists
	if f.GetSheetName(0) == "Sheet1" {
		f.DeleteSheet("Sheet1")
	}

	// Save to buffer
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, utils.NewInternalError("Failed to write Excel file", err)
	}

	return buf.Bytes(), nil
}

func sortAccountsForExport(accounts []models.Account) {
	sort.SliceStable(accounts, func(i, j int) bool {
		comparison := compareAccountCodes(accounts[i].Code, accounts[j].Code)
		if comparison != 0 {
			return comparison < 0
		}

		if accounts[i].Name != accounts[j].Name {
			return accounts[i].Name < accounts[j].Name
		}

		return accounts[i].ID < accounts[j].ID
	})
}

func compareAccountCodes(left, right string) int {
	leftParts := strings.Split(strings.TrimSpace(left), "-")
	rightParts := strings.Split(strings.TrimSpace(right), "-")
	partCount := len(leftParts)
	if len(rightParts) < partCount {
		partCount = len(rightParts)
	}

	for index := 0; index < partCount; index++ {
		leftPart := leftParts[index]
		rightPart := rightParts[index]
		leftNumber, leftNumberErr := strconv.Atoi(leftPart)
		rightNumber, rightNumberErr := strconv.Atoi(rightPart)

		if leftNumberErr == nil && rightNumberErr == nil {
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
			continue
		}

		if leftPart < rightPart {
			return -1
		}
		if leftPart > rightPart {
			return 1
		}
	}

	if len(leftParts) < len(rightParts) {
		return -1
	}
	if len(leftParts) > len(rightParts) {
		return 1
	}

	if left < right {
		return -1
	}
	if left > right {
		return 1
	}

	return 0
}
