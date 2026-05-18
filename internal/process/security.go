package process

import "strings"

// defaultSecurityCheckPatterns is the built-in list of case-insensitive
// substrings that mark a CI check as security-relevant. When a check whose
// name matches one of these patterns fails, the PR is reported with a
// distinct status so downstream tooling (and humans) do not treat it as
// ordinary build/test flakiness.
//
// Patterns are validated against real-world check-run names observed in
// production workflows (e.g. aquasecurity/trivy's "Scan Go vulnerabilities"
// matches via "vulnerabilities"). Some scanners use generic job names like
// the official github/codeql-action template's "Analyze (go)" -- the
// "codeql" substring will not catch that, so CodeQL users who rely on the
// default job name should extend this list via --security-patterns.
var defaultSecurityCheckPatterns = []string{
	"security",
	"govulncheck",
	"trivy",
	"codeql",
	"snyk",
	"gosec",
	"gitleaks",
	"semgrep",
	"checkov",
	"kics",
	"vulnerability",
	"vulnerabilities",
	"sast",
	"dast",
	"dependency-review",
	"dependency review",
}

// DefaultSecurityCheckPatterns returns a fresh copy of the built-in
// security-check pattern list. Returning a copy guarantees that callers
// cannot accidentally mutate the package-level default.
func DefaultSecurityCheckPatterns() []string {
	out := make([]string, len(defaultSecurityCheckPatterns))
	copy(out, defaultSecurityCheckPatterns)
	return out
}

// normalizePatterns lower-cases and trims each entry, dropping empties.
// Returning a normalized slice once lets the hot loop in
// classifySecurityFailure use a plain strings.Contains.
func normalizePatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// classifySecurityFailure returns the name of the first failing check that
// matches any of the configured security patterns, or "" if none match.
// Patterns must already be normalized (see normalizePatterns); matching is
// a case-insensitive substring match against the check name.
func classifySecurityFailure(failedChecks []string, patterns []string) string {
	if len(failedChecks) == 0 || len(patterns) == 0 {
		return ""
	}
	for _, name := range failedChecks {
		lower := strings.ToLower(name)
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				return name
			}
		}
	}
	return ""
}
