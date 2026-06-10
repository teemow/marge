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

Sometimes a check reports `failure` not because the code is broken but because GitHub never started the job -- a personal account or organization has exhausted its Actions budget. GitHub surfaces this as a job that never reached the runner:

- `The job was not started because an Actions budget is preventing further use.`

Verified against the live GitHub API, such a block shows up as a check run with conclusion `failure`, empty `output`, and a single failure-level **annotation** whose `message` carries the text above (the message is in the annotation, not the output fields). marge therefore inspects each failed check run's annotations and matches that message -- which never appears for a genuine test, build, or lint failure.

When **every** failing check on a PR is such a block, marge classifies the PR as `CI unavailable (budget)` rather than `Failed`. It is counted separately, surfaced under its own section, and kept out of any rescue path -- the fix is to raise or await the Actions budget, not to touch the code. If a PR has a mix of a genuine failure and a budget block, it is still reported as `Failed`.

> Only this API-verified message is matched; any unrecognized block degrades to the normal `Failed` path rather than risking a real failure being hidden.

#### PR age highlighting

Every table and report includes an **Age** column showing how long the PR has been open (`5h`, `3d`, `2w`). PRs older than 3 days are highlighted yellow, older than 7 days red -- an old dependency PR has already survived several sweeps and is the most likely to need manual work. Failure sections are sorted oldest-first for the same reason.

#### Rescue markers (prior AI rescue attempts)

When an automated rescue (a coding agent, a CI bot, a human with a script) tries to fix a failing dependency PR and gives up, it can record that attempt as a machine-readable **ai-rescue marker** inside an ordinary PR comment:

```markdown
**AI rescue failed** (klaus): nock v14 is ESM-only and breaks Jest CJS resolution.

<!-- ai-rescue: {"tool":"klaus","outcome":"failed","reason":"ESM-only breaks Jest CJS","head_sha":"d9f00bf2","at":"2026-06-09T18:40:00Z"} -->
```

On every sweep, marge reads the comments of each failing PR and annotates its entry with the most recent marker, e.g. `[rescue failed 1d ago (klaus): ESM-only breaks Jest CJS]`. The marker records the head SHA it was attempted against: when the PR branch is later rebased or gets a new version, the marker is reported as **stale** (`[rescue failed 3d ago (klaus), stale: new commits since]`) -- the attempt no longer describes the current code and the PR is fair game for another rescue.

This makes the daily triage call obvious at a glance:

- **failing + fresh failed rescue** -> automation already lost; a human is needed
- **failing + stale or no marker** -> dispatch (another) automated rescue

Use [`marge mark`](#marge-mark-pr-url-flags) to write markers without knowing the format. Any tool that can comment on a PR can participate -- there is no coupling to a specific agent framework.

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

### `marge mark <pr-url> [flags]`

Records a failed AI rescue attempt on a PR by posting an [ai-rescue marker](#rescue-markers-prior-ai-rescue-attempts) comment. The marker captures the PR's current head SHA, so it automatically goes stale when the branch changes.

| Flag | Default | Description |
|------|---------|-------------|
| `--outcome` | `failed` | Rescue outcome: `failed` (attempted, could not fix) or `blocked` (fix known but waits on something external) |
| `--reason` | | Short explanation of why the rescue did not succeed |
| `--tool` | `ai` | Name of the tool/agent that attempted the rescue (e.g. `klaus`) |

```bash
marge mark https://github.com/my-org/my-repo/pull/42 \
  --tool klaus --reason "nock v14 is ESM-only, needs Jest ESM migration"
```

Requires the token to have **Issues: Read & write** (comment) permission in addition to the permissions listed under [Setup](#setup).

### `marge serve`

Starts a stdio MCP server exposing two tools:

- **`sweep`** -- mirrors `marge sweep`, returning structured JSON (`summary`, `merged`, `security_failures`, `action_required`, `ci_unavailable`, `skipped`). Each PR entry includes `created_at`, `age_days`, and -- when a prior rescue attempt was found -- a `rescue` object (`tool`, `outcome`, `reason`, `at`, `stale`). Agent orchestrators should skip `action_required` entries whose rescue is `failed` and not `stale`, and escalate those to a human.
- **`mark`** -- mirrors `marge mark`, so rescue agents can record their own failed attempts.

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
5. Displays a live-updating table with columns for repository, dependency, version, age, author, and status. Failing entries are annotated with any prior AI rescue attempt found on the PR. Use `--no-tui` for plain-text output.

## Development

```bash
make build          # Build the binary
make test           # Run tests
make lint           # Run golangci-lint
make help           # Show all available targets
```

## License

MIT -- see [LICENSE](LICENSE) for details.
