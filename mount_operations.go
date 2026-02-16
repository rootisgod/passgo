// mount_operations.go - Mount data helpers (JSON parsing, no UI code)
package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

// MountInfo represents a mount point between local filesystem and VM.
type MountInfo struct {
	SourcePath string
	TargetPath string
	UIDMaps    []string
	GIDMaps    []string
}

// ─── JSON parsing types for multipass info --format json ───

type multipassInfoResponse struct {
	Errors []string                         `json:"errors"`
	Info   map[string]multipassVMInfoDetail `json:"info"`
}

type multipassVMInfoDetail struct {
	Mounts map[string]multipassMountDetail `json:"mounts"`
}

type multipassMountDetail struct {
	SourcePath  string   `json:"source_path"`
	GIDMappings []string `json:"gid_mappings"`
	UIDMappings []string `json:"uid_mappings"`
}

// getVMMounts retrieves the current mounts for a VM using JSON output.
func getVMMounts(vmName string) ([]MountInfo, error) {
	output, err := runMultipassCommand("info", vmName, "--format", "json")
	if err != nil {
		return nil, err
	}

	var response multipassInfoResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, fmt.Errorf("failed to parse VM info JSON: %v", err)
	}

	vmDetail, ok := response.Info[vmName]
	if !ok {
		return nil, fmt.Errorf("VM '%s' not found in info response", vmName)
	}

	var mounts []MountInfo
	for targetPath, detail := range vmDetail.Mounts {
		mounts = append(mounts, MountInfo{
			SourcePath: detail.SourcePath,
			TargetPath: targetPath,
			UIDMaps:    detail.UIDMappings,
			GIDMaps:    detail.GIDMappings,
		})
	}

	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].TargetPath < mounts[j].TargetPath
	})

	return mounts, nil
}
