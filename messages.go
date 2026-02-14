// messages.go - Custom tea.Msg types and tea.Cmd factories for async operations
package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Result Messages ───────────────────────────────────────────────────────────

// vmListResultMsg carries the result of fetching the VM list.
type vmListResultMsg struct {
	vms []vmData
	err error
}

// vmOperationResultMsg carries the result of a single VM operation.
type vmOperationResultMsg struct {
	vmName    string
	operation string
	err       error
}

// vmInfoResultMsg carries raw info output for a single VM.
type vmInfoResultMsg struct {
	vmName string
	info   string
	err    error
}

// snapshotListResultMsg carries parsed snapshots for a VM.
type snapshotListResultMsg struct {
	vmName    string
	snapshots []SnapshotInfo
	err       error
}

// mountListResultMsg carries parsed mounts for a VM.
type mountListResultMsg struct {
	vmName string
	mounts []MountInfo
	err    error
}

// shellFinishedMsg is sent when an interactive shell exits.
type shellFinishedMsg struct{ err error }

// confirmResultMsg carries the user's confirm/deny choice.
type confirmResultMsg struct{ confirmed bool }

// backToTableMsg tells the root model to return to the main table view.
type backToTableMsg struct{}

// ─── Command Factories ─────────────────────────────────────────────────────────

// fetchVMListCmd fetches the full VM list with details.
func fetchVMListCmd() tea.Cmd {
	return func() tea.Msg {
		listOutput, err := ListVMs()
		if err != nil {
			return vmListResultMsg{err: err}
		}

		lines := strings.Split(listOutput, "\n")
		var vmNames []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				vmNames = append(vmNames, fields[0])
			}
		}

		var vms []vmData
		for _, name := range vmNames {
			info, err := GetVMInfo(name)
			if err != nil {
				vms = append(vms, vmData{info: VMInfo{Name: name, State: "Error"}, err: err})
			} else {
				vms = append(vms, vmData{info: parseVMInfo(info), err: nil})
			}
		}
		return vmListResultMsg{vms: vms}
	}
}

// fetchVMInfoCmd fetches raw info for a single VM.
func fetchVMInfoCmd(vmName string) tea.Cmd {
	return func() tea.Msg {
		info, err := GetVMInfo(vmName)
		return vmInfoResultMsg{vmName: vmName, info: info, err: err}
	}
}

// stopVMCmd stops a VM.
func stopVMCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := StopVM(name)
		return vmOperationResultMsg{vmName: name, operation: "stop", err: err}
	}
}

// startVMCmd starts a VM.
func startVMCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := StartVM(name)
		return vmOperationResultMsg{vmName: name, operation: "start", err: err}
	}
}

// suspendVMCmd suspends a VM.
func suspendVMCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := runMultipassCommand("suspend", name)
		return vmOperationResultMsg{vmName: name, operation: "suspend", err: err}
	}
}

// deleteVMCmd deletes a VM (with purge).
func deleteVMCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := DeleteVM(name, true)
		return vmOperationResultMsg{vmName: name, operation: "delete", err: err}
	}
}

// recoverVMCmd recovers a deleted VM.
func recoverVMCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := RecoverVM(name)
		return vmOperationResultMsg{vmName: name, operation: "recover", err: err}
	}
}

// quickCreateCmd creates a VM with default settings.
func quickCreateCmd() tea.Cmd {
	return func() tea.Msg {
		name := VMNamePrefix + randomString(VMNameRandomLength)
		_, err := LaunchVM(name, DefaultUbuntuRelease)
		return vmOperationResultMsg{vmName: name, operation: "create", err: err}
	}
}

// advancedCreateCmd creates a VM with custom settings.
func advancedCreateCmd(name, release string, cpus, memoryMB, diskGB int, cloudInitFile string) tea.Cmd {
	return func() tea.Msg {
		var err error
		if cloudInitFile == "" {
			_, err = LaunchVMAdvanced(name, release, cpus, memoryMB, diskGB)
		} else {
			_, err = LaunchVMWithCloudInit(name, release, cpus, memoryMB, diskGB, cloudInitFile)
		}
		return vmOperationResultMsg{vmName: name, operation: "create", err: err}
	}
}

// stopAllVMsCmd stops all running VMs.
func stopAllVMsCmd(names []string) tea.Cmd {
	return func() tea.Msg {
		for _, name := range names {
			StopVM(name)
		}
		return vmOperationResultMsg{operation: "stop-all"}
	}
}

// startAllVMsCmd starts all stopped VMs.
func startAllVMsCmd(names []string) tea.Cmd {
	return func() tea.Msg {
		for _, name := range names {
			StartVM(name)
		}
		return vmOperationResultMsg{operation: "start-all"}
	}
}

// purgeAllVMsCmd purges all deleted VMs.
func purgeAllVMsCmd() tea.Cmd {
	return func() tea.Msg {
		_, err := runMultipassCommand("purge")
		return vmOperationResultMsg{operation: "purge", err: err}
	}
}

// fetchSnapshotsCmd fetches snapshots for a VM.
func fetchSnapshotsCmd(vmName string) tea.Cmd {
	return func() tea.Msg {
		output, err := ListSnapshots()
		if err != nil {
			return snapshotListResultMsg{vmName: vmName, err: err}
		}
		all := parseSnapshots(output)
		var filtered []SnapshotInfo
		for _, s := range all {
			if s.Instance == vmName {
				filtered = append(filtered, s)
			}
		}
		return snapshotListResultMsg{vmName: vmName, snapshots: filtered}
	}
}

// createSnapshotCmd creates a snapshot.
func createSnapshotCmd(vmName, snapName, comment string) tea.Cmd {
	return func() tea.Msg {
		_, err := CreateSnapshot(vmName, snapName, comment)
		return vmOperationResultMsg{vmName: vmName, operation: "snapshot", err: err}
	}
}

// restoreSnapshotCmd restores a snapshot.
func restoreSnapshotCmd(vmName, snapName string) tea.Cmd {
	return func() tea.Msg {
		_, err := RestoreSnapshot(vmName, snapName)
		return vmOperationResultMsg{vmName: vmName, operation: "restore", err: err}
	}
}

// deleteSnapshotCmd deletes a snapshot.
func deleteSnapshotCmd(vmName, snapName string) tea.Cmd {
	return func() tea.Msg {
		_, err := DeleteSnapshot(vmName, snapName)
		return vmOperationResultMsg{vmName: vmName, operation: "delete-snapshot", err: err}
	}
}

// fetchMountsCmd fetches mounts for a VM.
func fetchMountsCmd(vmName string) tea.Cmd {
	return func() tea.Msg {
		mounts, err := getVMMounts(vmName)
		return mountListResultMsg{vmName: vmName, mounts: mounts, err: err}
	}
}

// mountCmd mounts a local directory to a VM.
func mountCmd(source, vmName, target string) tea.Cmd {
	return func() tea.Msg {
		_, err := runMultipassCommand("mount", source, vmName+":"+target)
		return vmOperationResultMsg{vmName: vmName, operation: "mount", err: err}
	}
}

// umountCmd unmounts a directory from a VM.
func umountCmd(vmName, target string) tea.Cmd {
	return func() tea.Msg {
		_, err := runMultipassCommand("umount", vmName+":"+target)
		return vmOperationResultMsg{vmName: vmName, operation: "umount", err: err}
	}
}
