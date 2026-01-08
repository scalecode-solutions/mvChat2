package irido

import (
	"github.com/scalecode-solutions/runeseg"
)

// Graphemes handles Unicode grapheme cluster operations.
// This is important for correctly handling emoji and other multi-byte characters
// when calculating offsets for mentions and truncating text.
type Graphemes struct {
	original string
	// Byte offsets for each grapheme cluster
	offsets []int
}

// NewGraphemes creates a new Graphemes from a string.
func NewGraphemes(str string) *Graphemes {
	if str == "" {
		return &Graphemes{original: str, offsets: nil}
	}

	// Calculate byte offsets for each grapheme cluster
	offsets := make([]int, 0, len(str)/2) // Estimate
	offset := 0
	for state, remaining := -1, str; len(remaining) > 0; {
		var cluster string
		cluster, remaining, _, state = runeseg.StepString(remaining, state)
		offsets = append(offsets, offset)
		offset += len(cluster)
	}
	// Add final offset (end of string)
	offsets = append(offsets, offset)

	return &Graphemes{
		original: str,
		offsets:  offsets,
	}
}

// Length returns the number of grapheme clusters.
func (g *Graphemes) Length() int {
	if g == nil || len(g.offsets) == 0 {
		return 0
	}
	return len(g.offsets) - 1
}

// String returns the original string.
func (g *Graphemes) String() string {
	if g == nil {
		return ""
	}
	return g.original
}

// Slice returns a substring from grapheme index start to end (exclusive).
func (g *Graphemes) Slice(start, end int) string {
	if g == nil || len(g.offsets) == 0 {
		return ""
	}

	length := g.Length()
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start >= end {
		return ""
	}

	byteStart := g.offsets[start]
	byteEnd := g.offsets[end]
	return g.original[byteStart:byteEnd]
}

// At returns the grapheme cluster at the given index.
func (g *Graphemes) At(index int) string {
	if g == nil || index < 0 || index >= g.Length() {
		return ""
	}
	return g.original[g.offsets[index]:g.offsets[index+1]]
}

// ByteOffset returns the byte offset for a grapheme index.
func (g *Graphemes) ByteOffset(graphemeIndex int) int {
	if g == nil || graphemeIndex < 0 || graphemeIndex >= len(g.offsets) {
		return 0
	}
	return g.offsets[graphemeIndex]
}

// GraphemeIndex returns the grapheme index for a byte offset.
func (g *Graphemes) GraphemeIndex(byteOffset int) int {
	if g == nil || len(g.offsets) == 0 {
		return 0
	}

	for i, off := range g.offsets {
		if off > byteOffset {
			if i > 0 {
				return i - 1
			}
			return 0
		}
	}
	return g.Length()
}

// Truncate returns the string truncated to maxGraphemes with optional suffix.
func Truncate(str string, maxGraphemes int, suffix string) string {
	g := NewGraphemes(str)
	if g.Length() <= maxGraphemes {
		return str
	}
	return g.Slice(0, maxGraphemes) + suffix
}

// GraphemeLength returns the number of grapheme clusters in a string.
func GraphemeLength(str string) int {
	return NewGraphemes(str).Length()
}
