package cli

import (
	"path/filepath"
	"strings"
)

// bundleKeyToPath converts a bundle key into a filesystem path rooted at cwd.
//
// Bundle keys are forward-slash, "./"-anchored strings like "./skills/foo".
// The filesystem layer wants native separators and an absolute path; this
// helper is the single point of conversion so verify/init/pin don't each
// reinvent the trim-prefix/Join idiom.
//
// Accepts keys with OR without the "./" prefix. Validate enforces the prefix
// on lockfile-sourced keys, but internal callers (e.g. ExpandPatterns output)
// may pass either form. The bare-dot key "." means cwd itself and round-trips
// correctly through TrimPrefix + Join.
//
// filepath.Join handles slash-to-platform-separator translation on Windows.
func bundleKeyToPath(cwd, key string) string {
	// TrimPrefix (not slicing) so a key that is just "." doesn't lose a char.
	rel := strings.TrimPrefix(key, "./")
	return filepath.Join(cwd, rel)
}