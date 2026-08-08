// Package graph turns commits into the coloured lane layout drawn beside the history.
package graph

// Palette is the number of distinct lane colours.
const Palette = 8

// Link is one end of a line entering or leaving a row.
type Link struct {
	Lane  int `json:"lane"`
	Color int `json:"color"`
}

// Row is everything needed to draw one line of the graph.
type Row struct {
	Lane  int    `json:"lane"`  // where the dot goes
	Color int    `json:"color"` // palette index for the dot
	Merge bool   `json:"merge"` // more than one parent
	In    []Link `json:"in"`
	Out   []Link `json:"out"`
	Pass  []Link `json:"pass"`
}

// Builder assigns lanes as commits arrive.
type Builder struct {
	// lanes[i] holds the hash lane i is waiting to reach, or "" when free.
	lanes []string
	width int
}

func New() *Builder { return &Builder{} }

// Width is the highest number of lanes seen so far.
func (b *Builder) Width() int { return b.width }

// Add places one commit and returns its row.
func (b *Builder) Add(hash string, parents []string) Row {
	var in []Link
	myLane := -1

	// Every lane waiting for this commit converges on it.
	for i, want := range b.lanes {
		if want != hash {
			continue
		}
		if myLane < 0 {
			myLane = i // the leftmost waiting lane owns the dot
		}
		in = append(in, Link{Lane: i, Color: color(i)})
		b.lanes[i] = ""
	}

	// Nobody was waiting: this commit is a branch tip, so it starts a lane.
	if myLane < 0 {
		myLane = b.free()
	}

	// Whatever is still occupied runs straight through this row untouched.
	var pass []Link
	for i, want := range b.lanes {
		if want != "" && i != myLane {
			pass = append(pass, Link{Lane: i, Color: color(i)})
		}
	}

	// Parents claim the lanes leaving the bottom edge.
	var out []Link
	for k, p := range parents {
		// A parent already expected elsewhere means the two lines converge further down.
		if j := b.indexOf(p); j >= 0 {
			out = append(out, Link{Lane: j, Color: color(j)})
			continue
		}
		// The first parent inherits this commit's own lane.
		lane := myLane
		if k > 0 {
			lane = b.free()
		}
		b.lanes[lane] = p
		out = append(out, Link{Lane: lane, Color: color(lane)})
	}

	b.trim()
	b.grow(myLane + 1)

	// Empty, not nil: these cross the bridge and JSON null is not an array.
	if in == nil {
		in = []Link{}
	}
	if out == nil {
		out = []Link{}
	}
	if pass == nil {
		pass = []Link{}
	}

	return Row{
		Lane:  myLane,
		Color: color(myLane),
		Merge: len(parents) > 1,
		In:    in,
		Out:   out,
		Pass:  pass,
	}
}

// free returns the leftmost unused lane, appending one if all are taken.
func (b *Builder) free() int {
	for i, want := range b.lanes {
		if want == "" {
			return i
		}
	}
	b.lanes = append(b.lanes, "")
	b.grow(len(b.lanes))
	return len(b.lanes) - 1
}

func (b *Builder) indexOf(hash string) int {
	for i, want := range b.lanes {
		if want == hash {
			return i
		}
	}
	return -1
}

// trim drops trailing free lanes so the layout narrows again once a burst of parallel.
func (b *Builder) trim() {
	n := len(b.lanes)
	for n > 0 && b.lanes[n-1] == "" {
		n--
	}
	b.lanes = b.lanes[:n]
}

func (b *Builder) grow(n int) {
	if n > b.width {
		b.width = n
	}
}

func color(lane int) int { return lane % Palette }
