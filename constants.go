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

// LLM Configuration Defaults
const (
	// DefaultLLMBaseURL is the default API endpoint
	DefaultLLMBaseURL = "https://openrouter.ai/api/v1"

	// DefaultLLMModel is the default model to use
	DefaultLLMModel = "deepseek/deepseek-v3.2"

	// MaxAgentIterations limits the agent loop to prevent infinite tool calls
	MaxAgentIterations = 20
)

// LLMSystemPrompt is the base system prompt sent to the LLM.
// The actual prompt is built dynamically by buildSystemPrompt() which appends current VM state.
const LLMSystemPrompt = `You are an AI assistant managing Multipass virtual machines.

CURRENT VM STATE is provided below in the system context — use it to answer questions about instances, counts, status, IPs, etc. You do NOT need to call any tools to answer informational questions.

CRITICAL RULES:
- ONLY perform the exact action the user requests. Nothing more.
- NEVER create, launch, start, stop, or delete VMs unless the user explicitly asks.
- For informational questions (e.g. "how many VMs?", "what's running?"), answer from the VM STATE below. Do NOT call any tools.
- Only use tools for actions that change state (launch, start, stop, delete, exec_command, etc.).
- When you do perform operations, confirm what you did in your final response.
- Keep responses concise.`

// UbuntuReleases is the list of available Ubuntu releases for VM creation
var UbuntuReleases = []string{
	"22.04",
	"20.04",
	"18.04",
	"24.04",
	"daily",
}
