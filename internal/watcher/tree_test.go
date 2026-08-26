package watcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// waitForTreeChange polls until the observed callback count reaches want or
// the deadline passes; tree events depend on editor-unpredictable OS timing.
func waitForTreeChange(t *testing.T, changes chan int, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		select {
		case count := <-changes:
			if count >= want {
				return
			}
		case <-time.After(time.Until(deadline)):
			t.Fatalf("timed out waiting for %d debounced tree change(s)", want)
		}
	}
}

func TestWatchTreeReportsWritesCreatesAndDeletes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file-event timing varies on Windows CI")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes := make(chan int, 8)
	count := 0
	onChange := func() {
		count++
		select {
		case changes <- count:
		default:
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- WatchTree(ctx, root, 30*time.Millisecond, onChange, nil)
	}()
	// Give the recursive registration a moment before writing.
	time.Sleep(100 * time.Millisecond)

	// Existing nested file changes are visible.
	if err := os.WriteFile(filepath.Join(root, "sub", "guide.md"), []byte("# v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForTreeChange(t, changes, 1)

	// Files created after startup are visible, including inside a directory
	// that itself was created after startup.
	if err := os.MkdirAll(filepath.Join(root, "fresh", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fresh", "deeper", "new.md"), []byte("# new"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForTreeChange(t, changes, 2)

	// Deletions are visible too — the WebUI falls back to the default
	// document when the open one disappears.
	if err := os.Remove(filepath.Join(root, "guide.md")); err != nil {
		t.Fatal(err)
	}
	waitForTreeChange(t, changes, 3)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("WatchTree returned error: %v", err)
	}
}

func TestWatchTreeRejectsInvalidOptions(t *testing.T) {
	root := t.TempDir()
	if err := WatchTree(context.Background(), root, 0, func() {}, nil); err == nil {
		t.Fatal("WatchTree accepted a zero debounce")
	}
	if err := WatchTree(context.Background(), root, time.Millisecond, nil, nil); err == nil {
		t.Fatal("WatchTree accepted a missing callback")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WatchTree(ctx, root, time.Millisecond, func() {}, nil); err != nil {
		t.Fatalf("WatchTree with canceled context returned error: %v", err)
	}
	if err := WatchTree(context.Background(), filepath.Join(root, "missing"), time.Millisecond, func() {}, nil); err == nil {
		t.Fatal("WatchTree accepted a missing root")
	}
}

func TestWatchTreeDoesNotFollowDirectorySymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	outside := t.TempDir()
	outsideSub := filepath.Join(outside, "sub")
	if err := os.Mkdir(outsideSub, 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(outsideSub, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// addTree must register root and skip the linked directory; the error
	// path proves nothing by itself, so assert through the registered set by
	// watching: writes under the symlinked directory must not fire changes.
	changes := make(chan int, 4)
	count := 0
	onChange := func() {
		count++
		select {
		case changes <- count:
		default:
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = WatchTree(ctx, root, 30*time.Millisecond, onChange, nil)
	}()
	time.Sleep(100 * time.Millisecond)

	// A write inside the symlinked (outside) directory must stay invisible.
	if err := os.WriteFile(filepath.Join(outsideSub, "secret.md"), []byte("# secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
		t.Fatal("write under a symlinked directory was observed")
	case <-time.After(300 * time.Millisecond):
	}
}
