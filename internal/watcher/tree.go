package watcher

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchTree blocks until the context is canceled or the underlying watcher
// fails. It recursively observes one directory tree — every existing
// subdirectory plus each directory created later — and reports debounced
// changes. Directory symlinks inside the tree are never followed, matching the
// server's traversal boundary.
func WatchTree(ctx context.Context, root string, debounce time.Duration, onChange func(), logWriter io.Writer) error {
	if debounce <= 0 {
		return fmt.Errorf("watch tree %q: debounce must be greater than zero", root)
	}
	if onChange == nil {
		return fmt.Errorf("watch tree %q: change callback is required", root)
	}
	if err := ctx.Err(); err != nil {
		return nil
	}

	native, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher for %q: %w", root, err)
	}
	defer native.Close()
	if err := addTree(native, root); err != nil {
		return fmt.Errorf("watch tree %q: %w", root, err)
	}
	return runTree(ctx, native, root, debounce, onChange, logWriter)
}

// addTree registers root and every subdirectory below it, skipping symlinked
// directories so an in-tree link can never widen the watch beyond the root.
func addTree(native *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if current != root {
			info, err := os.Lstat(current)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fs.SkipDir
			}
		}
		return native.Add(current)
	})
}

func runTree(
	ctx context.Context,
	native *fsnotify.Watcher,
	root string,
	debounce time.Duration,
	onChange func(),
	logWriter io.Writer,
) error {
	if logWriter == nil {
		logWriter = io.Discard
	}

	var timer *time.Timer
	var timerEvents <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	reset := func() {
		if timer == nil {
			timer = time.NewTimer(debounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
		}
		timerEvents = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-native.Events:
			if !ok {
				return fmt.Errorf("watch tree %q: event stream closed", root)
			}
			// A symlink inside the tree is not part of the tree: activity in
			// its target is reported against the link's own path on some
			// platforms (macOS kqueue does), and following it would widen the
			// watch past the root.
			if info, err := os.Lstat(event.Name); err == nil && info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			// A new directory extends the tree; register it (and anything
			// already inside it) before the change is reported, so a file
			// written straight into a fresh subdirectory is not missed.
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Lstat(event.Name); err == nil && info.IsDir() {
					if err := addTree(native, event.Name); err != nil {
						_, _ = fmt.Fprintf(logWriter, "m2h: watch new directory %s: %v\n", event.Name, err)
					}
				}
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}
			_, _ = fmt.Fprintf(logWriter, "m2h: detected change under %s\n", filepath.Base(root))
			reset()
		case <-timerEvents:
			timerEvents = nil
			onChange()
		case err, ok := <-native.Errors:
			if !ok {
				return fmt.Errorf("watch tree %q: error stream closed", root)
			}
			return fmt.Errorf("watch tree %q: %w", root, err)
		}
	}
}
