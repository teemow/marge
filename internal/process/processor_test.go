package process

import (
	"testing"
)

func TestIsAuthorTrusted_defaultBots(t *testing.T) {
	p := NewProcessor(nil, false, false, "myuser", DefaultTrustedAuthors)

	tests := []struct {
		login string
		want  bool
	}{
		{"renovate[bot]", true},
		{"dependabot[bot]", true},
		{"myuser", true},
		{"MYUSER", true},
		{"evil-user", false},
		{"renovate", false},
		{"dependabot", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			got := p.isAuthorTrusted(tt.login)
			if got != tt.want {
				t.Errorf("isAuthorTrusted(%q) = %v, want %v", tt.login, got, tt.want)
			}
		})
	}
}

func TestIsAuthorTrusted_customList(t *testing.T) {
	custom := map[string]bool{
		"my-bot[bot]": true,
	}
	p := NewProcessor(nil, false, false, "admin", custom)

	tests := []struct {
		login string
		want  bool
	}{
		{"my-bot[bot]", true},
		{"admin", true},
		{"ADMIN", true},
		{"renovate[bot]", false},
		{"dependabot[bot]", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			got := p.isAuthorTrusted(tt.login)
			if got != tt.want {
				t.Errorf("isAuthorTrusted(%q) = %v, want %v", tt.login, got, tt.want)
			}
		})
	}
}

func TestIsAuthorTrusted_nilDefaults(t *testing.T) {
	p := NewProcessor(nil, false, false, "me", nil)

	if !p.isAuthorTrusted("renovate[bot]") {
		t.Error("nil trustedAuthors should default to DefaultTrustedAuthors; renovate[bot] should be trusted")
	}
	if !p.isAuthorTrusted("dependabot[bot]") {
		t.Error("nil trustedAuthors should default to DefaultTrustedAuthors; dependabot[bot] should be trusted")
	}
	if !p.isAuthorTrusted("me") {
		t.Error("own login should always be trusted")
	}
	if p.isAuthorTrusted("stranger") {
		t.Error("unknown author should not be trusted")
	}
}

func TestIsAuthorTrusted_selfCaseInsensitive(t *testing.T) {
	p := NewProcessor(nil, false, false, "MyUser", DefaultTrustedAuthors)

	if !p.isAuthorTrusted("myuser") {
		t.Error("self-login comparison should be case-insensitive")
	}
	if !p.isAuthorTrusted("MYUSER") {
		t.Error("self-login comparison should be case-insensitive")
	}
	if !p.isAuthorTrusted("MyUser") {
		t.Error("self-login comparison should be case-insensitive")
	}
}

func TestDefaultTrustedAuthors(t *testing.T) {
	expected := map[string]bool{
		"renovate[bot]":   true,
		"dependabot[bot]": true,
	}

	if len(DefaultTrustedAuthors) != len(expected) {
		t.Fatalf("DefaultTrustedAuthors has %d entries, want %d", len(DefaultTrustedAuthors), len(expected))
	}

	for k, v := range expected {
		if DefaultTrustedAuthors[k] != v {
			t.Errorf("DefaultTrustedAuthors[%q] = %v, want %v", k, DefaultTrustedAuthors[k], v)
		}
	}
}
