// Package watch reports that a folder tree changed, without saying what changed.
package watch

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is how long the tree must be quiet before a change is reported.
const DefaultDebounce = 400 * time.Millisecond

// DefaultLimit caps the watched folders, since each one costs an inotify handle.
const DefaultLimit = 8000

// Config describes one tree to watch.
type Config struct {
	// Root is the working tree; GitDir is watched separately because it can live elsewhere.
	Root   string
	GitDir string
	// Skip are folders to leave alone, with everything under them.
	Skip []string
	// Ignored is asked only about folders born after the start, like a fresh node_modules.
	Ignored func(dir string) bool
	// OnChange runs once per quiet period; refs is true when the git dir moved too.
	OnChange func(refs bool)
	Debounce time.Duration
	Limit    int
	Log      *slog.Logger
}

// Watcher turns filesystem noise into one call per quiet period.
type Watcher struct {
	cfg  Config
	fsw  *fsnotify.Watcher
	done chan struct{}
	once sync.Once

	// count is only touched by the goroutine that adds folders.
	count int
}

// Start watches the tree and calls cfg.OnChange until Close.
func Start(cfg Config) (*Watcher, error) {
	if cfg.Debounce <= 0 {
		cfg.Debounce = DefaultDebounce
	}
	if cfg.Limit <= 0 {
		cfg.Limit = DefaultLimit
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{cfg: cfg, fsw: fsw, done: make(chan struct{})}
	w.addTree(cfg.Root)
	// The refs live here, and so does HEAD: this is how a commit made elsewhere shows up.
	w.add(cfg.GitDir)
	w.addTree(filepath.Join(cfg.GitDir, "refs"))

	cfg.Log.Info("watching", slog.String("root", cfg.Root), slog.Int("dirs", w.count))
	go w.run()
	return w, nil
}

// Close stops watching; calling it twice is safe.
func (w *Watcher) Close() error {
	w.once.Do(func() { close(w.done) })
	return w.fsw.Close()
}

// run is the only goroutine reading events, so the debounce needs no lock.
func (w *Watcher) run() {
	// A nil channel blocks forever, which is what "no change pending" means here.
	var quiet <-chan time.Time
	// refs remembers whether this burst touched a branch, and not only the files.
	refs := false

	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if !w.interesting(ev) {
				continue
			}
			if under(ev.Name, w.cfg.GitDir) {
				refs = true
			}
			// A new folder is not watched by itself; its own files would go unseen.
			if ev.Has(fsnotify.Create) {
				if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					if w.cfg.Ignored != nil && w.cfg.Ignored(ev.Name) {
						// Remembered, so its contents are dropped without asking again.
						w.cfg.Skip = append(w.cfg.Skip, ev.Name)
						continue
					}
					w.addTree(ev.Name)
				}
			}
			// Every event pushes the deadline back, so a burst reports once.
			quiet = time.After(w.cfg.Debounce)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.cfg.Log.Warn("watcher", slog.String("err", err.Error()))
			// Events were lost, and what they said is unknown: reread everything.
			refs = true
			quiet = time.After(w.cfg.Debounce)

		case <-quiet:
			quiet = nil
			if w.cfg.OnChange != nil {
				w.cfg.OnChange(refs)
			}
			refs = false

		case <-w.done:
			return
		}
	}
}

// interesting drops the events git makes constantly and nobody can act on.
func (w *Watcher) interesting(ev fsnotify.Event) bool {
	// Chmod fires on every read of some filesystems; it never changes what we show.
	if ev.Op == fsnotify.Chmod {
		return false
	}
	base := filepath.Base(ev.Name)
	// Locks appear and vanish around every git write, including our own.
	if strings.HasSuffix(base, ".lock") {
		return false
	}
	return !w.skip(ev.Name)
}

// skip refuses the folders that change constantly or hold nothing worth showing.
func (w *Watcher) skip(path string) bool {
	// Objects and logs are written by every fetch; the refs alone say what moved.
	for _, noisy := range []string{"objects", "logs", "modules"} {
		if under(path, filepath.Join(w.cfg.GitDir, noisy)) {
			return true
		}
	}
	for _, dir := range w.cfg.Skip {
		if under(path, dir) {
			return true
		}
	}
	return false
}

// under reports whether path is dir or sits inside it.
func under(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+string(os.PathSeparator))
}

// add watches one folder, counting it against the limit.
func (w *Watcher) add(dir string) bool {
	if w.count >= w.cfg.Limit {
		return false
	}
	if err := w.fsw.Add(dir); err != nil {
		return false
	}
	w.count++
	return true
}

// addTree watches dir and everything under it that is worth watching.
func (w *Watcher) addTree(root string) {
	stopped := false
	// The walk ignores errors: a folder deleted mid-walk is not our problem.
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		// .git is reached through GitDir, which is watched selectively.
		if path != root && path == w.cfg.GitDir {
			return filepath.SkipDir
		}
		if w.skip(path) {
			return filepath.SkipDir
		}
		if !w.add(path) {
			if !stopped {
				stopped = true
				w.cfg.Log.Warn("watch limit reached, some folders are not watched",
					slog.Int("limit", w.cfg.Limit), slog.String("at", path))
			}
			return filepath.SkipDir
		}
		return nil
	})
}
