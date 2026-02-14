# Vanity

**Sync your GitHub contribution graph across multiple accounts.**

If you contribute from different GitHub accounts — work, personal, freelance, old employers — each one only tells part of the story. Vanity combines them so every account's graph reflects your real total activity.

Only dates and counts are shared. No code, no commit messages, no repo names. Ever.

## Install

### Homebrew (macOS / Linux)

```bash
brew install wdm0006/tap/vanity
```

### Download a binary

Grab the latest from [Releases](https://github.com/wdm0006/vanity/releases) and drop it on your `PATH`.

### Go

```bash
go install github.com/wdm0006/vanity/cmd/vanity@latest
```

> **Prerequisite:** The [GitHub CLI (`gh`)](https://cli.github.com/) must be installed and authenticated.

## Quick start

### 1. Create a shared private repo

```bash
gh repo create vanity-sync --private --clone
cd vanity-sync
vanity init
vanity sync
```

### 2. Add your other accounts

Invite them as collaborators (repo Settings > Collaborators), then from each account:

```bash
git clone https://github.com/you/vanity-sync.git
cd vanity-sync
vanity sync
```

Done — every synced account's contribution graph now shows the combined history.

### 3. Import accounts you can't log into

Have an old work account you no longer have access to? Import its public contribution data (or scrape for private contributions too):

```bash
vanity import old-work-username
vanity import --scrape old-work-username   # includes private contributions
vanity sync
```

## Commands

| Command | Description |
|---------|-------------|
| `vanity init` | Set up a repo for syncing |
| `vanity sync` | Fetch, mirror, and push contributions |
| `vanity import <user>` | Import contributions from another account |
| `vanity status` | Show sync state and connected accounts |

### Sync options

```
--dry-run          Preview changes without writing anything
--batch-size N     Push every N mirror commits (default 100)
--rebuild          Wipe history and re-mirror everything from scratch
```

`--batch-size` exists because GitHub's contribution indexer can drop older backdated commits when too many are pushed at once. Pushing in smaller batches avoids this.

`--rebuild` is useful when contributions are missing from the graph. It creates a fresh orphan branch, re-mirrors all contributions with batch pushing, and force-pushes.

## How it works

```
           Shared Private Repo
┌──────────────────────────────────────┐
│  .vanity/                            │
│    alice.json ── date/count pairs    │
│    bob.json   ── date/count pairs    │
│                                      │
│  + empty mirror commits              │
│    (backdated to contribution dates) │
└──────────────┬───────────────┬───────┘
               │               │
          vanity sync     vanity sync
               │               │
           Alice's          Bob's
           Account          Account
```

GitHub counts a commit toward your graph if you authored it and it lives in a repo you have access to. Vanity creates lightweight empty commits — no file changes, no code — authored by you and backdated to match other accounts' contribution dates.

Syncs are incremental. Vanity tracks what's already been mirrored so each run only creates commits for new activity.

## Privacy

**Shared:** contribution dates and counts (e.g. "2024-03-15: 7 contributions").

**Not shared:** repository names, commit messages, code, diffs, file names — nothing about *what* you worked on.

The sync repo contains only JSON metadata files and empty commits with generic messages like `vanity: mirror from alice`.

## FAQ

**Will this touch my other repos?**
No. Vanity only operates inside the shared sync repo.

**Is this against GitHub's ToS?**
Vanity creates real commits that you author — it's legitimate activity. Use it responsibly to unify your own contributions, not to inflate activity.

**What if I stop syncing?**
Existing mirror commits remain. Your graph keeps showing historical synced activity but won't pick up new contributions from others.

**Can I undo a sync?**
Run `vanity sync --rebuild` to wipe the commit history and start fresh, or manually rewrite history with `git rebase`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
