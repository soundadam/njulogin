package credential

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) (FileStore, string) {
	t.Helper()
	dir := t.TempDir()
	path, err := DefaultPath(dir)
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	return FileStore{Path: path}, dir
}

func requireMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat %s: %v", path, err)
	}
	if mode := info.Mode().Perm(); mode != want {
		t.Fatalf("%s mode = %o, want %o", path, mode, want)
	}
}

func TestDefaultPathUsesOverrideDirectory(t *testing.T) {
	dir := t.TempDir()
	path, err := DefaultPath(dir)
	if err != nil || path != filepath.Join(dir, "config.json") {
		t.Fatalf("path = %q, err = %v", path, err)
	}
}

func TestSaveAndLoadRoundTripWithStrictModes(t *testing.T) {
	store, dir := newStore(t)
	if loaded, err := store.Load(); err != nil || loaded != nil {
		t.Fatalf("missing file: %#v, %v", loaded, err)
	}
	const password = "  synthetic secret  "
	if err := store.Save(Credential{Username: " synthetic-user ", Password: password}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Username != "synthetic-user" || loaded.Password != password {
		t.Fatalf("round trip changed the credential: username=%q", loaded.Username)
	}
	requireMode(t, store.Path, 0o600)
	requireMode(t, dir, 0o700)
}

func TestSaveRepairsExistingPermissions(t *testing.T) {
	store, dir := newStore(t)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := os.WriteFile(store.Path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := store.Save(Credential{Username: "synthetic-user", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	requireMode(t, store.Path, 0o600)
	requireMode(t, dir, 0o700)
}

func TestLoadRejectsUnknownAndTrailingFields(t *testing.T) {
	store, _ := newStore(t)
	for _, data := range []string{
		`{"username":"synthetic-user","password":"synthetic-secret","keyring_service":"legacy"}`,
		`{"username":"synthetic-user","password":"synthetic-secret"} {}`,
		`{"username":"synthetic-user","password":" "}`,
		`{"username":" ","password":"synthetic-secret"}`,
	} {
		if err := os.WriteFile(store.Path, []byte(data), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := store.Load(); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
}

func TestPermissiveFileAndSymlinksAreRejected(t *testing.T) {
	store, dir := newStore(t)
	if err := store.Save(Credential{Username: "synthetic-user", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(store.Path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("accepted a 0644 credential file")
	}
	if err := os.Remove(store.Path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"username":"synthetic-user"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(target, store.Path); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("loaded through a file symlink")
	}
	if err := store.Save(Credential{Username: "synthetic-user", Password: "synthetic-secret"}); err == nil {
		t.Fatal("saved through a file symlink")
	}
}

func TestDirectorySymlinkIsRejected(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "target")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	link := filepath.Join(base, "credential-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	store := FileStore{Path: filepath.Join(link, "config.json")}
	if err := store.Save(Credential{Username: "synthetic-user", Password: "synthetic-secret"}); err == nil {
		t.Fatal("saved into a symlinked directory")
	}
	if err := os.WriteFile(filepath.Join(real, "config.json"), []byte(`{"username":"synthetic-user","password":"synthetic-secret"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("loaded from a symlinked directory")
	}
}

func TestCommandSourceIsBounded(t *testing.T) {
	secret, err := Command{Script: "printf 'synthetic-secret\\n'", Timeout: 5 * time.Second}.Secret(context.Background())
	if err != nil || secret != "synthetic-secret" {
		t.Fatalf("secret=%q err=%v", secret, err)
	}
	started := time.Now()
	_, err = Command{Script: "sleep 1; printf synthetic-secret", Timeout: 20 * time.Millisecond}.Secret(context.Background())
	if err == nil || err.Error() != "password command timed out" {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("timeout took %s", elapsed)
	}
	if _, err := (Command{Script: "exit 3", Timeout: 5 * time.Second}).Secret(context.Background()); err == nil {
		t.Fatal("accepted a failing password command")
	}
}

func TestLineSourceReadsOneLine(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("first\r\nsecond"))
	for _, want := range []string{"first", "second", ""} {
		got, err := Line{Reader: reader}.Secret(context.Background())
		if err != nil || got != want {
			t.Fatalf("got %q err=%v, want %q", got, err, want)
		}
	}
}

func TestTerminalSourceRefusesNonTerminal(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()
	var prompt strings.Builder
	source := Terminal{In: reader, Prompt: &prompt, Label: "synthetic: ", RequireTerminal: true}
	if _, err := source.Secret(context.Background()); err == nil || prompt.Len() != 0 {
		t.Fatalf("err=%v prompt=%q", err, prompt.String())
	}
}
