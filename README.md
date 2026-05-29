<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo-light.png">
    <img alt="marge" src="assets/logo-light.png" height="160">
  </picture>
</p>
<h1 align="center">marge</h1>
<p align="center">
  A housekeeping tool for dependency update PRs.<br>
  Automates the approve-and-merge workflow for
  <a href="https://docs.renovatebot.com/">Renovate</a> and
  <a href="https://docs.github.com/en/code-security/dependabot">Dependabot</a>
  pull requests that request your review.
</p>

---

It searches GitHub for open bot PRs, groups them interactively, waits for CI checks to pass, approves, and merges them -- with a live terminal table showing progress.

## Install

### From GitHub releases

Download the latest binary from the [releases page](https://github.com/teemow/marge/releases) for your platform (Linux, macOS, Windows; amd64 and arm64).

### From source

```bash
go install github.com/teemow/marge@latest
```

Or clone and build locally:

```bash
git clone https://github.com/teemow/marge.git
cd marge
make install
```

## Setup

Marge requires a GitHub personal access token. Export it as an environment variable:

```bash
export GITHUB_TOKEN="ghp_..."
```

**Classic token:** needs the `repo` scope.

**Fine-grained token:** select the repositories you want marge to manage, then grant these permissions:

| Permission | Access | Why |
|------------|--------|-----|
| Pull requests | Read & write | Approve and merge PRs |
| Checks | Read | Wait for CI status |
| Commit statuses | Read | Read combined commit status |
| Metadata | Read | Required by default |

## Usage

### `marge [query] [flags]` (default command)

When run without a query, marge enters **interactive mode**: it fetches all open bot PRs requesting your review and lets you pick a group to process.

When run with a query (e.g. a repo name or dependency), it filters PRs directly and processes them.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dry-run` | | `false` | Show what would be done without making changes |
| `--watch` | `-w` | `false` | Keep polling for new PRs every 60 seconds |
| `--grouping` | | `repo` | Group by `repo` or `dependency` |
| `--author` | | `all` | Filter by PR author: `renovate`, `dependabot`, or `all` |
| `--org` | | | Limit to repos owned by this org or user |
| `--no-tui` | | `false` | Disable the live table; print plain-text results instead |
| `--trusted-authors` | | `renovate[bot],dependabot[bot]` | Comma-separated list of trusted PR author logins |
| `--security-patterns` | | _(built-in)_ | Override the built-in security check pattern list (see below) |

#### Security check patterns

When a PR's CI fails, marge classifies the failure as security-related if any failing check's name contains one of the configured substrings (case-insensitive). Security failures are surfaced separately so they are not mistaken for ordinary build/test flakiness.

The built-in list contains: `security`, `govulncheck`, `trivy`, `codeql`, `snyk`, `gosec`, `gitleaks`, `semgrep`, `checkov`, `kics`, `vulnerability`, `vulnerabilities`, `sast`, `dast`, `dependency-review`, `dependency review`.

Pass `--security-patterns "Trivy,Govulncheck,CodeQL,Analyze"` to replace the list. The github/codeql-action template uses a job name like `Analyze (<lang>)` that the `codeql` substring will not match, so add `Analyze` if you rely on that template.

#### CI unavailable (Actions budget)

Sometimes a check reports `failure` not because the code is broken but because GitHub never started the job -- e.g. a personal account or organization has exhausted its Actions budget, hit its spending limit, or has a failed payment. GitHub surfaces this as a job that never reached the runner, with messages such as:

- `The job was not started because an Actions budget is preventing further use.`
- `The job was not started because recent account payments have failed or your spending limit needs to be increased. ...`
- `The job was not started because your account is locked due to a billing issue.`

Verified against the live GitHub API, such a block shows up as a check run with conclusion `failure`, empty `output`, and a single failure-level **annotation** whose `message` carries the text above (the message is in the annotation, not the output fields). marge therefore inspects each failed check run's annotations and detects the block by the platform prefix `the job was not started because` paired with a billing/budget marker -- a prefix that never appears for a genuine test, build, or lint failure.

When **every** failing check on a PR is such a block, marge classifies the PR as `CI unavailable (budget)` rather than `Failed`. It is counted separately, surfaced under its own section, and kept out of any rescue path -- the fix is to raise or await the Actions budget, not to touch the code. If a PR has a mix of a genuine failure and a budget block, it is still reported as `Failed`.

> Detection is deliberately conservative: an unrecognized variant degrades to the normal `Failed` path rather than risking a real failure being hidden.

### `marge sweep [flags]`

Processes all matching PRs without interactive grouping. After processing, prints a **Security failures** section, an **Action required** section listing any PRs that failed, have conflicts, or came from untrusted authors, and a **CI unavailable (Actions budget)** section for PRs whose checks never ran because an Actions spending limit was exhausted. Security failures (e.g. govulncheck, Trivy, CodeQL) are separated so they are not mistaken for ordinary CI flakiness, and budget-blocked PRs are separated so they are not mistaken for genuine failures (see [CI unavailable (Actions budget)](#ci-unavailable-actions-budget)).

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dry-run` | | `false` | Show what would be done without making changes |
| `--watch` | `-w` | `false` | Keep polling for new PRs every 60 seconds |
| `--author` | | `all` | Filter by PR author: `renovate`, `dependabot`, or `all` |
| `--org` | | | Limit to repos owned by this org or user |
| `--no-tui` | | `false` | Disable the live table; print plain-text results instead |
| `--merge-auto` | | `false` | Also merge PRs that have auto-merge enabled (by default these are skipped) |
| `--trusted-authors` | | `renovate[bot],dependabot[bot]` | Comma-separated list of trusted PR author logins |
| `--security-patterns` | | _(built-in)_ | Override the built-in security check pattern list (see [Security check patterns](#security-check-patterns)) |

### Other commands

```bash
marge version         # Print the current version
marge self-update     # Update to the latest release
```

### Examples

Process all bot PRs interactively, grouped by repository:

```bash
marge
```

Process only Renovate PRs:

```bash
marge --author renovate
```

Filter PRs matching a query and keep watching:

```bash
marge "my-org/my-repo" --watch
```

Dry run to preview what would happen:

```bash
marge --dry-run
```

Group by dependency instead of repository:

```bash
marge --grouping dependency
```

Sweep all PRs in a specific org:

```bash
marge sweep --org my-org
```

Sweep including PRs with auto-merge enabled:

```bash
marge sweep --merge-auto
```

## How it works

1. Searches GitHub for open PRs authored by `app/renovate` or `app/dependabot` that are either requesting your review or in your own repositories. Self-authored dependency-update PRs (e.g. from self-hosted Renovate) in your repos are also included.
2. In interactive mode, groups results by repository (or dependency) and presents a selector.
3. Validates each PR's author against a trusted allow-list (`renovate[bot]`, `dependabot[bot]`, and the authenticated user by default). PRs from untrusted authors are refused with a clear status message. You can extend the allow-list with `--trusted-authors`.
4. For each selected PR, processes it in parallel (up to 5 concurrent):
   - Checks combined commit status and check runs; polls every 15 seconds for up to 5 minutes if pending.
   - Approves the PR if not already approved.
   - If auto-merge is enabled, lets the merge queue handle it (unless `--merge-auto` is set).
   - Otherwise merges via squash.
5. Displays a live-updating table with columns for repository, dependency, version, author, and status. Use `--no-tui` for plain-text output.

## Development

```bash
make build          # Build the binary
make test           # Run tests
make lint           # Run golangci-lint
make help           # Show all available targets
```

## License

MIT -- see [LICENSE](LICENSE) for details.
