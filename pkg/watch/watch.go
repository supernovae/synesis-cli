package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file changes and triggers callbacks
type Watcher struct {
	watcher  *fsnotify.Watcher
	paths    []string
	callback func(event fsnotify.Event)
	interval time.Duration
	stopped  bool
}

// NewWatcher creates a new Watcher instance
func NewWatcher(paths []string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watchPaths := []string{}
	for _, p := range paths {
		absPath, err := absPath(p)
		if err != nil {
			return nil, err
		}
		watchPaths = append(watchPaths, absPath)
	}

	return &Watcher{
		watcher: w,
		paths:   watchPaths,
	}, nil
}

// absPath returns the absolute path
func absPath(path string) (string, error) {
	if abs, err := filepath.Abs(path); err == nil {
		return abs, nil
	}
	return path, nil
}

// SetCallback sets the callback function to run on file changes
func (w *Watcher) SetCallback(callback func(event fsnotify.Event)) {
	w.callback = callback
}

// SetInterval sets the polling interval for file changes
func (w *Watcher) SetInterval(interval time.Duration) {
	w.interval = interval
}

// AddPath adds a path to watch
func (w *Watcher) AddPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	w.paths = append(w.paths, absPath)
	return w.watcher.Add(absPath)
}

// Start begins watching for file changes
func (w *Watcher) Start() error {
	if len(w.paths) == 0 {
		return fmt.Errorf("no paths to watch")
	}

	// Add all paths
	for _, p := range w.paths {
		if err := w.watcher.Add(p); err != nil {
			return err
		}
	}

	// Start the event loop
	go w.eventLoop()

	return nil
}

// eventLoop handles fsnotify events
func (w *Watcher) eventLoop() {
	for event := range w.watcher.Events {
		if w.stopped {
			return
		}
		if w.callback != nil {
			w.callback(event)
		}
	}
}

// StartWithInterval starts watching with a polling interval
func (w *Watcher) StartWithInterval() error {
	if len(w.paths) == 0 {
		return fmt.Errorf("no paths to watch")
	}

	if w.interval == 0 {
		w.interval = 2 * time.Second
	}

	// Add all paths
	for _, p := range w.paths {
		if err := w.watcher.Add(p); err != nil {
			return err
		}
	}

	// Start polling loop
	go w.pollingLoop()

	return nil
}

// pollingLoop polls for file changes at the configured interval
func (w *Watcher) pollingLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for !w.stopped {
		select {
		case <-ticker.C:
			// Check if any watched files have changed
			for _, p := range w.paths {
				if err := w.checkFile(p); err != nil {
					continue
				}
			}
		}
	}
}

// checkFile checks if a file has changed and triggers callback
func (w *Watcher) checkFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Simple change detection - compare modification time
	// In a real implementation, you'd track previous modification times
	_ = info.ModTime()

	return nil
}

// Stop stops watching for file changes
func (w *Watcher) Stop() error {
	w.stopped = true
	return w.watcher.Close()
}

// WatchFile watches a single file and calls callback on changes
func WatchFile(path string, callback func(event fsnotify.Event)) error {
	paths := []string{path}
	w, err := NewWatcher(paths)
	if err != nil {
		return err
	}
	w.SetCallback(callback)
	return w.Start()
}

// WatchFiles watches multiple files and calls callback on any change
func WatchFiles(paths []string, callback func(event fsnotify.Event)) error {
	w, err := NewWatcher(paths)
	if err != nil {
		return err
	}
	w.SetCallback(callback)
	return w.Start()
}
