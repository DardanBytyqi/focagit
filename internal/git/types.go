package git

import "time"

// Person is one side of a commit's authorship.
type Person struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	When  time.Time `json:"when"`
}

// Commit is a single revision, as read from git log.
type Commit struct {
	Hash      string   `json:"hash"`
	Parents   []string `json:"parents"`
	Author    Person   `json:"author"`
	Committer Person   `json:"committer"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
}

// Short returns the abbreviated hash used throughout the UI.
func (c Commit) Short() string {
	if len(c.Hash) < 7 {
		return c.Hash
	}
	return c.Hash[:7]
}

// IsMerge reports whether the commit has more than one parent.
func (c Commit) IsMerge() bool { return len(c.Parents) > 1 }

// IsRoot reports whether the commit has no parents.
func (c Commit) IsRoot() bool { return len(c.Parents) == 0 }

// RefKind classifies a reference by its refs/ namespace.
type RefKind string

const (
	RefBranch RefKind = "branch"
	RefRemote RefKind = "remote"
	RefTag    RefKind = "tag"
	RefOther  RefKind = "other"
)

// Ref is a named pointer into the object graph.
type Ref struct {
	Name string  `json:"name"` // full name, e.g. refs/heads/main
	Kind RefKind `json:"kind"` // branch, remote, tag, other
	Hash string  `json:"hash"` // commit the ref resolves to, tags already peeled
	Head bool    `json:"head"` // true for the ref HEAD currently points at
	// Short is the display name: "main", "origin/main", "v1.0".
	Short string `json:"short"`
	// Upstream is the tracking branch of a local branch, empty otherwise.
	Upstream string `json:"upstream"`
}

// Change is the per-file state git reports in one half of the XY status code.
type Change string

const (
	ChangeNone     Change = ""
	ChangeAdded    Change = "A"
	ChangeModified Change = "M"
	ChangeDeleted  Change = "D"
	ChangeRenamed  Change = "R"
	ChangeCopied   Change = "C"
	ChangeTypeMod  Change = "T" // regular file <-> symlink <-> submodule
	ChangeUnmerged Change = "U"
)

// FileStatus is one entry of the working tree status.
type FileStatus struct {
	Path      string `json:"path"`
	OrigPath  string `json:"origPath,omitempty"` // set for renames and copies
	Staged    Change `json:"staged"`
	Unstaged  Change `json:"unstaged"`
	Untracked bool   `json:"untracked"`
	Unmerged  bool   `json:"unmerged"`
	Score     int    `json:"score,omitempty"` // rename/copy similarity, 0-100
}

// Status is the full working tree state plus the branch header.
type Status struct {
	Head     string `json:"head"`     // branch name, empty when detached
	Oid      string `json:"oid"`      // commit HEAD points at
	Upstream string `json:"upstream"` // tracking branch, if any
	Ahead    int    `json:"ahead"`
	// Unpushed counts commits on no remote, for a branch with no upstream.
	Unpushed int  `json:"unpushed"`
	Behind   int  `json:"behind"`
	Detached bool `json:"detached"`
	// Operation is the multi-step operation left open, empty when there is none.
	Operation Operation `json:"operation"`
	// OperationFrom names what that operation is applying.
	OperationFrom string `json:"operationFrom"`
	// Rebase says how far an open rebase has got; nil for every other operation.
	Rebase *RebaseState `json:"rebase,omitempty"`
	Files  []FileStatus `json:"files"`
}

// IsClean reports whether there is nothing to commit and nothing untracked.
func (s Status) IsClean() bool { return len(s.Files) == 0 }
