package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PayrollController struct {
	db *gorm.DB
}

const payrollExcelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

func NewPayrollController(db *gorm.DB) *PayrollController {
	return &PayrollController{db: db}
}

func canViewPayrollDetail(role string) bool {
	return models.NormalizeRole(role) == models.RoleAdmin ||
		models.IsHRRole(role) ||
		models.IsFinanceManagerRole(role)
}

func currentUserID(ctx *gin.Context) (uint, bool) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		return 0, false
	}

	switch value := userID.(type) {
	case uint:
		return value, true
	case int:
		return uint(value), true
	case float64:
		return uint(value), true
	default:
		return 0, false
	}
}

func respondPayrollClosedPeriod(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"error":   err.Error(),
		"code":    "PERIOD_CLOSED",
	})
}

func payrollDate(value string, required bool) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		if required {
			return nil, fmt.Errorf("date is required")
		}
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, use YYYY-MM-DD")
	}

	return &parsed, nil
}

func payrollResponse(run models.PayrollRun, detailAllowed bool) models.PayrollRunResponse {
	if !detailAllowed {
		run.Items = nil
		run.Settlements = nil
		run.Documents = nil
	}

	return models.PayrollRunResponse{
		PayrollRun:    run,
		DetailAllowed: detailAllowed,
	}
}

var payrollImportHeaderAliases = map[string]string{
	"paymentdate":               "payment_date",
	"paymentperiod":             "payment_period",
	"workingday":                "working_day",
	"employeeid":                "employee_code",
	"workunit":                  "work_unit",
	"paymentvia":                "payment_via",
	"accno":                     "account_no",
	"accountno":                 "account_no",
	"div":                       "division",
	"division":                  "division",
	"employeename":              "employee_name",
	"position":                  "position",
	"basicsalary":               "base_salary",
	"attendanceallowance":       "attendance_allowance",
	"attendance":                "attendance_allowance",
	"compensationpkwt":          "compensation_pkwt",
	"overtime":                  "overtime",
	"iuranjhtpemberikerja":      "bpjs_tk_jht_employer",
	"jhtpemberikerja":           "bpjs_tk_jht_employer",
	"bpjstkjhtpemberikerja":     "bpjs_tk_jht_employer",
	"iuranjhttenagakerja":       "bpjs_tk_jht_employee",
	"jhttenagakerja":            "bpjs_tk_jht_employee",
	"bpjstkjhttenagakerja":      "bpjs_tk_jht_employee",
	"bpjstkjht2":                "bpjs_tk_jht",
	"bpjstkjht":                 "bpjs_tk_jht",
	"iuranjppemberikerja":       "bpjs_tk_jp_employer",
	"jppemberikerja":            "bpjs_tk_jp_employer",
	"bpjstkjppemberikerja":      "bpjs_tk_jp_employer",
	"iuranjptenagakerja":        "bpjs_tk_jp_employee",
	"jptenagakerja":             "bpjs_tk_jp_employee",
	"bpjstkjptenagakerja":       "bpjs_tk_jp_employee",
	"bpjstkjp1":                 "bpjs_tk_jp",
	"bpjstkjp":                  "bpjs_tk_jp",
	"iuranjkkpemberikerja":      "bpjs_tk_jkk_employer",
	"jkkpemberikerja":           "bpjs_tk_jkk_employer",
	"bpjstkjkkpemberikerja":     "bpjs_tk_jkk_employer",
	"iuranjkktenagakerja":       "bpjs_tk_jkk_employee",
	"jkktenagakerja":            "bpjs_tk_jkk_employee",
	"bpjstkjkktenagakerja":      "bpjs_tk_jkk_employee",
	"iuranjkmpemberikerja":      "bpjs_tk_jkm_employer",
	"jkmpemberikerja":           "bpjs_tk_jkm_employer",
	"bpjstkjkmpemberikerja":     "bpjs_tk_jkm_employer",
	"iuranjkmtenagakerja":       "bpjs_tk_jkm_employee",
	"jkmtenagakerja":            "bpjs_tk_jkm_employee",
	"bpjstkjkmtenagakerja":      "bpjs_tk_jkm_employee",
	"bpjskesehatanpemberikerja": "bpjs_kesehatan_employer",
	"kesehatanpemberikerja":     "bpjs_kesehatan_employer",
	"bpjskesehatantenagakerja":  "bpjs_kesehatan_employee",
	"kesehatantenagakerja":      "bpjs_kesehatan_employee",
	"bpjskesehatan1":            "bpjs_kesehatan",
	"bpjskesehatan":             "bpjs_kesehatan",
	"proportional":              "proportional",
	"otherdeduction":            "other_deduction",
	"deductionpinjamankaryawan": "employee_loan_deduction",
	"pinjamankaryawan":          "employee_loan_deduction",
	"employeeloan":              "employee_loan_deduction",
	"employeeloandeduction":     "employee_loan_deduction",
	"loandeduction":             "employee_loan_deduction",
	"netpay":                    "net_pay",
	"pph":                       "pph",
	"pph21":                     "pph",
	"pph21terutang":             "pph",
	"incometax":                 "pph",
}

var requiredPayrollImportColumns = []string{
	"working_day",
	"employee_code",
	"work_unit",
	"payment_via",
	"account_no",
	"division",
	"employee_name",
	"position",
	"base_salary",
	"compensation_pkwt",
	"overtime",
	"proportional",
	"other_deduction",
	"net_pay",
}

var legacyPayrollBPJSImportColumns = []string{
	"bpjs_tk_jht",
	"bpjs_tk_jp",
	"bpjs_kesehatan",
}

var splitPayrollBPJSImportColumns = []string{
	"bpjs_tk_jht_employer",
	"bpjs_tk_jht_employee",
	"bpjs_tk_jp_employer",
	"bpjs_tk_jp_employee",
	"bpjs_tk_jkk_employer",
	"bpjs_tk_jkk_employee",
	"bpjs_tk_jkm_employer",
	"bpjs_tk_jkm_employee",
	"bpjs_kesehatan_employer",
	"bpjs_kesehatan_employee",
}

var payrollImportTemplateHeaders = []string{
	"PAYMENT DATE",
	"PAYMENT PERIOD",
	"WORKING DAY",
	"EMPLOYEE ID",
	"WORK UNIT",
	"DIV",
	"PAYMENT VIA",
	"ACC NO",
	"EMPLOYEE NAME",
	"POSITION",
	"BASIC SALARY",
	"ATTENDANCE ALLOWANCE",
	"COMPENSATION PKWT",
	"OVERTIME",
	"PPH21 TERUTANG",
	"IURAN JHT\n(PEMBERI KERJA)",
	"IURAN JHT\n(TENAGA KERJA)",
	"IURAN JP\n(PEMBERI KERJA)",
	"IURAN JP\n(TENAGA KERJA)",
	"IURAN JKK\n(PEMBERI KERJA)",
	"IURAN JKK\n(TENAGA KERJA)",
	"IURAN JKM\n(PEMBERI KERJA)",
	"IURAN JKM\n(TENAGA KERJA)",
	"BPJS KESEHATAN\n(PEMBERI KERJA)",
	"BPJS KESEHATAN\n(TENAGA KERJA)",
	"PROPORTIONAL",
	"OTHER DEDUCTION",
	"DEDUCTION (PINJAMAN KARYAWAN)",
	"NET PAY",
}

func payrollExcelSafeFilename(payrollNumber string) string {
	filename := strings.TrimSpace(payrollNumber)
	if filename == "" {
		filename = "payroll"
	}
	filename = strings.NewReplacer("/", "-", "\\", "-", " ", "_").Replace(filename)
	filename = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`).ReplaceAllString(filename, "-")
	filename = strings.Trim(filename, "-_.")
	if filename == "" {
		filename = "payroll"
	}
	return filename + "_pengajuan.xlsx"
}

func payrollRunPeriodLabel(run *models.PayrollRun) string {
	if run == nil {
		return ""
	}
	return fmt.Sprintf("%s to %s", run.PeriodStart.Format("02/01/2006"), run.PeriodEnd.Format("02/01/2006"))
}

func buildPayrollRunExcel(run *models.PayrollRun) ([]byte, error) {
	workbook := excelize.NewFile()
	defer workbook.Close()

	sheetName := "Salary"
	workbook.SetSheetName("Sheet1", sheetName)

	for index, header := range payrollImportTemplateHeaders {
		cell, err := excelize.CoordinatesToCellName(index+1, 1)
		if err != nil {
			return nil, err
		}
		if err := workbook.SetCellValue(sheetName, cell, header); err != nil {
			return nil, err
		}
	}

	headerStyle, _ := workbook.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"305496"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	dateStyle, _ := workbook.NewStyle(&excelize.Style{NumFmt: 14})
	numberStyle, _ := workbook.NewStyle(&excelize.Style{NumFmt: 4})

	workbook.SetCellStyle(sheetName, "A1", "AC1", headerStyle)
	workbook.SetRowHeight(sheetName, 1, 34)
	workbook.SetColWidth(sheetName, "A", "AC", 18)
	workbook.SetColWidth(sheetName, "B", "B", 24)
	workbook.SetColWidth(sheetName, "I", "I", 28)
	workbook.SetColWidth(sheetName, "J", "J", 24)
	workbook.SetColWidth(sheetName, "P", "Y", 22)
	workbook.SetColWidth(sheetName, "AB", "AB", 28)

	periodLabel := payrollRunPeriodLabel(run)
	for index, item := range run.Items {
		row := index + 2
		if run.PaymentDate != nil {
			workbook.SetCellValue(sheetName, fmt.Sprintf("A%d", row), *run.PaymentDate)
		}
		workbook.SetCellValue(sheetName, fmt.Sprintf("B%d", row), periodLabel)
		workbook.SetCellValue(sheetName, fmt.Sprintf("C%d", row), item.WorkingDay)
		workbook.SetCellValue(sheetName, fmt.Sprintf("D%d", row), item.EmployeeCode)
		workbook.SetCellValue(sheetName, fmt.Sprintf("E%d", row), item.WorkUnit)
		workbook.SetCellValue(sheetName, fmt.Sprintf("F%d", row), item.Division)
		workbook.SetCellValue(sheetName, fmt.Sprintf("G%d", row), item.PaymentVia)
		workbook.SetCellValue(sheetName, fmt.Sprintf("H%d", row), item.AccountNo)
		workbook.SetCellValue(sheetName, fmt.Sprintf("I%d", row), item.EmployeeName)
		workbook.SetCellValue(sheetName, fmt.Sprintf("J%d", row), item.Position)
		workbook.SetCellValue(sheetName, fmt.Sprintf("K%d", row), item.BaseSalary)
		workbook.SetCellValue(sheetName, fmt.Sprintf("L%d", row), item.AttendanceAllowance)
		workbook.SetCellValue(sheetName, fmt.Sprintf("M%d", row), item.CompensationPKWT)
		workbook.SetCellValue(sheetName, fmt.Sprintf("N%d", row), item.Overtime)
		workbook.SetCellValue(sheetName, fmt.Sprintf("O%d", row), item.PPh)
		workbook.SetCellValue(sheetName, fmt.Sprintf("P%d", row), item.BPJSTKJHTEmployer)
		workbook.SetCellValue(sheetName, fmt.Sprintf("Q%d", row), payrollModelBPJSTKJHTEmployee(item))
		workbook.SetCellValue(sheetName, fmt.Sprintf("R%d", row), item.BPJSTKJPEmployer)
		workbook.SetCellValue(sheetName, fmt.Sprintf("S%d", row), payrollModelBPJSTKJPEmployee(item))
		workbook.SetCellValue(sheetName, fmt.Sprintf("T%d", row), item.BPJSTKJKKEmployer)
		workbook.SetCellValue(sheetName, fmt.Sprintf("U%d", row), item.BPJSTKJKKEmployee)
		workbook.SetCellValue(sheetName, fmt.Sprintf("V%d", row), item.BPJSTKJKMEmployer)
		workbook.SetCellValue(sheetName, fmt.Sprintf("W%d", row), item.BPJSTKJKMEmployee)
		workbook.SetCellValue(sheetName, fmt.Sprintf("X%d", row), item.BPJSKesehatanEmployer)
		workbook.SetCellValue(sheetName, fmt.Sprintf("Y%d", row), payrollModelBPJSKesehatanEmployee(item))
		workbook.SetCellValue(sheetName, fmt.Sprintf("Z%d", row), item.Proportional)
		workbook.SetCellValue(sheetName, fmt.Sprintf("AA%d", row), item.OtherDeduction)
		workbook.SetCellValue(sheetName, fmt.Sprintf("AB%d", row), item.EmployeeLoanDeduction)
		workbook.SetCellValue(sheetName, fmt.Sprintf("AC%d", row), item.NetPay)
	}

	if len(run.Items) > 0 {
		lastRow := len(run.Items) + 1
		workbook.SetCellStyle(sheetName, "A2", fmt.Sprintf("A%d", lastRow), dateStyle)
		workbook.SetCellStyle(sheetName, "C2", fmt.Sprintf("C%d", lastRow), numberStyle)
		workbook.SetCellStyle(sheetName, "K2", fmt.Sprintf("AC%d", lastRow), numberStyle)
	}

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func payrollItemGross(item *models.PayrollItemCreateRequest) float64 {
	if item == nil {
		return 0
	}
	return item.BaseSalary + item.Bonus
}

func payrollCreateBPJSSplitTotal(item *models.PayrollItemCreateRequest) float64 {
	if item == nil {
		return 0
	}
	return item.BPJSTKJHTEmployer +
		item.BPJSTKJHTEmployee +
		item.BPJSTKJPEmployer +
		item.BPJSTKJPEmployee +
		item.BPJSTKJKKEmployer +
		item.BPJSTKJKKEmployee +
		item.BPJSTKJKMEmployer +
		item.BPJSTKJKMEmployee +
		item.BPJSKesehatanEmployer +
		item.BPJSKesehatanEmployee
}

func payrollCreateBPJSEmployerTotal(item *models.PayrollItemCreateRequest) float64 {
	if item == nil {
		return 0
	}
	return roundPayrollMoney(item.BPJSTKJHTEmployer + item.BPJSTKJPEmployer + item.BPJSTKJKKEmployer + item.BPJSTKJKMEmployer + item.BPJSKesehatanEmployer)
}

func payrollCreateBPJSEmployeeTotal(item *models.PayrollItemCreateRequest) float64 {
	if item == nil {
		return 0
	}
	splitTotal := item.BPJSTKJHTEmployee + item.BPJSTKJPEmployee + item.BPJSTKJKKEmployee + item.BPJSTKJKMEmployee + item.BPJSKesehatanEmployee
	if splitTotal > 0.01 {
		return roundPayrollMoney(splitTotal)
	}
	return roundPayrollMoney(item.BPJSTKJHT + item.BPJSTKJP + item.BPJSKesehatan)
}

func payrollCreateBPJSTotal(item *models.PayrollItemCreateRequest) float64 {
	if item == nil {
		return 0
	}
	splitTotal := payrollCreateBPJSSplitTotal(item)
	if splitTotal > 0.01 {
		return roundPayrollMoney(splitTotal)
	}
	if item.BPJS > 0.01 {
		return roundPayrollMoney(item.BPJS)
	}
	return roundPayrollMoney(item.BPJSTKJHT + item.BPJSTKJP + item.BPJSKesehatan)
}

func normalizePayrollCreateBPJSComponents(item *models.PayrollItemCreateRequest) {
	if item == nil {
		return
	}
	if payrollCreateBPJSSplitTotal(item) <= 0.01 {
		item.BPJSTKJHTEmployee = item.BPJSTKJHT
		item.BPJSTKJPEmployee = item.BPJSTKJP
		item.BPJSKesehatanEmployee = item.BPJSKesehatan
	}
	item.BPJSTKJHT = item.BPJSTKJHTEmployee
	item.BPJSTKJP = item.BPJSTKJPEmployee
	item.BPJSKesehatan = item.BPJSKesehatanEmployee
	item.BPJS = payrollCreateBPJSTotal(item)
}

func payrollModelBPJSTKJHTEmployee(item models.PayrollItem) float64 {
	if item.BPJSTKJHTEmployee > 0.01 {
		return item.BPJSTKJHTEmployee
	}
	return item.BPJSTKJHT
}

func payrollModelBPJSTKJPEmployee(item models.PayrollItem) float64 {
	if item.BPJSTKJPEmployee > 0.01 {
		return item.BPJSTKJPEmployee
	}
	return item.BPJSTKJP
}

func payrollModelBPJSKesehatanEmployee(item models.PayrollItem) float64 {
	if item.BPJSKesehatanEmployee > 0.01 {
		return item.BPJSKesehatanEmployee
	}
	return item.BPJSKesehatan
}

func payrollCalculatedNetPay(item *models.PayrollItemCreateRequest) float64 {
	if item == nil {
		return 0
	}
	return roundPayrollMoney(payrollItemGross(item) - payrollCreateBPJSEmployeeTotal(item) - item.OtherDeduction - item.EmployeeLoanDeduction)
}

func normalizePayrollImportHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func payrollHeaderKey(value string) string {
	normalized := normalizePayrollImportHeader(value)
	if key, ok := payrollImportHeaderAliases[normalized]; ok {
		return key
	}
	return normalized
}

func hasPayrollImportColumns(headerIndex map[string]int, columns []string) bool {
	for _, column := range columns {
		if _, ok := headerIndex[column]; !ok {
			return false
		}
	}
	return true
}

func missingPayrollImportColumns(headerIndex map[string]int) []string {
	missingColumns := []string{}
	for _, requiredColumn := range requiredPayrollImportColumns {
		if _, ok := headerIndex[requiredColumn]; !ok {
			missingColumns = append(missingColumns, requiredColumn)
		}
	}

	if !hasPayrollImportColumns(headerIndex, splitPayrollBPJSImportColumns) && !hasPayrollImportColumns(headerIndex, legacyPayrollBPJSImportColumns) {
		missingColumns = append(missingColumns, "bpjs split columns or legacy bpjs columns")
	}

	return missingColumns
}

func parsePayrollImportNumber(value string) float64 {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" || cleaned == "-" {
		return 0
	}

	cleaned = strings.ToUpper(cleaned)
	cleaned = strings.ReplaceAll(cleaned, "IDR", "")
	cleaned = strings.ReplaceAll(cleaned, "RP", "")
	cleaned = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, cleaned)

	negative := strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")")
	if negative {
		cleaned = strings.TrimSuffix(strings.TrimPrefix(cleaned, "("), ")")
	}

	if strings.Contains(cleaned, ",") && strings.Contains(cleaned, ".") {
		lastComma := strings.LastIndex(cleaned, ",")
		lastDot := strings.LastIndex(cleaned, ".")
		if lastComma > lastDot {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
			cleaned = strings.ReplaceAll(cleaned, ",", ".")
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		}
	} else if strings.Count(cleaned, ",") > 1 {
		cleaned = strings.ReplaceAll(cleaned, ",", "")
	} else if strings.Count(cleaned, ",") == 1 {
		separatorIndex := strings.LastIndex(cleaned, ",")
		if len(cleaned)-separatorIndex-1 == 3 {
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", ".")
		}
	} else if strings.Count(cleaned, ".") > 1 {
		cleaned = strings.ReplaceAll(cleaned, ".", "")
	} else if strings.Count(cleaned, ".") == 1 {
		separatorIndex := strings.LastIndex(cleaned, ".")
		if len(cleaned)-separatorIndex-1 == 3 {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
		}
	}

	amount, _ := strconv.ParseFloat(cleaned, 64)
	if negative {
		amount = -amount
	}
	return amount
}

func getPayrollImportValue(row []string, headerIndex map[string]int, key string) string {
	index, ok := headerIndex[key]
	if !ok || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func getPayrollImportNumber(row []string, headerIndex map[string]int, key string) float64 {
	return parsePayrollImportNumber(getPayrollImportValue(row, headerIndex, key))
}

var payrollImportDatePattern = regexp.MustCompile(`\d{4}-\d{1,2}-\d{1,2}|\d{1,2}[/-]\d{1,2}[/-]\d{2,4}`)

func parsePayrollImportDate(value string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return ""
	}

	if serial, err := strconv.ParseFloat(cleaned, 64); err == nil && serial > 20000 {
		date := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(math.Floor(serial)))
		return date.Format("2006-01-02")
	}

	layouts := []string{
		"2006-01-02",
		"2006-1-2",
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2-1-2006",
		"02/01/06",
		"2/1/06",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, cleaned); err == nil {
			return parsed.Format("2006-01-02")
		}
	}

	return ""
}

func parsePayrollImportPeriod(value string) (string, string) {
	matches := payrollImportDatePattern.FindAllString(value, 2)
	if len(matches) < 2 {
		return "", ""
	}
	return parsePayrollImportDate(matches[0]), parsePayrollImportDate(matches[1])
}

func payrollImportItemTotals(item *models.PayrollItemCreateRequest) {
	componentBonus := item.AttendanceAllowance + item.CompensationPKWT + item.Overtime + item.Proportional
	if componentBonus > 0 {
		item.Bonus = componentBonus
	} else if item.Bonus == 0 {
		item.Bonus = item.CompensationPKWT + item.Overtime + item.Proportional
	}
	normalizePayrollCreateBPJSComponents(item)
	if item.Department == "" {
		item.Department = item.WorkUnit
	}
	item.NetPay = payrollCalculatedNetPay(item)
}

func roundPayrollMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func remainingPayrollPPh(run *models.PayrollRun) float64 {
	if run == nil {
		return 0
	}
	remaining := roundPayrollMoney(run.TotalPPh - run.PPhPaidAmount)
	if remaining < 0.01 {
		return 0
	}
	return remaining
}

func payrollPPhPaymentStatus(totalPPh float64, paidAmount float64) string {
	totalPPh = roundPayrollMoney(totalPPh)
	paidAmount = roundPayrollMoney(paidAmount)
	if totalPPh <= 0.01 || paidAmount >= totalPPh-0.01 {
		return models.PayrollPPhPaymentStatusPaid
	}
	if paidAmount > 0.01 {
		return models.PayrollPPhPaymentStatusPartialPaid
	}
	return models.PayrollPPhPaymentStatusUnpaid
}

func payrollSalaryPaymentStatus(totalSalary float64, paidAmount float64) string {
	if totalSalary <= 0.01 || paidAmount >= totalSalary-0.01 {
		return models.PayrollSalaryPaymentStatusPaid
	}
	if paidAmount > 0.01 {
		return models.PayrollSalaryPaymentStatusPartialPaid
	}
	return models.PayrollSalaryPaymentStatusUnpaid
}

func payrollBPJSPaymentStatus(totalBPJS float64, paidAmount float64) string {
	totalBPJS = roundPayrollMoney(totalBPJS)
	paidAmount = roundPayrollMoney(paidAmount)
	if totalBPJS <= 0.01 || paidAmount >= totalBPJS-0.01 {
		return models.PayrollSalaryPaymentStatusPaid
	}
	if paidAmount > 0.01 {
		return models.PayrollSalaryPaymentStatusPartialPaid
	}
	return models.PayrollSalaryPaymentStatusUnpaid
}

func remainingPayrollSalary(run *models.PayrollRun) float64 {
	if run == nil {
		return 0
	}
	remaining := roundPayrollMoney(run.TotalOverall - run.SalaryPaidAmount)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func remainingPayrollBPJS(run *models.PayrollRun) float64 {
	if run == nil {
		return 0
	}
	remaining := roundPayrollMoney(run.TotalBPJS - run.BPJSPaidAmount)
	if remaining < 0.01 {
		return 0
	}
	return remaining
}

func defaultPayrollRecognitionDate(run *models.PayrollRun) time.Time {
	date := run.PeriodEnd
	if run.PaymentDate != nil && run.PaymentDate.Before(date) {
		date = *run.PaymentDate
	}
	if date.Before(run.PeriodStart) {
		date = run.PeriodStart
	}
	return date
}

func resolvePayrollRecognitionDate(tx *gorm.DB, run *models.PayrollRun) (time.Time, error) {
	if run == nil {
		return time.Time{}, fmt.Errorf("payroll is required")
	}
	if run.PPhPayableJournalEntryID != nil {
		var journal models.SSOTJournalEntry
		if err := tx.Select("id", "entry_date").First(&journal, *run.PPhPayableJournalEntryID).Error; err == nil {
			return journal.EntryDate, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, fmt.Errorf("failed to load payroll accrual journal: %w", err)
		}
	}
	if run.PPhRecognitionDate != nil {
		return *run.PPhRecognitionDate, nil
	}
	return defaultPayrollRecognitionDate(run), nil
}

func resolvePayrollAccountByCodes(tx *gorm.DB, codes ...string) (*models.Account, error) {
	var account models.Account
	for _, code := range codes {
		if strings.TrimSpace(code) == "" {
			continue
		}
		if err := tx.
			Where("code = ? AND deleted_at IS NULL AND is_active = ? AND is_header = ?", code, true, false).
			First(&account).Error; err == nil {
			return &account, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func resolvePayrollPPhPayableAccount(tx *gorm.DB) (*models.Account, error) {
	if account, err := resolvePayrollAccountByCodes(tx, "2102-003", "2104"); err == nil {
		return account, nil
	}

	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND LOWER(name) LIKE ? AND LOWER(name) LIKE ?",
			true,
			false,
			models.AccountTypeLiability,
			"%pph%",
			"%21%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	return nil, fmt.Errorf("akun Hutang PPh 21 tidak ditemukan")
}

func resolvePayrollSalaryExpenseAccount(tx *gorm.DB) (*models.Account, error) {
	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND (LOWER(name) LIKE ? OR LOWER(name) LIKE ? OR LOWER(name) LIKE ?)",
			true,
			false,
			models.AccountTypeExpense,
			"%gaji%",
			"%salary%",
			"%upah%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	if account, err := resolvePayrollAccountByCodes(tx, "6105", "5101-001", "5201", "6101"); err == nil {
		return account, nil
	}

	return nil, fmt.Errorf("akun Beban Gaji tidak ditemukan")
}

func resolvePayrollSalaryPayableAccount(tx *gorm.DB) (*models.Account, error) {
	if account, err := resolvePayrollAccountByCodes(tx, "2103-001", "2103"); err == nil {
		return account, nil
	}

	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND (LOWER(name) LIKE ? OR LOWER(name) LIKE ?)",
			true,
			false,
			models.AccountTypeLiability,
			"%utang gaji%",
			"%hutang gaji%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	return nil, fmt.Errorf("akun Utang Gaji tidak ditemukan")
}

func resolvePayrollDeductionPayableAccount(tx *gorm.DB) (*models.Account, error) {
	if account, err := resolvePayrollAccountByCodes(tx, "2103-002"); err == nil {
		return account, nil
	}

	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND (LOWER(name) LIKE ? OR LOWER(name) LIKE ?)",
			true,
			false,
			models.AccountTypeLiability,
			"%bpjs%",
			"%potongan payroll%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	return nil, fmt.Errorf("akun Utang BPJS & Potongan Payroll (2103-002) tidak ditemukan")
}

func resolvePayrollEmployeeLoanAccount(tx *gorm.DB) (*models.Account, error) {
	if account, err := resolvePayrollAccountByCodes(tx, "1107-001"); err == nil {
		return account, nil
	}

	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND LOWER(name) LIKE ? AND LOWER(name) LIKE ?",
			true,
			false,
			models.AccountTypeAsset,
			"%pinjaman%",
			"%karyawan%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	return nil, fmt.Errorf("akun Pinjaman Karyawan (1107-001) tidak ditemukan")
}

func validatePayrollEmployeeLoanAccount(account models.Account) error {
	if !account.IsActive || account.IsHeader || account.Type != models.AccountTypeAsset {
		return fmt.Errorf("akun Pinjaman Karyawan harus akun detail aset yang aktif")
	}
	code := strings.TrimSpace(account.Code)
	name := strings.ToLower(strings.TrimSpace(account.Name))
	if !strings.EqualFold(code, "1107-001") && !(strings.Contains(name, "pinjaman") && strings.Contains(name, "karyawan")) {
		return fmt.Errorf("akun Pinjaman Karyawan harus menggunakan COA 1107-001 atau akun aset Pinjaman Karyawan")
	}
	return nil
}

func resolvePayrollPPh21ExpenseAccount(tx *gorm.DB, fallback *models.Account) (*models.Account, error) {
	if account, err := resolvePayrollAccountByCodes(tx, "9101"); err == nil {
		return account, nil
	}

	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND (LOWER(name) LIKE ? OR LOWER(name) LIKE ?)",
			true,
			false,
			models.AccountTypeExpense,
			"%pph%",
			"%pajak%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	if fallback != nil {
		return fallback, nil
	}

	return nil, fmt.Errorf("akun Beban PPh 21 tidak ditemukan")
}

func resolvePayrollBPJSEmployerExpenseAccount(tx *gorm.DB, fallback *models.Account) (*models.Account, error) {
	if account, err := resolvePayrollAccountByCodes(tx, "6107"); err == nil {
		return account, nil
	}

	var account models.Account
	if err := tx.
		Where("deleted_at IS NULL AND is_active = ? AND is_header = ? AND type = ? AND LOWER(name) LIKE ? AND (LOWER(name) LIKE ? OR LOWER(name) LIKE ?)",
			true,
			false,
			models.AccountTypeExpense,
			"%bpjs%",
			"%beban%",
			"%iuran%",
		).
		Order("code ASC").
		First(&account).Error; err == nil {
		return &account, nil
	}

	if fallback != nil {
		return fallback, nil
	}

	return nil, fmt.Errorf("akun Beban BPJS Pemberi Kerja tidak ditemukan")
}

func (c *PayrollController) createPayrollAccrualJournal(tx *gorm.DB, run *models.PayrollRun, userID uint, customExpenseAccountID *uint, customEmployeeLoanAccountID *uint, recognitionDate time.Time) (*uint64, error) {
	if run == nil || (run.TotalSalary+run.TotalBonus+run.TotalPPh+run.TotalBPJS <= 0.01) {
		return nil, nil
	}

	sourceID := uint64(run.ID)
	var existing models.SSOTJournalEntry
	if err := tx.
		Where("source_type IN ? AND source_id = ? AND deleted_at IS NULL", []string{"PAYROLL_ACCRUAL", "PAYROLL_PPH_PAYABLE"}, sourceID).
		Order("id ASC").
		First(&existing).Error; err == nil {
		return &existing.ID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check payroll accrual journal: %w", err)
	}

	var expenseAccount *models.Account
	var err error
	if customExpenseAccountID != nil && *customExpenseAccountID > 0 {
		var acc models.Account
		if err = tx.First(&acc, *customExpenseAccountID).Error; err != nil {
			return nil, fmt.Errorf("akun Beban Gaji kustom tidak ditemukan: %w", err)
		}
		expenseAccount = &acc
	} else {
		expenseAccount, err = resolvePayrollSalaryExpenseAccount(tx)
		if err != nil {
			return nil, err
		}
	}

	var pphPayableAccount *models.Account
	if run.TotalPPh > 0.01 {
		pphPayableAccount, err = resolvePayrollPPhPayableAccount(tx)
		if err != nil {
			return nil, err
		}
	}

	salaryPayableAccount, err := resolvePayrollSalaryPayableAccount(tx)
	if err != nil {
		return nil, err
	}

	var deductionPayableAccount *models.Account
	grossSalary := roundPayrollMoney(run.TotalSalary + run.TotalBonus)
	employeeLoanDeduction := roundPayrollMoney(run.TotalEmployeeLoanDeduction)
	employerBPJS := roundPayrollMoney(run.TotalBPJSEmployer)
	withheldDeductions := roundPayrollMoney(grossSalary - run.TotalOverall - employeeLoanDeduction)
	if withheldDeductions < 0 {
		withheldDeductions = 0
	}
	payrollDeductionLiability := roundPayrollMoney(withheldDeductions + employerBPJS)
	if payrollDeductionLiability > 0.01 {
		deductionPayableAccount, err = resolvePayrollDeductionPayableAccount(tx)
		if err != nil {
			return nil, err
		}
	}

	var employeeLoanAccount *models.Account
	if employeeLoanDeduction > 0.01 {
		if customEmployeeLoanAccountID != nil && *customEmployeeLoanAccountID > 0 {
			var acc models.Account
			if err = tx.First(&acc, *customEmployeeLoanAccountID).Error; err != nil {
				return nil, fmt.Errorf("akun Pinjaman Karyawan kustom tidak ditemukan: %w", err)
			}
			if err = validatePayrollEmployeeLoanAccount(acc); err != nil {
				return nil, err
			}
			employeeLoanAccount = &acc
		} else {
			employeeLoanAccount, err = resolvePayrollEmployeeLoanAccount(tx)
			if err != nil {
				return nil, err
			}
		}
	}

	var lines []services.JournalLineRequest

	lines = append(lines, services.JournalLineRequest{
		AccountID:    uint64(expenseAccount.ID),
		DebitAmount:  decimal.NewFromFloat(grossSalary),
		CreditAmount: decimal.Zero,
		Description:  fmt.Sprintf("Beban Gaji & Tunjangan Periode %s - %s", run.PayrollNumber, run.Notes),
	})

	if employerBPJS > 0.01 {
		bpjsEmployerExpenseAccount, err := resolvePayrollBPJSEmployerExpenseAccount(tx, expenseAccount)
		if err != nil {
			return nil, err
		}
		lines = append(lines, services.JournalLineRequest{
			AccountID:    uint64(bpjsEmployerExpenseAccount.ID),
			DebitAmount:  decimal.NewFromFloat(employerBPJS),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Beban BPJS pemberi kerja Periode %s", run.PayrollNumber),
		})
	}

	if run.TotalPPh > 0.01 && pphPayableAccount != nil {
		pphExpenseAccount, err := resolvePayrollPPh21ExpenseAccount(tx, expenseAccount)
		if err != nil {
			return nil, err
		}
		lines = append(lines, services.JournalLineRequest{
			AccountID:    uint64(pphExpenseAccount.ID),
			DebitAmount:  decimal.NewFromFloat(roundPayrollMoney(run.TotalPPh)),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Beban PPh 21 ditanggung perusahaan Periode %s", run.PayrollNumber),
		})
		lines = append(lines, services.JournalLineRequest{
			AccountID:    uint64(pphPayableAccount.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(roundPayrollMoney(run.TotalPPh)),
			Description:  fmt.Sprintf("Hutang PPh 21 Karyawan Periode %s", run.PayrollNumber),
		})
	}

	lines = append(lines, services.JournalLineRequest{
		AccountID:    uint64(salaryPayableAccount.ID),
		DebitAmount:  decimal.Zero,
		CreditAmount: decimal.NewFromFloat(roundPayrollMoney(run.TotalOverall)),
		Description:  fmt.Sprintf("Utang Net Gaji Karyawan Periode %s", run.PayrollNumber),
	})

	if payrollDeductionLiability > 0.01 && deductionPayableAccount != nil {
		lines = append(lines, services.JournalLineRequest{
			AccountID:    uint64(deductionPayableAccount.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(payrollDeductionLiability),
			Description:  fmt.Sprintf("Utang BPJS & Potongan Payroll Periode %s", run.PayrollNumber),
		})
	}

	if employeeLoanDeduction > 0.01 && employeeLoanAccount != nil {
		lines = append(lines, services.JournalLineRequest{
			AccountID:    uint64(employeeLoanAccount.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(employeeLoanDeduction),
			Description:  fmt.Sprintf("Pelunasan Pinjaman Karyawan via Payroll Periode %s", run.PayrollNumber),
		})
	}

	journalService := services.NewUnifiedJournalService(tx)
	journalEntry, err := journalService.CreateJournalEntryWithTx(tx, &services.JournalEntryRequest{
		EntryDate:   recognitionDate,
		Reference:   fmt.Sprintf("PAYROLL-%s", run.PayrollNumber),
		Description: fmt.Sprintf("Jurnal Pengakuan Payroll - %s", run.PayrollNumber),
		Notes:       "Beban gaji, BPJS pemberi kerja, dan PPh 21 yang ditanggung perusahaan diakui sebagai kewajiban. Tidak ada pengurangan kas/bank sampai settlement aktual.",
		SourceType:  "PAYROLL_ACCRUAL",
		SourceID:    sourceID,
		AutoPost:    true,
		CreatedBy:   uint64(userID),
		Lines:       lines,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create payroll accrual journal: %w", err)
	}

	return &journalEntry.ID, nil
}

func (c *PayrollController) nextPayrollNumber(tx *gorm.DB) (string, error) {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0)

	var count int64
	if err := tx.Model(&models.PayrollRun{}).
		Where("created_at >= ? AND created_at < ?", periodStart, periodEnd).
		Count(&count).Error; err != nil {
		return "", err
	}

	return fmt.Sprintf("PAY/%04d/%02d/%04d", now.Year(), now.Month(), count+1), nil
}

func (c *PayrollController) GetPayrollRuns(ctx *gin.Context) {
	role := ctx.GetString("role")
	detailAllowed := canViewPayrollDetail(role)

	query := c.db.Model(&models.PayrollRun{}).
		Preload("CreatedBy").
		Preload("ApprovedBy").
		Preload("RejectedBy").
		Preload("EmployeeLoanAccount").
		Order("created_at DESC")
	if detailAllowed {
		query = query.Preload("Documents", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).Preload("Documents.Uploader")
	}

	if status := strings.TrimSpace(ctx.Query("status")); status != "" {
		query = query.Where("status = ?", strings.ToUpper(status))
	}

	var runs []models.PayrollRun
	if err := query.Find(&runs).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll data"})
		return
	}

	responses := make([]models.PayrollRunResponse, 0, len(runs))
	for _, run := range runs {
		responses = append(responses, payrollResponse(run, detailAllowed))
	}

	ctx.JSON(http.StatusOK, gin.H{"data": responses})
}

func (c *PayrollController) GetPayrollRun(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	role := ctx.GetString("role")
	detailAllowed := canViewPayrollDetail(role)

	query := c.db.Preload("CreatedBy").Preload("ApprovedBy").Preload("RejectedBy").Preload("EmployeeLoanAccount")
	if detailAllowed {
		query = query.Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).Preload("Settlements", func(db *gorm.DB) *gorm.DB {
			return db.Order("payment_date ASC, id ASC")
		}).Preload("Settlements.CashBank").
			Preload("Documents", func(db *gorm.DB) *gorm.DB {
				return db.Order("created_at ASC")
			}).
			Preload("Documents.Uploader")
	}

	var run models.PayrollRun
	if err := query.First(&run, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll detail"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": payrollResponse(run, detailAllowed)})
}

func (c *PayrollController) UploadDocuments(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	var run models.PayrollRun
	if err := c.db.First(&run, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll"})
		return
	}
	if run.Status == models.PayrollStatusRejected {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "documents cannot be uploaded to a rejected payroll"})
		return
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form", "details": err.Error()})
		return
	}

	documentFiles := append([]*multipart.FileHeader{}, form.File["documents"]...)
	documentFiles = append(documentFiles, form.File["billing_documents"]...)
	documentFiles = append(documentFiles, form.File["realization_documents"]...)
	documentFiles = append(documentFiles, form.File["attachments"]...)
	documentFiles = append(documentFiles, form.File["document"]...)
	if len(documentFiles) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	const uploadDir = "./uploads/payroll"
	documents := make([]models.PayrollDocument, 0, len(documentFiles))
	savedPaths := make([]string, 0, len(documentFiles))

	for _, fileHeader := range documentFiles {
		filename, filePath, mimeType, err := prepareValidatedDocumentUpload(fileHeader, uint(id), uploadDir)
		if err != nil {
			cleanupUploadedFiles(savedPaths)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := ctx.SaveUploadedFile(fileHeader, filePath); err != nil {
			cleanupUploadedFiles(savedPaths)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Dokumen belum bisa disimpan di server. Hubungi admin untuk mengecek folder upload."})
			return
		}
		savedPaths = append(savedPaths, filePath)
		documents = append(documents, models.PayrollDocument{
			PayrollRunID: uint(id),
			DocumentType: models.PayrollDocumentBillingRealization,
			FileName:     fileHeader.Filename,
			FilePath:     "/uploads/payroll/" + filename,
			FileSize:     fileHeader.Size,
			MimeType:     mimeType,
			UploadedBy:   userID,
		})
	}

	if err := c.db.Create(&documents).Error; err != nil {
		cleanupUploadedFiles(savedPaths)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save payroll documents"})
		return
	}

	documentIDs := make([]uint, 0, len(documents))
	for _, document := range documents {
		documentIDs = append(documentIDs, document.ID)
	}

	var savedDocuments []models.PayrollDocument
	if err := c.db.Preload("Uploader").
		Where("id IN ?", documentIDs).
		Order("created_at ASC").
		Find(&savedDocuments).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll documents"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":   "Payroll documents uploaded successfully",
		"documents": savedDocuments,
	})
}

func (c *PayrollController) DownloadDocument(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("document_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll document ID"})
		return
	}

	if !canViewPayrollDetail(ctx.GetString("role")) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied: payroll document detail is restricted"})
		return
	}

	var document models.PayrollDocument
	if err := c.db.First(&document, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll document not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll document"})
		return
	}

	sendStoredUploadAttachment(ctx, document.FilePath, document.FileName, document.MimeType, "payroll", "payroll-imports")
}

func (c *PayrollController) DownloadPayrollRunExcel(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	if !canViewPayrollDetail(ctx.GetString("role")) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied: payroll detail is restricted"})
		return
	}

	var run models.PayrollRun
	if err := c.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).First(&run, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll data"})
		return
	}

	fileBytes, err := buildPayrollRunExcel(&run)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat file Excel payroll"})
		return
	}

	filename := payrollExcelSafeFilename(run.PayrollNumber)
	ctx.Header("Content-Type", payrollExcelContentType)
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	ctx.Data(http.StatusOK, payrollExcelContentType, fileBytes)
}

func (c *PayrollController) DownloadPayrollImportTemplate(ctx *gin.Context) {
	workbook := excelize.NewFile()
	sheetName := "Salary"
	workbook.SetSheetName("Sheet1", sheetName)

	for index, header := range payrollImportTemplateHeaders {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		workbook.SetCellValue(sheetName, cell, header)
	}

	headerStyle, _ := workbook.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"305496"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	highlightHeaderStyle, _ := workbook.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "000000"},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"9DC3E6"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	dateStyle, _ := workbook.NewStyle(&excelize.Style{
		NumFmt: 14,
	})
	workbook.SetCellStyle(sheetName, "A1", "AC1", headerStyle)
	workbook.SetCellStyle(sheetName, "A1", "B1", highlightHeaderStyle)
	workbook.SetCellStyle(sheetName, "P1", "Y1", highlightHeaderStyle)
	workbook.SetCellStyle(sheetName, "AB1", "AB1", highlightHeaderStyle)
	workbook.SetRowHeight(sheetName, 1, 44)
	workbook.SetColWidth(sheetName, "A", "AC", 18)
	workbook.SetColWidth(sheetName, "B", "B", 24)
	workbook.SetColWidth(sheetName, "P", "Y", 22)
	workbook.SetColWidth(sheetName, "AB", "AB", 28)

	// Light sample values help HR keep the expected data shape without saving real employee data.
	workbook.SetCellValue(sheetName, "A2", time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC))
	workbook.SetCellStyle(sheetName, "A2", "A2", dateStyle)
	workbook.SetCellValue(sheetName, "B2", "26/05/2026 to 25/06/2026")
	workbook.SetCellValue(sheetName, "G2", "Payroll")
	workbook.SetCellValue(sheetName, "G3", "Transfer by Finance")

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat template payroll"})
		return
	}

	ctx.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Header("Content-Disposition", "attachment; filename=payroll_import_template.xlsx")
	ctx.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (c *PayrollController) ImportPayrollPreview(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "File Excel wajib diupload"})
		return
	}

	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if extension != ".xlsx" && extension != ".xlsm" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format file harus Excel modern (.xlsx atau .xlsm)"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membuka file Excel"})
		return
	}
	defer file.Close()

	workbook, err := excelize.OpenReader(file)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "File Excel tidak dapat dibaca", "details": err.Error()})
		return
	}
	defer workbook.Close()

	sheetName := "Salary"
	if index, err := workbook.GetSheetIndex(sheetName); err != nil || index == -1 {
		sheets := workbook.GetSheetList()
		if len(sheets) == 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "File Excel tidak memiliki sheet"})
			return
		}
		sheetName = sheets[0]
	}

	rows, err := workbook.GetRows(sheetName, excelize.Options{RawCellValue: true})
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca sheet payroll", "details": err.Error()})
		return
	}

	headerRowIndex := -1
	headerIndex := map[string]int{}
	var rawHeaders []string
	for rowIndex, row := range rows {
		if rowIndex > 10 {
			break
		}
		currentHeaderIndex := map[string]int{}
		for cellIndex, cell := range row {
			key := payrollHeaderKey(cell)
			if key != "" {
				currentHeaderIndex[key] = cellIndex
			}
		}
		if _, hasEmployee := currentHeaderIndex["employee_name"]; hasEmployee {
			if _, hasSalary := currentHeaderIndex["base_salary"]; hasSalary {
				headerRowIndex = rowIndex
				headerIndex = currentHeaderIndex
				rawHeaders = row
				break
			}
		}
	}

	if headerRowIndex == -1 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Header payroll tidak ditemukan. Pastikan file memakai template dengan kolom EMPLOYEE NAME dan BASIC SALARY.",
		})
		return
	}

	missingColumns := missingPayrollImportColumns(headerIndex)
	if len(missingColumns) > 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":           "Header template payroll belum lengkap",
			"missing_columns": missingColumns,
		})
		return
	}

	response := models.PayrollImportPreviewResponse{
		Items:    []models.PayrollItemCreateRequest{},
		Warnings: []string{},
		Headers:  rawHeaders,
	}

	for rowIndex := headerRowIndex + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		response.Summary.TotalRows++

		if response.SuggestedPaymentDate == "" {
			response.SuggestedPaymentDate = parsePayrollImportDate(getPayrollImportValue(row, headerIndex, "payment_date"))
		}
		if response.SuggestedPeriodStart == "" || response.SuggestedPeriodEnd == "" {
			periodStart, periodEnd := parsePayrollImportPeriod(getPayrollImportValue(row, headerIndex, "payment_period"))
			if periodStart != "" && periodEnd != "" {
				response.SuggestedPeriodStart = periodStart
				response.SuggestedPeriodEnd = periodEnd
			}
		}

		employeeName := getPayrollImportValue(row, headerIndex, "employee_name")
		employeeCode := getPayrollImportValue(row, headerIndex, "employee_code")
		baseSalary := getPayrollImportNumber(row, headerIndex, "base_salary")
		netPay := getPayrollImportNumber(row, headerIndex, "net_pay")
		rowIsEmpty := employeeName == "" && employeeCode == "" && baseSalary == 0 && netPay == 0
		if rowIsEmpty {
			continue
		}

		if employeeName == "" {
			response.Warnings = append(response.Warnings, fmt.Sprintf("Baris %d dilewati: EMPLOYEE NAME kosong", rowIndex+1))
			continue
		}

		item := models.PayrollItemCreateRequest{
			EmployeeName:          employeeName,
			EmployeeCode:          employeeCode,
			WorkingDay:            getPayrollImportNumber(row, headerIndex, "working_day"),
			WorkUnit:              getPayrollImportValue(row, headerIndex, "work_unit"),
			PaymentVia:            getPayrollImportValue(row, headerIndex, "payment_via"),
			AccountNo:             getPayrollImportValue(row, headerIndex, "account_no"),
			Division:              getPayrollImportValue(row, headerIndex, "division"),
			Position:              getPayrollImportValue(row, headerIndex, "position"),
			BaseSalary:            baseSalary,
			AttendanceAllowance:   getPayrollImportNumber(row, headerIndex, "attendance_allowance"),
			CompensationPKWT:      getPayrollImportNumber(row, headerIndex, "compensation_pkwt"),
			Overtime:              getPayrollImportNumber(row, headerIndex, "overtime"),
			PPh:                   getPayrollImportNumber(row, headerIndex, "pph"),
			BPJSTKJHT:             getPayrollImportNumber(row, headerIndex, "bpjs_tk_jht"),
			BPJSTKJP:              getPayrollImportNumber(row, headerIndex, "bpjs_tk_jp"),
			BPJSKesehatan:         getPayrollImportNumber(row, headerIndex, "bpjs_kesehatan"),
			BPJSTKJHTEmployer:     getPayrollImportNumber(row, headerIndex, "bpjs_tk_jht_employer"),
			BPJSTKJHTEmployee:     getPayrollImportNumber(row, headerIndex, "bpjs_tk_jht_employee"),
			BPJSTKJPEmployer:      getPayrollImportNumber(row, headerIndex, "bpjs_tk_jp_employer"),
			BPJSTKJPEmployee:      getPayrollImportNumber(row, headerIndex, "bpjs_tk_jp_employee"),
			BPJSTKJKKEmployer:     getPayrollImportNumber(row, headerIndex, "bpjs_tk_jkk_employer"),
			BPJSTKJKKEmployee:     getPayrollImportNumber(row, headerIndex, "bpjs_tk_jkk_employee"),
			BPJSTKJKMEmployer:     getPayrollImportNumber(row, headerIndex, "bpjs_tk_jkm_employer"),
			BPJSTKJKMEmployee:     getPayrollImportNumber(row, headerIndex, "bpjs_tk_jkm_employee"),
			BPJSKesehatanEmployer: getPayrollImportNumber(row, headerIndex, "bpjs_kesehatan_employer"),
			BPJSKesehatanEmployee: getPayrollImportNumber(row, headerIndex, "bpjs_kesehatan_employee"),
			Proportional:          getPayrollImportNumber(row, headerIndex, "proportional"),
			OtherDeduction:        getPayrollImportNumber(row, headerIndex, "other_deduction"),
			EmployeeLoanDeduction: getPayrollImportNumber(row, headerIndex, "employee_loan_deduction"),
			NetPay:                netPay,
		}
		payrollImportItemTotals(&item)

		response.Items = append(response.Items, item)
		response.Summary.ImportedRows++
		response.Summary.TotalSalary += item.BaseSalary
		response.Summary.TotalBonus += item.Bonus
		response.Summary.TotalPPh += item.PPh
		response.Summary.TotalBPJS += item.BPJS
		response.Summary.TotalBPJSEmployer += payrollCreateBPJSEmployerTotal(&item)
		response.Summary.TotalBPJSEmployee += payrollCreateBPJSEmployeeTotal(&item)
		response.Summary.TotalEmployeeLoanDeduction += item.EmployeeLoanDeduction
		response.Summary.TotalOverall += item.NetPay
	}

	if response.Summary.ImportedRows == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada baris payroll valid yang bisa diimport"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": response})
}

func bindPayrollRunCreateRequest(ctx *gin.Context) (models.PayrollRunCreateRequest, *multipart.FileHeader, error) {
	var req models.PayrollRunCreateRequest
	if strings.HasPrefix(strings.ToLower(ctx.ContentType()), "multipart/form-data") {
		payload := strings.TrimSpace(ctx.PostForm("payload"))
		if payload == "" {
			payload = strings.TrimSpace(ctx.PostForm("data"))
		}
		if payload == "" {
			return req, nil, fmt.Errorf("payload payroll wajib diisi")
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			return req, nil, fmt.Errorf("payload payroll tidak valid: %w", err)
		}

		fileHeader, err := ctx.FormFile("import_file")
		if err != nil {
			if !errors.Is(err, http.ErrMissingFile) {
				return req, nil, fmt.Errorf("file import payroll tidak bisa dibaca: %w", err)
			}
			return req, nil, nil
		}
		extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if extension != ".xlsx" && extension != ".xlsm" {
			return req, nil, fmt.Errorf("file import payroll harus .xlsx atau .xlsm")
		}
		return req, fileHeader, nil
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return req, nil, err
	}
	return req, nil, nil
}

func (c *PayrollController) CreatePayrollRun(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	req, importFileHeader, err := bindPayrollRunCreateRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	periodStart, err := payrollDate(req.PeriodStart, true)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period_start", "details": err.Error()})
		return
	}

	periodEnd, err := payrollDate(req.PeriodEnd, true)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period_end", "details": err.Error()})
		return
	}

	if periodEnd.Before(*periodStart) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "period_end tidak boleh lebih awal dari period_start"})
		return
	}

	if err := services.CheckClosedPeriodRange(c.db, *periodStart, *periodEnd); err != nil {
		respondPayrollClosedPeriod(ctx, err)
		return
	}

	paymentDate, err := payrollDate(req.PaymentDate, false)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment_date", "details": err.Error()})
		return
	}
	if paymentDate != nil {
		if err := services.CheckClosedPeriod(c.db, *paymentDate); err != nil {
			respondPayrollClosedPeriod(ctx, err)
			return
		}
	}

	if len(req.Items) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payroll minimal memiliki 1 detail karyawan"})
		return
	}

	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start payroll transaction"})
		return
	}

	payrollNumber, err := c.nextPayrollNumber(tx)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate payroll number"})
		return
	}

	run := models.PayrollRun{
		PayrollNumber: payrollNumber,
		PeriodStart:   *periodStart,
		PeriodEnd:     *periodEnd,
		PaymentDate:   paymentDate,
		Notes:         strings.TrimSpace(req.Notes),
		Status:        models.PayrollStatusPendingApproval,
		CreatedByID:   userID,
	}

	for _, reqItem := range req.Items {
		if strings.TrimSpace(reqItem.EmployeeName) == "" {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Nama karyawan wajib diisi"})
			return
		}
		payrollImportItemTotals(&reqItem)
		if reqItem.BaseSalary < 0 || reqItem.Bonus < 0 || reqItem.PPh < 0 || reqItem.BPJS < 0 ||
			reqItem.AttendanceAllowance < 0 ||
			reqItem.CompensationPKWT < 0 || reqItem.Overtime < 0 || reqItem.BPJSTKJHT < 0 ||
			reqItem.BPJSTKJP < 0 || reqItem.BPJSKesehatan < 0 || reqItem.Proportional < 0 ||
			reqItem.BPJSTKJHTEmployer < 0 || reqItem.BPJSTKJHTEmployee < 0 ||
			reqItem.BPJSTKJPEmployer < 0 || reqItem.BPJSTKJPEmployee < 0 ||
			reqItem.BPJSTKJKKEmployer < 0 || reqItem.BPJSTKJKKEmployee < 0 ||
			reqItem.BPJSTKJKMEmployer < 0 || reqItem.BPJSTKJKMEmployee < 0 ||
			reqItem.BPJSKesehatanEmployer < 0 || reqItem.BPJSKesehatanEmployee < 0 ||
			reqItem.OtherDeduction < 0 || reqItem.EmployeeLoanDeduction < 0 || reqItem.NetPay < 0 {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Nilai payroll tidak boleh negatif"})
			return
		}

		item := models.PayrollItem{
			EmployeeName:          strings.TrimSpace(reqItem.EmployeeName),
			EmployeeCode:          strings.TrimSpace(reqItem.EmployeeCode),
			WorkingDay:            reqItem.WorkingDay,
			WorkUnit:              strings.TrimSpace(reqItem.WorkUnit),
			PaymentVia:            strings.TrimSpace(reqItem.PaymentVia),
			AccountNo:             strings.TrimSpace(reqItem.AccountNo),
			Division:              strings.TrimSpace(reqItem.Division),
			Department:            strings.TrimSpace(reqItem.Department),
			Position:              strings.TrimSpace(reqItem.Position),
			BaseSalary:            reqItem.BaseSalary,
			Bonus:                 reqItem.Bonus,
			AttendanceAllowance:   reqItem.AttendanceAllowance,
			CompensationPKWT:      reqItem.CompensationPKWT,
			Overtime:              reqItem.Overtime,
			PPh:                   reqItem.PPh,
			BPJS:                  reqItem.BPJS,
			BPJSTKJHT:             reqItem.BPJSTKJHT,
			BPJSTKJP:              reqItem.BPJSTKJP,
			BPJSKesehatan:         reqItem.BPJSKesehatan,
			BPJSTKJHTEmployer:     reqItem.BPJSTKJHTEmployer,
			BPJSTKJHTEmployee:     reqItem.BPJSTKJHTEmployee,
			BPJSTKJPEmployer:      reqItem.BPJSTKJPEmployer,
			BPJSTKJPEmployee:      reqItem.BPJSTKJPEmployee,
			BPJSTKJKKEmployer:     reqItem.BPJSTKJKKEmployer,
			BPJSTKJKKEmployee:     reqItem.BPJSTKJKKEmployee,
			BPJSTKJKMEmployer:     reqItem.BPJSTKJKMEmployer,
			BPJSTKJKMEmployee:     reqItem.BPJSTKJKMEmployee,
			BPJSKesehatanEmployer: reqItem.BPJSKesehatanEmployer,
			BPJSKesehatanEmployee: reqItem.BPJSKesehatanEmployee,
			Proportional:          reqItem.Proportional,
			OtherDeduction:        reqItem.OtherDeduction,
			EmployeeLoanDeduction: reqItem.EmployeeLoanDeduction,
			NetPay:                reqItem.NetPay,
			Notes:                 strings.TrimSpace(reqItem.Notes),
		}

		run.TotalSalary += item.BaseSalary
		run.TotalBonus += item.Bonus
		run.TotalPPh += item.PPh
		run.TotalBPJS += item.BPJS
		run.TotalBPJSEmployer += payrollCreateBPJSEmployerTotal(&reqItem)
		run.TotalBPJSEmployee += payrollCreateBPJSEmployeeTotal(&reqItem)
		run.TotalEmployeeLoanDeduction += item.EmployeeLoanDeduction
		run.TotalOverall += item.NetPay
		run.Items = append(run.Items, item)
	}

	pphRecognitionDate := *periodEnd
	run.PPhRecognitionDate = &pphRecognitionDate
	run.PPhPaymentStatus = payrollPPhPaymentStatus(run.TotalPPh, run.PPhPaidAmount)
	run.SalaryPaymentStatus = payrollSalaryPaymentStatus(run.TotalOverall, run.SalaryPaidAmount)
	run.BPJSPaymentStatus = payrollBPJSPaymentStatus(run.TotalBPJS, run.BPJSPaidAmount)

	if err := tx.Create(&run).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payroll"})
		return
	}

	savedPaths := []string{}
	if importFileHeader != nil {
		const importUploadDir = "./uploads/payroll-imports"
		filename, filePath, mimeType, err := prepareValidatedDocumentUpload(importFileHeader, run.ID, importUploadDir)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := ctx.SaveUploadedFile(importFileHeader, filePath); err != nil {
			tx.Rollback()
			cleanupUploadedFiles(savedPaths)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save payroll import file"})
			return
		}
		savedPaths = append(savedPaths, filePath)

		sourceDocument := models.PayrollDocument{
			PayrollRunID: run.ID,
			DocumentType: models.PayrollDocumentImportSource,
			FileName:     importFileHeader.Filename,
			FilePath:     "/uploads/payroll-imports/" + filename,
			FileSize:     importFileHeader.Size,
			MimeType:     mimeType,
			UploadedBy:   userID,
		}
		if err := tx.Create(&sourceDocument).Error; err != nil {
			tx.Rollback()
			cleanupUploadedFiles(savedPaths)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save payroll import document"})
			return
		}
	}

	if err := tx.Preload("CreatedBy").
		Preload("Items").
		Preload("Documents", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Documents.Uploader").
		First(&run, run.ID).Error; err != nil {
		tx.Rollback()
		cleanupUploadedFiles(savedPaths)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load created payroll"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		cleanupUploadedFiles(savedPaths)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save payroll"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": payrollResponse(run, true)})
}

func (c *PayrollController) ApprovePayrollRun(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	var req struct {
		ExpenseAccountID      *uint  `json:"expense_account_id"`
		EmployeeLoanAccountID *uint  `json:"employee_loan_account_id"`
		PostingDate           string `json:"posting_date"`
	}
	_ = ctx.ShouldBindJSON(&req) // Ignore EOF/errors to remain backward compatible

	now := time.Now()
	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start payroll approval transaction"})
		return
	}

	var run models.PayrollRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, uint(id)).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payroll"})
		return
	}

	if run.Status != models.PayrollStatusPendingApproval {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payroll hanya bisa di-approve saat status menunggu approval"})
		return
	}
	if err := services.CheckClosedPeriodRange(tx, run.PeriodStart, run.PeriodEnd); err != nil {
		tx.Rollback()
		respondPayrollClosedPeriod(ctx, err)
		return
	}
	recognitionDate := defaultPayrollRecognitionDate(&run)
	if strings.TrimSpace(req.PostingDate) != "" {
		parsedPostingDate, err := payrollDate(req.PostingDate, true)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tanggal posting payroll tidak valid", "details": err.Error()})
			return
		}
		recognitionDate = *parsedPostingDate
	}
	if recognitionDate.Before(run.PeriodStart) || recognitionDate.After(run.PeriodEnd) {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tanggal posting payroll harus berada di dalam periode payroll"})
		return
	}
	if err := services.CheckClosedPeriod(tx, recognitionDate); err != nil {
		tx.Rollback()
		respondPayrollClosedPeriod(ctx, err)
		return
	}

	updates := map[string]interface{}{
		"status":                models.PayrollStatusApproved,
		"approved_by_id":        userID,
		"approved_at":           now,
		"rejected_by_id":        nil,
		"rejected_at":           nil,
		"rejection_reason":      "",
		"pph_payment_status":    payrollPPhPaymentStatus(run.TotalPPh, run.PPhPaidAmount),
		"salary_payment_status": payrollSalaryPaymentStatus(run.TotalOverall, run.SalaryPaidAmount),
		"bpjs_payment_status":   payrollBPJSPaymentStatus(run.TotalBPJS, run.BPJSPaidAmount),
	}
	updates["pph_recognition_date"] = recognitionDate
	run.PPhRecognitionDate = &recognitionDate

	if run.TotalEmployeeLoanDeduction > 0.01 {
		employeeLoanAccountID := req.EmployeeLoanAccountID
		if employeeLoanAccountID == nil || *employeeLoanAccountID == 0 {
			account, err := resolvePayrollEmployeeLoanAccount(tx)
			if err != nil {
				tx.Rollback()
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			employeeLoanAccountID = &account.ID
		}
		updates["employee_loan_account_id"] = *employeeLoanAccountID
		run.EmployeeLoanAccountID = employeeLoanAccountID
	}

	if (run.TotalSalary+run.TotalBonus+run.TotalPPh > 0.01) && run.PPhPayableJournalEntryID == nil {
		journalID, err := c.createPayrollAccrualJournal(tx, &run, userID, req.ExpenseAccountID, run.EmployeeLoanAccountID, recognitionDate)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error":   "Gagal membuat jurnal payroll",
				"details": err.Error(),
			})
			return
		}
		if journalID != nil {
			updates["pph_payable_journal_entry_id"] = *journalID
		}
	}

	if err := tx.Model(&run).Updates(updates).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to approve payroll",
			"details": err.Error(),
		})
		return
	}

	if err := tx.Preload("CreatedBy").Preload("ApprovedBy").Preload("Items").Preload("Settlements").First(&run, run.ID).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load approved payroll"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save payroll approval"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": payrollResponse(run, true), "message": "Payroll approved successfully"})
}

func (c *PayrollController) RejectPayrollRun(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	var req models.PayrollRejectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var run models.PayrollRun
	if err := c.db.First(&run, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payroll"})
		return
	}

	if run.Status != models.PayrollStatusPendingApproval {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payroll hanya bisa di-reject saat status menunggu approval"})
		return
	}
	if err := services.CheckClosedPeriodRange(c.db, run.PeriodStart, run.PeriodEnd); err != nil {
		respondPayrollClosedPeriod(ctx, err)
		return
	}
	if run.PaymentDate != nil {
		if err := services.CheckClosedPeriod(c.db, *run.PaymentDate); err != nil {
			respondPayrollClosedPeriod(ctx, err)
			return
		}
	}

	now := time.Now()
	if err := c.db.Model(&run).Updates(map[string]interface{}{
		"status":           models.PayrollStatusRejected,
		"rejected_by_id":   userID,
		"rejected_at":      now,
		"rejection_reason": strings.TrimSpace(req.Reason),
	}).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject payroll"})
		return
	}

	c.db.Preload("CreatedBy").Preload("RejectedBy").Preload("Items").First(&run, run.ID)
	ctx.JSON(http.StatusOK, gin.H{"data": payrollResponse(run, true), "message": "Payroll rejected successfully"})
}

func (c *PayrollController) PayPayrollPPh(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	var req models.PayrollPPhPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	paymentDate, err := payrollDate(req.PaymentDate, true)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment_date", "details": err.Error()})
		return
	}
	if err := services.CheckClosedPeriod(c.db, *paymentDate); err != nil {
		respondPayrollClosedPeriod(ctx, err)
		return
	}

	amount := roundPayrollMoney(req.Amount)
	if amount <= 0.01 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Nominal pembayaran PPh harus lebih besar dari 0"})
		return
	}

	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start PPh payment transaction"})
		return
	}

	var run models.PayrollRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, uint(id)).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payroll"})
		return
	}

	if run.Status != models.PayrollStatusApproved {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "PPh payroll hanya bisa dibayar setelah payroll approved"})
		return
	}
	if run.TotalPPh <= 0.01 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payroll ini tidak memiliki PPh untuk dibayar"})
		return
	}
	if run.PPhPayableJournalEntryID == nil {
		if err := services.CheckClosedPeriodRange(tx, run.PeriodStart, run.PeriodEnd); err != nil {
			tx.Rollback()
			respondPayrollClosedPeriod(ctx, err)
			return
		}
	}

	remaining := remainingPayrollPPh(&run)
	if amount > remaining+0.01 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Nominal pembayaran melebihi sisa PPh. Sisa: %.2f", remaining)})
		return
	}

	var cashBank models.CashBank
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL AND is_active = ?", req.CashBankID, true).
		First(&cashBank).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Akun kas/bank tidak ditemukan atau tidak aktif"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cash/bank account"})
		return
	}
	if cashBank.AccountID == 0 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Akun kas/bank belum terhubung ke GL/COA"})
		return
	}
	if strings.ToUpper(strings.TrimSpace(cashBank.Currency)) != "IDR" {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Pembayaran PPh payroll hanya didukung dari akun kas/bank IDR"})
		return
	}
	if cashBank.AvailableBalance()+0.01 < amount {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Saldo kas/bank tidak cukup. Saldo tersedia: %.2f", cashBank.AvailableBalance())})
		return
	}

	recognitionDate, err := resolvePayrollRecognitionDate(tx, &run)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca tanggal pengakuan payroll", "details": err.Error()})
		return
	}
	run.PPhRecognitionDate = &recognitionDate
	if paymentDate.Before(recognitionDate) {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tanggal pembayaran PPh tidak boleh mendahului tanggal pengakuan payroll"})
		return
	}
	if run.PPhPayableJournalEntryID == nil {
		journalID, err := c.createPayrollAccrualJournal(tx, &run, userID, nil, nil, recognitionDate)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error":   "Gagal membuat jurnal hutang PPh payroll",
				"details": err.Error(),
			})
			return
		}
		run.PPhPayableJournalEntryID = journalID
	}

	pphPayableAccount, err := resolvePayrollPPhPayableAccount(tx)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = fmt.Sprintf("PAY-PPH-%s", run.PayrollNumber)
	}
	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = fmt.Sprintf("Pembayaran PPh payroll %s", run.PayrollNumber)
	}

	journalService := services.NewUnifiedJournalService(tx)
	sourceID := uint64(run.ID)
	journalEntry, err := journalService.CreateJournalEntryWithTx(tx, &services.JournalEntryRequest{
		EntryDate:   *paymentDate,
		Reference:   reference,
		Description: fmt.Sprintf("Pembayaran PPh Payroll - %s", run.PayrollNumber),
		Notes:       notes,
		SourceType:  "PAYROLL_PPH_PAYMENT",
		SourceID:    sourceID,
		AutoPost:    true,
		CreatedBy:   uint64(userID),
		Lines: []services.JournalLineRequest{
			{
				AccountID:    uint64(pphPayableAccount.ID),
				DebitAmount:  decimal.NewFromFloat(amount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Settlement hutang PPh payroll - %s", run.PayrollNumber),
			},
			{
				AccountID:    uint64(cashBank.AccountID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(amount),
				Description:  fmt.Sprintf("Kas/bank keluar bayar PPh payroll - %s", run.PayrollNumber),
			},
		},
	})
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membuat jurnal pembayaran PPh", "details": err.Error()})
		return
	}

	cashBank.Balance = roundPayrollMoney(cashBank.Balance - amount)
	cashBank.BookBalanceIDR = cashBank.Balance
	cashBank.LastExchangeRate = 1
	if err := tx.Save(&cashBank).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cash/bank balance"})
		return
	}

	cashBankTransaction := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYROLL_PPH_PAYMENT",
		ReferenceID:     run.ID,
		Amount:          -amount,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -amount,
		BalanceAfter:    cashBank.Balance,
		TransactionDate: *paymentDate,
		Notes:           notes,
	}
	if err := tx.Create(cashBankTransaction).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record cash/bank transaction"})
		return
	}

	guard := services.NewCashBankCurrencyGuard(c.db)
	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate cash/bank balance", "details": err.Error()})
		return
	}

	settlement := models.PayrollSettlement{
		PayrollRunID:   run.ID,
		SettlementType: models.PayrollSettlementTypePPh21,
		Amount:         amount,
		PaymentDate:    *paymentDate,
		CashBankID:     cashBank.ID,
		JournalEntryID: journalEntry.ID,
		Reference:      reference,
		Notes:          notes,
		Source:         "PAYROLL",
		CreatedByID:    userID,
	}
	if err := tx.Create(&settlement).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record payroll PPh settlement history"})
		return
	}

	paidAmount := roundPayrollMoney(run.PPhPaidAmount + amount)
	paymentJournalID := journalEntry.ID
	updates := map[string]interface{}{
		"pph_paid_amount":              paidAmount,
		"pph_payment_status":           payrollPPhPaymentStatus(run.TotalPPh, paidAmount),
		"pph_payment_date":             *paymentDate,
		"pph_payment_reference":        reference,
		"pph_payment_notes":            notes,
		"pph_payment_journal_entry_id": paymentJournalID,
		"pph_recognition_date":         *run.PPhRecognitionDate,
	}
	if run.PPhPayableJournalEntryID != nil {
		updates["pph_payable_journal_entry_id"] = *run.PPhPayableJournalEntryID
	}
	if err := tx.Model(&run).Updates(updates).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payroll PPh status"})
		return
	}

	if err := tx.Preload("CreatedBy").Preload("ApprovedBy").Preload("Items").Preload("Settlements").Preload("Settlements.CashBank").First(&run, run.ID).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated payroll"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save PPh payment"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": payrollResponse(run, true), "message": "Pembayaran PPh payroll berhasil dicatat"})
}

// PayPayrollBPJS settles only the BPJS portion of payroll deductions. Other
// deductions can remain in 2103-002 until a separate settlement or adjustment is
// posted, which keeps BPJS payment history aligned with the payroll number.
func (c *PayrollController) PayPayrollBPJS(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	var req models.PayrollBPJSPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	paymentDate, err := payrollDate(req.PaymentDate, true)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment_date", "details": err.Error()})
		return
	}
	if err := services.CheckClosedPeriod(c.db, *paymentDate); err != nil {
		respondPayrollClosedPeriod(ctx, err)
		return
	}

	amount := roundPayrollMoney(req.Amount)
	if amount <= 0.01 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Nominal pembayaran BPJS harus lebih besar dari 0"})
		return
	}

	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start BPJS payment transaction"})
		return
	}

	var run models.PayrollRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, uint(id)).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payroll"})
		return
	}

	if run.Status != models.PayrollStatusApproved {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "BPJS payroll hanya bisa dibayar setelah payroll approved"})
		return
	}
	if run.TotalBPJS <= 0.01 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payroll ini tidak memiliki BPJS untuk dibayar"})
		return
	}
	if run.PPhPayableJournalEntryID == nil {
		if err := services.CheckClosedPeriodRange(tx, run.PeriodStart, run.PeriodEnd); err != nil {
			tx.Rollback()
			respondPayrollClosedPeriod(ctx, err)
			return
		}
	}

	remaining := remainingPayrollBPJS(&run)
	if amount > remaining+0.01 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Nominal pembayaran melebihi sisa BPJS. Sisa: %.2f", remaining)})
		return
	}

	var cashBank models.CashBank
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL AND is_active = ?", req.CashBankID, true).
		First(&cashBank).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Akun kas/bank tidak ditemukan atau tidak aktif"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cash/bank account"})
		return
	}
	if cashBank.AccountID == 0 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Akun kas/bank belum terhubung ke GL/COA"})
		return
	}
	if strings.ToUpper(strings.TrimSpace(cashBank.Currency)) != "IDR" {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Pembayaran BPJS payroll hanya didukung dari akun kas/bank IDR"})
		return
	}
	if cashBank.AvailableBalance()+0.01 < amount {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Saldo kas/bank tidak cukup. Saldo tersedia: %.2f", cashBank.AvailableBalance())})
		return
	}

	recognitionDate, err := resolvePayrollRecognitionDate(tx, &run)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca tanggal pengakuan payroll", "details": err.Error()})
		return
	}
	run.PPhRecognitionDate = &recognitionDate
	if paymentDate.Before(recognitionDate) {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tanggal pembayaran BPJS tidak boleh mendahului tanggal pengakuan payroll"})
		return
	}
	if run.PPhPayableJournalEntryID == nil {
		journalID, err := c.createPayrollAccrualJournal(tx, &run, userID, nil, nil, recognitionDate)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error":   "Gagal membuat jurnal pengakuan payroll",
				"details": err.Error(),
			})
			return
		}
		run.PPhPayableJournalEntryID = journalID
	}

	deductionPayableAccount, err := resolvePayrollDeductionPayableAccount(tx)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = fmt.Sprintf("PAY-BPJS-%s", run.PayrollNumber)
	}
	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = fmt.Sprintf("Pembayaran BPJS payroll %s", run.PayrollNumber)
	}

	journalService := services.NewUnifiedJournalService(tx)
	sourceID := uint64(run.ID)
	journalEntry, err := journalService.CreateJournalEntryWithTx(tx, &services.JournalEntryRequest{
		EntryDate:   *paymentDate,
		Reference:   reference,
		Description: fmt.Sprintf("Pembayaran BPJS Payroll - %s", run.PayrollNumber),
		Notes:       notes,
		SourceType:  "PAYROLL_BPJS_PAYMENT",
		SourceID:    sourceID,
		AutoPost:    true,
		CreatedBy:   uint64(userID),
		Lines: []services.JournalLineRequest{
			{
				AccountID:    uint64(deductionPayableAccount.ID),
				DebitAmount:  decimal.NewFromFloat(amount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Settlement hutang BPJS payroll - %s", run.PayrollNumber),
			},
			{
				AccountID:    uint64(cashBank.AccountID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(amount),
				Description:  fmt.Sprintf("Kas/bank keluar bayar BPJS payroll - %s", run.PayrollNumber),
			},
		},
	})
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membuat jurnal pembayaran BPJS", "details": err.Error()})
		return
	}

	cashBank.Balance = roundPayrollMoney(cashBank.Balance - amount)
	cashBank.BookBalanceIDR = cashBank.Balance
	cashBank.LastExchangeRate = 1
	if err := tx.Save(&cashBank).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cash/bank balance"})
		return
	}

	cashBankTransaction := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYROLL_BPJS_PAYMENT",
		ReferenceID:     run.ID,
		Amount:          -amount,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -amount,
		BalanceAfter:    cashBank.Balance,
		TransactionDate: *paymentDate,
		Notes:           notes,
	}
	if err := tx.Create(cashBankTransaction).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record cash/bank transaction"})
		return
	}

	guard := services.NewCashBankCurrencyGuard(c.db)
	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate cash/bank balance", "details": err.Error()})
		return
	}

	settlement := models.PayrollSettlement{
		PayrollRunID:   run.ID,
		SettlementType: models.PayrollSettlementTypeBPJS,
		Amount:         amount,
		PaymentDate:    *paymentDate,
		CashBankID:     cashBank.ID,
		JournalEntryID: journalEntry.ID,
		Reference:      reference,
		Notes:          notes,
		Source:         "PAYROLL",
		CreatedByID:    userID,
	}
	if err := tx.Create(&settlement).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record payroll BPJS settlement history"})
		return
	}

	paidAmount := roundPayrollMoney(run.BPJSPaidAmount + amount)
	updates := map[string]interface{}{
		"bpjs_paid_amount":              paidAmount,
		"bpjs_payment_status":           payrollBPJSPaymentStatus(run.TotalBPJS, paidAmount),
		"bpjs_payment_date":             *paymentDate,
		"bpjs_payment_reference":        reference,
		"bpjs_payment_notes":            notes,
		"bpjs_payment_journal_entry_id": journalEntry.ID,
		"pph_recognition_date":          *run.PPhRecognitionDate,
	}
	if run.PPhPayableJournalEntryID != nil {
		updates["pph_payable_journal_entry_id"] = *run.PPhPayableJournalEntryID
	}
	if err := tx.Model(&run).Updates(updates).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payroll BPJS status"})
		return
	}

	if err := tx.Preload("CreatedBy").Preload("ApprovedBy").Preload("Items").Preload("Settlements").Preload("Settlements.CashBank").First(&run, run.ID).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated payroll"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save BPJS payment"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": payrollResponse(run, true), "message": "Pembayaran BPJS payroll berhasil dicatat"})
}

// PayPayrollSalary settles the net salary liability created by the payroll
// accrual. Approval never credits cash/bank; cash only moves through this
// endpoint on the actual payment date.
func (c *PayrollController) PayPayrollSalary(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payroll ID"})
		return
	}

	var req models.PayrollSalaryPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	paymentDate, err := payrollDate(req.PaymentDate, true)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment_date", "details": err.Error()})
		return
	}
	if err := services.CheckClosedPeriod(c.db, *paymentDate); err != nil {
		respondPayrollClosedPeriod(ctx, err)
		return
	}

	amount := roundPayrollMoney(req.Amount)
	if amount <= 0.01 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Nominal pembayaran gaji harus lebih besar dari 0"})
		return
	}

	tx := c.db.Begin()
	if tx.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start salary payment transaction"})
		return
	}

	var run models.PayrollRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, uint(id)).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payroll not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payroll"})
		return
	}
	if run.Status != models.PayrollStatusApproved {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gaji hanya bisa dibayar setelah payroll approved"})
		return
	}
	if run.TotalOverall <= 0.01 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payroll ini tidak memiliki net salary untuk dibayar"})
		return
	}

	remaining := remainingPayrollSalary(&run)
	if amount > remaining+0.01 {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Nominal pembayaran melebihi sisa gaji. Sisa: %.2f", remaining)})
		return
	}

	recognitionDate, err := resolvePayrollRecognitionDate(tx, &run)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca tanggal pengakuan payroll", "details": err.Error()})
		return
	}
	run.PPhRecognitionDate = &recognitionDate
	if paymentDate.Before(recognitionDate) {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tanggal pembayaran gaji tidak boleh mendahului tanggal pengakuan payroll"})
		return
	}
	if run.PPhPayableJournalEntryID == nil {
		if err := services.CheckClosedPeriod(tx, recognitionDate); err != nil {
			tx.Rollback()
			respondPayrollClosedPeriod(ctx, err)
			return
		}
		journalID, err := c.createPayrollAccrualJournal(tx, &run, userID, nil, nil, recognitionDate)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membuat jurnal pengakuan payroll", "details": err.Error()})
			return
		}
		run.PPhPayableJournalEntryID = journalID
	}

	var cashBank models.CashBank
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL AND is_active = ?", req.CashBankID, true).
		First(&cashBank).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Akun kas/bank tidak ditemukan atau tidak aktif"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cash/bank account"})
		return
	}
	if cashBank.AccountID == 0 || strings.ToUpper(strings.TrimSpace(cashBank.Currency)) != "IDR" {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Pembayaran gaji hanya didukung dari akun kas/bank IDR yang terhubung ke GL/COA"})
		return
	}
	if cashBank.AvailableBalance()+0.01 < amount {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Saldo kas/bank tidak cukup. Saldo tersedia: %.2f", cashBank.AvailableBalance())})
		return
	}

	salaryPayableAccount, err := resolvePayrollSalaryPayableAccount(tx)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = fmt.Sprintf("PAY-SALARY-%s", run.PayrollNumber)
	}
	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = fmt.Sprintf("Pembayaran net salary payroll %s", run.PayrollNumber)
	}

	journalService := services.NewUnifiedJournalService(tx)
	sourceID := uint64(run.ID)
	journalEntry, err := journalService.CreateJournalEntryWithTx(tx, &services.JournalEntryRequest{
		EntryDate:   *paymentDate,
		Reference:   reference,
		Description: fmt.Sprintf("Pembayaran Net Salary Payroll - %s", run.PayrollNumber),
		Notes:       notes,
		SourceType:  "PAYROLL_SALARY_PAYMENT",
		SourceID:    sourceID,
		AutoPost:    true,
		CreatedBy:   uint64(userID),
		Lines: []services.JournalLineRequest{
			{AccountID: uint64(salaryPayableAccount.ID), DebitAmount: decimal.NewFromFloat(amount), CreditAmount: decimal.Zero, Description: fmt.Sprintf("Settlement utang gaji - %s", run.PayrollNumber)},
			{AccountID: uint64(cashBank.AccountID), DebitAmount: decimal.Zero, CreditAmount: decimal.NewFromFloat(amount), Description: fmt.Sprintf("Kas/bank keluar bayar gaji - %s", run.PayrollNumber)},
		},
	})
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membuat jurnal pembayaran gaji", "details": err.Error()})
		return
	}

	cashBank.Balance = roundPayrollMoney(cashBank.Balance - amount)
	cashBank.BookBalanceIDR = cashBank.Balance
	cashBank.LastExchangeRate = 1
	if err := tx.Save(&cashBank).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cash/bank balance"})
		return
	}
	cashBankTransaction := models.CashBankTransaction{
		CashBankID: cashBank.ID, ReferenceType: "PAYROLL_SALARY_PAYMENT", ReferenceID: run.ID,
		Amount: -amount, Currency: "IDR", ExchangeRate: 1, BookAmountIDR: -amount,
		BalanceAfter: cashBank.Balance, TransactionDate: *paymentDate, Notes: notes,
	}
	if err := tx.Create(&cashBankTransaction).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record cash/bank transaction"})
		return
	}
	guard := services.NewCashBankCurrencyGuard(c.db)
	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate cash/bank balance", "details": err.Error()})
		return
	}

	settlement := models.PayrollSettlement{
		PayrollRunID: run.ID, SettlementType: models.PayrollSettlementTypeSalary,
		Amount: amount, PaymentDate: *paymentDate, CashBankID: cashBank.ID,
		JournalEntryID: journalEntry.ID, Reference: reference, Notes: notes,
		Source: "PAYROLL", CreatedByID: userID,
	}
	if err := tx.Create(&settlement).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record payroll salary settlement history"})
		return
	}

	paidAmount := roundPayrollMoney(run.SalaryPaidAmount + amount)
	updates := map[string]interface{}{
		"salary_paid_amount":              paidAmount,
		"salary_payment_status":           payrollSalaryPaymentStatus(run.TotalOverall, paidAmount),
		"salary_payment_date":             *paymentDate,
		"salary_payment_reference":        reference,
		"salary_payment_notes":            notes,
		"salary_payment_journal_entry_id": journalEntry.ID,
		"pph_recognition_date":            *run.PPhRecognitionDate,
	}
	if run.PPhPayableJournalEntryID != nil {
		updates["pph_payable_journal_entry_id"] = *run.PPhPayableJournalEntryID
	}
	if err := tx.Model(&run).Updates(updates).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payroll salary status"})
		return
	}
	if err := tx.Preload("CreatedBy").Preload("ApprovedBy").Preload("Items").Preload("Settlements").Preload("Settlements.CashBank").First(&run, run.ID).Error; err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated payroll"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save salary payment"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": payrollResponse(run, true), "message": "Pembayaran gaji payroll berhasil dicatat"})
}
