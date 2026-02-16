// multipass.go - Functions to interact with Multipass command-line tool
package main

import (
	"bufio"         // For reading files line by line
	"bytes"         // For handling byte data (used with command output)
	"fmt"           // For formatted printing and string formatting
	"os"            // For operating system operations (like reading directories)
	"os/exec"       // For running external commands (like multipass)
	"path/filepath" // For handling file paths in a cross-platform way
	"strings"       // For string manipulation functions
)

// runMultipassCommand executes multipass commands with variadic arguments
func runMultipassCommand(args ...string) (string, error) {
	cmd := exec.Command("multipass", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if appLogger != nil {
		appLogger.Printf("exec: multipass %s", strings.Join(args, " "))
	}
	err := cmd.Run()
	if err != nil {
		if appLogger != nil {
			appLogger.Printf("exec error: %v; stderr: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("command failed: %v\nStderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// LaunchVM creates a new virtual machine with basic settings
func LaunchVM(name, release string) (string, error) {
	args := []string{"launch", "--name", name, release}
	return runMultipassCommand(args...)
}

// LaunchVMAdvanced creates VM with custom resource settings
func LaunchVMAdvanced(name, release string, cpus int, memoryMB int, diskGB int) (string, error) {
	args := []string{
		"launch",
		"--name", name,
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%dM", memoryMB),
		"--disk", fmt.Sprintf("%dG", diskGB),
		release,
	}

	return runMultipassCommand(args...)
}

func ListVMs() (string, error) {
	return runMultipassCommand("list")
}

func StopVM(name string) (string, error) {
	return runMultipassCommand("stop", name)
}

func StartVM(name string) (string, error) {
	return runMultipassCommand("start", name)
}

func DeleteVM(name string, purge bool) (string, error) {
	args := []string{"delete", name}
	if purge {
		args = append(args, "--purge")
	}
	return runMultipassCommand(args...)
}

func RecoverVM(name string) (string, error) {
	return runMultipassCommand("recover", name)
}

func ExecInVM(vmName string, commandArgs ...string) (string, error) {
	args := append([]string{"exec", vmName, "--"}, commandArgs...)
	return runMultipassCommand(args...)
}

func ShellVM(vmName string) error {
	cmd := exec.Command("multipass", "shell", vmName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func GetVMInfo(name string) (string, error) {
	return runMultipassCommand("info", name)
}

func CreateSnapshot(vmName, snapshotName, description string) (string, error) {
	args := []string{"snapshot", "--name", snapshotName, "--comment", description, vmName}
	return runMultipassCommand(args...)
}

func ListSnapshots() (string, error) {
	return runMultipassCommand("list", "--snapshots")
}

func RestoreSnapshot(vmName, snapshotName string) (string, error) {
	snapshotID := vmName + "." + snapshotName
	args := []string{"restore", "--destructive", snapshotID}
	return runMultipassCommand(args...)
}

func DeleteSnapshot(vmName, snapshotName string) (string, error) {
	snapshotID := vmName + "." + snapshotName
	args := []string{"delete", "--purge", snapshotID}
	return runMultipassCommand(args...)
}

// ScanCloudInitFiles finds YAML files with "#cloud-config" header for VM configuration
func ScanCloudInitFiles() ([]string, error) {
	var cloudInitFiles []string
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}
	files, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		if !strings.HasSuffix(strings.ToLower(fileName), ".yml") && !strings.HasSuffix(strings.ToLower(fileName), ".yaml") {
			continue
		}

		filePath := filepath.Join(currentDir, fileName)
		fileHandle, err := os.Open(filePath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(fileHandle)
		if scanner.Scan() {
			firstLine := strings.TrimSpace(scanner.Text())
			if firstLine == "#cloud-config" {
				cloudInitFiles = append(cloudInitFiles, fileName)
			}
		}
		fileHandle.Close()
	}
	return cloudInitFiles, nil
}

func LaunchVMWithCloudInit(name, release string, cpus int, memoryMB int, diskGB int, cloudInitFile string) (string, error) {
	args := []string{
		"launch",
		"--name", name,
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%dM", memoryMB),
		"--disk", fmt.Sprintf("%dG", diskGB),
		"--cloud-init", cloudInitFile,
		release,
	}

	return runMultipassCommand(args...)
}

// TemplateOption represents a selectable cloud-init template
type TemplateOption struct {
	Label string
	Path  string
}

// ReadConfigGithubRepo reads the hidden .config file in the current directory and extracts the github-cloud-init-repo value
func ReadConfigGithubRepo() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}
	configPath := filepath.Join(currentDir, ".config")
	if appLogger != nil {
		appLogger.Printf("reading config: %s", configPath)
	}
	file, err := os.Open(configPath)
	if err != nil {
		if appLogger != nil {
			appLogger.Printf("config open error: %v", err)
		}
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	key := "github-cloud-init-repo"
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, key) {
			continue
		}
		// Extract the substring following the key, preserving URL colons
		idx := strings.Index(line, key)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len(key):])
		// Remove one leading separator if present
		if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ":") {
			rest = strings.TrimSpace(rest[1:])
		}
		// Remove optional additional whitespace separators
		rest = strings.TrimLeft(rest, " \t")
		// Drop optional leading '@'
		rest = strings.TrimPrefix(rest, "@")
		if rest != "" {
			if appLogger != nil {
				appLogger.Printf("config repo url: %s", rest)
			}
			return rest, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("github-cloud-init-repo not found in .config")
}

// CloneRepoAndScanYAMLs clones the provided repo into a temp dir and returns cloud-init YAML templates found
func CloneRepoAndScanYAMLs(repoURL string) ([]TemplateOption, string, error) {
	if repoURL == "" {
		return nil, "", fmt.Errorf("empty repo URL")
	}

	tmpDir, err := os.MkdirTemp("", "passgo-cloudinit-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	if appLogger != nil {
		appLogger.Printf("cloning repo %s into %s", repoURL, tmpDir)
	}

	// Shallow clone
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if appLogger != nil {
			appLogger.Printf("git clone failed: %v; %s", err, strings.TrimSpace(stderr.String()))
		}
		os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("git clone failed: %v; %s", err, stderr.String())
	}

	// Walk repo and collect all .yml/.yaml files (no header requirement)
	var options []TemplateOption
	err = filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		label := "repo/" + rel
		options = append(options, TemplateOption{Label: label, Path: path})
		return nil
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("failed to scan repo: %v", err)
	}
	if appLogger != nil {
		appLogger.Printf("found %d yaml templates in repo", len(options))
	}

	return options, tmpDir, nil
}

// GetAllCloudInitTemplateOptions aggregates local and (optional) repo templates.
// Returns the options, any temp dirs to cleanup after use, and error.
func GetAllCloudInitTemplateOptions() ([]TemplateOption, []string, error) {
	var all []TemplateOption
	var cleanupDirs []string

	// Local templates (current dir)
	local, err := ScanCloudInitFiles()
	if err == nil {
		for _, name := range local {
			all = append(all, TemplateOption{Label: name, Path: filepath.Join(".", name)})
		}
		if appLogger != nil {
			appLogger.Printf("found %d local cloud-init templates", len(local))
		}
	}

	// Repo templates via .config
	if repoURL, err := ReadConfigGithubRepo(); err == nil && repoURL != "" {
		if opts, tmpDir, err := CloneRepoAndScanYAMLs(repoURL); err == nil {
			all = append(all, opts...)
			if tmpDir != "" {
				cleanupDirs = append(cleanupDirs, tmpDir)
			}
			if appLogger != nil {
				appLogger.Printf("aggregated %d total templates (local+repo)", len(all))
			}
		} else if appLogger != nil {
			appLogger.Printf("repo scan error: %v", err)
		}
	}

	return all, cleanupDirs, nil
}

// CleanupTempDirs removes temporary directories created during repo cloning
func CleanupTempDirs(dirs []string) {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if appLogger != nil {
			appLogger.Printf("cleanup temp dir: %s", d)
		}
		_ = os.RemoveAll(d)
	}
}
