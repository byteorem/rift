package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var destPath string
	var projectName string
	var excludePatterns []string

	// Parse arguments
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--to":
			if i+1 >= len(args) {
				return fmt.Errorf("--to requires a path argument")
			}
			i++
			destPath = args[i]
		case "--name":
			if i+1 >= len(args) {
				return fmt.Errorf("--name requires a name argument")
			}
			i++
			projectName = args[i]
		case "--exclude":
			if i+1 >= len(args) {
				return fmt.Errorf("--exclude requires a pattern argument")
			}
			i++
			excludePatterns = append(excludePatterns, args[i])
		case "-h", "--help":
			printUsage()
			return nil
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag: %s", args[i])
			}
		}
	}

	if destPath == "" {
		return fmt.Errorf("--to flag is required")
	}

	// Get current working directory
	srcPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Use current directory name if --name not provided
	if projectName == "" {
		projectName = filepath.Base(srcPath)
	}

	// Build full destination path
	fullDest := filepath.Join(destPath, projectName)

	// Always exclude .git
	patterns := []string{".git"}

	// Parse .gitignore if present
	gitignorePath := filepath.Join(srcPath, ".gitignore")
	if gitignorePatterns, err := parseGitignore(gitignorePath); err == nil {
		patterns = append(patterns, gitignorePatterns...)
	}

	// Add user-specified exclusions
	patterns = append(patterns, excludePatterns...)

	// Perform sync
	return sync(srcPath, fullDest, patterns)
}

func printUsage() {
	fmt.Println(`rift - Sync project files to a destination

Usage:
  rift --to <destination> [--name <name>] [--exclude <pattern>]...

Flags:
  --to        Destination path (required)
  --name      Name for destination folder (defaults to current directory name)
  --exclude   Additional patterns to exclude (repeatable)
  -h, --help  Show this help

Examples:
  rift --to /backup
  rift --to /games/addons --name MyAddon
  rift --to ~/projects-backup --exclude "*.log" --exclude "tmp/"`)
}

func parseGitignore(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip negation patterns for simplicity
		if strings.HasPrefix(line, "!") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

func shouldExclude(relPath string, patterns []string, isDir bool) bool {
	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		if matchPattern(relPath, pattern, isDir) {
			return true
		}
	}
	return false
}

func matchPattern(relPath, pattern string, isDir bool) bool {
	pattern = filepath.ToSlash(pattern)

	// Handle directory-only patterns (trailing /)
	dirOnly := strings.HasSuffix(pattern, "/")
	if dirOnly {
		pattern = strings.TrimSuffix(pattern, "/")
		if !isDir {
			return false
		}
	}

	// Handle ** prefix (matches any path)
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		// Match against any path component
		parts := strings.Split(relPath, "/")
		for i := range parts {
			subPath := strings.Join(parts[i:], "/")
			if matched, _ := filepath.Match(suffix, subPath); matched {
				return true
			}
			// Also check just the filename/dirname
			if matched, _ := filepath.Match(suffix, parts[i]); matched {
				return true
			}
		}
		return false
	}

	// Handle patterns without path separator - match against any component
	if !strings.Contains(pattern, "/") {
		// Match against the filename/dirname itself
		base := filepath.Base(relPath)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Also match against each path component
		parts := strings.Split(relPath, "/")
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
		return false
	}

	// Pattern with / - match from root
	pattern = strings.TrimPrefix(pattern, "/")
	matched, _ := filepath.Match(pattern, relPath)
	return matched
}

func sync(src, dest string, patterns []string) error {
	// Track valid paths in destination for cleanup
	validPaths := make(map[string]bool)

	// Walk source directory
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		isDir := d.IsDir()

		// Check exclusions
		if shouldExclude(relPath, patterns, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		destPath := filepath.Join(dest, relPath)
		validPaths[destPath] = true

		if isDir {
			// Create directory
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		return copyFile(path, destPath)
	})

	if err != nil {
		return fmt.Errorf("walking source: %w", err)
	}

	// Clean orphaned files in destination
	return cleanOrphans(dest, validPaths)
}

func copyFile(src, dest string) error {
	// Get source file info
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Check if destination exists and is identical
	if destInfo, err := os.Stat(dest); err == nil {
		if destInfo.Size() == info.Size() && destInfo.ModTime().Equal(info.ModTime()) {
			return nil // Skip identical files
		}
	}

	// Open source
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination
	destFile, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	// Preserve modification time
	return os.Chtimes(dest, info.ModTime(), info.ModTime())
}

func cleanOrphans(dest string, validPaths map[string]bool) error {
	// If destination doesn't exist, nothing to clean
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return nil
	}

	var toRemove []string

	err := filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip root
		if path == dest {
			return nil
		}

		// If path is not in valid paths, mark for removal
		if !validPaths[path] {
			toRemove = append(toRemove, path)
			if d.IsDir() {
				return filepath.SkipDir // Don't descend into dirs we'll remove
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("scanning destination: %w", err)
	}

	// Remove orphaned paths
	for _, path := range toRemove {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}

	return nil
}
