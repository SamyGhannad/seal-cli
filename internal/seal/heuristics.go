package seal

import (
	"os"
	"path/filepath"
	"strings"
)

// Entry describes one well-known agent layout. Adding {Name, Pattern} is
// the only change required to support a new layout at `seal init` time.
type Entry struct {
	// Name is human-readable, shown in init to explain why a pattern was
	// proposed (e.g. "Claude Code skills").
	Name string

	// Pattern is a discovery glob; MUST pass ValidatePattern.
	// TestRegistry_AllPatternsValid pins this for every shipped entry.
	Pattern string
}

// Registry is the set of known layouts. Consulted ONLY by `seal init` —
// verification is driven solely by the lockfile's discovery array so that
// CI stays deterministic across implementations. Do NOT import from
// RunVerify or RunPin.
var Registry = []Entry{
	{Name: "Claude Code plugins", Pattern: ".claude/plugins/*"},
	{Name: "Claude Code skills", Pattern: ".claude/skills/*"},
	{Name: "Codex skills", Pattern: ".codex/skills/*"},
	{Name: "Agent Skills", Pattern: ".agents/skills/*"},
}

// ProposeFor returns the subset of Registry patterns whose parent directory
// actually exists under projectRoot. Keeps init noise-free: a fresh Go
// project doesn't get proposed Python-tool layouts.
//
// Output order is Registry declaration order, not lexicographic, so init's
// UI is consistent across invocations.
func ProposeFor(projectRoot string) []string {
	var out []string
	for _, e := range Registry {
		// Parent of "a/b/*" is "a/b".
		parent := patternParent(e.Pattern)
		if parent == "" {
			// A pattern with no "/" has no parent to gate on. Current registry
			// has none such; skip conservatively rather than auto-propose.
			continue
		}
		full := filepath.Join(projectRoot, parent)
		info, err := os.Stat(full)
		// Require directory: a regular file at the parent path isn't a layout
		// we should match.
		if err == nil && info.IsDir() {
			out = append(out, e.Pattern)
		}
	}
	return out
}

// patternParent strips the trailing "/<segment>" from a pattern.
//
//	".claude/skills/*" → ".claude/skills"
//	"a/b/c"            → "a/b"
//	"top-level"        → ""
//
// Only used at init time; we deliberately don't handle `*` in non-leaf
// segments because no current Registry entry needs it.
func patternParent(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	return p[:idx]
}
