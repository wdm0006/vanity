# Contributing to Vanity

Thanks for your interest in contributing to Vanity!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/vanity.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run checks (see below)
6. Commit and push
7. Open a pull request

## Development Setup

### Prerequisites

- Go 1.21 or later
- GitHub CLI (`gh`) for testing

### Building

```bash
go build -o vanity ./cmd/vanity
```

### Running Checks

Before submitting a PR, please run:

```bash
# Format code
gofmt -w .

# Run linter
go vet ./...

# Run tests
go test ./...

# Verify it builds
go build ./cmd/vanity
```

## Code Style

- Follow standard Go conventions
- Run `gofmt` before committing
- Keep functions focused and small
- Add comments for exported functions
- Error messages should be lowercase and not end with punctuation

## Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Keep the first line under 72 characters
- Reference issues when relevant

Examples:
```
Add dry-run flag to sync command

Fix duplicate commits when count increases

Update README with installation instructions
```

## Pull Requests

- Fill out the PR template
- Link related issues
- Keep PRs focused on a single change
- Add tests for new functionality
- Update documentation if needed

## Reporting Bugs

When reporting bugs, please include:

- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
- Any error messages

## Feature Requests

Feature requests are welcome! Please:

- Check existing issues first
- Describe the use case
- Explain why it would be useful

## Questions?

Open an issue with your question and we'll do our best to help!
