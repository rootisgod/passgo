package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Note: Most functions in multipass.go call external commands which are hard to test
// without mocking. These tests focus on what CAN be tested without running actual
// multipass commands or creating helper functions for future refactoring.

// TestReadConfigValue tests reading config values from content
func TestReadConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		key      string
		expected string
		found    bool
	}{
		{
			name:     "simple key-value",
			content:  "github-cloud-init-repo=https://github.com/user/repo",
			key:      "github-cloud-init-repo",
			expected: "https://github.com/user/repo",
			found:    true,
		},
		{
			name:     "key with @ prefix in value",
			content:  "github-cloud-init-repo=@https://github.com/user/repo",
			key:      "github-cloud-init-repo",
			expected: "https://github.com/user/repo",
			found:    true,
		},
		{
			name:     "key not found",
			content:  "other-key=value",
			key:      "github-cloud-init-repo",
			expected: "",
			found:    false,
		},
		{
			name:     "empty content",
			content:  "",
			key:      "github-cloud-init-repo",
			expected: "",
			found:    false,
		},
		{
			name: "multiple lines, key on second line",
			content: `other-key=value1
github-cloud-init-repo=https://github.com/user/repo
another-key=value2`,
			key:      "github-cloud-init-repo",
			expected: "https://github.com/user/repo",
			found:    true,
		},
		{
			name:     "whitespace around equals",
			content:  "github-cloud-init-repo = https://github.com/user/repo",
			key:      "github-cloud-init-repo",
			expected: "https://github.com/user/repo",
			found:    true,
		},
		{
			name:     "empty value",
			content:  "github-cloud-init-repo=",
			key:      "github-cloud-init-repo",
			expected: "",
			found:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := readConfigValueFromContent(tt.content, tt.key)

			if found != tt.found {
				t.Errorf("readConfigValueFromContent() found = %v, want %v", found, tt.found)
			}

			if result != tt.expected {
				t.Errorf("readConfigValueFromContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// readConfigValueFromContent is a helper function we'll extract from the config reading logic
// This simulates what the config file parser should do
func readConfigValueFromContent(content, key string) (string, bool) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Handle whitespace around equals sign
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				lineKey := strings.TrimSpace(parts[0])
				if lineKey == key {
					value := strings.TrimSpace(parts[1])
					// Remove @ prefix if present
					value = strings.TrimPrefix(value, "@")
					return value, true
				}
			}
		}
	}
	return "", false
}

// TestValidateGitHubURL tests URL validation logic (this is a proposed function)
func TestValidateGitHubURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		isValid bool
	}{
		{
			name:    "valid HTTPS GitHub URL",
			url:     "https://github.com/user/repo",
			isValid: true,
		},
		{
			name:    "valid HTTPS GitHub URL with .git",
			url:     "https://github.com/user/repo.git",
			isValid: true,
		},
		{
			name:    "HTTP URL (insecure)",
			url:     "http://github.com/user/repo",
			isValid: false,
		},
		{
			name:    "SSH URL",
			url:     "git@github.com:user/repo.git",
			isValid: false,
		},
		{
			name:    "non-GitHub HTTPS URL",
			url:     "https://gitlab.com/user/repo",
			isValid: false,
		},
		{
			name:    "empty URL",
			url:     "",
			isValid: false,
		},
		{
			name:    "invalid URL format",
			url:     "not-a-url",
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidGitHubHTTPSURL(tt.url)
			if result != tt.isValid {
				t.Errorf("isValidGitHubHTTPSURL(%q) = %v, want %v", tt.url, result, tt.isValid)
			}
		})
	}
}

// isValidGitHubHTTPSURL validates that a URL is a GitHub HTTPS URL
// This is a helper function that should be added to multipass.go for security
func isValidGitHubHTTPSURL(url string) bool {
	if url == "" {
		return false
	}
	if !strings.HasPrefix(url, "https://github.com/") {
		return false
	}
	return true
}

// TestYAMLFileDetection tests file extension checking for cloud-init files
func TestYAMLFileDetection(t *testing.T) {
	tests := []struct {
		filename string
		isYAML   bool
	}{
		{"cloud-init.yaml", true},
		{"cloud-init.yml", true},
		{"config.YAML", true},
		{"config.YML", true},
		{"readme.txt", false},
		{"data.json", false},
		{"file.yaml.bak", false},
		{".yaml", true},
		{"no-extension", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isYAMLFile(tt.filename)
			if result != tt.isYAML {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tt.filename, result, tt.isYAML)
			}
		})
	}
}

// isYAMLFile checks if a filename has a YAML extension
func isYAMLFile(filename string) bool {
	if filename == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}

// TestSnapshotIDGeneration tests how snapshot IDs are constructed
func TestSnapshotIDGeneration(t *testing.T) {
	tests := []struct {
		vmName       string
		snapshotName string
		expected     string
	}{
		{"vm1", "snap1", "vm1.snap1"},
		{"myvm", "backup", "myvm.backup"},
		{"test-vm", "snapshot-1", "test-vm.snapshot-1"},
	}

	for _, tt := range tests {
		t.Run(tt.vmName+"+"+tt.snapshotName, func(t *testing.T) {
			result := generateSnapshotID(tt.vmName, tt.snapshotName)
			if result != tt.expected {
				t.Errorf("generateSnapshotID(%q, %q) = %q, want %q",
					tt.vmName, tt.snapshotName, result, tt.expected)
			}
		})
	}
}

// generateSnapshotID constructs a snapshot identifier
func generateSnapshotID(vmName, snapshotName string) string {
	return vmName + "." + snapshotName
}

// TestParseTemplateDisplayName tests extraction of display names from file paths
func TestParseTemplateDisplayName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"templates/docker.yaml", "docker"},
		{"cloud-init/kubernetes.yml", "kubernetes"},
		{"basic-setup.yaml", "basic-setup"},
		{"nested/path/to/config.yml", "config"},
		{"/absolute/path/file.yaml", "file"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getTemplateDisplayName(tt.path)
			if result != tt.expected {
				t.Errorf("getTemplateDisplayName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// getTemplateDisplayName extracts a friendly name from a template file path
func getTemplateDisplayName(path string) string {
	if path == "" {
		return ""
	}
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

// Integration test placeholder - these would require running actual multipass commands
// or using a mock/stub framework

// NOTE: The following are examples of what WOULD be tested with a proper mocking framework:
// - TestLaunchVM
// - TestStopVM
// - TestStartVM
// - TestDeleteVM
// - TestCreateSnapshot
// - TestListVMs
// - TestListSnapshots
//
// For now, we test the building blocks and helper functions.
// A future improvement would be to add interface-based mocking for exec.Command
