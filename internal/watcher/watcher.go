// Package watcher observes one Markdown file through its parent directory.
package watcher

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/lz-wang/m2h/internal/files"
)

// DefaultDebounce merges the event bursts commonly emitted by editors.
const DefaultDebounce = 150 * time.Millisecond

// Watch blocks until the context is canceled or the underlying watcher fails.
// It watches the parent directory so temp-file plus rename saves remain visible.
func Watch(ctx context.Context, source string, debounce time.Duration, onChange func(), logWriter io.Writer) error {
	if debounce <= 0 {
		return fmt.Errorf("watch %q: debounce must be greater than zero", source)
	}
	if onChange == nil {
		return fmt.Errorf("watch %q: change callback is required", source)
	}
	if err := ctx.Err(); err != nil {
		return nil
	}

	input, err := files.Resolve(source)
	if err != nil {
		return fmt.Errorf("watch source: %w", err)
	}
	if input.Kind != files.KindFile {
		return fmt.Errorf("watch %q: expected a regular file", input.Path)
	}

	native, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher for %q: %w", input.Path, err)
	}
	defer native.Close()

	parent := filepath.Dir(input.Path)
	if err := native.Add(parent); err != nil {
		return fmt.Errorf("watch parent directory %q: %w", parent, err)
	}
	return run(ctx, native.Events, native.Errors, input.Path, debounce, onChange, logWriter)
}

func run(
	ctx context.Context,
	events <-chan fsnotify.Event,
	errors <-chan error,
	target string,
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

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return fmt.Errorf("watch %q: event stream closed", target)
			}
			if filepath.Clean(event.Name) != target || event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
				continue
			}
			_, _ = fmt.Fprintf(logWriter, "m2h: detected change to %s\n", filepath.Base(target))
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
		case <-timerEvents:
			timerEvents = nil
			onChange()
		case err, ok := <-errors:
			if !ok {
				return fmt.Errorf("watch %q: error stream closed", target)
			}
			return fmt.Errorf("watch %q: %w", target, err)
		}
	}
}
