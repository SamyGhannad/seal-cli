package seal

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Decode parses lockfile JSON into a Lockfile, rejecting unknown fields and
// trailing bytes. The inbound boundary for every read/validation path —
// stdlib's plain json.Unmarshal silently ignores unknowns, which would let a
// malicious lockfile smuggle data past our checks.
//
// Only enforces JSON-shape constraints (well-formedness, no unknown fields,
// no trailing data). Semantic validation (key shape, hash format,
// contentHash recomputation) lives in validate.go.
func Decode(b []byte) (*Lockfile, error) {
	// json.Decoder is the only stdlib path that exposes DisallowUnknownFields.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()

	var lf Lockfile
	if err := dec.Decode(&lf); err != nil {
		return nil, fmt.Errorf("decode lockfile: %w", err)
	}

	// Trailing data after the top-level value is suspicious (likely a smuggled
	// concatenation); fail closed.
	if dec.More() {
		return nil, fmt.Errorf("decode lockfile: unexpected trailing data")
	}

	return &lf, nil
}
