# marge

A housekeeping tool for dependency update PRs. Marge automates the approve-and-merge workflow for [Renovate](https://docs.renovatebot.com/) and [Dependabot](https://docs.github.com/en/code-security/dependabot) pull requests that request your review.

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

Marge requires a GitHub personal access token with repo scope. Export it as an environment variable:

```bash
export GITHUB_TOKEN="ghp_..."
```

## Usage

```
marge [query] [flags]
```

When run without a query, marge enters **interactive mode**: it fetches all open bot PRs requesting your review and lets you pick a group to process.

When run with a query (e.g. a repo name or dependency), it filters PRs directly and processes them.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dry-run` | | `false` | Show what would be done without making changes |
| `--watch` | `-w` | `false` | Keep polling for new PRs every 60 seconds |
| `--grouping` | | `repo` | Group by `repo` or `dependency` |
| `--author` | | `all` | Filter by PR author: `renovate`, `dependabot`, or `all` |

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

### Other commands

```bash
marge version         # Print the current version
marge self-update     # Update to the latest release
```

## How it works

1. Searches GitHub for open PRs authored by `app/renovate` or `app/dependabot` that are either requesting your review or in your own repositories.
2. In interactive mode, groups results by repository (or dependency) and presents a selector.
3. For each selected PR, processes it in parallel (up to 5 concurrent):
   - Checks combined commit status and check runs; polls for up to 5 minutes if pending.
   - Approves the PR if not already approved.
   - If auto-merge is enabled, lets the merge queue handle it.
   - Otherwise determines the merge method (squash preferred) and merges directly.
4. Displays a live-updating table with PR status throughout.

## Development

```bash
make build          # Build the binary
make test           # Run tests
make lint           # Run golangci-lint
make help           # Show all available targets
```

## License

See [LICENSE](LICENSE) for details.
