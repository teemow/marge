package cmd

import (
	"reflect"
	"testing"
)

func TestParseSecurityPatterns(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantSet bool
	}{
		{"empty string is unset", "", nil, false},
		{"whitespace only is unset", "   ", nil, false},
		{"single pattern", "trivy", []string{"trivy"}, true},
		{"multiple patterns", "Trivy, govulncheck ,CodeQL", []string{"Trivy", "govulncheck", "CodeQL"}, true},
		{"empty entries are skipped", "trivy,,gosec, ", []string{"trivy", "gosec"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSecurityPatterns(tt.input)
			if ok != tt.wantSet {
				t.Errorf("parseSecurityPatterns(%q) ok=%v, want %v", tt.input, ok, tt.wantSet)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSecurityPatterns(%q) = %v, want %v", tt.input, got, tt.want)
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
