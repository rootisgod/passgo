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
	err := cmd.Run()
	if err != nil {
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
		defer fileHandle.Close()

		scanner := bufio.NewScanner(fileHandle)
		if scanner.Scan() {
			firstLine := strings.TrimSpace(scanner.Text())
			if firstLine == "#cloud-config" {
				cloudInitFiles = append(cloudInitFiles, fileName)
			}
		}
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
