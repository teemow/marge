package process

import "strings"

// DefaultSecurityCheckPatterns is the default list of case-insensitive
// substrings that mark a CI check as security-relevant. When a check whose
// name matches one of these patterns fails, the PR is reported with a
// distinct status so downstream tooling (and humans) do not treat it as
// ordinary build/test flakiness.
var DefaultSecurityCheckPatterns = []string{
	"security scan",
	"security",
	"govulncheck",
	"trivy",
	"codeql",
	"snyk",
	"gosec",
	"vulnerability",
	"vulnerabilities",
	"sast",
	"dast",
	"dependency-review",
	"dependency review",
}

// classifySecurityFailure returns the name of the first failing check that
// matches any of the configured security patterns, or "" if none match.
// Matching is case-insensitive substring match against the check name.
func classifySecurityFailure(failedChecks []string, patterns []string) string {
	if len(failedChecks) == 0 || len(patterns) == 0 {
		return ""
	}
	for _, name := range failedChecks {
		lower := strings.ToLower(name)
		for _, p := range patterns {
			p = strings.TrimSpace(strings.ToLower(p))
			if p == "" {
				continue
			}
			if strings.Contains(lower, p) {
				return name
			}
		}
	}
	return ""
}
