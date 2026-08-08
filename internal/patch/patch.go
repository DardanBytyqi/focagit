// Package patch builds the partial patches that make line-level staging work.
package patch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dardanbytyqi/focagit/internal/git"
)

// ErrNothingSelected means the selection did not name a single changed line.
var ErrNothingSelected = errors.New("no changed lines selected")

// LineRef identifies one line of a FileDiff by its position.
type LineRef struct {
	Hunk int `json:"hunk"`
	Line int `json:"line"`
}

// Build renders a patch of the selected lines; set reverse for `git apply --reverse`.
func Build(d git.FileDiff, selected []LineRef, reverse bool) (string, error) {
	if d.Binary {
		return "", errors.New("cannot stage part of a binary file")
	}

	sel := make(map[LineRef]bool, len(selected))
	for _, r := range selected {
		sel[r] = true
	}

	var hunks []string
	offset := 0 // accumulated drift of the synthesised side

	for hi, h := range d.Hunks {
		body, adds, dels, ok := buildHunkBody(h, hi, sel, reverse)
		if !ok {
			continue // nothing selected in this hunk
		}

		ctx := countContext(body)
		oldLines := ctx + dels
		newLines := ctx + adds

		// One side keeps the real line numbers, the other is the fiction we are constructing.
		var oldStart, newStart int
		if reverse {
			newStart = h.NewStart
			oldStart = h.NewStart + offset
			offset += dels - adds
		} else {
			oldStart = h.OldStart
			newStart = h.OldStart + offset
			offset += adds - dels
		}

		header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldLines, newStart, newLines)
		if h.Header != "" {
			header += " " + h.Header
		}
		hunks = append(hunks, header+"\n"+strings.Join(body, "\n")+"\n")
	}

	if len(hunks) == 0 {
		return "", ErrNothingSelected
	}

	var b strings.Builder
	for _, line := range fileHeader(d) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, h := range hunks {
		b.WriteString(h)
	}
	return b.String(), nil
}

// buildHunkBody renders one hunk, reporting whether anything in it was selected.
func buildHunkBody(h git.Hunk, hi int, sel map[LineRef]bool, reverse bool) (body []string, adds, dels int, any bool) {
	for li, l := range h.Lines {
		selected := sel[LineRef{Hunk: hi, Line: li}]

		var sign string
		switch l.Kind {
		case git.LineContext:
			sign = " "

		case git.LineAdd:
			switch {
			case selected:
				sign, adds, any = "+", adds+1, true
			case reverse:
				// Already in the index and staying there.
				sign = " "
			default:
				// Not in the index yet; must not appear on the old side.
				continue
			}

		case git.LineDelete:
			switch {
			case selected:
				sign, dels, any = "-", dels+1, true
			case reverse:
				// Not in the index; nothing to describe.
				continue
			default:
				// Still in the index and staying there.
				sign = " "
			}

		default:
			continue
		}

		body = append(body, sign+l.Text)
		if l.NoNewline {
			body = append(body, `\ No newline at end of file`)
		}
	}
	return body, adds, dels, any
}

func countContext(body []string) int {
	n := 0
	for _, l := range body {
		if strings.HasPrefix(l, " ") {
			n++
		}
	}
	return n
}

// fileHeader replays git's own header and appends the --- / +++ pair. /dev/null on one.
func fileHeader(d git.FileDiff) []string {
	out := make([]string, 0, len(d.Header)+2)
	if len(d.Header) > 0 {
		out = append(out, d.Header...)
	} else {
		// Only reachable when a caller hands us a hand-built FileDiff.
		out = append(out, fmt.Sprintf("diff --git a/%s b/%s", pathOr(d.OldPath, d.Path), pathOr(d.Path, d.OldPath)))
	}

	if d.OldPath == "" {
		out = append(out, "--- /dev/null")
	} else {
		out = append(out, "--- a/"+d.OldPath)
	}
	if d.Path == "" {
		out = append(out, "+++ /dev/null")
	} else {
		out = append(out, "+++ b/"+d.Path)
	}
	return out
}

func pathOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
