// vm_operations.go - VM lifecycle helpers (no UI code, just data logic)
package main

// isVMStopped checks if the VM at the given index is in "Stopped" state.
func isVMStoppedByData(vms []vmData, idx int) bool {
	if idx >= 0 && idx < len(vms) {
		return vms[idx].info.State == "Stopped"
	}
	return false
}

// isVMRunningByData checks if the VM at the given index is in "Running" state.
func isVMRunningByData(vms []vmData, idx int) bool {
	if idx >= 0 && idx < len(vms) {
		return vms[idx].info.State == "Running"
	}
	return false
}
