// main.go - Root model, view routing, and program setup for bubbletea TUI
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// appLogger is a file-backed logger shared across the app.
var appLogger *log.Logger

// vmData holds VM information and any errors from fetching it.
type vmData struct {
	info VMInfo
	err  error
}

// ─── View State ────────────────────────────────────────────────────────────────

type viewState int

const (
	viewTable viewState = iota
	viewHelp
	viewVersion
	viewInfo
	viewLoading
	viewError
	viewConfirm
	viewAdvCreate
	viewSnapCreate
	viewSnapManage
	viewMountManage
	viewMountAdd
	viewMountModify
	viewLLMSettings
)

// ─── Root Model ────────────────────────────────────────────────────────────────

type rootModel struct {
	currentView viewState
	width       int
	height      int

	// Child models
	table       tableModel
	help        helpModel
	version     versionModel
	info        infoModel
	loading     loadingModel
	errModal    errorModel
	confirm     confirmModel
	advCreate   advCreateModel
	snapCreate  snapCreateModel
	snapManage  snapManageModel
	mountManage mountManageModel
	mountAdd    mountAddModel
	mountModify mountModifyModel
	llmSettings llmSettingsModel

	// Chat panel
	chat      chatModel
	chatOpen  bool
	chatFocus bool // true = chat has focus, false = table has focus

	// Pending operation for confirm dialogs
	pendingCmd tea.Cmd

	// Context for returning to sub-views after operations
	lastMountVM string
	lastSnapVM  string

	// VM list fetch coordination (prevents overlapping fetch commands).
	vmListFetchInFlight     bool
	vmListFetchPending      bool
	vmListPendingBackground bool

	// Program reference for p.Send() in agent loop
	program *tea.Program
}

// setChildSizes stamps the current terminal dimensions onto every child model.
// Call this after creating or replacing any child model.
func (m *rootModel) setChildSizes() {
	m.table.width = m.width
	m.table.height = m.height
	m.loading.width = m.width
	m.loading.height = m.height
	m.help.width = m.width
	m.help.height = m.height
	m.version.width = m.width
	m.version.height = m.height
	m.info.width = m.width
	m.info.height = m.height
	m.errModal.width = m.width
	m.errModal.height = m.height
	m.confirm.width = m.width
	m.confirm.height = m.height
	m.advCreate.width = m.width
	m.advCreate.height = m.height
	m.snapCreate.width = m.width
	m.snapCreate.height = m.height
	m.snapManage.width = m.width
	m.snapManage.height = m.height
	m.mountManage.width = m.width
	m.mountManage.height = m.height
	m.mountAdd.width = m.width
	m.mountAdd.height = m.height
	m.mountModify.width = m.width
	m.mountModify.height = m.height
	m.llmSettings.width = m.width
	m.llmSettings.height = m.height

	// Chat panel gets 40% width when open
	if m.chatOpen {
		chatWidth := m.width * 40 / 100
		m.chat.setSize(chatWidth, m.height)
	}
}

func initialModel() rootModel {
	chat := newChatModel()

	// Load LLM config
	cfg, err := loadLLMConfig()
	if err != nil {
		if appLogger != nil {
			appLogger.Printf("failed to load LLM config: %v", err)
		}
		cfg = defaultLLMConfig()
	}
	chat.config = cfg
	chat.llmClient = NewLLMClient(cfg)

	return rootModel{
		currentView: viewLoading,
		table:       newTableModel(),
		loading:     newLoadingModel("Loading VMs…"),
		chat:        chat,
		// Init schedules fetchVMListCmd immediately.
		vmListFetchInFlight: true,
	}
}

func (m *rootModel) requestVMListFetch(background bool) tea.Cmd {
	if m.vmListFetchInFlight {
		if !m.vmListFetchPending {
			m.vmListFetchPending = true
			m.vmListPendingBackground = background
		} else if !background {
			// Foreground refresh takes priority over pending background refresh.
			m.vmListPendingBackground = false
		}
		return nil
	}

	m.vmListFetchInFlight = true
	if background {
		return fetchVMListBackgroundCmd()
	}
	return fetchVMListCmd()
}

func (m *rootModel) dequeuePendingVMListFetch() tea.Cmd {
	if !m.vmListFetchPending {
		return nil
	}
	background := m.vmListPendingBackground
	m.vmListFetchPending = false
	m.vmListPendingBackground = false

	if background && m.currentView != viewTable {
		return nil
	}
	return m.requestVMListFetch(background)
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.loading.Init(),
		m.table.spinner.Tick,
		fetchVMListCmd(),
		autoRefreshTickCmd(),
	)
}

// programReadyMsg carries the *tea.Program reference.
type programReadyMsg struct{ program *tea.Program }

// ─── Update ────────────────────────────────────────────────────────────────────

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case programReadyMsg:
		m.program = msg.program
		m.chat.program = msg.program
		return m, nil

	// ── Window resize ──
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.setChildSizes()
		return m, nil

	// ── Key messages ──
	case tea.KeyMsg:
		return m.handleKey(msg)

	// ── Mouse messages ──
	case tea.MouseMsg:
		if m.currentView == viewTable {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
		if m.currentView == viewInfo {
			var cmd tea.Cmd
			m.info, cmd = m.info.Update(msg)
			return m, cmd
		}

	// ── Info refresh tick ──
	case infoRefreshTickMsg:
		if m.currentView == viewInfo {
			var cmd tea.Cmd
			m.info, cmd = m.info.Update(msg)
			return m, cmd
		}
		return m, nil // discard tick if no longer on info view

	// ── Auto-refresh tick ──
	case autoRefreshTickMsg:
		// Only auto-refresh when we're on the table view
		cmds := []tea.Cmd{autoRefreshTickCmd()} // always reschedule
		if m.currentView == viewTable {
			if cmd := m.requestVMListFetch(true); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	// ── Async results ──
	case vmListResultMsg:
		m.vmListFetchInFlight = false

		if msg.err != nil {
			if !msg.background {
				m.errModal = newErrorModel("VM List Error", msg.err.Error())
				m.setChildSizes()
				m.currentView = viewError
			}
		} else {
			m.table.setVMs(msg.vms)
			m.table.lastRefresh = time.Now()
			m.chat.currentVMs = msg.vms // keep chat VM state in sync
			if !msg.background {
				m.currentView = viewTable
			}
		}
		return m, m.dequeuePendingVMListFetch()

	case vmInfoResultMsg:
		if m.currentView == viewInfo {
			// Delegate to info model for live chart updates
			var cmd tea.Cmd
			m.info, cmd = m.info.Update(msg)
			return m, cmd
		}
		if msg.err != nil {
			m.errModal = newErrorModel("Info Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.info.setContent(msg.info)
		}
		return m, nil

	case vmOperationResultMsg:
		// Capture timing before clearing busy state
		var elapsed time.Duration
		if busy, ok := m.table.busyVMs[msg.vmName]; ok {
			elapsed = time.Since(busy.startTime)
		}
		delete(m.table.busyVMs, msg.vmName)

		if msg.err != nil {
			// Toast the error too
			toastCmd := m.table.addToast(
				fmt.Sprintf("✗ %s failed: %s", msg.operation, msg.err.Error()), "error")
			if msg.inline {
				if refreshCmd := m.requestVMListFetch(true); refreshCmd != nil {
					return m, tea.Batch(toastCmd, refreshCmd)
				}
				return m, toastCmd
			}
			m.errModal = newErrorModel("Operation Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
			return m, toastCmd
		}

		// Build toast message
		toastMsg := operationToastMessage(msg.vmName, msg.operation, elapsed)
		toastCmd := m.table.addToast(toastMsg, "success")

		// Inline operations: stay on table, refresh in background
		if msg.inline {
			if refreshCmd := m.requestVMListFetch(true); refreshCmd != nil {
				return m, tea.Batch(toastCmd, refreshCmd)
			}
			return m, toastCmd
		}

		// Return to mount/snap manager if that's where we came from
		if m.lastMountVM != "" && (msg.operation == "mount" || msg.operation == "umount") {
			vmName := m.lastMountVM
			m.loading = newLoadingModel("Refreshing mounts…")
			m.setChildSizes()
			m.currentView = viewLoading
			return m, tea.Batch(m.loading.Init(), fetchMountsCmd(vmName), toastCmd)
		}
		if m.lastSnapVM != "" && (msg.operation == "snapshot" || msg.operation == "delete-snapshot" || msg.operation == "restore") {
			vmName := m.lastSnapVM
			m.loading = newLoadingModel("Refreshing snapshots…")
			m.setChildSizes()
			m.currentView = viewLoading
			return m, tea.Batch(m.loading.Init(), fetchSnapshotsCmd(vmName), toastCmd)
		}
		m.loading = newLoadingModel("Refreshing…")
		m.setChildSizes()
		m.currentView = viewLoading
		if refreshCmd := m.requestVMListFetch(false); refreshCmd != nil {
			return m, tea.Batch(m.loading.Init(), refreshCmd, toastCmd)
		}
		return m, tea.Batch(m.loading.Init(), toastCmd)

	case snapshotListResultMsg:
		if msg.err != nil {
			m.errModal = newErrorModel("Snapshot Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.snapManage = newSnapManageModel(msg.vmName, m.width, m.height)
			m.snapManage.setSnapshots(msg.snapshots)
			m.currentView = viewSnapManage
		}
		return m, nil

	case mountListResultMsg:
		if msg.err != nil {
			m.errModal = newErrorModel("Mount Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.mountManage = newMountManageModel(msg.vmName, m.width, m.height)
			m.mountManage.mounts = msg.mounts
			m.currentView = viewMountManage
		}
		return m, nil

	case shellFinishedMsg:
		m.loading = newLoadingModel("Refreshing…")
		m.setChildSizes()
		m.currentView = viewLoading
		if refreshCmd := m.requestVMListFetch(false); refreshCmd != nil {
			return m, tea.Batch(m.loading.Init(), refreshCmd)
		}
		return m, m.loading.Init()

	case confirmResultMsg:
		if msg.confirmed && m.pendingCmd != nil {
			cmd := m.pendingCmd
			m.pendingCmd = nil
			m.loading = newLoadingModel("Processing…")
			m.setChildSizes()
			m.currentView = viewLoading
			return m, tea.Batch(m.loading.Init(), cmd)
		}
		m.pendingCmd = nil
		m.currentView = viewTable
		return m, nil

	case backToTableMsg:
		m.lastMountVM = ""
		m.lastSnapVM = ""
		m.currentView = viewTable
		return m, nil

	case advCreateMsg:
		// Return to table with placeholder row and busy animation
		placeholder := vmData{info: VMInfo{Name: msg.name, State: "Creating"}}
		m.table.vms = append(m.table.vms, placeholder)
		m.table.applyFilterAndSort()
		for i, vm := range m.table.filteredVMs {
			if vm.info.Name == msg.name {
				m.table.cursor = i
				visible := m.table.visibleRows()
				if m.table.cursor >= m.table.offset+visible {
					m.table.offset = m.table.cursor - visible + 1
				}
				break
			}
		}
		m.table.busyVMs[msg.name] = busyInfo{operation: "Creating", startTime: time.Now()}
		m.currentView = viewTable
		return m, advancedCreateCmd(msg.name, msg.release, msg.cpus, msg.memoryMB, msg.diskGB, msg.cloudInitFile, msg.networkName)

	case mountAddRequestMsg:
		m.mountAdd = newMountAddModel(msg.vmName, m.width, m.height)
		m.currentView = viewMountAdd
		return m, nil

	case mountModifyRequestMsg:
		m.mountModify = newMountModifyModel(msg.vmName, msg.mount, m.width, m.height)
		m.currentView = viewMountModify
		return m, m.mountModify.Init()

	case llmSettingsSavedMsg:
		// Update chat model with new config
		m.chat.config = msg.config
		m.chat.llmClient = NewLLMClient(msg.config)
		// Reset MCP state so it re-initializes with new config
		m.chat.mcpReady = false
		m.chat.mcpInitFailed = false
		m.chat.entries = append(m.chat.entries, chatEntry{
			role:    "system",
			content: "Settings saved. LLM config updated.",
		})
		m.chat.refreshViewport()
		m.currentView = viewTable
		return m, nil

	case mountModifySubmitMsg:
		m.loading = newLoadingModel("Updating mount…")
		m.setChildSizes()
		m.currentView = viewLoading
		return m, tea.Batch(m.loading.Init(), func() tea.Msg {
			err := runMountModifyOperation(runMultipassCommand, msg.vmName, msg.oldTarget, msg.newSource, msg.newTarget)
			return vmOperationResultMsg{vmName: msg.vmName, operation: "mount", err: err}
		})
	}

	// ── Chat messages (always route to chat model) ──
	switch msg.(type) {
	case chatToolStartMsg, chatToolDoneMsg, chatAgentResultMsg, chatMCPReadyMsg, chatMCPInitDoneMsg, chatMCPDownloadProgressMsg:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		return m, cmd
	}

	// ── Toast expiry (always route to table regardless of view) ──
	if expire, ok := msg.(toastExpireMsg); ok {
		m.table, _ = m.table.Update(expire)
		return m, nil
	}

	// ── Forward spinner ticks to chat when thinking ──
	if m.chatOpen && m.chat.thinking {
		if _, ok := msg.(spinner.TickMsg); ok {
			var chatCmd tea.Cmd
			m.chat, chatCmd = m.chat.Update(msg)
			// Also forward to table for its spinners
			if m.currentView == viewTable {
				var tableCmd tea.Cmd
				m.table, tableCmd = m.table.Update(msg)
				return m, tea.Batch(chatCmd, tableCmd)
			}
			return m, chatCmd
		}
	}

	// ── Delegate to active view for non-key messages ──
	switch m.currentView {
	case viewTable:
		// Forward spinner ticks to the table for inline busy indicators
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case viewLoading:
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		return m, cmd
	case viewInfo:
		var cmd tea.Cmd
		m.info, cmd = m.info.Update(msg)
		return m, cmd
	case viewAdvCreate:
		var cmd tea.Cmd
		m.advCreate, cmd = m.advCreate.Update(msg)
		return m, cmd
	case viewSnapCreate:
		var cmd tea.Cmd
		m.snapCreate, cmd = m.snapCreate.Update(msg)
		return m, cmd
	case viewMountAdd:
		var cmd tea.Cmd
		m.mountAdd, cmd = m.mountAdd.Update(msg)
		return m, cmd
	case viewMountModify:
		var cmd tea.Cmd
		m.mountModify, cmd = m.mountModify.Update(msg)
		return m, cmd
	case viewLLMSettings:
		var cmd tea.Cmd
		m.llmSettings, cmd = m.llmSettings.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ─── Key Handling ──────────────────────────────────────────────────────────────

func (m rootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global: ? toggles chat panel (except when typing in chat input or filter)
	if msg.String() == "?" && m.currentView == viewTable && !m.chatFocus && !m.table.filterFocused {
		m.chatOpen = !m.chatOpen
		if m.chatOpen {
			m.chatFocus = true
			m.chat.Focus()
			m.chat.currentVMs = m.table.vms // sync current VM state
			chatWidth := m.width * 40 / 100
			m.chat.setSize(chatWidth, m.height)
			// Create default config file if it doesn't exist
			if path, err := llmConfigPath(); err == nil {
				if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
					_ = saveLLMConfig(m.chat.config)
				}
			}
		} else {
			m.chatFocus = false
			m.chat.Blur()
		}
		return m, nil
	}

	// Tab switches focus between table and chat when chat is open
	if msg.String() == "tab" && m.chatOpen && m.currentView == viewTable && !m.table.filterFocused {
		m.chatFocus = !m.chatFocus
		if m.chatFocus {
			m.chat.Focus()
		} else {
			m.chat.Blur()
		}
		return m, nil
	}

	// When chat has focus, forward keys to chat (except ? to toggle off)
	if m.chatOpen && m.chatFocus && m.currentView == viewTable {
		if msg.String() == "esc" {
			// Esc in chat: unfocus chat, return focus to table
			m.chatFocus = false
			m.chat.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		return m, cmd
	}

	switch m.currentView {

	// ── Main table ──
	case viewTable:
		if m.table.filterFocused {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			// Cleanup MCP on quit
			if m.chat.mcpClient != nil {
				m.chat.mcpClient.Close()
			}
			return m, tea.Quit
		case "esc":
			if m.table.filterVisible {
				m.table.filterText = ""
				m.table.filterInput.SetValue("")
				m.table.filterVisible = false
				m.table.applyFilterAndSort()
				return m, nil
			}
			if m.chatOpen {
				m.chatOpen = false
				m.chatFocus = false
				m.chat.Blur()
				return m, nil
			}
			return m, tea.Quit
		case "h":
			m.help = newHelpModel()
			m.setChildSizes()
			m.currentView = viewHelp
			return m, nil
		case "v":
			m.version = newVersionModel()
			m.setChildSizes()
			m.currentView = viewVersion
			return m, nil
		case "i":
			if vm, ok := m.table.selectedVM(); ok {
				m.info = newInfoModel(vm.Name, m.width, m.height)
				m.currentView = viewInfo
				return m, tea.Batch(fetchVMInfoCmd(vm.Name), infoRefreshTickCmd())
			}
		case "c":
			name := VMNamePrefix + randomString(VMNameRandomLength)
			// Add placeholder row and busy animation
			placeholder := vmData{info: VMInfo{Name: name, State: "Creating"}}
			m.table.vms = append(m.table.vms, placeholder)
			m.table.applyFilterAndSort()
			// Move cursor to the new row
			for i, vm := range m.table.filteredVMs {
				if vm.info.Name == name {
					m.table.cursor = i
					visible := m.table.visibleRows()
					if m.table.cursor >= m.table.offset+visible {
						m.table.offset = m.table.cursor - visible + 1
					}
					break
				}
			}
			m.table.busyVMs[name] = busyInfo{operation: "Creating", startTime: time.Now()}
			return m, quickCreateCmd(name)
		case "C":
			m.advCreate = newAdvCreateModel(m.width, m.height)
			m.currentView = viewAdvCreate
			return m, m.advCreate.Init()
		case "[":
			if vm, ok := m.table.selectedVM(); ok {
				m.table.busyVMs[vm.Name] = busyInfo{operation: "Stopping", startTime: time.Now()}
				return m, stopVMCmd(vm.Name)
			}
		case "]":
			if vm, ok := m.table.selectedVM(); ok {
				m.table.busyVMs[vm.Name] = busyInfo{operation: "Starting", startTime: time.Now()}
				return m, startVMCmd(vm.Name)
			}
		case "p":
			if vm, ok := m.table.selectedVM(); ok {
				m.table.busyVMs[vm.Name] = busyInfo{operation: "Suspending", startTime: time.Now()}
				return m, suspendVMCmd(vm.Name)
			}
		case "<":
			names := m.table.allVMNames()
			if len(names) > 0 {
				m.confirm = newConfirmModel("Stop ALL VMs?")
				m.setChildSizes()
				m.pendingCmd = stopAllVMsCmd(names)
				m.currentView = viewConfirm
			}
			return m, nil
		case ">":
			names := m.table.allVMNames()
			if len(names) > 0 {
				m.confirm = newConfirmModel("Start ALL VMs?")
				m.setChildSizes()
				m.pendingCmd = startAllVMsCmd(names)
				m.currentView = viewConfirm
			}
			return m, nil
		case "d":
			if vm, ok := m.table.selectedVM(); ok {
				m.confirm = newConfirmModel(fmt.Sprintf("Delete VM '%s'? This will purge it.", vm.Name))
				m.setChildSizes()
				m.pendingCmd = deleteVMCmd(vm.Name)
				m.currentView = viewConfirm
			}
			return m, nil
		case "r":
			if vm, ok := m.table.selectedVM(); ok {
				m.table.busyVMs[vm.Name] = busyInfo{operation: "Recovering", startTime: time.Now()}
				return m, recoverVMCmd(vm.Name)
			}
		case "!":
			m.confirm = newConfirmModel("PURGE ALL deleted VMs? This cannot be undone.")
			m.setChildSizes()
			m.pendingCmd = purgeAllVMsCmd()
			m.currentView = viewConfirm
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
			idx := int(msg.String()[0] - '1') // '1'→0, '2'→1, ...
			if msg.String() == "0" {
				idx = 9
			}
			setTheme(idx)
			return m, nil
		case "/":
			m.loading = newLoadingModel("Refreshing…")
			m.setChildSizes()
			m.currentView = viewLoading
			if refreshCmd := m.requestVMListFetch(false); refreshCmd != nil {
				return m, tea.Batch(m.loading.Init(), refreshCmd)
			}
			return m, m.loading.Init()
		case "f":
			m.table.toggleFilter()
			return m, nil
		case "s":
			if vm, ok := m.table.selectedVM(); ok {
				c := exec.Command("multipass", "shell", vm.Name) // #nosec G204 -- VM name from table selection
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return shellFinishedMsg{err: err}
				})
			}
		case "n":
			if vm, ok := m.table.selectedVM(); ok {
				if vm.State == "Stopped" {
					m.lastSnapVM = vm.Name
					m.snapCreate = newSnapCreateModel(vm.Name, m.width, m.height)
					m.currentView = viewSnapCreate
					return m, m.snapCreate.Init()
				}
				m.errModal = newErrorModel("Snapshot Error", fmt.Sprintf("VM '%s' must be stopped to create a snapshot.", vm.Name))
				m.setChildSizes()
				m.currentView = viewError
			}
			return m, nil
		case "m":
			if vm, ok := m.table.selectedVM(); ok {
				m.lastSnapVM = vm.Name
				m.loading = newLoadingModel("Loading snapshots…")
				m.setChildSizes()
				m.currentView = viewLoading
				return m, tea.Batch(m.loading.Init(), fetchSnapshotsCmd(vm.Name))
			}
		case "L":
			m.llmSettings = newLLMSettingsModel(m.chat.config, m.width, m.height)
			m.currentView = viewLLMSettings
			return m, m.llmSettings.Init()
		case "M":
			if vm, ok := m.table.selectedVM(); ok {
				if vm.State != "Running" {
					m.errModal = newErrorModel("Mount Error", fmt.Sprintf("VM '%s' must be running for mount operations.", vm.Name))
					m.setChildSizes()
					m.currentView = viewError
					return m, nil
				}
				m.lastMountVM = vm.Name
				m.loading = newLoadingModel("Loading mounts…")
				m.setChildSizes()
				m.currentView = viewLoading
				return m, tea.Batch(m.loading.Init(), fetchMountsCmd(vm.Name))
			}
		}

		// Pass remaining keys to table for navigation
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	// ── Simple modals ──
	case viewHelp, viewVersion:
		switch msg.String() {
		case "esc", "enter", "q":
			m.currentView = viewTable
		}
		return m, nil

	case viewError:
		switch msg.String() {
		case "esc", "enter":
			m.currentView = viewTable
		}
		return m, nil

	case viewConfirm:
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd

	// ── Complex views ──
	case viewInfo:
		var cmd tea.Cmd
		m.info, cmd = m.info.Update(msg)
		return m, cmd

	case viewAdvCreate:
		var cmd tea.Cmd
		m.advCreate, cmd = m.advCreate.Update(msg)
		return m, cmd

	case viewSnapCreate:
		var cmd tea.Cmd
		m.snapCreate, cmd = m.snapCreate.Update(msg)
		return m, cmd

	case viewSnapManage:
		var cmd tea.Cmd
		m.snapManage, cmd = m.snapManage.Update(msg)
		return m, cmd

	case viewMountManage:
		var cmd tea.Cmd
		m.mountManage, cmd = m.mountManage.Update(msg)
		return m, cmd

	case viewMountAdd:
		var cmd tea.Cmd
		m.mountAdd, cmd = m.mountAdd.Update(msg)
		return m, cmd

	case viewMountModify:
		var cmd tea.Cmd
		m.mountModify, cmd = m.mountModify.Update(msg)
		return m, cmd

	case viewLLMSettings:
		var cmd tea.Cmd
		m.llmSettings, cmd = m.llmSettings.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ─── View ──────────────────────────────────────────────────────────────────────

func (m rootModel) View() string {
	switch m.currentView {
	case viewTable:
		if m.chatOpen {
			chatWidth := m.width * 40 / 100
			tableWidth := m.width - chatWidth

			// 1. Full-width title bar with chat label on the right
			titleBar := m.table.RenderTitleBar(m.width, m.chat.chatTitleText())

			// 2. Full-width footer
			footer := m.table.RenderFooter(m.width)
			footerHeight := lipgloss.Height(footer)

			// 3. Content height = total - title(1) - footer
			contentHeight := m.height - 1 - footerHeight

			// 4. Table content (no title, no footer)
			oldWidth := m.table.width
			oldHeight := m.table.height
			m.table.width = tableWidth
			m.table.height = contentHeight + 5 // add footer lines back for visibleRows calc
			tableContent := m.table.ViewContentOnly()
			m.table.width = oldWidth
			m.table.height = oldHeight

			// Force table side to exact content height
			tableContent = lipgloss.NewStyle().
				Width(tableWidth).
				Height(contentHeight).
				Render(tableContent)

			// 5. Chat content (no outer border, no title)
			m.chat.setSize(chatWidth, contentHeight)
			chatContent := m.chat.ViewContent()

			// Add left border to chat as a vertical separator
			chatContent = lipgloss.NewStyle().
				Width(chatWidth).
				Height(contentHeight).
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(currentTheme().Subtle).
				Render(chatContent)

			// 6. Compose: title + (table | chat) + footer
			midRow := lipgloss.JoinHorizontal(lipgloss.Top, tableContent, chatContent)
			return titleBar + "\n" + midRow + "\n" + footer
		}
		return m.table.View()
	case viewHelp:
		return m.help.View()
	case viewVersion:
		return m.version.View()
	case viewInfo:
		return m.info.View()
	case viewLoading:
		return m.loading.View()
	case viewError:
		return m.errModal.View()
	case viewConfirm:
		return m.confirm.View()
	case viewAdvCreate:
		return m.advCreate.View()
	case viewSnapCreate:
		return m.snapCreate.View()
	case viewSnapManage:
		return m.snapManage.View()
	case viewMountManage:
		return m.mountManage.View()
	case viewMountAdd:
		return m.mountAdd.View()
	case viewMountModify:
		return m.mountModify.View()
	case viewLLMSettings:
		return m.llmSettings.View()
	default:
		return "Unknown view"
	}
}

// ─── Toast Helpers ──────────────────────────────────────────────────────────────

func operationToastMessage(vmName, operation string, elapsed time.Duration) string {
	secs := elapsed.Seconds()
	timeStr := ""
	if secs >= 0.5 {
		timeStr = fmt.Sprintf(" in %.1fs", secs)
	}

	switch operation {
	case "stop":
		return fmt.Sprintf("✓ %s stopped%s", vmName, timeStr)
	case "start":
		return fmt.Sprintf("✓ %s started%s", vmName, timeStr)
	case "suspend":
		return fmt.Sprintf("✓ %s suspended%s", vmName, timeStr)
	case "recover":
		return fmt.Sprintf("✓ %s recovered%s", vmName, timeStr)
	case "delete":
		return fmt.Sprintf("✓ %s deleted%s", vmName, timeStr)
	case "create":
		return fmt.Sprintf("✓ %s created%s", vmName, timeStr)
	case "snapshot":
		return fmt.Sprintf("✓ Snapshot created for %s%s", vmName, timeStr)
	case "restore":
		return fmt.Sprintf("✓ Snapshot restored for %s%s", vmName, timeStr)
	case "delete-snapshot":
		return fmt.Sprintf("✓ Snapshot deleted from %s%s", vmName, timeStr)
	case "mount":
		return fmt.Sprintf("✓ Mount added to %s%s", vmName, timeStr)
	case "umount":
		return fmt.Sprintf("✓ Mount removed from %s%s", vmName, timeStr)
	case "stop-all":
		return fmt.Sprintf("✓ All VMs stopped%s", timeStr)
	case "start-all":
		return fmt.Sprintf("✓ All VMs started%s", timeStr)
	case "purge":
		return fmt.Sprintf("✓ All deleted VMs purged%s", timeStr)
	default:
		return fmt.Sprintf("✓ %s %s%s", vmName, operation, timeStr)
	}
}

// ─── Sort (moved from old main.go) ────────────────────────────────────────────

func sortVMs(vms []vmData, column int, ascending bool) {
	sort.SliceStable(vms, func(i, j int) bool {
		cmp := compareVMByColumn(vms[i], vms[j], column)
		if cmp == 0 {
			cmp = compareStringsFold(vms[i].info.Name, vms[j].info.Name)
		}
		if ascending {
			return cmp < 0
		}
		return cmp > 0
	})
}

func compareVMByColumn(a, b vmData, column int) int {
	switch column {
	case 0:
		return compareStringsFold(a.info.Name, b.info.Name)
	case 1:
		return compareStringsFold(a.info.State, b.info.State)
	case 2:
		return compareIntegerFields(a.info.Snapshots, b.info.Snapshots)
	case 3:
		return compareStringsFold(a.info.IPv4, b.info.IPv4)
	case 4:
		return compareIntegerFields(a.info.CPUs, b.info.CPUs)
	case 5:
		return compareUsageFields(a.info.DiskUsage, b.info.DiskUsage)
	case 6:
		return compareUsageFields(a.info.MemoryUsage, b.info.MemoryUsage)
	default:
		return 0
	}
}

func compareUsageFields(a, b string) int {
	aFrac, aOK := parseUsageFraction(a)
	bFrac, bOK := parseUsageFraction(b)

	switch {
	case aOK && bOK:
		return compareFloat64(aFrac, bFrac)
	case aOK:
		return 1
	case bOK:
		return -1
	default:
		return compareStringsFold(a, b)
	}
}

func compareIntegerFields(a, b string) int {
	aInt, aOK := parseIntField(a)
	bInt, bOK := parseIntField(b)

	switch {
	case aOK && bOK:
		return compareInt(aInt, bInt)
	case aOK:
		return 1
	case bOK:
		return -1
	default:
		return compareStringsFold(a, b)
	}
}

func parseIntField(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "--" {
		return 0, false
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n, true
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err == nil {
		return n, true
	}
	return 0, false
}

func compareStringsFold(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareFloat64(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func runMountModifyOperation(runCmd func(args ...string) (string, error), vmName, oldTarget, newSource, newTarget string) error {
	oldMount := vmName + ":" + oldTarget
	if _, err := runCmd("umount", oldMount); err != nil {
		return fmt.Errorf("failed to unmount %s: %w", oldMount, err)
	}

	newMount := vmName + ":" + newTarget
	if _, err := runCmd("mount", newSource, newMount); err != nil {
		return fmt.Errorf("failed to mount %s to %s: %w", newSource, newMount, err)
	}

	return nil
}

// ─── Logger ────────────────────────────────────────────────────────────────────

func initLogger() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(home, ".passgo")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "passgo.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) // #nosec G304 -- path from UserHomeDir
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	appLogger = log.New(f, "", log.LstdFlags)
	return nil
}

// ─── Entry Point ───────────────────────────────────────────────────────────────

func main() {
	if err := initLogger(); err != nil {
		log.Printf("logger init failed: %v", err)
	} else {
		appLogger.Println("passgo starting up")
	}

	model := initialModel()
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Store program reference for p.Send() in agent loop.
	// This is set via the programReadyMsg on first tick.
	go func() {
		// Small delay to ensure program is running before sending
		p.Send(programReadyMsg{program: p})
	}()

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	// Cleanup MCP subprocess
	if model.chat.mcpClient != nil {
		model.chat.mcpClient.Close()
	}
}
