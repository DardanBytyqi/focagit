package graph

import (
	"fmt"
	"testing"
)

type commit struct {
	hash    string
	parents []string
}

// build feeds commits to a Builder in order, the way a history walk does.
func build(commits []commit) ([]Row, int) {
	b := New()
	rows := make([]Row, len(commits))
	for i, c := range commits {
		rows[i] = b.Add(c.hash, c.parents)
	}
	return rows, b.Width()
}

func lanes(links []Link) []int {
	out := make([]int, len(links))
	for i, l := range links {
		out[i] = l.Lane
	}
	return out
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// check compares one row against the lanes it is expected to produce.
func check(t *testing.T, i int, got Row, lane int, in, out, pass []int) {
	t.Helper()
	if got.Lane != lane {
		t.Errorf("row %d: Lane = %d, want %d", i, got.Lane, lane)
	}
	if !equal(lanes(got.In), in) {
		t.Errorf("row %d: In = %v, want %v", i, lanes(got.In), in)
	}
	if !equal(lanes(got.Out), out) {
		t.Errorf("row %d: Out = %v, want %v", i, lanes(got.Out), out)
	}
	if !equal(lanes(got.Pass), pass) {
		t.Errorf("row %d: Pass = %v, want %v", i, lanes(got.Pass), pass)
	}
}

func TestLinearHistory(t *testing.T) {
	// C → B → A, a single line of development.
	rows, width := build([]commit{
		{"C", []string{"B"}},
		{"B", []string{"A"}},
		{"A", nil},
	})

	if width != 1 {
		t.Errorf("width = %d, want 1", width)
	}
	check(t, 0, rows[0], 0, nil, []int{0}, nil) // tip: nothing enters
	check(t, 1, rows[1], 0, []int{0}, []int{0}, nil)
	check(t, 2, rows[2], 0, []int{0}, nil, nil) // root: nothing leaves

	for i, r := range rows {
		if r.Merge {
			t.Errorf("row %d marked as a merge", i)
		}
	}
}

func TestBranchAndMerge(t *testing.T) {
	// M merge of C and F |\ C | | F |/ B A
	rows, width := build([]commit{
		{"M", []string{"C", "F"}},
		{"C", []string{"B"}},
		{"F", []string{"B"}},
		{"B", []string{"A"}},
		{"A", nil},
	})

	if width != 2 {
		t.Errorf("width = %d, want 2", width)
	}

	// The merge opens a second lane for its second parent.
	check(t, 0, rows[0], 0, nil, []int{0, 1}, nil)
	if !rows[0].Merge {
		t.Error("row 0: Merge = false, want true")
	}

	// C stays in lane 0; F's lane crosses this row untouched.
	check(t, 1, rows[1], 0, []int{0}, []int{0}, []int{1})

	// F sits in lane 1 and its parent B is already expected in lane 0.
	check(t, 2, rows[2], 1, []int{1}, []int{0}, []int{0})

	check(t, 3, rows[3], 0, []int{0}, []int{0}, nil)
	check(t, 4, rows[4], 0, []int{0}, nil, nil)
}

func TestFirstParentKeepsItsLane(t *testing.T) {
	// The rule that matters visually.
	rows, _ := build([]commit{
		{"M", []string{"main", "side"}},
		{"main", nil},
	})

	if rows[0].Out[0].Lane != rows[0].Lane {
		t.Errorf("first parent went to lane %d, want the commit's own lane %d",
			rows[0].Out[0].Lane, rows[0].Lane)
	}
	if rows[0].Out[1].Lane == rows[0].Lane {
		t.Error("second parent reused the commit's lane instead of opening one")
	}
}

func TestOctopusMerge(t *testing.T) {
	// Three parents: rare, but git allows it and the layout must not break.
	rows, width := build([]commit{
		{"M", []string{"A", "B", "C"}},
	})

	if width != 3 {
		t.Errorf("width = %d, want 3", width)
	}
	check(t, 0, rows[0], 0, nil, []int{0, 1, 2}, nil)
	if !rows[0].Merge {
		t.Error("octopus merge not marked as a merge")
	}
}

func TestMultipleRoots(t *testing.T) {
	// Two unrelated histories in one repository.
	rows, width := build([]commit{
		{"X", nil},
		{"Y", nil},
	})

	if width != 1 {
		t.Errorf("width = %d, want 1", width)
	}
	// The first root frees its lane immediately.
	check(t, 0, rows[0], 0, nil, nil, nil)
	check(t, 1, rows[1], 0, nil, nil, nil)
}

func TestTwoTipsSharingAParent(t *testing.T) {
	// A and B are independent tips that both descend from C.
	rows, width := build([]commit{
		{"A", []string{"C"}},
		{"B", []string{"C"}},
		{"C", nil},
	})

	if width != 2 {
		t.Errorf("width = %d, want 2", width)
	}
	check(t, 0, rows[0], 0, nil, []int{0}, nil)
	// B is a tip, so nothing enters it; its parent is already expected in lane 0.
	check(t, 1, rows[1], 1, nil, []int{0}, []int{0})
	// Only lane 0 was waiting for C, so a single line enters it.
	check(t, 2, rows[2], 0, []int{0}, nil, nil)
}

func TestLaneIsReusedAfterABranchEnds(t *testing.T) {
	// A finished line of development must free its lane, or the graph drifts rightwards.
	rows, width := build([]commit{
		{"A", []string{"B"}},
		{"B", nil}, // this line ends here
		{"D", []string{"E"}},
		{"E", nil},
	})

	if width != 1 {
		t.Errorf("width = %d, want 1 (the lane should be reused)", width)
	}
	if rows[2].Lane != 0 {
		t.Errorf("row 2 took lane %d, want the freed lane 0", rows[2].Lane)
	}
}

func TestConvergingLanes(t *testing.T) {
	// Two lanes waiting for the same commit converge on it: both appear in In.
	rows, _ := build([]commit{
		{"A", []string{"Z"}},
		{"B", []string{"Z"}},
		{"Z", nil},
	})

	// B's parent Z is already expected in lane 0.
	if got := len(rows[2].In); got != 1 {
		t.Errorf("Z has %d incoming lines, want 1: %v", got, lanes(rows[2].In))
	}
	if rows[2].Lane != 0 {
		t.Errorf("Z sits in lane %d, want 0", rows[2].Lane)
	}
}

func TestColorsFollowLanes(t *testing.T) {
	rows, _ := build([]commit{
		{"M", []string{"A", "B", "C"}},
	})
	for i, l := range rows[0].Out {
		if want := l.Lane % Palette; l.Color != want {
			t.Errorf("Out[%d]: Color = %d, want %d", i, l.Color, want)
		}
	}
	if rows[0].Color != rows[0].Lane%Palette {
		t.Errorf("Color = %d, want %d", rows[0].Color, rows[0].Lane%Palette)
	}
}

func TestManyTipsOnOneParentStayNarrow(t *testing.T) {
	// Ten tips all pointing at the same root.
	var commits []commit
	for i := range 10 {
		commits = append(commits, commit{fmt.Sprintf("tip%d", i), []string{"root"}})
	}
	commits = append(commits, commit{"root", nil})

	rows, width := build(commits)

	if width != 2 {
		t.Errorf("width = %d, want 2", width)
	}
	// The first tip hands lane 0 to the shared parent and keeps it.
	if rows[0].Lane != 0 {
		t.Errorf("tip0 sits in lane %d, want 0", rows[0].Lane)
	}
	for i := 1; i < 10; i++ {
		if rows[i].Lane != 1 {
			t.Errorf("tip%d sits in lane %d, want the borrowed lane 1", i, rows[i].Lane)
		}
		// Its line leaves leftwards into the trunk...
		if !equal(lanes(rows[i].Out), []int{0}) {
			t.Errorf("tip%d: Out = %v, want [0]", i, lanes(rows[i].Out))
		}
		// ...while the trunk itself keeps running down the same row.
		if !equal(lanes(rows[i].Pass), []int{0}) {
			t.Errorf("tip%d: Pass = %v, want [0]", i, lanes(rows[i].Pass))
		}
	}
	// Every tip's line ends at the root, but only lane 0 was ever waiting.
	if got := len(rows[10].In); got != 1 {
		t.Errorf("root has %d incoming lines, want 1", got)
	}
}

func TestEmptyHistory(t *testing.T) {
	rows, width := build(nil)
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0", len(rows))
	}
	if width != 0 {
		t.Errorf("width = %d, want 0", width)
	}
}
