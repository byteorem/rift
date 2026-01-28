package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		relPath  string
		pattern  string
		isDir    bool
		expected bool
	}{
		// Simple filename patterns
		{"foo.log", "*.log", false, true},
		{"foo.txt", "*.log", false, false},
		{"dir/foo.log", "*.log", false, true},

		// Directory patterns
		{"node_modules", "node_modules/", true, true},
		{"node_modules", "node_modules/", false, false}, // file named node_modules
		{"src/node_modules", "node_modules/", true, true},

		// Exact matches
		{".git", ".git", true, true},
		{".gitignore", ".git", false, false},

		// Double-star patterns
		{"src/test/foo.go", "**/test", true, true},
		{"deep/nested/test", "**/test", true, true},

		// Path patterns
		{"build/output", "build/output", true, true},
		{"src/build/output", "build/output", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.relPath+"_"+tt.pattern, func(t *testing.T) {
			got := matchPattern(tt.relPath, tt.pattern, tt.isDir)
			if got != tt.expected {
				t.Errorf("matchPattern(%q, %q, %v) = %v, want %v",
					tt.relPath, tt.pattern, tt.isDir, got, tt.expected)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	patterns := []string{".git", "*.log", "node_modules/", "dist/"}

	tests := []struct {
		relPath  string
		isDir    bool
		expected bool
	}{
		{".git", true, true},
		{"src/main.go", false, false},
		{"debug.log", false, true},
		{"src/debug.log", false, true},
		{"node_modules", true, true},
		{"node_modules", false, false},
		{"dist", true, true},
		{"src/index.ts", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			got := shouldExclude(tt.relPath, patterns, tt.isDir)
			if got != tt.expected {
				t.Errorf("shouldExclude(%q, patterns, %v) = %v, want %v",
					tt.relPath, tt.isDir, got, tt.expected)
			}
		})
	}
}

func TestParseGitignore(t *testing.T) {
	// Create temp gitignore file
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")

	content := `# Comment
*.log
node_modules/

# Another comment
dist/
!important.log
`
	if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	patterns, err := parseGitignore(gitignorePath)
	if err != nil {
		t.Fatalf("parseGitignore() error = %v", err)
	}

	expected := []string{"*.log", "node_modules/", "dist/"}
	if len(patterns) != len(expected) {
		t.Fatalf("got %d patterns, want %d", len(patterns), len(expected))
	}

	for i, p := range expected {
		if patterns[i] != p {
			t.Errorf("patterns[%d] = %q, want %q", i, patterns[i], p)
		}
	}
}

func TestParseGitignoreMissing(t *testing.T) {
	_, err := parseGitignore("/nonexistent/.gitignore")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRunMissingToFlag(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Error("expected error when --to flag is missing")
	}
}

func TestRunToFlagMissingArg(t *testing.T) {
	err := run([]string{"--to"})
	if err == nil {
		t.Error("expected error when --to has no argument")
	}
}

func TestRunExcludeFlagMissingArg(t *testing.T) {
	err := run([]string{"--to", "/tmp", "--exclude"})
	if err == nil {
		t.Error("expected error when --exclude has no argument")
	}
}

func TestRunNameFlagMissingArg(t *testing.T) {
	err := run([]string{"--to", "/tmp", "--name"})
	if err == nil {
		t.Error("expected error when --name has no argument")
	}
}

func TestRunUnknownFlag(t *testing.T) {
	err := run([]string{"--unknown"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestSync(t *testing.T) {
	// Create source directory with files
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create source files
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ignore.log"), []byte("logs"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync with exclusion
	err := sync(srcDir, destDir, []string{"*.log"})
	if err != nil {
		t.Fatalf("sync() error = %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(destDir, "file1.txt")); err != nil {
		t.Error("file1.txt should exist in destination")
	}
	if _, err := os.Stat(filepath.Join(destDir, "subdir", "file2.txt")); err != nil {
		t.Error("subdir/file2.txt should exist in destination")
	}

	// Verify excluded file doesn't exist
	if _, err := os.Stat(filepath.Join(destDir, "ignore.log")); err == nil {
		t.Error("ignore.log should NOT exist in destination")
	}
}

func TestSyncRemovesOrphans(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create source file
	if err := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create orphan in destination
	if err := os.WriteFile(filepath.Join(destDir, "orphan.txt"), []byte("orphan"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync
	err := sync(srcDir, destDir, nil)
	if err != nil {
		t.Fatalf("sync() error = %v", err)
	}

	// Verify orphan was removed
	if _, err := os.Stat(filepath.Join(destDir, "orphan.txt")); err == nil {
		t.Error("orphan.txt should have been removed")
	}

	// Verify source file was synced
	if _, err := os.Stat(filepath.Join(destDir, "keep.txt")); err != nil {
		t.Error("keep.txt should exist in destination")
	}
}

func TestRunWithNameFlag(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create source file
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to source directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(srcDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Run with custom name
	err = run([]string{"--to", destDir, "--name", "CustomName"})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	// Verify file exists at custom-named destination
	if _, err := os.Stat(filepath.Join(destDir, "CustomName", "test.txt")); err != nil {
		t.Error("test.txt should exist in CustomName destination folder")
	}
}

func TestRunWithoutNameFlag(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create source file
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to source directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(srcDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Run without --name (should use directory name)
	err = run([]string{"--to", destDir})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	// Verify file exists using source directory name
	srcDirName := filepath.Base(srcDir)
	if _, err := os.Stat(filepath.Join(destDir, srcDirName, "test.txt")); err != nil {
		t.Errorf("test.txt should exist in %s destination folder", srcDirName)
	}
}
