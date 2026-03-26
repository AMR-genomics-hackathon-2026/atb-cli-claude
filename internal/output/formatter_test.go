package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatTSV(t *testing.T) {
	rows := []Row{
		{"sample_accession": "SAMD00000001", "organism": "Homo sapiens"},
		{"sample_accession": "SAMD00000002", "organism": "Mus musculus"},
	}
	columns := []string{"sample_accession", "organism"}

	var buf bytes.Buffer
	if err := Format(&buf, rows, columns, "tsv"); err != nil {
		t.Fatalf("Format tsv error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 data), got %d", len(lines))
	}

	headerFields := strings.Split(lines[0], "\t")
	if len(headerFields) != 2 {
		t.Fatalf("expected 2 header fields, got %d", len(headerFields))
	}
	if headerFields[0] != "sample_accession" || headerFields[1] != "organism" {
		t.Errorf("unexpected header: %v", headerFields)
	}

	row1Fields := strings.Split(lines[1], "\t")
	if row1Fields[0] != "SAMD00000001" || row1Fields[1] != "Homo sapiens" {
		t.Errorf("unexpected row1: %v", row1Fields)
	}

	row2Fields := strings.Split(lines[2], "\t")
	if row2Fields[0] != "SAMD00000002" || row2Fields[1] != "Mus musculus" {
		t.Errorf("unexpected row2: %v", row2Fields)
	}
}

func TestFormatCSV(t *testing.T) {
	rows := []Row{
		{"sample_accession": "SAMD00000001", "organism": "Homo sapiens"},
		{"sample_accession": "SAMD00000002", "organism": "Mus musculus"},
	}
	columns := []string{"sample_accession", "organism"}

	var buf bytes.Buffer
	if err := Format(&buf, rows, columns, "csv"); err != nil {
		t.Fatalf("Format csv error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 data), got %d", len(lines))
	}

	if !strings.Contains(lines[0], ",") {
		t.Errorf("header line should be comma-separated, got: %s", lines[0])
	}

	headerFields := strings.Split(lines[0], ",")
	if headerFields[0] != "sample_accession" || headerFields[1] != "organism" {
		t.Errorf("unexpected header: %v", headerFields)
	}

	if !strings.Contains(lines[1], "SAMD00000001") {
		t.Errorf("row1 should contain SAMD00000001, got: %s", lines[1])
	}
}

func TestFormatJSON(t *testing.T) {
	rows := []Row{
		{"sample_accession": "SAMD00000001", "organism": "Homo sapiens"},
		{"sample_accession": "SAMD00000002", "organism": "Mus musculus"},
	}
	columns := []string{"sample_accession", "organism"}

	var buf bytes.Buffer
	if err := Format(&buf, rows, columns, "json"); err != nil {
		t.Fatalf("Format json error: %v", err)
	}

	var result []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 JSON objects, got %d", len(result))
	}

	if result[0]["sample_accession"] != "SAMD00000001" {
		t.Errorf("unexpected first object: %v", result[0])
	}
	if result[1]["sample_accession"] != "SAMD00000002" {
		t.Errorf("unexpected second object: %v", result[1])
	}
}

func TestFormatEmpty(t *testing.T) {
	columns := []string{"sample_accession", "organism"}

	var buf bytes.Buffer
	if err := Format(&buf, nil, columns, "tsv"); err != nil {
		t.Fatalf("Format tsv with nil rows error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (header only), got %d: %v", len(lines), lines)
	}

	headerFields := strings.Split(lines[0], "\t")
	if len(headerFields) != 2 {
		t.Fatalf("expected 2 header fields, got %d", len(headerFields))
	}
}

func TestInferColumns(t *testing.T) {
	rows := []Row{
		{"organism": "Homo sapiens", "sample_accession": "SAMD00000001", "age": "30"},
		{"organism": "Mus musculus", "sample_accession": "SAMD00000002", "country": "Japan"},
	}

	cols := InferColumns(rows)

	if len(cols) == 0 {
		t.Fatal("expected non-empty columns")
	}

	if cols[0] != "sample_accession" {
		t.Errorf("expected sample_accession first, got %s", cols[0])
	}

	// rest should be sorted alphabetically
	rest := cols[1:]
	for i := 1; i < len(rest); i++ {
		if rest[i-1] > rest[i] {
			t.Errorf("columns not sorted after sample_accession: %v", rest)
		}
	}

	// verify all keys are present
	colSet := make(map[string]bool)
	for _, c := range cols {
		colSet[c] = true
	}
	for _, key := range []string{"organism", "sample_accession", "age", "country"} {
		if !colSet[key] {
			t.Errorf("missing column %s in inferred columns: %v", key, cols)
		}
	}
}
