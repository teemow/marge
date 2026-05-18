package cmd

import (
	"reflect"
	"testing"
)

func TestParseCSVList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string is nil", "", nil},
		{"whitespace only is nil", "   ", nil},
		{"single pattern", "trivy", []string{"trivy"}},
		{"multiple patterns", "Trivy, govulncheck ,CodeQL", []string{"Trivy", "govulncheck", "CodeQL"}},
		{"empty entries are skipped", "trivy,,gosec, ", []string{"trivy", "gosec"}},
		{"only separators yields nil", ",, ,", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCSVList(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCSVList(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTrustedAuthors(t *testing.T) {
	got := parseTrustedAuthors("renovate[bot], dependabot[bot] , ,custom[bot]")
	want := map[string]bool{
		"renovate[bot]":   true,
		"dependabot[bot]": true,
		"custom[bot]":     true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseTrustedAuthors = %v, want %v", got, want)
	}
}
