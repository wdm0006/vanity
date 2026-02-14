# Contributing to Vanity

Thanks for your interest in contributing!

## Getting started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/vanity.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run checks (see below)
6. Commit and push
7. Open a pull request

## Prerequisites

- Go 1.21+
- [GitHub CLI (`gh`)](https://cli.github.com/) for testing

## Building

```bash
go build -o vanity ./cmd/vanity
```

## Running checks

Before submitting a PR:

```bash
gofmt -w .          # format code
go vet ./...        # lint
go test ./...       # tests
go build ./cmd/vanity  # verify build
```

## Project structure

```
vanity/
├── cmd/vanity/
│   └── main.go              # Entry point
├── internal/
│   ├── cli/                 # Cobra command definitions
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── sync.go
│   │   ├── import.go
│   │   └── status.go
│   ├── github/
│   │   └── contributions.go # GitHub API via gh CLI
│   ├── git/
│   │   └── commits.go       # Git operations (commits, push, branches)
│   └── sync/
│       ├── engine.go        # Core sync/rebuild logic
│       └── state.go         # State and contribution data persistence
├── .goreleaser.yaml
├── go.mod
└── go.sum
```

## Data format

Contribution data is stored in `.vanity/<username>.json`:

```json
{
  "username": "alice",
  "last_updated": "2024-01-15T10:30:00Z",
  "contributions": [
    { "date": "2024-01-01", "count": 5 },
    { "date": "2024-01-02", "count": 3 }
  ]
}
```

Sync state is tracked in `.vanity/<username>-state.json`:

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

`mirrored_counts` tracks how many commits have been mirrored per user/date, enabling incremental syncs — only deltas are created.

## Code style

- Standard Go conventions, run `gofmt` before committing
- Keep functions focused and small
- Comments on exported functions
- Lowercase error messages without trailing punctuation

## Commit messages

- Present tense, imperative mood ("Add feature" not "Added feature")
- First line under 72 characters
- Reference issues when relevant

## Reporting bugs

Please include: Go version, OS/arch, steps to reproduce, expected vs actual behavior, and any error messages.
