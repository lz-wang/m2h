package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/store"
)

func testUIFilesystem() fstest.MapFS {
	return fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="root"></div>`)}}
}

func symlink(t *testing.T, target, link string) error {
	t.Helper()
	return os.Symlink(target, link)
}

func localAuthConfig() *auth.Config {
	return &auth.Config{
		Mode: auth.ModeLocal,
		LocalUsers: []auth.User{
			{Username: "alice", Password: "password-1"},
			{Username: "bob", Password: "password-2"},
		},
	}
}

// identityOf builds a workspace from raw input paths and assigns persistent
// identities through a real store rooted at dataDir.
func identityOf(t *testing.T, dataDir string, inputs ...string) workspace {
	t.Helper()
	resolved := make([]files.Input, 0, len(inputs))
	for _, raw := range inputs {
		input, err := files.Resolve(raw)
		if err != nil {
			t.Fatalf("resolve %q: %v", raw, err)
		}
		resolved = append(resolved, input)
	}
	created, err := newWorkspace(resolved, files.DiscoverOptions{}, false)
	if err != nil {
		t.Fatalf("newWorkspace(%v) failed: %v", inputs, err)
	}
	dataStore, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := created.assignIdentities(context.Background(), dataStore.EnsureDocumentRoot); err != nil {
		t.Fatalf("assignIdentities failed: %v", err)
	}
	return created
}

func TestWorkspaceIdentitiesStableAcrossInputReordering(t *testing.T) {
	base := t.TempDir()
	docsA := filepath.Join(base, "docs-a")
	docsB := filepath.Join(base, "docs-b")
	for _, dir := range []string{docsA, docsB} {
		writeTestFile(t, filepath.Join(dir, "a.md"), "# A")
	}

	forward := identityOf(t, filepath.Join(base, "data1"), docsA, docsB)
	reversed := identityOf(t, filepath.Join(base, "data1"), docsB, docsA)

	if forward.roots[0].identity == "" || forward.roots[1].identity == "" {
		t.Fatalf("identities missing: %+v", forward.roots)
	}
	if forward.roots[0].identity != reversed.roots[1].identity ||
		forward.roots[1].identity != reversed.roots[0].identity {
		t.Fatalf("input reordering changed identities: %v vs %v",
			[]string{forward.roots[0].identity, forward.roots[1].identity},
			[]string{reversed.roots[0].identity, reversed.roots[1].identity})
	}
	if _, found := reversed.rootByIdentity(forward.roots[0].identity); !found {
		t.Fatalf("rootByIdentity could not find persisted identity")
	}
}

func TestWorkspaceIdentitySurvivesRestartAndFileRecreation(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	single := filepath.Join(base, "docs", "note.md")
	writeTestFile(t, single, "# Note")

	first := identityOf(t, dataDir, single)
	// The file is deleted and recreated: same canonical path, same identity.
	writeTestFile(t, single, "# Rewritten")
	second := identityOf(t, dataDir, single)
	if first.primary().identity != second.primary().identity {
		t.Fatalf("file recreation changed identity: %q vs %q",
			first.primary().identity, second.primary().identity)
	}

	// Serving the same tree after moving the data directory elsewhere
	// (another machine) finds no identity and starts a fresh one — no
	// content-based guessing.
	third := identityOf(t, filepath.Join(base, "other-data"), single)
	if third.primary().identity == first.primary().identity {
		t.Fatalf("moved data directory kept identities")
	}
}

func TestWorkspaceIdentitiesDistinguishSameNamedFiles(t *testing.T) {
	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	writeTestFile(t, filepath.Join(left, "design.md"), "# Left")
	writeTestFile(t, filepath.Join(right, "design.md"), "# Right")

	created := identityOf(t, filepath.Join(base, "data"), left, right)
	if created.roots[0].identity == created.roots[1].identity {
		t.Fatalf("same-named documents in different roots share identity %q",
			created.roots[0].identity)
	}
	if created.roots[0].id == created.roots[1].id {
		t.Fatalf("positional ids collide")
	}
}

func TestWorkspaceIdentityThroughSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	writeTestFile(t, filepath.Join(real, "a.md"), "# A")
	alias := filepath.Join(base, "alias")
	if err := symlink(t, real, alias); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	direct := identityOf(t, filepath.Join(base, "data1"), real)
	viaSymlink := identityOf(t, filepath.Join(base, "data1"), alias)
	if direct.primary().identity != viaSymlink.primary().identity {
		t.Fatalf("symlinked root got a different identity: %q vs %q",
			direct.primary().identity, viaSymlink.primary().identity)
	}
}

func TestWorkspaceIdentityDiffersBetweenFileAndDirectoryRoot(t *testing.T) {
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "a.md"), "# A")

	fileRoot := identityOf(t, filepath.Join(base, "data"), filepath.Join(docs, "a.md"))
	dirRoot := identityOf(t, filepath.Join(base, "data"), docs)
	if fileRoot.primary().identity == dirRoot.primary().identity {
		t.Fatalf("file root and directory root share identity")
	}
}

func TestCheckDataDirBoundary(t *testing.T) {
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "a.md"), "# A")
	resolved, err := files.Resolve(docs)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	created, err := newWorkspace([]files.Input{resolved}, files.DiscoverOptions{}, false)
	if err != nil {
		t.Fatalf("newWorkspace failed: %v", err)
	}

	tests := []struct {
		name    string
		dataDir string
		wantErr string
	}{
		{name: "inside root", dataDir: filepath.Join(docs, "data"), wantErr: "must not live inside served root"},
		{name: "equal to root", dataDir: docs, wantErr: "must not live inside served root"},
		{name: "sibling of root", dataDir: filepath.Join(base, "data")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkDataDirBoundary(test.dataDir, created)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDataDirBoundary(%q) failed: %v", test.dataDir, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("checkDataDirBoundary(%q) = %v, want %q", test.dataDir, err, test.wantErr)
			}
		})
	}

	rootInsideData := filepath.Join(base, "data", "docs")
	writeTestFile(t, filepath.Join(rootInsideData, "a.md"), "# A")
	if err := checkDataDirBoundary(filepath.Join(base, "data"), func() workspace {
		inner, innerErr := files.Resolve(rootInsideData)
		if innerErr != nil {
			t.Fatalf("resolve inner failed: %v", innerErr)
		}
		built, buildErr := newWorkspace([]files.Input{inner}, files.DiscoverOptions{}, false)
		if buildErr != nil {
			t.Fatalf("newWorkspace failed: %v", buildErr)
		}
		return built
	}()); err == nil || !strings.Contains(err.Error(), "must not live inside data directory") {
		t.Fatalf("root inside data directory accepted: %v", err)
	}
}

// startAuthenticatedServer boots one authenticated run on an ephemeral
// port and hands back its base URL. It backs the full-stack auth tests.
func startAuthenticatedServer(t *testing.T, options Options) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	listening := make(chan string, 1)
	deps := testDependencies()
	deps.openBrowser = func(string) error { return nil }
	deps.listen = func(network, address string) (net.Listener, error) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			select {
			case listening <- "http://" + listener.Addr().String() + "/":
			default:
			}
		}
		return listener, err
	}
	if options.OnListening != nil {
		panic("startAuthenticatedServer owns OnListening")
	}
	options.OnListening = func(address string) {
		select {
		case listening <- address:
		default:
		}
	}
	done := make(chan error, 1)
	go func() { done <- run(ctx, options, deps) }()
	// Cancel first, then drain the run — one cleanup so ordering cannot
	// invert.
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("authenticated run returned %v", err)
		}
	})
	select {
	case address := <-listening:
		return address
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("authenticated server never listened")
		return ""
	}
}

func authenticatedOptions(t *testing.T, config *auth.Config) Options {
	t.Helper()
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "guide.md"), "# Guide")
	writeTestFile(t, filepath.Join(docs, "secret.md"), "# Secret")
	return Options{
		Inputs:     []string{docs},
		Mode:       markdown.ModeAuto,
		AuthMode:   config.Mode,
		AuthConfig: config,
		DataDir:    filepath.Join(base, "m2h-data"),
		Log:        io.Discard,
		UI:         testUIFilesystem(),
	}
}

// TestRunAuthModeInitializesStore verifies an authenticated run boots with
// the SQLite foundation: database created, migrated, root identity
// registered — checked from a second process-like reopen.
func TestRunAuthModeInitializesStore(t *testing.T) {
	options := authenticatedOptions(t, localAuthConfig())
	address := startAuthenticatedServer(t, options)

	if response, err := http.Get(address + "api/config"); err != nil {
		t.Fatalf("api/config failed: %v", err)
	} else {
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("api/config = %d", response.StatusCode)
		}
	}

	database := filepath.Join(options.DataDir, "m2h.db")
	if _, statErr := os.Stat(database); statErr != nil {
		t.Fatalf("database not initialized: %v", statErr)
	}
	dataStore, openErr := store.Open(options.DataDir)
	if openErr != nil {
		t.Fatalf("reopen store failed: %v", openErr)
	}
	defer dataStore.Close()
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate after run failed: %v", err)
	}
	var roots int
	if err := dataStore.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM document_roots`).Scan(&roots); err != nil {
		t.Fatalf("count document roots: %v", err)
	}
	if roots != 1 {
		t.Fatalf("document_roots rows = %d, want 1", roots)
	}
}

// TestRunNoneModeStaysDatabaseFree is the statelessness contract: the
// default mode never initializes SQLite, whatever the data directory says.
func TestRunNoneModeStaysDatabaseFree(t *testing.T) {
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "guide.md"), "# Guide")
	dataDir := filepath.Join(base, "m2h-data")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listening := make(chan string, 1)
	deps := testDependencies()
	deps.openBrowser = func(string) error { return nil }
	deps.listen = func(network, address string) (net.Listener, error) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			listening <- "http://" + listener.Addr().String() + "/"
		}
		return listener, err
	}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{
			Inputs:  []string{docs},
			Mode:    markdown.ModeAuto,
			DataDir: dataDir,
			Log:     io.Discard,
			UI:      testUIFilesystem(),
			OnListening: func(address string) {
				select {
				case listening <- address:
				default:
				}
			},
		}, deps)
	}()

	<-listening
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run() returned %v", err)
	}
	if _, err := os.Stat(dataDir); err == nil {
		t.Fatalf("default mode created the data directory %q", dataDir)
	}
}

func TestNormalizeOptionsAuthValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(options *Options)
		wantErr string
	}{
		{
			name:    "invalid mode",
			mutate:  func(options *Options) { options.AuthMode = auth.Mode("oauth") },
			wantErr: "invalid auth mode",
		},
		{
			name: "local without config",
			mutate: func(options *Options) {
				options.AuthMode = auth.ModeLocal
			},
			wantErr: "requires an auth configuration",
		},
		{
			name: "mode mismatch",
			mutate: func(options *Options) {
				options.AuthMode = auth.ModeLDAP
				options.AuthConfig = localAuthConfig()
			},
			wantErr: "does not match",
		},
		{
			name: "missing data dir",
			mutate: func(options *Options) {
				options.AuthMode = auth.ModeLocal
				options.AuthConfig = localAuthConfig()
			},
			wantErr: "requires a data directory",
		},
		{
			name: "invalid public URL",
			mutate: func(options *Options) {
				options.PublicURL = "docs.example.com"
			},
			wantErr: "invalid public URL",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := Options{Inputs: []string{"docs"}, Mode: markdown.ModeAuto}
			test.mutate(&options)
			_, err := normalizeOptions(options)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("normalizeOptions() = %v, want %q", err, test.wantErr)
			}
		})
	}
}
