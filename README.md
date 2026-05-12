# seal

> Pin and verify the AI-agent bundles in your repo. Catch tampered skills, prompts, and sub-agents before they ship.

`seal` records the trusted state of your project-local agent assets — Claude Code skills, Codex skills, sub-agent definitions — into a single lockfile (`seal.json`). Run `seal verify` in CI to refuse builds when those assets drift from what was pinned.

Think `package-lock.json`, but for the context your agents read.

---

## Install

```sh
go install github.com/SamyGhannad/seal-cli/cmd/seal@latest
```

Or grab a prebuilt binary from the [releases page](https://github.com/SamyGhannad/seal-cli/releases) and drop it on your `PATH`.

Verify the install:

```sh
seal --version
```

---

## Quickstart

```sh
# 1. Bootstrap the lockfile (auto-detects .claude/skills, .agents/skills, etc.)
seal init

# 2. Commit seal.json alongside the assets it pins
git add seal.json && git commit -m "chore: seal agent bundles"

# 3. Verify in CI — exits non-zero on tampering
seal verify

# 4. After an intentional change, re-pin
seal pin && git commit -am "chore: re-pin bundles"
```

That's the whole loop.

---

## What it auto-detects

`seal init` scans for these layouts and proposes them as discovery patterns:

| Layout                 | Pattern              |
|------------------------|----------------------|
| Claude Code plugins    | `.claude/plugins/*`  |
| Claude Code skills     | `.claude/skills/*`   |
| Codex skills           | `.codex/skills/*`    |
| Agent Skills           | `.agents/skills/*`   |

You can edit `seal.json` afterwards to add or remove discovery globs.

---

## Commands

### `seal init`

Bootstrap a fresh `seal.json` in the current directory.

```sh
seal init           # default policy: block
seal init --warn    # policy: warn — drift reported, not blocking
seal init -v        # show per-file detail in the summary
```

Refuses to overwrite an existing lockfile.

### `seal pin [path...]`

Re-pin bundles into the lockfile.

```sh
seal pin                              # bulk re-pin every discovery match
seal pin --prune                      # also drop entries no longer on disk
seal pin .claude/skills/my-skill      # targeted: re-pin one bundle
```

Targeted mode is byte-faithful: `seal pin Foo` refuses if the directory on disk is actually `foo`.

### `seal verify`

Compare on-disk state to the lockfile. Used in CI.

```sh
seal verify           # human output to stderr; exit code is the verdict
seal verify --json    # machine-readable output to stdout
seal verify --quiet   # exit code only, no output
```

---

## Outcomes & exit codes

| Outcome    | What happened                                                | Exit |
|------------|--------------------------------------------------------------|------|
| `Verified` | On-disk state matches the lockfile exactly                   | 0    |
| `Drift`    | New bundles appeared that aren't pinned (informational)      | 0    |
| `Warning`  | Bundles changed and policy is `warn`                         | 0    |
| `Blocked`  | Bundles changed and policy is `block`                        | 1    |

Fatal errors (missing/invalid lockfile, I/O failure) always exit `2`.

---

## Policy

`seal.json` carries a single `policy` field:

- **`block`** *(default)* — `seal verify` exits `1` on any mismatch. Use this in CI.
- **`warn`** — `seal verify` still reports drift on stderr, but exits `0`. Useful during early development.

Switch policy by hand-editing `seal.json` or re-running `seal init --warn`.

---

## CI example

GitHub Actions:

```yaml
- name: Install seal
  run: go install github.com/SamyGhannad/seal-cli/cmd/seal@latest

- name: Verify agent bundles
  run: seal verify
```

Exit code 1 fails the job. Exit code 0 means the agent context shipping with this commit is exactly what was pinned.

---

## How it works

- **Per-file hashes.** Each bundle file is hashed (SHA-256), recorded under its NFC-normalised forward-slash path.
- **Aggregate content hash.** Per-bundle hash over the sorted `<path>:<hash>` entries — one value to verify the whole bundle.
- **Deterministic encoding.** `seal.json` is byte-identical across machines: sorted keys, fixed indent, LF newlines, NFC text.
- **Atomic writes.** Lockfile updates use OS-level file locking and `tmp+rename` so concurrent runs can't corrupt the file.

---

## License

MIT.
