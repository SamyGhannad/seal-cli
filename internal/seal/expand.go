package seal

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// ExpandPatterns returns a sorted, deduplicated list of bundle keys
// ("./<rel>") for sealed roots that actually exist under projectRoot. Used
// by bulk pin and by verify when the lockfile has discovery patterns.
//
// Each match must be a non-empty directory: a leaf that resolves to a file
// is silently dropped, and a directory containing only excluded entries
// (.git etc.) is dropped.
//
// projectRoot is opaque — joined onto, never resolved — so callers may pass
// absolute or cwd-relative paths.
func ExpandPatterns(projectRoot string, patterns []string) ([]string, error) {
	// Set dedups when two patterns hit the same directory.
	set := make(map[string]struct{})

	for _, p := range patterns {
		// Validate at the boundary so a malformed pattern fails with a clear
		// error rather than producing subtly-wrong matches.
		if err := ValidatePattern(p); err != nil {
			return nil, fmt.Errorf("expand %q: %w", p, err)
		}
		matches, err := expandOne(projectRoot, p)
		if err != nil {
			return nil, fmt.Errorf("expand %q: %w", p, err)
		}
		for _, m := range matches {
			set[m] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// expandOne resolves a single pattern by walking the filesystem segment by
// segment. We do this rather than calling filepath.Glob so we can apply NFC
// normalisation and enforce the directory-only leaf rule.
func expandOne(projectRoot, pattern string) ([]string, error) {
	// strings.Split on "/" — pattern grammar uses "/" regardless of host OS.
	segs := strings.Split(pattern, "/")
	return walkPattern(projectRoot, "", segs)
}

// walkPattern recursively descends through pattern segments. Returns
// "./<rel>" bundle keys at leaves so callers don't post-process.
func walkPattern(projectRoot, relPath string, segs []string) ([]string, error) {
	if len(segs) == 0 {
		return nil, nil
	}
	head, tail := segs[0], segs[1:]

	// Missing parent is "no match", not an error — discovery patterns are
	// advisory and may legitimately point at not-yet-created directories.
	dir := filepath.Join(projectRoot, filepath.FromSlash(relPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	// Sort entries by NFC form so traversal order is independent of the host
	// filesystem's reporting order (matters on case-folded/NFD volumes).
	sort.Slice(entries, func(i, j int) bool {
		return norm.NFC.String(entries[i].Name()) < norm.NFC.String(entries[j].Name())
	})

	// path.Match uses POSIX glob; we escape ?/[/] so Seal's stricter grammar
	// treats them as literals.
	escaped := escapeSpecLiterals(head)

	var out []string
	for _, e := range entries {
		// NFC entry names so matches are stable across APFS (NFD) and others.
		name := norm.NFC.String(e.Name())
		ok, err := path.Match(escaped, name)
		if err != nil {
			// ValidatePattern + escapeSpecLiterals should preclude this; surface
			// rather than silently skip.
			return nil, err
		}
		if !ok {
			continue
		}

		childRel := name
		if relPath != "" {
			childRel = relPath + "/" + name
		}

		if len(tail) == 0 {
			// Leaf: must be a non-empty directory.
			if !e.IsDir() {
				continue
			}
			full := filepath.Join(projectRoot, filepath.FromSlash(childRel))
			empty, err := isEmptyOrAllExcluded(full)
			if err != nil {
				return nil, err
			}
			if empty {
				continue
			}
			out = append(out, "./"+childRel)
			continue
		}

		// Intermediate segment: only descend into directories.
		if !e.IsDir() {
			continue
		}
		more, err := walkPattern(projectRoot, childRel, tail)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
	}
	return out, nil
}

// errStopWalk is the sentinel that terminates isEmptyOrAllExcluded's
// short-circuit walk. Package-level so identity stays stable across calls.
var errStopWalk = errors.New("stop walk")

// isEmptyOrAllExcluded reports whether dir contains zero tracked files.
// Excluded entries are `.git` (any depth, directory or file) and seal.json
// at the sealed root.
//
// Short-circuits on the first tracked file found via errStopWalk; we don't
// depend on traversal order, only on existence of any match.
func isEmptyOrAllExcluded(dir string) (bool, error) {
	found := false
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Prune the entire .git tree.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		// Bare ".git" file (git uses these as submodule pointers).
		if d.Name() == ".git" {
			return nil
		}
		found = true
		return errStopWalk
	})
	if err != nil && !errors.Is(err, errStopWalk) {
		return false, err
	}
	return !found, nil
}

// escapeSpecLiterals escapes ?, [, ] so path.Match treats them as literals.
// Seal's discovery grammar uses only `*` as a wildcard; the other metachars
// path.Match supports have no place here, and escaping at the boundary lets
// us keep using path.Match instead of writing a parallel matcher.
func escapeSpecLiterals(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		// A backslash + next-char tells path.Match "this char, literal".
		switch r {
		case '?', '[', ']':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}