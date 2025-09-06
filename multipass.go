// multipass.go
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// runMultipassCommand executes a Multipass command and captures output
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

// LaunchVM launches a new VM with the given name and release
func LaunchVM(name, release string) (string, error) {
	args := []string{"launch", "--name", name, release}
	return runMultipassCommand(args...)
}

// LaunchVMAdvanced launches a new VM with advanced configuration options
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

// ListVMs lists all VMs
func ListVMs() (string, error) {
	return runMultipassCommand("list")
}

// StopVM stops a VM
func StopVM(name string) (string, error) {
	return runMultipassCommand("stop", name)
}

// StartVM starts a VM
func StartVM(name string) (string, error) {
	return runMultipassCommand("start", name)
}

// DeleteVM deletes a VM (purge=true for permanent deletion)
func DeleteVM(name string, purge bool) (string, error) {
	args := []string{"delete", name}
	if purge {
		args = append(args, "--purge")
	}
	return runMultipassCommand(args...)
}

// ExecInVM executes a command inside a VM
func ExecInVM(vmName string, commandArgs ...string) (string, error) {
	args := append([]string{"exec", vmName, "--"}, commandArgs...)
	return runMultipassCommand(args...)
}

// GetVMInfo gets detailed info about a VM
func GetVMInfo(name string) (string, error) {
	return runMultipassCommand("info", name)
}

// CreateSnapshot creates a snapshot of a VM with the given name and description
func CreateSnapshot(vmName, snapshotName, description string) (string, error) {
	args := []string{"snapshot", "--name", snapshotName, "--comment", description, vmName}
	return runMultipassCommand(args...)
}

// ListSnapshots lists all available snapshots
func ListSnapshots() (string, error) {
	return runMultipassCommand("list", "--snapshots")
}

// RestoreSnapshot restores a VM to a specific snapshot
func RestoreSnapshot(vmName, snapshotName string) (string, error) {
	snapshotID := vmName + "." + snapshotName
	args := []string{"restore", "--destructive", snapshotID}
	return runMultipassCommand(args...)
}

// DeleteSnapshot deletes a specific snapshot
func DeleteSnapshot(vmName, snapshotName string) (string, error) {
	snapshotID := vmName + "." + snapshotName
	args := []string{"delete", "--purge", snapshotID}
	return runMultipassCommand(args...)
}
