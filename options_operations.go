// options_operations.go - VM resource configuration management (data logic, no UI)
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// ResourceConfig holds VM resource settings
type ResourceConfig struct {
	CPUs   int
	Memory string // Format: "8.0GiB"
	Disk   string // Format: "40.0GiB"
}

// HostLimits holds maximum resource limits from host
type HostLimits struct {
	MaxCPUs   int
	MaxMemory int // in MB
}

// parseMemoryToMB converts memory string to MB
// Supports: "8.0GiB", "8G", "512.0MiB", "512MB", "512M", "512" (assumes MB)
func parseMemoryToMB(memStr string) (int, error) {
	memStr = strings.TrimSpace(strings.ToUpper(memStr))
	if memStr == "" {
		return 0, fmt.Errorf("memory string is empty")
	}

	var value float64
	var unit string
	var err error

	// Try to parse with different suffixes
	if strings.HasSuffix(memStr, "GIB") {
		valueStr := strings.TrimSuffix(memStr, "GIB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", valueStr)
		}
		unit = "G"
	} else if strings.HasSuffix(memStr, "MIB") {
		valueStr := strings.TrimSuffix(memStr, "MIB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", valueStr)
		}
		unit = "M"
	} else if strings.HasSuffix(memStr, "GB") {
		valueStr := strings.TrimSuffix(memStr, "GB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", valueStr)
		}
		unit = "G"
	} else if strings.HasSuffix(memStr, "G") {
		valueStr := strings.TrimSuffix(memStr, "G")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", valueStr)
		}
		unit = "G"
	} else if strings.HasSuffix(memStr, "MB") {
		valueStr := strings.TrimSuffix(memStr, "MB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", valueStr)
		}
		unit = "M"
	} else if strings.HasSuffix(memStr, "M") {
		valueStr := strings.TrimSuffix(memStr, "M")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", valueStr)
		}
		unit = "M"
	} else {
		// No unit, assume MB
		value, err = strconv.ParseFloat(memStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory value: %s", memStr)
		}
		unit = "M"
	}

	// Convert to MB
	if unit == "G" {
		return int(value * 1024), nil
	}
	return int(value), nil
}

// formatMemoryToMiB formats MB to MiB format for multipass
// Preserves exact MB value without precision loss from GiB conversion
func formatMemoryToMiB(memMB int) string {
	return fmt.Sprintf("%d.0MiB", memMB)
}

// parseDiskToGB converts disk string to GB
// Supports: "40.0GiB", "40G", "40GB", "512.0MiB", "512" (assumes GB if no unit)
func parseDiskToGB(diskStr string) (int, error) {
	diskStr = strings.TrimSpace(strings.ToUpper(diskStr))
	if diskStr == "" {
		return 0, fmt.Errorf("disk string is empty")
	}

	var value float64
	var err error

	// Try to parse with different suffixes
	if strings.HasSuffix(diskStr, "GIB") {
		valueStr := strings.TrimSuffix(diskStr, "GIB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid disk value: %s", valueStr)
		}
	} else if strings.HasSuffix(diskStr, "MIB") {
		valueStr := strings.TrimSuffix(diskStr, "MIB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid disk value: %s", valueStr)
		}
		value = value / 1024 // Convert MiB to GB
	} else if strings.HasSuffix(diskStr, "GB") {
		valueStr := strings.TrimSuffix(diskStr, "GB")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid disk value: %s", valueStr)
		}
	} else if strings.HasSuffix(diskStr, "G") {
		valueStr := strings.TrimSuffix(diskStr, "G")
		value, err = strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid disk value: %s", valueStr)
		}
	} else {
		// No unit, assume GB
		value, err = strconv.ParseFloat(diskStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid disk value: %s", diskStr)
		}
	}

	return int(value), nil
}

// formatDiskToGiB converts GB to GiB format for multipass
func formatDiskToGiB(diskGB int) string {
	return fmt.Sprintf("%d.0GiB", diskGB)
}

// getVMResourceConfig retrieves current resource settings for a VM
func getVMResourceConfig(vmName string) (ResourceConfig, error) {
	cpuOut, err := runMultipassCommand("get", fmt.Sprintf("local.%s.cpus", vmName))
	if err != nil {
		return ResourceConfig{}, fmt.Errorf("failed to get CPU config: %v", err)
	}

	memOut, err := runMultipassCommand("get", fmt.Sprintf("local.%s.memory", vmName))
	if err != nil {
		return ResourceConfig{}, fmt.Errorf("failed to get memory config: %v", err)
	}

	diskOut, err := runMultipassCommand("get", fmt.Sprintf("local.%s.disk", vmName))
	if err != nil {
		return ResourceConfig{}, fmt.Errorf("failed to get disk config: %v", err)
	}

	cpus, err := strconv.Atoi(strings.TrimSpace(cpuOut))
	if err != nil {
		return ResourceConfig{}, fmt.Errorf("invalid CPU value: %v", err)
	}

	return ResourceConfig{
		CPUs:   cpus,
		Memory: strings.TrimSpace(memOut),
		Disk:   strings.TrimSpace(diskOut),
	}, nil
}

// getHostLimits retrieves maximum resource limits from host
func getHostLimits() (HostLimits, error) {
	maxCPUs := runtime.NumCPU()

	// Get total memory using "free -m"
	output, err := exec.Command("free", "-m").Output()
	if err != nil {
		// Fallback to reasonable defaults if free command fails
		if appLogger != nil {
			appLogger.Printf("Warning: failed to get memory info: %v, using defaults", err)
		}
		return HostLimits{
			MaxCPUs:   maxCPUs,
			MaxMemory: 32768, // 32GB fallback
		}, nil
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return HostLimits{}, fmt.Errorf("unexpected free output format")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return HostLimits{}, fmt.Errorf("unexpected free output fields")
	}

	totalMemMB, err := strconv.Atoi(fields[1])
	if err != nil {
		return HostLimits{}, fmt.Errorf("invalid memory value: %v", err)
	}

	// Use 75% of total memory as maximum to leave headroom
	maxMemMB := int(float64(totalMemMB) * 0.75)

	return HostLimits{
		MaxCPUs:   maxCPUs,
		MaxMemory: maxMemMB,
	}, nil
}

// setVMResourceConfig applies new resource settings to a VM
func setVMResourceConfig(vmName string, config ResourceConfig) error {
	if appLogger != nil {
		appLogger.Printf("Setting resources for %s: CPUs=%d, Memory=%s, Disk=%s",
			vmName, config.CPUs, config.Memory, config.Disk)
	}

	// Set CPU
	_, err := runMultipassCommand("set", fmt.Sprintf("local.%s.cpus=%d", vmName, config.CPUs))
	if err != nil {
		return fmt.Errorf("failed to set CPUs: %v", err)
	}

	// Set Memory
	_, err = runMultipassCommand("set", fmt.Sprintf("local.%s.memory=%s", vmName, config.Memory))
	if err != nil {
		return fmt.Errorf("failed to set memory: %v", err)
	}

	// Set Disk
	_, err = runMultipassCommand("set", fmt.Sprintf("local.%s.disk=%s", vmName, config.Disk))
	if err != nil {
		return fmt.Errorf("failed to set disk: %v", err)
	}

	if appLogger != nil {
		appLogger.Printf("Successfully updated resources for %s", vmName)
	}

	return nil
}
