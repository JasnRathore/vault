package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeURLPath(t *testing.T) {
	tests := map[string]string{
		"":              "/",
		"/":             "/",
		"folder":        "/folder",
		"/folder":       "/folder",
		"\\folder\\sub": "/folder/sub",
		"/a/../b":       "/b",
	}

	for input, want := range tests {
		if got := normalizeURLPath(input); got != want {
			t.Fatalf("normalizeURLPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSafePathAllowsRootedBrowserPaths(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWD)

	got, err := safePath("/docs")
	if err != nil {
		t.Fatalf("safePath returned error: %v", err)
	}

	want := filepath.Join(tempDir, "docs")
	if got != want {
		t.Fatalf("safePath returned %q, want %q", got, want)
	}
}

func TestListDirectoryUsesURLStylePaths(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWD)

	entries, err := listDirectory("/")
	if err != nil {
		t.Fatalf("listDirectory returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("listDirectory returned %d entries, want 1", len(entries))
	}
	if entries[0].Path != "/docs" {
		t.Fatalf("entry path = %q, want %q", entries[0].Path, "/docs")
	}
}
