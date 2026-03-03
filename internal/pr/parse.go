package pr

import (
	"fmt"
	"regexp"
	"strings"
)

// ExtractOwnerRepo parses owner and repo from a GitHub HTML URL
// (e.g. "https://github.com/OWNER/REPO/pull/123").
func ExtractOwnerRepo(htmlURL string) (string, string, error) {
	parts := strings.Split(strings.TrimPrefix(htmlURL, "https://github.com/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected URL format: %s", htmlURL)
	}
	return parts[0], parts[1], nil
}

var conventionalCommitPrefixRE = regexp.MustCompile(`(?i)^\w+(\([^)]*\))?:\s*`)

func stripConventionalCommitPrefix(title string) string {
	return conventionalCommitPrefixRE.ReplaceAllString(title, "")
}

var dependencyPatterns = []*regexp.Regexp{
	// Renovate: "Update dependency foo/bar to v1.2.3"
	regexp.MustCompile(`(?i)update dependency ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Update module foo/bar to v1.2.3"
	regexp.MustCompile(`(?i)update module ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Update github-actions action foo/bar to v1.2.3"
	regexp.MustCompile(`(?i)update [\w\-]+ action ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Update rust crate kube to v3"
	regexp.MustCompile(`(?i)update [\w\-]+ crate ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Update terraform aws to v5"
	regexp.MustCompile(`(?i)update terraform ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Update opentelemetry-go monorepo" or "Update eslint monorepo to v10"
	regexp.MustCompile(`(?i)update ([@\w\-./]+(?:/[@\w\-./]+)*) monorepo`),
	// Renovate: "Update all non-major dependencies"
	regexp.MustCompile(`(?i)update (all [\w\-]+ dependencies)`),
	// Renovate: "Pin dependency foo to v1.2.3"
	regexp.MustCompile(`(?i)pin dependency ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Replace dependency foo with bar"
	regexp.MustCompile(`(?i)replace dependency ([@\w\-./]+(?:/[@\w\-./]+)*)`),
	// Renovate: "Lock file maintenance"
	regexp.MustCompile(`(?i)^(lock file maintenance)$`),
	// Renovate: "Update foo/bar to v1.2.3" (generic fallback, must be after specific patterns)
	regexp.MustCompile(`(?i)^update ([@\w\-./]+(?:/[@\w\-./]+)*) to `),
	// Renovate: grouped update "Update github-actions (major)" or "Update npm (minor)"
	regexp.MustCompile(`(?i)^update ([@\w\-./]+(?:/[@\w\-./]+)*)\s+\((?:major|minor|patch|digest)\)\s*$`),
	// Dependabot: "Bump foo from 1.2.3 to 1.2.4"
	regexp.MustCompile(`(?i)bump ([@\w\-./]+(?:/[@\w\-./]+)*) from`),
	// Dependabot: "Bump the foo group ..."
	regexp.MustCompile(`(?i)bump the ([\w\-]+) group`),
	// Dependabot: "Bump foo to 1.2.3"
	regexp.MustCompile(`(?i)bump ([@\w\-./]+(?:/[@\w\-./]+)*) to `),
	// Renovate onboarding: "Configure Renovate"
	regexp.MustCompile(`(?i)^(configure renovate)$`),
	// Dependabot onboarding: "Set package ecosystem to 'gomod' in dependabot config"
	regexp.MustCompile(`(?i)set package[- ]ecosystem to '?([\w\-]+)'?`),
}

// IsDependencyUpdateTitle returns true if the PR title looks like an automated
// dependency update (Renovate or Dependabot), regardless of who authored it.
func IsDependencyUpdateTitle(title string) bool {
	title = strings.TrimSpace(title)
	lower := strings.ToLower(title)

	// Conventional commit with deps scope: "chore(deps):", "fix(deps):", etc.
	if idx := strings.Index(lower, ":"); idx != -1 {
		if strings.Contains(lower[:idx], "deps") {
			return true
		}
	}

	// Known dependency update title patterns (Renovate/Dependabot)
	return ExtractDependencyName(title) != ""
}

var versionPatterns = []*regexp.Regexp{
	// "from vX.Y.Z to vA.B.C" – captures both source and target
	regexp.MustCompile(`(?i)\bfrom\s+(v?\d[\w.\-+]*)\s+to\s+(v?\d[\w.\-+]*)`),
	// "to vX.Y.Z" at end of title (with optional parenthetical like "(major)")
	regexp.MustCompile(`(?i)\bto\s+(v?\d[\w.\-+]*)(?:\s*\(.*\))?\s*$`),
}

func ExtractVersion(title string) string {
	title = strings.TrimSpace(title)
	title = stripConventionalCommitPrefix(title)
	for _, pat := range versionPatterns {
		matches := pat.FindStringSubmatch(title)
		if len(matches) >= 3 {
			return matches[1] + " -> " + matches[2]
		}
		if len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}

func ExtractDependencyName(title string) string {
	title = strings.TrimSpace(title)
	title = stripConventionalCommitPrefix(title)
	for _, pat := range dependencyPatterns {
		matches := pat.FindStringSubmatch(title)
		if len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}
