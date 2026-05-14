package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// writeBundleAndLockfile is a verify-test helper: writes a single bundle on
// disk under cwd/<key>, hashes it, and writes a.
// seal.json so a fresh `RunVerify(cwd)` will produce a clean "Verified"
// result. Lets every happy-path test be three lines.
//
// We keep this in verify_test.go (not testutil/) so the harness is visible
// right next to the tests that consume it.
func writeBundleAndLockfile(t *testing.T, cwd, key, file, content string) {
	t.Helper()

	// Drop the leading "./" so we can Join to a real path.
	rel := strings.TrimPrefix(key, "./")

	// Write the single tracked file under the bundle dir.
	full := filepath.Join(cwd, rel, file)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute the real hash so the lockfile actually matches disk.
	files, ch, err := seal.HashSealedRoot(filepath.Join(cwd, rel))
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}

	lf := &seal.Lockfile{
		Version: 1,
		Policy:  "block",
		Bundles: map[string]seal.Bundle{
			key: {ContentHash: ch, Files: files},
		},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestRunVerify_HappyPath is the integration smoke test: a freshly pinned
// bundle on disk + matching lockfile ⇒ exit 0, "Verified" surfaces in human
// output. If this one fails, the whole verify pipeline (Find → Read →
// buildOnDiskMap → Classify → derivedOutcome → output) is broken end-to-end
// and small unit tests probably won't localise the bug.
func TestRunVerify_HappyPath(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./skills/foo", "SKILL.md", "hello")

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdout: &stdout,
	})

	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Result: Verified") {
		t.Fatalf("missing 'Result: Verified' in:\n%s", stderr.String())
	}
}

// TestRunVerify_MissingLockfile pins the "no seal.json in cwd". It must
// surface a clear stderr message (operators look here first) and produce no
// stdout — JSON output is reserved for actual verify results.
func TestRunVerify_MissingLockfile(t *testing.T) {
	cwd := t.TempDir() // intentionally empty

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdout: &stdout,
	})

	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "seal.json") {
		t.Errorf("stderr should mention seal.json:\n%s", stderr.String())
	}
	// it SHOULD NOT output JSON to stdout." We follow that strictly.
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on exit 2; got:\n%s", stdout.String())
	}
}

// TestRunVerify_BlockedOnMismatch verifies the failure path: a pinned bundle
// whose contents have drifted on disk ⇒ Blocked.
// the default "block" policy ⇒ exit 1.
func TestRunVerify_BlockedOnMismatch(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./skills/foo", "SKILL.md", "original")

	// Edit the file out from under the lockfile — now the recorded hash no
	// longer matches what's on disk.
	if err := os.WriteFile(
		filepath.Join(cwd, "skills/foo/SKILL.md"),
		[]byte("tampered"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunVerify(VerifyOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdout: &bytes.Buffer{},
	})

	if code != 1 {
		t.Fatalf("exit %d, want 1 (Blocked); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Blocked") {
		t.Errorf("stderr should mention Blocked:\n%s", stderr.String())
	}
}

// TestRunVerify_JSONFlagWritesStdout ensures the --json flag routes
// machine-readable output to stdout and suppresses the human summary on
// stderr.
func TestRunVerify_JSONFlagWritesStdout(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./skills/foo", "SKILL.md", "hello")

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdout: &stdout,
		JSON:   true,
	})

	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	// stdout must contain the JSON shape (we don't re-parse here;
	// that's what TestVerifyJSON_Shape is for).
	if !strings.Contains(stdout.String(), `"status": "Verified"`) {
		t.Errorf("stdout missing JSON status field:\n%s", stdout.String())
	}
	// stderr should NOT contain the human summary when --json is.
	// (otherwise piping `seal verify --json | jq` produces noise).
	if strings.Contains(stderr.String(), "Result:") {
		t.Errorf("stderr should be quiet under --json:\n%s", stderr.String())
	}
}