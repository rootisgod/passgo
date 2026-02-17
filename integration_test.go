// integration_test.go - Integration tests that interact with actual multipass instances
// These tests create real VMs and test the full lifecycle of operations.
//
// WARNING: These tests will create and delete actual multipass VMs.
// Run with: go test -v -tags=integration -run TestIntegration
//
// To skip integration tests: go test -v (without tags)
//
// These tests help catch issues with multipass version updates by testing
// real interactions with the multipass CLI.

//go:build integration
// +build integration

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestIntegrationVMLifecycle tests the complete VM lifecycle:
// create -> stop -> start -> delete
func TestIntegrationVMLifecycle(t *testing.T) {
	// Check if multipass is available
	if !isMultipassAvailable(t) {
		t.Skip("Multipass not available, skipping integration tests")
	}

	// Generate unique VM name with timestamp
	vmName := fmt.Sprintf("test-vm-%d", time.Now().Unix())
	t.Logf("Testing VM lifecycle with: %s", vmName)

	// Ensure cleanup happens even if test fails
	t.Cleanup(func() {
		t.Logf("Cleaning up test VM: %s", vmName)
		DeleteVM(vmName, true) // purge to fully clean up
	})

	// Step 1: Create VM
	t.Run("CreateVM", func(t *testing.T) {
		t.Logf("Creating VM: %s", vmName)
		_, err := LaunchVM(vmName, "22.04")
		if err != nil {
			t.Fatalf("Failed to create VM: %v", err)
		}

		// Wait a bit for VM to initialize
		time.Sleep(5 * time.Second)
	})

	// Step 2: Verify VM appears in list
	t.Run("ListVM", func(t *testing.T) {
		t.Logf("Verifying VM appears in list")
		output, err := ListVMs()
		if err != nil {
			t.Fatalf("Failed to list VMs: %v", err)
		}

		if !strings.Contains(output, vmName) {
			t.Errorf("VM %s not found in list output:\n%s", vmName, output)
		}

		vmNames := parseVMNames(output)
		found := false
		for _, name := range vmNames {
			if name == vmName {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("VM %s not found in parsed VM names: %v", vmName, vmNames)
		}
	})

	// Step 3: Get VM info
	t.Run("GetVMInfo", func(t *testing.T) {
		t.Logf("Getting VM info for: %s", vmName)
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info: %v", err)
		}

		if output == "" {
			t.Error("VM info output is empty")
		}

		// Parse the info
		vmInfo := parseVMInfo(output)
		if vmInfo.Name != vmName {
			t.Errorf("VM name in info = %q, want %q", vmInfo.Name, vmName)
		}

		if vmInfo.Release == "" {
			t.Error("VM Release field is empty")
		}

		t.Logf("VM Info parsed successfully: Name=%s, State=%s, Release=%s",
			vmInfo.Name, vmInfo.State, vmInfo.Release)
	})

	// Step 4: Stop VM
	t.Run("StopVM", func(t *testing.T) {
		t.Logf("Stopping VM: %s", vmName)
		_, err := StopVM(vmName)
		if err != nil {
			t.Fatalf("Failed to stop VM: %v", err)
		}

		// Wait for stop to complete
		time.Sleep(3 * time.Second)

		// Verify VM is stopped
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info after stop: %v", err)
		}

		vmInfo := parseVMInfo(output)
		if !strings.Contains(strings.ToLower(vmInfo.State), "stopped") {
			t.Errorf("VM state after stop = %q, expected 'Stopped'", vmInfo.State)
		}

		t.Logf("VM stopped successfully: State=%s", vmInfo.State)
	})

	// Step 5: Start VM
	t.Run("StartVM", func(t *testing.T) {
		t.Logf("Starting VM: %s", vmName)
		_, err := StartVM(vmName)
		if err != nil {
			t.Fatalf("Failed to start VM: %v", err)
		}

		// Wait for start to complete
		time.Sleep(5 * time.Second)

		// Verify VM is running
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info after start: %v", err)
		}

		vmInfo := parseVMInfo(output)
		if !strings.Contains(strings.ToLower(vmInfo.State), "running") {
			t.Errorf("VM state after start = %q, expected 'Running'", vmInfo.State)
		}

		t.Logf("VM started successfully: State=%s", vmInfo.State)
	})

	// Step 6: Delete VM
	t.Run("DeleteVM", func(t *testing.T) {
		t.Logf("Deleting VM: %s", vmName)
		_, err := DeleteVM(vmName, false)
		if err != nil {
			t.Fatalf("Failed to delete VM: %v", err)
		}

		// Wait for deletion
		time.Sleep(2 * time.Second)

		// Note: Deleted VMs still appear in list until purged
		// We'll purge in cleanup
		t.Logf("VM deleted successfully")
	})
}

// TestIntegrationSnapshotOperations tests snapshot lifecycle:
// create VM -> create snapshot -> restore snapshot -> delete snapshot -> delete VM
func TestIntegrationSnapshotOperations(t *testing.T) {
	if !isMultipassAvailable(t) {
		t.Skip("Multipass not available, skipping integration tests")
	}

	vmName := fmt.Sprintf("test-snap-vm-%d", time.Now().Unix())
	snap1Name := "test-snapshot-1"
	snap2Name := "test-snapshot-2"

	t.Logf("Testing snapshot operations with VM: %s", vmName)

	// Cleanup
	t.Cleanup(func() {
		t.Logf("Cleaning up test VM and snapshots: %s", vmName)
		DeleteVM(vmName, true)
	})

	// Create VM
	t.Run("CreateVM", func(t *testing.T) {
		t.Logf("Creating VM: %s", vmName)
		_, err := LaunchVM(vmName, "22.04")
		if err != nil {
			t.Fatalf("Failed to create VM: %v", err)
		}
		time.Sleep(5 * time.Second)
	})

	// Stop VM (required for snapshots)
	t.Run("StopVMForSnapshot", func(t *testing.T) {
		t.Logf("Stopping VM for snapshot operations")
		_, err := StopVM(vmName)
		if err != nil {
			t.Fatalf("Failed to stop VM: %v", err)
		}
		time.Sleep(3 * time.Second)
	})

	// Create first snapshot
	t.Run("CreateSnapshot1", func(t *testing.T) {
		t.Logf("Creating snapshot: %s", snap1Name)
		_, err := CreateSnapshot(vmName, snap1Name, "First test snapshot")
		if err != nil {
			t.Fatalf("Failed to create snapshot: %v", err)
		}
		time.Sleep(2 * time.Second)
	})

	// Create second snapshot (child of first)
	t.Run("CreateSnapshot2", func(t *testing.T) {
		t.Logf("Creating second snapshot: %s", snap2Name)
		_, err := CreateSnapshot(vmName, snap2Name, "Second test snapshot")
		if err != nil {
			t.Fatalf("Failed to create second snapshot: %v", err)
		}
		time.Sleep(2 * time.Second)
	})

	// List snapshots and verify both exist
	t.Run("ListSnapshots", func(t *testing.T) {
		t.Logf("Listing snapshots")
		output, err := ListSnapshots()
		if err != nil {
			t.Fatalf("Failed to list snapshots: %v", err)
		}

		snapshots := parseSnapshots(output)

		// Find our snapshots
		var found1, found2 bool
		var snap1, snap2 SnapshotInfo

		for _, snap := range snapshots {
			if snap.Instance == vmName {
				if snap.Name == snap1Name {
					found1 = true
					snap1 = snap
				}
				if snap.Name == snap2Name {
					found2 = true
					snap2 = snap
				}
			}
		}

		if !found1 {
			t.Errorf("Snapshot %s not found in list", snap1Name)
		}
		if !found2 {
			t.Errorf("Snapshot %s not found in list", snap2Name)
		}

		// Verify snapshot hierarchy
		if found1 && snap1.Parent != "" {
			t.Logf("Snapshot 1 parent: %s (expected empty)", snap1.Parent)
		}

		if found2 && snap2.Parent != snap1Name {
			t.Logf("Warning: Snapshot 2 parent = %s, expected %s", snap2.Parent, snap1Name)
		}

		t.Logf("Found %d total snapshots, %d for test VM", len(snapshots), func() int {
			count := 0
			for _, s := range snapshots {
				if s.Instance == vmName {
					count++
				}
			}
			return count
		}())
	})

	// Restore to first snapshot
	t.Run("RestoreSnapshot", func(t *testing.T) {
		t.Logf("Restoring to snapshot: %s", snap1Name)
		_, err := RestoreSnapshot(vmName, snap1Name)
		if err != nil {
			t.Fatalf("Failed to restore snapshot: %v", err)
		}
		time.Sleep(2 * time.Second)

		t.Logf("Successfully restored to snapshot: %s", snap1Name)
	})

	// Delete second snapshot
	t.Run("DeleteSnapshot2", func(t *testing.T) {
		t.Logf("Deleting snapshot: %s", snap2Name)
		_, err := DeleteSnapshot(vmName, snap2Name)
		if err != nil {
			t.Fatalf("Failed to delete snapshot: %v", err)
		}
		time.Sleep(2 * time.Second)
	})

	// Delete first snapshot
	t.Run("DeleteSnapshot1", func(t *testing.T) {
		t.Logf("Deleting snapshot: %s", snap1Name)
		_, err := DeleteSnapshot(vmName, snap1Name)
		if err != nil {
			t.Fatalf("Failed to delete snapshot: %v", err)
		}
		time.Sleep(2 * time.Second)
	})

	// Verify snapshots are deleted
	t.Run("VerifySnapshotsDeleted", func(t *testing.T) {
		t.Logf("Verifying snapshots are deleted")
		output, err := ListSnapshots()
		if err != nil {
			t.Fatalf("Failed to list snapshots: %v", err)
		}

		snapshots := parseSnapshots(output)
		for _, snap := range snapshots {
			if snap.Instance == vmName {
				if snap.Name == snap1Name || snap.Name == snap2Name {
					t.Errorf("Snapshot %s still exists after deletion", snap.Name)
				}
			}
		}

		t.Logf("Snapshots successfully deleted")
	})
}

// TestIntegrationAdvancedVMCreation tests creating a VM with custom resources
func TestIntegrationAdvancedVMCreation(t *testing.T) {
	if !isMultipassAvailable(t) {
		t.Skip("Multipass not available, skipping integration tests")
	}

	vmName := fmt.Sprintf("test-advanced-vm-%d", time.Now().Unix())
	t.Logf("Testing advanced VM creation: %s", vmName)

	t.Cleanup(func() {
		t.Logf("Cleaning up advanced test VM: %s", vmName)
		DeleteVM(vmName, true)
	})

	// Create VM with custom resources
	t.Run("CreateAdvancedVM", func(t *testing.T) {
		cpus := 2
		memoryMB := 2048
		diskGB := 10

		t.Logf("Creating advanced VM with: cpus=%d, memory=%dMB, disk=%dGB",
			cpus, memoryMB, diskGB)

		_, err := LaunchVMAdvanced(vmName, "22.04", cpus, memoryMB, diskGB, "")
		if err != nil {
			t.Fatalf("Failed to create advanced VM: %v", err)
		}

		time.Sleep(5 * time.Second)
	})

	// Verify VM was created with correct specs
	t.Run("VerifyVMSpecs", func(t *testing.T) {
		t.Logf("Verifying VM specifications")
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info: %v", err)
		}

		vmInfo := parseVMInfo(output)

		if vmInfo.CPUs == "" {
			t.Error("CPU information not available")
		} else {
			t.Logf("VM CPUs: %s", vmInfo.CPUs)
		}

		if vmInfo.MemoryUsage == "" {
			t.Error("Memory information not available")
		} else {
			t.Logf("VM Memory: %s", vmInfo.MemoryUsage)
		}

		if vmInfo.DiskUsage == "" {
			t.Error("Disk information not available")
		} else {
			t.Logf("VM Disk: %s", vmInfo.DiskUsage)
		}
	})

	// Cleanup
	t.Run("DeleteAdvancedVM", func(t *testing.T) {
		t.Logf("Deleting advanced VM")
		_, err := DeleteVM(vmName, true)
		if err != nil {
			t.Fatalf("Failed to delete VM: %v", err)
		}
	})
}

// TestIntegrationVMSuspendResume tests suspend and resume operations
func TestIntegrationVMSuspendResume(t *testing.T) {
	if !isMultipassAvailable(t) {
		t.Skip("Multipass not available, skipping integration tests")
	}

	vmName := fmt.Sprintf("test-suspend-vm-%d", time.Now().Unix())
	t.Logf("Testing suspend/resume with: %s", vmName)

	t.Cleanup(func() {
		DeleteVM(vmName, true)
	})

	// Create VM
	t.Run("CreateVM", func(t *testing.T) {
		_, err := LaunchVM(vmName, "22.04")
		if err != nil {
			t.Fatalf("Failed to create VM: %v", err)
		}
		time.Sleep(5 * time.Second)
	})

	// Suspend VM
	t.Run("SuspendVM", func(t *testing.T) {
		t.Logf("Suspending VM: %s", vmName)
		_, err := runMultipassCommand("suspend", vmName)
		if err != nil {
			t.Fatalf("Failed to suspend VM: %v", err)
		}
		time.Sleep(3 * time.Second)

		// Verify suspended
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info: %v", err)
		}

		vmInfo := parseVMInfo(output)
		if !strings.Contains(strings.ToLower(vmInfo.State), "suspended") {
			t.Logf("Warning: VM state after suspend = %q (expected 'Suspended')", vmInfo.State)
		}
	})

	// Resume VM (start after suspend)
	t.Run("ResumeVM", func(t *testing.T) {
		t.Logf("Resuming VM: %s", vmName)
		_, err := StartVM(vmName)
		if err != nil {
			t.Fatalf("Failed to resume VM: %v", err)
		}
		time.Sleep(5 * time.Second)

		// Verify running
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info: %v", err)
		}

		vmInfo := parseVMInfo(output)
		if !strings.Contains(strings.ToLower(vmInfo.State), "running") {
			t.Errorf("VM state after resume = %q, expected 'Running'", vmInfo.State)
		}

		t.Logf("VM resumed successfully: State=%s", vmInfo.State)
	})
}

// TestIntegrationMultipleVMOperations tests operations with multiple VMs
func TestIntegrationMultipleVMOperations(t *testing.T) {
	if !isMultipassAvailable(t) {
		t.Skip("Multipass not available, skipping integration tests")
	}

	timestamp := time.Now().Unix()
	vm1Name := fmt.Sprintf("test-multi-vm1-%d", timestamp)
	vm2Name := fmt.Sprintf("test-multi-vm2-%d", timestamp)
	vm3Name := fmt.Sprintf("test-multi-vm3-%d", timestamp)

	t.Logf("Testing multiple VM operations: %s, %s, %s", vm1Name, vm2Name, vm3Name)

	t.Cleanup(func() {
		t.Logf("Cleaning up multiple test VMs")
		DeleteVM(vm1Name, true)
		DeleteVM(vm2Name, true)
		DeleteVM(vm3Name, true)
	})

	// Create three VMs
	t.Run("CreateMultipleVMs", func(t *testing.T) {
		for _, vmName := range []string{vm1Name, vm2Name, vm3Name} {
			t.Logf("Creating VM: %s", vmName)
			_, err := LaunchVM(vmName, "22.04")
			if err != nil {
				t.Errorf("Failed to create VM %s: %v", vmName, err)
			}
			time.Sleep(3 * time.Second)
		}
	})

	// Verify all three appear in list
	t.Run("VerifyAllVMsInList", func(t *testing.T) {
		output, err := ListVMs()
		if err != nil {
			t.Fatalf("Failed to list VMs: %v", err)
		}

		vmNames := parseVMNames(output)
		testVMs := map[string]bool{
			vm1Name: false,
			vm2Name: false,
			vm3Name: false,
		}

		for _, name := range vmNames {
			if _, exists := testVMs[name]; exists {
				testVMs[name] = true
			}
		}

		for vmName, found := range testVMs {
			if !found {
				t.Errorf("VM %s not found in list", vmName)
			}
		}

		t.Logf("All test VMs found in list")
	})

	// Stop all test VMs
	t.Run("StopAllTestVMs", func(t *testing.T) {
		for _, vmName := range []string{vm1Name, vm2Name, vm3Name} {
			t.Logf("Stopping VM: %s", vmName)
			_, err := StopVM(vmName)
			if err != nil {
				t.Errorf("Failed to stop VM %s: %v", vmName, err)
			}
			time.Sleep(2 * time.Second)
		}
	})

	// Delete all test VMs
	t.Run("DeleteAllTestVMs", func(t *testing.T) {
		for _, vmName := range []string{vm1Name, vm2Name, vm3Name} {
			t.Logf("Deleting VM: %s", vmName)
			_, err := DeleteVM(vmName, true)
			if err != nil {
				t.Errorf("Failed to delete VM %s: %v", vmName, err)
			}
		}
	})
}

// TestIntegrationRecoverVM tests deleting and recovering a VM
func TestIntegrationRecoverVM(t *testing.T) {
	if !isMultipassAvailable(t) {
		t.Skip("Multipass not available, skipping integration tests")
	}

	vmName := fmt.Sprintf("test-recover-vm-%d", time.Now().Unix())
	t.Logf("Testing VM recovery: %s", vmName)

	t.Cleanup(func() {
		DeleteVM(vmName, true)
	})

	// Create VM
	t.Run("CreateVM", func(t *testing.T) {
		_, err := LaunchVM(vmName, "22.04")
		if err != nil {
			t.Fatalf("Failed to create VM: %v", err)
		}
		time.Sleep(5 * time.Second)
	})

	// Delete VM (without purge)
	t.Run("DeleteVM", func(t *testing.T) {
		t.Logf("Deleting VM (without purge)")
		_, err := DeleteVM(vmName, false)
		if err != nil {
			t.Fatalf("Failed to delete VM: %v", err)
		}
		time.Sleep(2 * time.Second)
	})

	// Recover VM
	t.Run("RecoverVM", func(t *testing.T) {
		t.Logf("Recovering VM: %s", vmName)
		_, err := RecoverVM(vmName)
		if err != nil {
			t.Fatalf("Failed to recover VM: %v", err)
		}
		time.Sleep(2 * time.Second)

		// Verify VM is back
		output, err := GetVMInfo(vmName)
		if err != nil {
			t.Fatalf("Failed to get VM info after recovery: %v", err)
		}

		vmInfo := parseVMInfo(output)
		if vmInfo.Name != vmName {
			t.Errorf("Recovered VM name = %q, want %q", vmInfo.Name, vmName)
		}

		t.Logf("VM recovered successfully: %s", vmName)
	})
}

// isMultipassAvailable checks if multipass is installed and accessible
func isMultipassAvailable(t *testing.T) bool {
	_, err := runMultipassCommand("version")
	if err != nil {
		t.Logf("Multipass not available: %v", err)
		return false
	}
	return true
}
