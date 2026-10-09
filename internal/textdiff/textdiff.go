// Package textdiff compares two texts line by line, as a unified diff does, for the
// page that has no script to do it: the editor's own diff is Monaco's. It is Myers'
// algorithm, and it gives up on texts so unlike each other that comparing them would
// cost more than showing them is worth.
package textdiff

import "strings"

// Kind is what a line did.
type Kind uint8

// The kinds of line in a diff.
const (
	Equal Kind = iota
	Delete
	Insert
)

// Line is one line of a diff. Old and New are its numbers in the two texts, counting
// from 1, and 0 on the side where it is not.
type Line struct {
	Kind     Kind
	Old, New int
	Text     string
}

// Hunk is a stretch of changes with the lines around them.
type Hunk struct {
	Lines []Line
}

// limits: past either, a comparison is refused.
const (
	maxLines = 50000
	maxEdits = 1000
)

// Unified returns the hunks that turn a into b, each with context lines on both sides of
// its changes, and how many lines were added and removed. ok is false when the texts
// are too large or too different to compare here; no changes is no hunks and ok.
func Unified(a, b string, context int) (hunks []Hunk, added, removed int, ok bool) {
	x, y := lines(a), lines(b)
	if len(x) > maxLines || len(y) > maxLines {
		return nil, 0, 0, false
	}
	script, ok := diff(x, y)
	if !ok {
		return nil, 0, 0, false
	}
	for _, l := range script {
		switch l.Kind {
		case Insert:
			added++
		case Delete:
			removed++
		case Equal:
		}
	}
	return group(script, context), added, removed, true
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// diff is the shortest edit script from x to y, in order, with the line numbers set.
func diff(x, y []string) ([]Line, bool) {
	n, m := len(x), len(y)
	total := n + m
	if total == 0 {
		return nil, true
	}
	limit := min(total, maxEdits)
	// trace[d] is V after d edits, to walk back through.
	var trace [][]int
	v := make([]int, 2*limit+2)
	offset := limit
	found := -1
search:
	for d := 0; d <= limit; d++ {
		snapshot := make([]int, len(v))
		copy(snapshot, v)
		trace = append(trace, snapshot)
		for k := -d; k <= d; k += 2 {
			var px int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				px = v[offset+k+1] // down: an insertion
			} else {
				px = v[offset+k-1] + 1 // right: a deletion
			}
			py := px - k
			for px < n && py < m && x[px] == y[py] {
				px++
				py++
			}
			v[offset+k] = px
			if px >= n && py >= m {
				found = d
				break search
			}
		}
	}
	if found < 0 {
		return nil, false
	}

	// Walk back from the end to the start, collecting the script in reverse.
	var rev []Line
	px, py := n, m
	for d := found; d > 0; d-- {
		vd := trace[d]
		k := px - py
		var prevK int
		if k == -d || (k != d && vd[offset+k-1] < vd[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := vd[offset+prevK]
		prevY := prevX - prevK
		for px > prevX && py > prevY && px > 0 && py > 0 && x[px-1] == y[py-1] && (px-1 != prevX || py != prevY+1 || prevK != k+1) && (px != prevX+1 || py-1 != prevY || prevK != k-1) {
			rev = append(rev, Line{Kind: Equal, Old: px, New: py, Text: x[px-1]})
			px--
			py--
		}
		if prevK == k+1 { // an insertion of y[prevY]
			rev = append(rev, Line{Kind: Insert, New: prevY + 1, Text: y[prevY]})
		} else { // a deletion of x[prevX]
			rev = append(rev, Line{Kind: Delete, Old: prevX + 1, Text: x[prevX]})
		}
		px, py = prevX, prevY
	}
	for px > 0 && py > 0 {
		rev = append(rev, Line{Kind: Equal, Old: px, New: py, Text: x[px-1]})
		px--
		py--
	}
	out := make([]Line, len(rev))
	for i, l := range rev {
		out[len(rev)-1-i] = l
	}
	return out, true
}

// group cuts a script into hunks: each change with context lines either side, and
// changes closer than twice the context together.
func group(script []Line, context int) []Hunk {
	var hunks []Hunk
	var current []Line
	quiet := 0 // equal lines since the last change
	for i, l := range script {
		if l.Kind != Equal {
			if current == nil {
				// The context before the first change of a hunk.
				from := max(0, i-context)
				current = append(current, script[from:i]...)
				if current == nil {
					current = []Line{}
				}
			}
			current = append(current, l)
			quiet = 0
			continue
		}
		if current == nil {
			continue
		}
		quiet++
		if quiet <= context {
			current = append(current, l)
			continue
		}
		if quiet == 2*context+1 || i == len(script)-1 {
			// Far enough from the last change to end the hunk, trimmed to the context.
			hunks = append(hunks, Hunk{Lines: current})
			current, quiet = nil, 0
			continue
		}
		current = append(current, l)
	}
	if current != nil {
		hunks = append(hunks, Hunk{Lines: trim(current, context)})
	}
	for i := range hunks {
		hunks[i].Lines = trim(hunks[i].Lines, context)
	}
	return hunks
}

// trim drops equal lines beyond context from the end of a hunk.
func trim(lines []Line, context int) []Line {
	end := len(lines)
	trailing := 0
	for end > 0 && lines[end-1].Kind == Equal {
		end--
		trailing++
	}
	if trailing > context {
		return lines[:end+context]
	}
	return lines
}
