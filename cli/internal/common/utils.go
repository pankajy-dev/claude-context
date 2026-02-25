package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NormalizePath expands ~ and converts to absolute path
func NormalizePath(path string) (string, error) {
	// Expand ~
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	} else if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = homeDir
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return absPath, nil
}

// CreateSymlink creates a symlink
func CreateSymlink(source, target string) error {
	// Remove existing symlink/file if it exists
	if _, err := os.Lstat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove existing target: %w", err)
		}
	}

	// Create symlink
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// RemoveSymlink removes a symlink
func RemoveSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return fmt.Errorf("failed to stat symlink: %w", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("not a symlink: %s", path)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	return nil
}

// IsSymlink checks if a path is a symlink
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// SymlinkTarget returns the target of a symlink
func SymlinkTarget(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink: %w", err)
	}
	return target, nil
}

// ValidateSymlink checks if a symlink exists and points to the correct target
func ValidateSymlink(symlinkPath, expectedTarget string) (bool, error) {
	if !IsSymlink(symlinkPath) {
		return false, nil
	}

	target, err := SymlinkTarget(symlinkPath)
	if err != nil {
		return false, err
	}

	// Check if target exists
	if _, err := os.Stat(target); err != nil {
		return false, nil // Broken symlink
	}

	// Compare with expected target
	return target == expectedTarget, nil
}

// GitCommit commits changes to git with a message
func GitCommit(repoPath, message string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[DRY RUN] Would commit to git: %s\n", message)
		return nil
	}

	// Check if auto_commit is enabled (could be read from config)
	// For now, we'll always commit

	// Stage all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\n%s", err, output)
	}

	// Commit
	commitMsg := fmt.Sprintf("%s\n\nCo-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>", message)
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if there are no changes to commit
		if strings.Contains(string(output), "nothing to commit") {
			return nil // Not an error
		}
		return fmt.Errorf("git commit failed: %w\n%s", err, output)
	}

	return nil
}

// EnsureDir ensures a directory exists
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Confirm prompts the user for confirmation
func Confirm(prompt string, defaultYes bool) bool {
	var response string
	defaultStr := "y/N"
	if defaultYes {
		defaultStr = "Y/n"
	}

	fmt.Printf("%s [%s]: ", prompt, defaultStr)
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}

// GetGitBranch returns the current git branch name, or empty string if not in a git repo
func GetGitBranch() string {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// GetGitCommit returns the latest git commit hash, or empty string if not in a git repo
func GetGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// GetGitCommitShort returns the short version of the latest git commit hash
func GetGitCommitShort() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	// Read source file
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Get source file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := EnsureDir(dstDir); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write destination file with same permissions
	if err := os.WriteFile(dst, content, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

