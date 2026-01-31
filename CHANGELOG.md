# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.2.0]: https://github.com/wdm0006/vanity/releases/tag/v0.2.0
[0.1.0]: https://github.com/wdm0006/vanity/releases/tag/v0.1.0
