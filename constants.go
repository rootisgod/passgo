// constants.go - Application-wide constants and configuration values
package main

// VM Configuration Defaults
const (
	// DefaultUbuntuRelease is the default Ubuntu version for new VMs
	DefaultUbuntuRelease = "24.04"

	// DefaultReleaseIndex is the dropdown index for the default release
	DefaultReleaseIndex = 3

	// DefaultCPUCores is the default number of CPU cores for new VMs
	DefaultCPUCores = 2

	// DefaultRAMMB is the default RAM allocation in megabytes
	DefaultRAMMB = 1024

	// DefaultDiskGB is the default disk size in gigabytes
	DefaultDiskGB = 8
)

// VM Resource Limits
const (
	// MinCPUCores is the minimum number of CPU cores allowed
	MinCPUCores = 1

	// MinRAMMB is the minimum RAM allocation in megabytes
	MinRAMMB = 512

	// MinDiskGB is the minimum disk size in gigabytes
	MinDiskGB = 1
)

// VM Naming Configuration
const (
	// VMNamePrefix is the prefix used for auto-generated VM names
	VMNamePrefix = "VM-"

	// VMNameRandomLength is the length of the random suffix for VM names
	VMNameRandomLength = 4
)

// UbuntuReleases is the list of available Ubuntu releases for VM creation
var UbuntuReleases = []string{
	"22.04",
	"20.04",
	"18.04",
	"24.04",
	"daily",
}
