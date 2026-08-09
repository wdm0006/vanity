# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.1] - 2026-08-08

### Fixed

- `go install github.com/wdm0006/vanity/cmd/vanity@latest` now works - the module path declared in `go.mod` did not match the repository, so installing from source failed
- `vanity sync --rebuild` now replaces the branch you were actually on instead of assuming `main`, and refuses to run from a detached HEAD rather than leaving the repository on a stray `temp-rebuild` branch
- `vanity sync --rebuild` now clears files inherited from the previous history when it creates the fresh branch, so a rebuild no longer leaves stale tracked files behind
- `vanity sync --rebuild --dry-run` now previews the full re-mirror a real rebuild would perform. It previously reported only incremental deltas - usually "No new contributions to mirror" - which understated the work by the entire mirror history
- `.vanity/<user>.json` is now written in date order on every sync. Contributions were previously serialized in random map order, so each sync produced a large spurious diff
- `vanity import --scrape` no longer hangs indefinitely on an unresponsive connection and no longer gets rejected by GitHub for using the default Go user agent. The request now carries a 30s timeout, a `vanity contribution scraper` user agent, and `Accept-Language: en-US,en;q=0.9` so the page is returned in English, which the tooltip parser requires
- `vanity import --scrape` now reports an error when a year's page yields zero parsed days while its header claims a non-zero total, instead of silently writing an empty year

## [0.4.0] - 2026-02-14

### Added

- `vanity sync --batch-size N` (default `100`) - pushes mirror commits in smaller increments. GitHub's contribution indexer silently drops older backdated commits when too many arrive in a single push
- `vanity sync --rebuild` - wipes the branch history and re-mirrors every stored contribution from scratch, for repairing a graph whose older backdated commits were never indexed
- Sync state is now checkpointed to disk at each batch push, so an interrupted run resumes instead of re-creating commits

## [0.3.0] - 2026-02-07

### Added

- `vanity import --scrape` flag - Scrape the GitHub profile page to capture private contributions that aren't available via the API

## [0.2.0] - 2026-01-31

### Added

- `vanity import <username>` - Import contributions from accounts you can't log into (e.g., old work accounts)
- Full contribution history fetching - imports now retrieve all years, not just the last year
- `--version` / `-v` flag to display version information
- Improved help text with examples for all commands
- Homebrew formula now declares `gh` as a dependency

### Improved

- Better error messages when `gh` CLI is not installed or not authenticated
- Enhanced command descriptions with usage examples

## [0.1.0] - 2026-01-31

### Added

- Initial release of Vanity CLI
- `vanity init` - Initialize a repository for contribution syncing
- `vanity sync` - Fetch contributions, export data, and create mirror commits
- `vanity sync --dry-run` - Preview sync without making changes
- `vanity status` - Show sync status and connected accounts
- Incremental sync support - only creates commits for new contributions
- Delta-based mirroring - tracks exact counts to handle contribution updates
- GitHub CLI (`gh`) integration for fetching contribution data
- Backdated empty commits to mirror contribution patterns
- Privacy-focused design - only dates and counts are shared, no commit content

### Technical

- Built with Go 1.21+
- Uses Cobra for CLI framework
- Requires GitHub CLI (`gh`) for authentication
- Cross-platform binaries (Linux, macOS, Windows)
- Homebrew tap available at `wdm0006/tap/vanity`

[0.4.1]: https://github.com/wdm0006/vanity/releases/tag/v0.4.1
[0.4.0]: https://github.com/wdm0006/vanity/releases/tag/v0.4.0
[0.3.0]: https://github.com/wdm0006/vanity/releases/tag/v0.3.0
[0.2.0]: https://github.com/wdm0006/vanity/releases/tag/v0.2.0
[0.1.0]: https://github.com/wdm0006/vanity/releases/tag/v0.1.0
