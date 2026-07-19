package push

import (
	"log"
	"path/filepath"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/fsnotify/fsnotify"
)

// buildFileIndex groups push-enabled type: file scrapers by their cleaned
// absolute path and parent directory, so multiple scrapers sharing a
// directory (e.g. two log files both in /var/log/nginx/) only need one
// fsnotify watch on that directory.
func buildFileIndex(sender *Sender, pushCfg config.PushConfig, scrapers []config.ScraperConfig) (
	byPath map[string]*fileScraper, byDir map[string][]*fileScraper, dirs map[string]struct{},
) {
	byPath = make(map[string]*fileScraper)
	byDir = make(map[string][]*fileScraper)
	dirs = make(map[string]struct{})

	for _, sc := range scrapers {
		abs, err := filepath.Abs(sc.Source.Path)
		if err != nil {
			log.Printf("probestyx push: skipping %q, cannot resolve path %q: %v", sc.Name, sc.Source.Path, err)
			continue
		}
		abs = filepath.Clean(abs)
		dir := filepath.Dir(abs)

		fs := &fileScraper{cfg: sc, path: abs, dir: dir, sender: sender, pushCfg: pushCfg}
		byPath[abs] = fs
		byDir[dir] = append(byDir[dir], fs)
		dirs[dir] = struct{}{}
	}
	return byPath, byDir, dirs
}

// runFileWatch watches the directories containing push-enabled type: file
// scrapers and debounces a collect+push whenever a watched file changes.
// Directories are watched rather than the files themselves: a file-level
// watch follows an inode, and a rename-based log rotation (the original file
// renamed to e.g. error.log.1, a new empty file created at the original path)
// detaches the watch from the path. A directory watch is stable across that -
// it fires Create when the new file appears at the same path - and because
// metrics.CollectLog/tailFile always os.Open(path) fresh rather than holding a
// handle, re-triggering a collect after that Create is all that's needed to
// pick up the new file. This blocks until the watcher dies or the process
// exits; any failure degrades to a logged warning rather than taking down
// pull mode (/metrics is unaffected either way).
func runFileWatch(sender *Sender, pushCfg config.PushConfig, scrapers []config.ScraperConfig) {
	if len(scrapers) == 0 {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("probestyx push: cannot create fsnotify watcher, file push disabled: %v", err)
		return
	}
	defer watcher.Close()

	byPath, byDir, dirs := buildFileIndex(sender, pushCfg, scrapers)

	// Prime so tailFile seeds offsets at startup EOF and the first real event
	// doesn't dump history.
	for _, fs := range byPath {
		collectAndPush(fs.sender, fs.pushCfg, fs.cfg)
	}

	pending := make(map[string]struct{})
	for dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			log.Printf("probestyx push: watch %q failed, will retry: %v", dir, err)
			pending[dir] = struct{}{}
		}
	}

	retry := time.NewTicker(30 * time.Second)
	defer retry.Stop()

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				log.Printf("probestyx push: fsnotify events channel closed, file watch stopped")
				return
			}
			// Only Write/Create carry new data to tail. Rename/Remove are the
			// old file going away (nothing to read); the recreation that
			// follows fires Create, which is what we act on. Chmod (e.g.
			// logrotate fixing permissions on the new file) carries no data.
			if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if fs, ok := byPath[filepath.Clean(ev.Name)]; ok {
				fs.schedule(debounceDelay)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				log.Printf("probestyx push: fsnotify errors channel closed, file watch stopped")
				return
			}
			log.Printf("probestyx push: fsnotify error (continuing): %v", err)

		case <-retry.C:
			for dir := range pending {
				if err := watcher.Add(dir); err == nil {
					delete(pending, dir)
					log.Printf("probestyx push: now watching %q", dir)
					for _, fs := range byDir[dir] {
						collectAndPush(fs.sender, fs.pushCfg, fs.cfg) // catch data written before the watch attached
					}
				}
			}
		}
	}
}
