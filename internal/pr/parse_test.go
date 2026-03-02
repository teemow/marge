package pr

import (
	"testing"
)

func TestParseRepoFromURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "standard GitHub URL",
			url:       "https://github.com/giantswarm/flux-app",
			wantOwner: "giantswarm",
			wantRepo:  "flux-app",
		},
		{
			name:      "GitHub URL with trailing slash",
			url:       "https://github.com/giantswarm/flux-app/",
			wantOwner: "giantswarm",
			wantRepo:  "flux-app",
		},
		{
			name:      "GitHub URL with .git suffix",
			url:       "https://github.com/giantswarm/flux-app.git",
			wantOwner: "giantswarm",
			wantRepo:  "flux-app",
		},
		{
			name:      "GitHub URL with path segments",
			url:       "https://github.com/giantswarm/flux-app/pull/42",
			wantOwner: "giantswarm",
			wantRepo:  "flux-app",
		},
		{
			name:      "SSH-style URL",
			url:       "git@github.com/giantswarm/flux-app",
			wantOwner: "giantswarm",
			wantRepo:  "flux-app",
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "unrelated URL",
			url:     "https://example.com/foo",
			wantErr: true,
		},
		{
			name:    "only owner, no repo",
			url:     "https://github.com/giantswarm",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRepoFromURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got owner=%q repo=%q", owner, repo)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestExtractDependencyName(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		// Renovate: "Update dependency X to Y"
		{
			name:  "renovate update dependency",
			title: "Update dependency github.com/foo/bar to v1.2.3",
			want:  "github.com/foo/bar",
		},
		{
			name:  "renovate update dependency case insensitive",
			title: "update dependency lodash to 4.17.21",
			want:  "lodash",
		},
		// Renovate: "Update X to Y" (without "dependency")
		{
			name:  "renovate update without dependency keyword",
			title: "Update golang.org/x/net to v0.5.0",
			want:  "golang.org/x/net",
		},
		// Renovate: "Update module X to Y"
		{
			name:  "renovate update module",
			title: "Update module github.com/aws/aws-sdk-go to v1.44.0",
			want:  "github.com/aws/aws-sdk-go",
		},
		// Renovate: "Update github-actions action X to Y"
		{
			name:  "renovate update github-actions action",
			title: "Update github-actions action actions/checkout to v4",
			want:  "actions/checkout",
		},
		// Renovate: "Pin dependency X to Y"
		{
			name:  "renovate pin dependency",
			title: "Pin dependency typescript to 5.3.3",
			want:  "typescript",
		},
		// Renovate: "Replace dependency X with Y"
		{
			name:  "renovate replace dependency",
			title: "Replace dependency io/ioutil with os",
			want:  "io/ioutil",
		},
		// Renovate: "Lock file maintenance"
		{
			name:  "renovate lock file maintenance",
			title: "Lock file maintenance",
			want:  "Lock file maintenance",
		},
		// Dependabot: "Bump X from Y to Z"
		{
			name:  "dependabot bump from to",
			title: "Bump lodash from 4.17.20 to 4.17.21",
			want:  "lodash",
		},
		{
			name:  "dependabot bump scoped package",
			title: "Bump github.com/spf13/cobra from 1.8.0 to 1.9.1",
			want:  "github.com/spf13/cobra",
		},
		// Dependabot: "Bump the X group"
		{
			name:  "dependabot bump group",
			title: "Bump the go-deps group across 3 directories with 5 updates",
			want:  "go-deps",
		},
		{
			name:  "dependabot bump group with updates",
			title: "Bump the npm-production group with 2 updates",
			want:  "npm-production",
		},
		// Dependabot: "Bump X to Y"
		{
			name:  "dependabot bump to",
			title: "Bump actions/checkout to v4",
			want:  "actions/checkout",
		},
		// Edge cases
		{
			name:  "empty title",
			title: "",
			want:  "",
		},
		{
			name:  "unrecognized title",
			title: "Fix a bug in the widget",
			want:  "",
		},
		{
			name:  "whitespace-padded title",
			title: "  Update dependency foo to v1.0.0  ",
			want:  "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDependencyName(tt.title)
			if got != tt.want {
				t.Errorf("ExtractDependencyName(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}
