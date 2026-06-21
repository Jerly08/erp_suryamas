package utils

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestCleanExportTextRepairsMojibakeAndRemovesSymbols(t *testing.T) {
	input := "âœ… BANK â€“ GIRO ðŸ›¡ï¸"
	actual := strings.TrimSpace(CleanExportText(input))
	if actual != "BANK - GIRO" {
		t.Fatalf("CleanExportText() = %q, expected %q", actual, "BANK - GIRO")
	}
}

func TestCleanExportTextRepairsDoubleEncodedLogSymbols(t *testing.T) {
	input := "Ã¢Å“â€¦ Account created Ã°Å¸â€Â"
	actual := strings.TrimSpace(CleanExportText(input))
	if actual != "Account created" {
		t.Fatalf("CleanExportText() = %q, expected %q", actual, "Account created")
	}
}

func TestSanitizedCSVWriterCleansEveryCell(t *testing.T) {
	var buf bytes.Buffer
	writer := NewSanitizedCSVWriter(&buf)
	if err := writer.Write([]string{"âœ… Status", "BANK – GIRO"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("writer error = %v", err)
	}

	reader := csv.NewReader(&buf)
	record, err := reader.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if record[0] != "Status" || record[1] != "BANK - GIRO" {
		t.Fatalf("record = %#v", record)
	}
}
