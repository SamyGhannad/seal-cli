package seal

import (
	"strings"
	"testing"
)

// TestDecode_Valid verifies a well-formed lockfile decodes without error and
// produces the expected struct shape. This is the happy-path smoke test;
// deeper structural checks live in validate_test.go.
func TestDecode_Valid(t *testing.T) {
	in := `{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./.claude/skills/foo": {
      "contentHash": "sha256:4bc6ee3c79cf31fe7f32fb3fbcd0f96027a23dea307e5d3fcc7afa7b292e5989",
      "files": {
        "SKILL.md": "sha256:3bfc269594ef649228e9a74bab00f042efc91d5acc6fbee31a382e80d42388fe"
      }
    }
  }
}`
	lf, err := Decode([]byte(in))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Spot-check: version, policy, one bundle with one file.
	if lf.Version != 1 {
		t.Errorf("Version: got %d, want 1", lf.Version)
	}
	if lf.Policy != "block" {
		t.Errorf("Policy: got %q, want %q", lf.Policy, "block")
	}
	if got := len(lf.Bundles); got != 1 {
		t.Errorf("Bundles count: got %d, want 1", got)
	}
}

// TestDecode_UnknownField verifies at any level invalidates the lockfile. The
// decoder must reject silently added fields rather than ignoring them, since
// attackers could otherwise hide data inside a lockfile that older parsers
// would not flag.
func TestDecode_UnknownField(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{
			name: "unknown top-level field",
			in:   `{"version":1,"policy":"block","bundles":{},"surprise":true}`,
		},
		{
			name: "unknown bundle field",
			in: `{"version":1,"policy":"block","bundles":{"./a":{` +
				`"contentHash":"sha256:0000000000000000000000000000000000000000000000000000000000000000",` +
				`"files":{"x":"sha256:0000000000000000000000000000000000000000000000000000000000000000"},` +
				`"extra":"nope"}}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode([]byte(c.in))
			if err == nil {
				t.Fatal("expected unknown-field error, got nil")
			}
			// Match on the substring stdlib's DisallowUnknownFields uses so a future
			// change to the error wording surfaces in CI.
			if !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("error %q does not mention unknown field", err)
			}
		})
	}
}

// TestDecode_NotJSON verifies the decoder rejects malformed JSON cleanly
// rather than panicking. A panic at the lockfile boundary would crash the
// CLI before the user-facing 'invalid lockfile' diagnostic could appear.
//
// Note: "null" is intentionally NOT in this list — it is valid JSON and
// stdlib decodes it into a zero-value Lockfile. The semantic validator
// (validate.go) is responsible for rejecting that zero value because
// version=0 is not supported and policy="" is not "block"/"warn". This keeps
// Decode a pure shape-check and Validate the semantic gate.
func TestDecode_NotJSON(t *testing.T) {
	for _, in := range []string{"", "not json", "{", "[", "{\"version\":"} {
		t.Run(in, func(t *testing.T) {
			_, err := Decode([]byte(in))
			if err == nil {
				t.Fatalf("expected error for input %q, got nil", in)
			}
		})
	}
}

// TestDecode_TrailingGarbage verifies the decoder rejects content after the
// top-level JSON value. A lockfile with trailing data is suspicious (likely
// concatenation of two lockfiles, or smuggled bytes) and should fail closed
// rather than silently accept the first object.
func TestDecode_TrailingGarbage(t *testing.T) {
	in := `{"version":1,"policy":"block","bundles":{}} extra`
	_, err := Decode([]byte(in))
	if err == nil {
		t.Fatal("expected trailing-data error, got nil")
	}
}
