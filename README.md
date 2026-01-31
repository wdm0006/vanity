# Vanity

**Sync your GitHub contribution graph across multiple accounts.**

Vanity is a CLI tool that lets you maintain a unified contribution history across multiple GitHub accounts. If you contribute to projects from different accounts (work, personal, client, etc.), Vanity ensures each account's activity graph reflects your total contributions.

---

## The Problem

GitHub's contribution graph only shows activity from repositories where you're a contributor. If you have multiple GitHub accounts, each one tells an incomplete story—your work account doesn't show your open source contributions, and your personal account doesn't show your professional work.

## The Solution

Vanity creates a shared private repository where collaborating accounts sync their contribution metadata. When you run `vanity sync`, it:

1. **Exports** your contribution data (dates and counts) to the shared repo
2. **Imports** other collaborators' contribution data
3. **Creates mirror commits** backdated to match their contribution patterns

The result: every synced account's GitHub graph displays the combined contribution history of all participants.

---

## Privacy & Security

### What IS shared
- **Dates** of contributions (e.g., "2024-01-15")
- **Counts** per day (e.g., "5 contributions")

### What is NOT shared
- Repository names
- Commit messages
- Code or diffs
- File names
- Any actual content

The shared repository contains only:
- Empty commits with generic messages (`vanity: mirror from <user>`)
- JSON files with date/count pairs

**Your contribution patterns are visible to collaborators, but nothing about what you actually worked on.**

---

## Installation

### Prerequisites

- [GitHub CLI (`gh`)](https://cli.github.com/) installed and authenticated

### Homebrew (macOS/Linux)

```bash
brew install wdm0006/tap/vanity
```

### Download Binary

Download the latest release from [GitHub Releases](https://github.com/wdm0006/vanity/releases):

```bash
# macOS (Apple Silicon)
curl -Lo vanity https://github.com/wdm0006/vanity/releases/latest/download/vanity_darwin_arm64.tar.gz
tar -xzf vanity_darwin_arm64.tar.gz
mv vanity /usr/local/bin/

# macOS (Intel)
curl -Lo vanity https://github.com/wdm0006/vanity/releases/latest/download/vanity_darwin_amd64.tar.gz

# Linux (amd64)
curl -Lo vanity https://github.com/wdm0006/vanity/releases/latest/download/vanity_linux_amd64.tar.gz

# Linux (arm64)
curl -Lo vanity https://github.com/wdm0006/vanity/releases/latest/download/vanity_linux_arm64.tar.gz
```

### Go Install

```bash
go install github.com/wdm0006/vanity/cmd/vanity@latest
```

### Build from Source

```bash
git clone https://github.com/wdm0006/vanity.git
cd vanity
go build -o vanity ./cmd/vanity
mv vanity /usr/local/bin/
```

---

## Quick Start

### 1. Create a shared repository

Create a new **private** repository on GitHub. This will be the sync hub for all your accounts.

```bash
# Clone the empty repo
git clone https://github.com/you/vanity-sync.git
cd vanity-sync

# Initialize vanity
vanity init
vanity sync
```

### 2. Invite your other accounts

Add your other GitHub accounts as collaborators to the private repo (Settings → Collaborators).

### 3. Sync from each account

From each account, clone the repo and sync:

```bash
git clone https://github.com/you/vanity-sync.git
cd vanity-sync
vanity sync
```

That's it. Each account now has mirror commits reflecting everyone's combined contributions.

---

## Commands

### `vanity init`

Initializes the current repository for vanity syncing. Creates a `.vanity/` directory to store contribution data and sync state.

```bash
vanity init
```

### `vanity sync`

The main command. Fetches your contributions, exports them, imports others', and creates mirror commits.

```bash
vanity sync           # Full sync
vanity sync --dry-run # Preview without making changes
```

### `vanity status`

Shows the current sync status, connected accounts, and contribution counts.

```bash
vanity status
```

Example output:
```
Current user: alice

Synced users:
  - alice (you): 847 contributions, last updated 2024-01-15 10:30
  - bob: 523 contributions, last updated 2024-01-14 18:45

Last sync: 2024-01-15 10:30
Mirrored from:
  - bob: 142 dates
```

---

## How It Works

### The Sync Process

```
┌─────────────────────────────────────────────────────────────────┐
│                     Shared Private Repo                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ .vanity/                                                 │   │
│  │   ├── alice.json        ← Alice's contribution data     │   │
│  │   ├── alice-state.json  ← Alice's sync state            │   │
│  │   ├── bob.json          ← Bob's contribution data       │   │
│  │   └── bob-state.json    ← Bob's sync state              │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  + Empty mirror commits (backdated to contribution dates)       │
└─────────────────────────────────────────────────────────────────┘
          ▲                                       ▲
          │ vanity sync                           │ vanity sync
          │                                       │
    ┌─────┴─────┐                           ┌─────┴─────┐
    │  Alice's  │                           │   Bob's   │
    │  Account  │                           │  Account  │
    └───────────┘                           └───────────┘
```

### Why Empty Commits?

GitHub counts commits toward your contribution graph if:
1. You are the **author** of the commit
2. The commit is in a repository you have access to

Vanity creates empty commits (no file changes) authored by you, backdated to match others' contribution dates. These commits are:
- Lightweight (no code bloat)
- Private (in your shared private repo)
- Legitimate activity that GitHub counts

### Incremental Sync

Vanity tracks what's already been synced to avoid duplicate commits. Each sync only processes new contributions since your last sync.

---

## Data Format

### Contribution Data (`.vanity/<username>.json`)

```json
{
  "username": "alice",
  "last_updated": "2024-01-15T10:30:00Z",
  "contributions": [
    { "date": "2024-01-01", "count": 5 },
    { "date": "2024-01-02", "count": 3 },
    { "date": "2024-01-03", "count": 8 }
  ]
}
```

### Sync State (`.vanity/<username>-state.json`)

```json
{
  "username": "alice",
  "last_sync": "2024-01-15T10:30:00Z",
  "mirrored_counts": {
    "bob": {
      "2024-01-01": 5,
      "2024-01-02": 3
    }
  }
}
```

The `mirrored_counts` tracks exactly how many commits have been mirrored for each date from each user. This enables **incremental updates**: if Bob's contribution count for a day increases from 3 to 5, only 2 new commits are created on your next sync.

---

## FAQ

### Will this affect my actual repositories?

No. Vanity only operates within the shared sync repository. Your other repositories are never touched.

### Is this against GitHub's Terms of Service?

Vanity creates real commits that you author—it's legitimate activity. However, the contribution graph is meant to reflect genuine work. Use this tool responsibly to unify your own contributions across accounts, not to artificially inflate activity.

### Can I sync with people I don't know?

You can, but the shared repo will reveal your contribution patterns to all collaborators. Only sync with accounts you trust.

### What if I stop syncing?

The mirror commits already created will remain. Your contribution graph will continue showing historical synced activity, but won't update with new contributions from others.

### Can I undo a sync?

You'd need to manually remove the mirror commits from the shared repo's git history (e.g., with `git rebase`). There's no built-in undo command.

---

## Development

### Project Structure

```
vanity/
├── cmd/vanity/
│   └── main.go              # CLI entry point
├── internal/
│   ├── cli/                 # Command implementations
│   │   ├── root.go          # Root command & setup
│   │   ├── init.go          # vanity init
│   │   ├── sync.go          # vanity sync
│   │   └── status.go        # vanity status
│   ├── github/
│   │   └── contributions.go # GitHub API via gh CLI
│   ├── git/
│   │   └── commits.go       # Git operations
│   └── sync/
│       ├── engine.go        # Core sync logic
│       └── state.go         # State management
├── go.mod
└── go.sum
```

### Building

```bash
# Build the binary
go build -o vanity ./cmd/vanity

# Build and install to $GOPATH/bin
go install ./cmd/vanity
```

### Formatting

```bash
# Format all Go files
gofmt -w .

# Check formatting (returns non-zero if files need formatting)
gofmt -l .
```

### Linting

```bash
# Run go vet
go vet ./...

# Run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
staticcheck ./...
```

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Dependencies

```bash
# Download dependencies
go mod download

# Tidy dependencies (remove unused, add missing)
go mod tidy
```

---

## License

MIT

---

## Contributing

Contributions welcome! Please open an issue or PR.

Before submitting:
1. Run `gofmt -w .` to format code
2. Run `go vet ./...` to check for issues
3. Ensure `go build ./cmd/vanity` succeeds
