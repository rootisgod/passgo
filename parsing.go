// parsing.go - Data structures and parsing functions for VM and snapshot information
package main

import "strings"

// VMInfo represents information about a virtual machine
type VMInfo struct {
	Name        string
	State       string
	Snapshots   string
	IPv4        string
	Release     string
	CPUs        string
	Load        string
	DiskUsage   string
	MemoryUsage string
	Mounts      string
}

// SnapshotInfo represents a snapshot
type SnapshotInfo struct {
	Instance string
	Name     string
	Parent   string
	Comment  string
}

// parseVMInfo parses VM info output from multipass info command
func parseVMInfo(info string) VMInfo {
	vm := VMInfo{}
	lines := strings.Split(info, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)

			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "Name":
					vm.Name = value
				case "State":
					vm.State = value
				case "Snapshots":
					vm.Snapshots = value
				case "IPv4":
					vm.IPv4 = value
				case "Release":
					vm.Release = value
				case "CPU(s)":
					vm.CPUs = value
				case "Load":
					vm.Load = value
				case "Disk usage":
					vm.DiskUsage = value
				case "Memory usage":
					vm.MemoryUsage = value
				case "Mounts":
					vm.Mounts = value
				}
			}
		}
	}

	return vm
}

// parseVMNames extracts VM names from multipass list output
func parseVMNames(listOutput string) []string {
	lines := strings.Split(listOutput, "\n")
	vmNames := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
			continue // Skip header and separator lines
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			vmNames = append(vmNames, fields[0])
		}
	}
	return vmNames
}

// parseSnapshots parses the output from multipass list --snapshots
func parseSnapshots(output string) []SnapshotInfo {
	var snapshots []SnapshotInfo
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header line and empty lines
		if i == 0 || line == "" || strings.Contains(line, "Instance") {
			continue
		}

		// Split by whitespace and take first 4 fields
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			snapshot := SnapshotInfo{
				Instance: fields[0],
				Name:     fields[1],
			}

			if len(fields) >= 3 && fields[2] != "--" {
				snapshot.Parent = fields[2]
			}

			if len(fields) >= 4 && fields[3] != "--" {
				snapshot.Comment = fields[3]
			}

			snapshots = append(snapshots, snapshot)
		}
	}

	return snapshots
}
