package watcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestRunFiltersAndDebouncesEvents(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "guide.md")
	events := make(chan fsnotify.Event, 8)
	errorsChannel := make(chan error)
	changed := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, events, errorsChannel, target, 20*time.Millisecond, func() {
			changed <- struct{}{}
		}, nil)
	}()

	events <- fsnotify.Event{Name: filepath.Join(filepath.Dir(target), "other.md"), Op: fsnotify.Write}
	events <- fsnotify.Event{Name: target, Op: fsnotify.Chmod}
	events <- fsnotify.Event{Name: target, Op: fsnotify.Write}
	events <- fsnotify.Event{Name: target, Op: fsnotify.Create}
	events <- fsnotify.Event{Name: target, Op: fsnotify.Rename}

	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("debounced change was not delivered")
	}
	select {
	case <-changed:
		t.Fatal("event burst delivered more than one change")
	case <-time.After(60 * time.Millisecond):
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run() returned error after cancellation: %v", err)
	}
}

func TestRunReportsClosedStreamsAndWatcherErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		events func() <-chan fsnotify.Event
		errors func() <-chan error
		want   string
	}{
		{
			name: "events closed",
			events: func() <-chan fsnotify.Event {
				channel := make(chan fsnotify.Event)
				close(channel)
				return channel
			},
			errors: func() <-chan error { return make(chan error) },
			want:   "event stream closed",
		},
		{
			name:   "errors closed",
			events: func() <-chan fsnotify.Event { return make(chan fsnotify.Event) },
			errors: func() <-chan error {
				channel := make(chan error)
				close(channel)
				return channel
			},
			want: "error stream closed",
		},
		{
			name:   "watcher error",
			events: func() <-chan fsnotify.Event { return make(chan fsnotify.Event) },
			errors: func() <-chan error {
				channel := make(chan error, 1)
				channel <- errors.New("overflow")
				return channel
			},
			want: "overflow",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := run(context.Background(), test.events(), test.errors(), "guide.md", time.Millisecond, func() {}, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestWatchDetectsWriteAndAtomicRename(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan struct{}, 4)
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, source, 30*time.Millisecond, func() {
			changed <- struct{}{}
		}, nil)
	}()
	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(source, []byte("written"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, changed, "ordinary write")

	temporary := filepath.Join(root, ".guide.md.tmp")
	if err := os.WriteFile(temporary, []byte("renamed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, source); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, changed, "atomic rename")

	if err := os.WriteFile(filepath.Join(root, "unrelated.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
		t.Fatal("unrelated file triggered a change")
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Watch() returned error: %v", err)
	}
}

func TestWatchValidatesArguments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		source   string
		debounce time.Duration
		callback func()
	}{
		{name: "invalid debounce", source: source, callback: func() {}},
		{name: "missing callback", source: source, debounce: time.Millisecond},
		{name: "directory", source: root, debounce: time.Millisecond, callback: func() {}},
		{name: "missing source", source: filepath.Join(root, "missing.md"), debounce: time.Millisecond, callback: func() {}},
	} {
		if err := Watch(context.Background(), test.source, test.debounce, test.callback, nil); err == nil {
			t.Errorf("Watch() accepted %s", test.name)
		}
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Watch(canceled, filepath.Join(root, "missing.md"), time.Millisecond, func() {}, nil); err != nil {
		t.Fatalf("Watch() canceled error = %v", err)
	}
}

func waitForChange(t *testing.T, changed <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-changed:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not trigger a change", operation)
	}
}
