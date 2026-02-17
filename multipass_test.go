package main

import (
	"fmt"
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

// TestBuildLaunchVMArgs tests argument construction for launching VMs
func TestBuildLaunchVMArgs(t *testing.T) {
	tests := []struct {
		name     string
		vmName   string
		release  string
		expected []string
	}{
		{
			name:     "basic launch",
			vmName:   "test-vm",
			release:  "22.04",
			expected: []string{"launch", "--name", "test-vm", "22.04"},
		},
		{
			name:     "with Ubuntu codename",
			vmName:   "dev-box",
			release:  "jammy",
			expected: []string{"launch", "--name", "dev-box", "jammy"},
		},
		{
			name:     "empty release (should still work)",
			vmName:   "vm1",
			release:  "",
			expected: []string{"launch", "--name", "vm1", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLaunchArgs(tt.vmName, tt.release)

			if len(result) != len(tt.expected) {
				t.Errorf("buildLaunchArgs() returned %d args, want %d", len(result), len(tt.expected))
				return
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("Arg[%d] = %q, want %q", i, arg, tt.expected[i])
				}
			}
		})
	}
}

// buildLaunchArgs constructs arguments for VM launch (helper for testing)
func buildLaunchArgs(name, release string) []string {
	return []string{"launch", "--name", name, release}
}

// TestBuildLaunchVMAdvancedArgs tests argument construction for advanced VM launch
func TestBuildLaunchVMAdvancedArgs(t *testing.T) {
	tests := []struct {
		name     string
		vmName   string
		release  string
		cpus     int
		memoryMB int
		diskGB   int
		expected []string
	}{
		{
			name:     "standard resources",
			vmName:   "dev-vm",
			release:  "22.04",
			cpus:     2,
			memoryMB: 2048,
			diskGB:   20,
			expected: []string{
				"launch", "--name", "dev-vm",
				"--cpus", "2",
				"--memory", "2048M",
				"--disk", "20G",
				"22.04",
			},
		},
		{
			name:     "high resources",
			vmName:   "build-vm",
			release:  "jammy",
			cpus:     8,
			memoryMB: 16384,
			diskGB:   100,
			expected: []string{
				"launch", "--name", "build-vm",
				"--cpus", "8",
				"--memory", "16384M",
				"--disk", "100G",
				"jammy",
			},
		},
		{
			name:     "minimal resources",
			vmName:   "tiny-vm",
			release:  "20.04",
			cpus:     1,
			memoryMB: 512,
			diskGB:   5,
			expected: []string{
				"launch", "--name", "tiny-vm",
				"--cpus", "1",
				"--memory", "512M",
				"--disk", "5G",
				"20.04",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLaunchAdvancedArgs(tt.vmName, tt.release, tt.cpus, tt.memoryMB, tt.diskGB, "")

			if len(result) != len(tt.expected) {
				t.Errorf("Got %d args, want %d\nGot: %v\nWant: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("Arg[%d] = %q, want %q", i, arg, tt.expected[i])
				}
			}
		})
	}
}

// buildLaunchAdvancedArgs constructs arguments for advanced VM launch
func buildLaunchAdvancedArgs(name, release string, cpus, memoryMB, diskGB int, networkName string) []string {
	args := []string{
		"launch",
		"--name", name,
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%dM", memoryMB),
		"--disk", fmt.Sprintf("%dG", diskGB),
	}
	if networkName == "bridged" {
		args = append(args, "--bridged")
	} else if networkName != "" {
		args = append(args, "--network", networkName)
	}
	args = append(args, release)
	return args
}

// TestResourceFormatting tests the formatting of resource specifications
func TestResourceFormatting(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		unit     string
		expected string
	}{
		{"memory 1GB", 1024, "M", "1024M"},
		{"memory 512MB", 512, "M", "512M"},
		{"disk 20GB", 20, "G", "20G"},
		{"disk 100GB", 100, "G", "100G"},
		{"cpus 1", 1, "", "1"},
		{"cpus 16", 16, "", "16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.unit != "" {
				result = fmt.Sprintf("%d%s", tt.value, tt.unit)
			} else {
				result = fmt.Sprintf("%d", tt.value)
			}

			if result != tt.expected {
				t.Errorf("Got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestSnapshotCommandConstruction tests snapshot-related argument building
func TestSnapshotCommandConstruction(t *testing.T) {
	tests := []struct {
		name         string
		vmName       string
		snapshotName string
		comment      string
		expectedArgs []string
	}{
		{
			name:         "snapshot with comment",
			vmName:       "vm1",
			snapshotName: "backup1",
			comment:      "before-update",
			expectedArgs: []string{"snapshot", "--name", "backup1", "--comment", "before-update", "vm1"},
		},
		{
			name:         "snapshot without comment",
			vmName:       "vm2",
			snapshotName: "snap1",
			comment:      "",
			expectedArgs: []string{"snapshot", "--name", "snap1", "--comment", "", "vm2"},
		},
		{
			name:         "snapshot with complex name",
			vmName:       "production-vm",
			snapshotName: "2024-01-01-backup",
			comment:      "monthly backup",
			expectedArgs: []string{"snapshot", "--name", "2024-01-01-backup", "--comment", "monthly backup", "production-vm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSnapshotArgs(tt.vmName, tt.snapshotName, tt.comment)

			if len(result) != len(tt.expectedArgs) {
				t.Errorf("Got %d args, want %d", len(result), len(tt.expectedArgs))
				return
			}

			for i, arg := range result {
				if arg != tt.expectedArgs[i] {
					t.Errorf("Arg[%d] = %q, want %q", i, arg, tt.expectedArgs[i])
				}
			}
		})
	}
}

// buildSnapshotArgs constructs snapshot creation arguments
func buildSnapshotArgs(vmName, snapshotName, comment string) []string {
	return []string{"snapshot", "--name", snapshotName, "--comment", comment, vmName}
}

// TestDeleteVMArgConstruction tests delete command argument patterns
func TestDeleteVMArgConstruction(t *testing.T) {
	tests := []struct {
		name         string
		vmName       string
		purge        bool
		expectedArgs []string
	}{
		{
			name:         "delete without purge",
			vmName:       "vm1",
			purge:        false,
			expectedArgs: []string{"delete", "vm1"},
		},
		{
			name:         "delete with purge",
			vmName:       "vm2",
			purge:        true,
			expectedArgs: []string{"delete", "vm2", "--purge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDeleteArgs(tt.vmName, tt.purge)

			if len(result) != len(tt.expectedArgs) {
				t.Errorf("Got %d args, want %d\nGot: %v\nWant: %v",
					len(result), len(tt.expectedArgs), result, tt.expectedArgs)
				return
			}

			for i, arg := range result {
				if arg != tt.expectedArgs[i] {
					t.Errorf("Arg[%d] = %q, want %q", i, arg, tt.expectedArgs[i])
				}
			}
		})
	}
}

// buildDeleteArgs constructs delete command arguments
func buildDeleteArgs(vmName string, purge bool) []string {
	args := []string{"delete", vmName}
	if purge {
		args = append(args, "--purge")
	}
	return args
}

// TestExecCommandConstruction tests exec command with multiple arguments
func TestExecCommandConstruction(t *testing.T) {
	tests := []struct {
		name         string
		vmName       string
		command      []string
		expectedArgs []string
	}{
		{
			name:         "simple command",
			vmName:       "vm1",
			command:      []string{"ls", "-la"},
			expectedArgs: []string{"exec", "vm1", "--", "ls", "-la"},
		},
		{
			name:         "complex command",
			vmName:       "dev-vm",
			command:      []string{"bash", "-c", "echo hello"},
			expectedArgs: []string{"exec", "dev-vm", "--", "bash", "-c", "echo hello"},
		},
		{
			name:         "single command no args",
			vmName:       "vm2",
			command:      []string{"pwd"},
			expectedArgs: []string{"exec", "vm2", "--", "pwd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExecArgs(tt.vmName, tt.command...)

			if len(result) != len(tt.expectedArgs) {
				t.Errorf("Got %d args, want %d\nGot: %v\nWant: %v",
					len(result), len(tt.expectedArgs), result, tt.expectedArgs)
				return
			}

			for i, arg := range result {
				if arg != tt.expectedArgs[i] {
					t.Errorf("Arg[%d] = %q, want %q", i, arg, tt.expectedArgs[i])
				}
			}
		})
	}
}

// buildExecArgs constructs exec command arguments
func buildExecArgs(vmName string, command ...string) []string {
	args := []string{"exec", vmName, "--"}
	args = append(args, command...)
	return args
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
