package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdate_SuccessPath(t *testing.T) {
	binaryContent := []byte("fake binary v2")
	sum := sha256.Sum256(binaryContent)
	expectedSHA := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binaryContent)
	}))
	defer srv.Close()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "vh-agent")
	if err := os.WriteFile(binaryPath, []byte("old binary v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := &Updater{
		BinaryPath: binaryPath,
		HTTPClient: srv.Client(),
	}

	err := u.Update(srv.URL+"/bin", expectedSHA)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := os.ReadFile(binaryPath)
	if string(got) != string(binaryContent) {
		t.Errorf("binary content = %q, want %q", got, binaryContent)
	}

	info, _ := os.Stat(binaryPath)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
}

func TestUpdate_SHA256Mismatch_DoesNotReplace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tampered binary"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "vh-agent")
	if err := os.WriteFile(binaryPath, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := &Updater{
		BinaryPath: binaryPath,
		HTTPClient: srv.Client(),
	}

	err := u.Update(srv.URL+"/bin", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for SHA256 mismatch")
	}

	got, _ := os.ReadFile(binaryPath)
	if string(got) != "original" {
		t.Errorf("binary was replaced despite SHA256 mismatch: got %q", got)
	}

	// Verify no .new leftover
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".new" {
			t.Errorf("leftover .new file: %s", e.Name())
		}
	}
}

func TestUpdate_DownloadFailure_DoesNotReplace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "vh-agent")
	if err := os.WriteFile(binaryPath, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := &Updater{BinaryPath: binaryPath, HTTPClient: srv.Client()}
	err := u.Update(srv.URL+"/bin", "any")
	if err == nil {
		t.Fatal("expected error")
	}

	got, _ := os.ReadFile(binaryPath)
	if string(got) != "original" {
		t.Errorf("binary replaced despite 500: %q", got)
	}
}
