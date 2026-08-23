package reporting

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

// reportTypeFromPath pulls the report slug out of a chi-style path segment.
// e.g. /api/v1/reports/trial-balance/export -> "trial-balance".
func reportTypeFromPath(path string) (reportType, bool) {
	// chi does not capture into the handler signature here; we read the path.
	// Expected shape: .../{reportType}/export
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return "", false
	}
	seg := parts[len(parts)-2] // the segment before "export"
	if _, ok := reportTitles[reportType(seg)]; !ok {
		return "", false
	}
	return reportType(seg), true
}

// Export handles GET /reports/{reportType}/export?format=pdf|xlsx.
func (service *Service) Export(writer http.ResponseWriter, request *http.Request) {
	rtype, ok := reportTypeFromPath(request.URL.Path)
	if !ok {
		writeError(writer, http.StatusNotFound, "UNKNOWN_REPORT", "unknown report type")
		return
	}

	format := strings.ToLower(request.URL.Query().Get("format"))
	if format != "pdf" && format != "xlsx" {
		writeError(writer, http.StatusBadRequest, "INVALID_FORMAT", "format must be 'pdf' or 'xlsx'")
		return
	}

	f, err := parseReportFilter(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := service.fetchReportData(request, rtype, f)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}

	title := reportTitles[rtype]
	label := dateRangeLabel(f.fromDate, f.toDate)

	switch format {
	case "pdf":
		pdf := buildPDF(rtype, title, label, result)
		filename := exportFilename(title, "pdf")
		writer.Header().Set("Content-Type", "application/pdf")
		writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		writer.WriteHeader(http.StatusOK)
		_ = pdf.Output(writer)
	case "xlsx":
		file := buildXLSX(rtype, title, label, result)
		defer func() { _ = file.Close() }()
		filename := exportFilename(title, "xlsx")
		writer.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		writer.WriteHeader(http.StatusOK)
		_ = file.Write(writer)
	}
}

// exportFilename builds the attachment filename, e.g. "Trial_Balance.pdf".
func exportFilename(title, ext string) string {
	return strings.ReplaceAll(title, " ", "_") + "." + ext
}

/* ------------------------------- PDF ------------------------------- */

// buildPDF renders a simple tabular PDF for the given report. Layouts differ
// per report type because the data shapes differ.
func buildPDF(rtype reportType, title, subtitle string, result any) *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, title, "0", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 6, subtitle, "0", 1, "L", false, 0, "")
	pdf.Ln(2)

	switch rtype {
	case reportTrialBalance:
		drawTrialBalancePDF(pdf, result.(TrialBalanceResult))
	case reportProfitLoss:
		drawProfitLossPDF(pdf, result.(ProfitLossResult))
	case reportBalanceSheet:
		drawBalanceSheetPDF(pdf, result.(BalanceSheetResult))
	case reportCashFlow:
		drawCashFlowPDF(pdf, result.(CashFlowResult))
	}
	return pdf
}

func drawTrialBalancePDF(pdf *gofpdf.Fpdf, r TrialBalanceResult) {
	widths := []float64{25, 85, 35, 35}
	drawTableHeader(pdf, []string{"Code", "Account", "Debit", "Credit"}, widths)

	pdf.SetFont("Arial", "", 10)
	for _, row := range r.Rows {
		drawRow(pdf, widths, []string{
			row.AccountCode,
			truncate(pdf, row.AccountName, widths[1]-4),
			formatCents(row.DebitCents),
			formatCents(row.CreditCents),
		})
	}
	pdf.SetFont("Arial", "B", 10)
	drawRow(pdf, widths, []string{"", "Total", formatCents(r.TotalDebitCents), formatCents(r.TotalCreditCents)})

	status := "BALANCED"
	if !r.Balanced {
		status = "NOT BALANCED"
	}
	pdf.Ln(2)
	pdf.SetFont("Arial", "I", 9)
	pdf.CellFormat(0, 6, status, "0", 1, "L", false, 0, "")
}

func drawProfitLossPDF(pdf *gofpdf.Fpdf, r ProfitLossResult) {
	drawKeyValuePDF(pdf, [][2]string{
		{"Revenue", formatCents(r.RevenueCents)},
		{"Expenses", formatCents(r.ExpenseCents)},
		{"Net Profit", formatCents(r.ProfitCents)},
	})
}

func drawBalanceSheetPDF(pdf *gofpdf.Fpdf, r BalanceSheetResult) {
	drawKeyValuePDF(pdf, [][2]string{
		{"Assets", formatCents(r.AssetCents)},
		{"Liabilities", formatCents(r.LiabilityCents)},
		{"Equity", formatCents(r.EquityCents)},
		{"Current-period profit", formatCents(r.ProfitCents)},
	})
	pdf.Ln(2)
	pdf.SetFont("Arial", "I", 9)
	status := "BALANCED"
	if !r.Balanced {
		status = "NOT BALANCED"
	}
	pdf.CellFormat(0, 6, status, "0", 1, "L", false, 0, "")
}

func drawCashFlowPDF(pdf *gofpdf.Fpdf, r CashFlowResult) {
	drawKeyValuePDF(pdf, [][2]string{
		{"Inflow", formatCents(r.InflowCents)},
		{"Outflow", formatCents(r.OutflowCents)},
		{"Net Cash Flow", formatCents(r.NetCashFlowCents)},
	})
}

func drawKeyValuePDF(pdf *gofpdf.Fpdf, rows [][2]string) {
	widths := []float64{110, 70}
	drawTableHeader(pdf, []string{"", "Amount"}, widths)
	pdf.SetFont("Arial", "", 10)
	for _, row := range rows {
		drawRow(pdf, widths, []string{row[0], row[1]})
	}
}

func drawTableHeader(pdf *gofpdf.Fpdf, cols []string, widths []float64) {
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	for i, col := range cols {
		pdf.CellFormat(widths[i], 7, col, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
}

func drawRow(pdf *gofpdf.Fpdf, widths []float64, cols []string) {
	for i, col := range cols {
		pdf.CellFormat(widths[i], 6, col, "LR", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)
}

// truncate shrinks s so its rendered width fits within maxMM using the
// currently-selected font. gofpdf has no auto-fit, so we clip from the right.
func truncate(pdf *gofpdf.Fpdf, s string, maxMM float64) string {
	if pdf.GetStringWidth(s) <= maxMM {
		return s
	}
	for len(s) > 0 {
		s = s[:len(s)-1]
		if pdf.GetStringWidth(s+"…") <= maxMM {
			return s + "…"
		}
	}
	return ""
}

/* ------------------------------- XLSX ------------------------------ */

// buildXLSX renders a simple spreadsheet for the given report.
func buildXLSX(rtype reportType, title, subtitle string, result any) *excelize.File {
	f := excelize.NewFile()
	_ = f.SetDefaultFont("Calibri")
	sheet := f.GetSheetName(0)
	_ = f.SetSheetName(sheet, "Report")

	_ = f.SetCellValue("Report", "A1", title)
	_ = f.SetCellValue("Report", "A2", subtitle)

	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	switch rtype {
	case reportTrialBalance:
		writeTrialBalanceXLSX(f, result.(TrialBalanceResult), bold)
	case reportProfitLoss:
		writeProfitLossXLSX(f, result.(ProfitLossResult), bold)
	case reportBalanceSheet:
		writeBalanceSheetXLSX(f, result.(BalanceSheetResult), bold)
	case reportCashFlow:
		writeCashFlowXLSX(f, result.(CashFlowResult), bold)
	}
	return f
}

func writeTrialBalanceXLSX(f *excelize.File, r TrialBalanceResult, bold int) {
	const sheet = "Report"
	headers := []string{"Code", "Account", "Debit", "Credit"}
	for i, h := range headers {
		setCell(f, sheet, i+1, 4, h)
	}
	_ = f.SetCellStyle(sheet, "A4", "D4", bold)

	for i, row := range r.Rows {
		rowNum := 5 + i
		setCell(f, sheet, 1, rowNum, row.AccountCode)
		setCell(f, sheet, 2, rowNum, row.AccountName)
		setCell(f, sheet, 3, rowNum, formatCents(row.DebitCents))
		setCell(f, sheet, 4, rowNum, formatCents(row.CreditCents))
	}
	totalRow := 5 + len(r.Rows)
	setCell(f, sheet, 2, totalRow, "Total")
	setCell(f, sheet, 3, totalRow, formatCents(r.TotalDebitCents))
	setCell(f, sheet, 4, totalRow, formatCents(r.TotalCreditCents))
	startCell, _ := excelize.CoordinatesToCellName(1, totalRow)
	endCell, _ := excelize.CoordinatesToCellName(4, totalRow)
	_ = f.SetCellStyle(sheet, startCell, endCell, bold)

	status := "BALANCED"
	if !r.Balanced {
		status = "NOT BALANCED"
	}
	setCell(f, sheet, 1, totalRow+2, status)

	_ = f.SetColWidth(sheet, "A", "A", 16)
	_ = f.SetColWidth(sheet, "B", "B", 40)
	_ = f.SetColWidth(sheet, "C", "C", 18)
	_ = f.SetColWidth(sheet, "D", "D", 18)
}

func writeProfitLossXLSX(f *excelize.File, r ProfitLossResult, bold int) {
	const sheet = "Report"
	_ = f.SetCellStyle(sheet, "A4", "B4", bold)
	setCell(f, sheet, 1, 4, "Item")
	setCell(f, sheet, 2, 4, "Amount")
	rows := [][2]string{
		{"Revenue", formatCents(r.RevenueCents)},
		{"Expenses", formatCents(r.ExpenseCents)},
		{"Net Profit", formatCents(r.ProfitCents)},
	}
	for i, row := range rows {
		setCell(f, sheet, 1, 5+i, row[0])
		setCell(f, sheet, 2, 5+i, row[1])
	}
	_ = f.SetColWidth(sheet, "A", "A", 30)
	_ = f.SetColWidth(sheet, "B", "B", 20)
}

func writeBalanceSheetXLSX(f *excelize.File, r BalanceSheetResult, bold int) {
	const sheet = "Report"
	_ = f.SetCellStyle(sheet, "A4", "B4", bold)
	setCell(f, sheet, 1, 4, "Item")
	setCell(f, sheet, 2, 4, "Amount")
	rows := [][2]string{
		{"Assets", formatCents(r.AssetCents)},
		{"Liabilities", formatCents(r.LiabilityCents)},
		{"Equity", formatCents(r.EquityCents)},
		{"Current-period profit", formatCents(r.ProfitCents)},
	}
	for i, row := range rows {
		setCell(f, sheet, 1, 5+i, row[0])
		setCell(f, sheet, 2, 5+i, row[1])
	}
	status := "BALANCED"
	if !r.Balanced {
		status = "NOT BALANCED"
	}
	setCell(f, sheet, 1, 5+len(rows)+1, status)
	_ = f.SetColWidth(sheet, "A", "A", 30)
	_ = f.SetColWidth(sheet, "B", "B", 20)
}

func writeCashFlowXLSX(f *excelize.File, r CashFlowResult, bold int) {
	const sheet = "Report"
	_ = f.SetCellStyle(sheet, "A4", "B4", bold)
	setCell(f, sheet, 1, 4, "Item")
	setCell(f, sheet, 2, 4, "Amount")
	rows := [][2]string{
		{"Inflow", formatCents(r.InflowCents)},
		{"Outflow", formatCents(r.OutflowCents)},
		{"Net Cash Flow", formatCents(r.NetCashFlowCents)},
	}
	for i, row := range rows {
		setCell(f, sheet, 1, 5+i, row[0])
		setCell(f, sheet, 2, 5+i, row[1])
	}
	_ = f.SetColWidth(sheet, "A", "A", 30)
	_ = f.SetColWidth(sheet, "B", "B", 20)
}

func setCell(f *excelize.File, sheet string, col, row int, value string) {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return
	}
	_ = f.SetCellValue(sheet, cell, value)
}

/* ----------------------------- formatting -------------------------- */

// formatCents renders an integer-cent value as a decimal string with thousands
// separators. Negative values keep their leading minus. Kept pure-Go (no
// locale) so PDF and XLSX output is deterministic across servers.
func formatCents(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	s := intToStrWithSeparators(whole)
	out := fmt.Sprintf("%s.%02d", s, frac)
	if neg {
		out = "-" + out
	}
	return out
}

// intToStrWithSeparators renders a non-negative int64 as decimal with thousands
// separators. Uses strconv then inserts commas.
func intToStrWithSeparators(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	first := len(s) % 3
	if first > 0 {
		b.WriteString(s[:first])
		b.WriteByte(',')
	}
	for i := first; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}
