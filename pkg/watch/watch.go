package watch

import (
	"fmt"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file changes and triggers callbacks
type Watcher struct {
	watcher  *fsnotify.Watcher
	paths    []string
	callback func(event fsnotify.Event)
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
