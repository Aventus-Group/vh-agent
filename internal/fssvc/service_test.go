package fssvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
)

func TestReadFile_Normal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New()
	resp, err := svc.ReadFile(context.Background(), &agentpb.ReadFileRequest{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Content) != "hello world" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.Size != 11 {
		t.Errorf("size = %d", resp.Size)
	}
	if resp.Truncated {
		t.Error("unexpected truncation")
	}
}

func TestReadFile_Truncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	content := strings.Repeat("x", 2000)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New()
	resp, err := svc.ReadFile(context.Background(), &agentpb.ReadFileRequest{Path: path, MaxBytes: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1000 {
		t.Errorf("content len = %d want 1000", len(resp.Content))
	}
	if !resp.Truncated {
		t.Error("expected truncated=true")
	}
	if resp.Size != 2000 {
		t.Errorf("size = %d want 2000", resp.Size)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	svc := New()
	_, err := svc.ReadFile(context.Background(), &agentpb.ReadFileRequest{Path: "/nope/doesnotexist"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteFile_Creates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "out.txt")
	svc := New()
	resp, err := svc.WriteFile(context.Background(), &agentpb.WriteFileRequest{
		Path:       path,
		Content:    []byte("payload"),
		Mode:       0o644,
		CreateDirs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.BytesWritten != 7 {
		t.Errorf("bytes_written = %d", resp.BytesWritten)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("file content = %q", got)
	}
}

func TestWriteFile_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := New()
	_, err := svc.WriteFile(context.Background(), &agentpb.WriteFileRequest{
		Path:    path,
		Content: []byte("new"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("overwrite failed: got %q", got)
	}
}

func TestListDir_Basic(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("22"), 0o644)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o755)

	svc := New()
	resp, err := svc.ListDir(context.Background(), &agentpb.ListDirRequest{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 3 {
		t.Errorf("entries len = %d want 3", len(resp.Entries))
	}
	names := map[string]*agentpb.DirEntry{}
	for _, e := range resp.Entries {
		names[e.Name] = e
	}
	if e, ok := names["a.txt"]; !ok || e.IsDir || e.Size != 1 {
		t.Errorf("bad a.txt entry: %+v", e)
	}
	if e, ok := names["sub"]; !ok || !e.IsDir {
		t.Errorf("bad sub entry: %+v", e)
	}
}

func TestListDir_NotFound(t *testing.T) {
	svc := New()
	_, err := svc.ListDir(context.Background(), &agentpb.ListDirRequest{Path: "/nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}
