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
