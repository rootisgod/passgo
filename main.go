// main.go - Root model, view routing, and program setup for bubbletea TUI
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	// Pending operation for confirm dialogs
	pendingCmd tea.Cmd

	// Context for returning to sub-views after operations
	lastMountVM string
	lastSnapVM  string
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
}

func initialModel() rootModel {
	return rootModel{
		currentView: viewLoading,
		table:       newTableModel(),
		loading:     newLoadingModel("Loading VMs…"),
	}
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.loading.Init(),
		m.table.spinner.Tick,
		fetchVMListCmd(),
		autoRefreshTickCmd(),
	)
}

// ─── Update ────────────────────────────────────────────────────────────────────

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

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
			cmds = append(cmds, fetchVMListBackgroundCmd())
		}
		return m, tea.Batch(cmds...)

	// ── Async results ──
	case vmListResultMsg:
		if msg.err != nil {
			if msg.background {
				// Silently ignore background fetch errors
				return m, nil
			}
			m.errModal = newErrorModel("VM List Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.table.setVMs(msg.vms)
			m.table.lastRefresh = time.Now()
			if !msg.background {
				m.currentView = viewTable
			}
		}
		return m, nil

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
				return m, tea.Batch(toastCmd, fetchVMListBackgroundCmd())
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
			return m, tea.Batch(toastCmd, fetchVMListBackgroundCmd())
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
		return m, tea.Batch(m.loading.Init(), fetchVMListCmd(), toastCmd)

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
		return m, tea.Batch(m.loading.Init(), fetchVMListCmd())

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
		return m, advancedCreateCmd(msg.name, msg.release, msg.cpus, msg.memoryMB, msg.diskGB, msg.cloudInitFile)

	case mountAddRequestMsg:
		m.mountAdd = newMountAddModel(msg.vmName, m.width, m.height)
		m.currentView = viewMountAdd
		return m, nil

	case mountModifyRequestMsg:
		m.mountModify = newMountModifyModel(msg.vmName, msg.mount, m.width, m.height)
		m.currentView = viewMountModify
		return m, m.mountModify.Init()

	case mountModifySubmitMsg:
		m.loading = newLoadingModel("Updating mount…")
		m.setChildSizes()
		m.currentView = viewLoading
		return m, tea.Batch(m.loading.Init(), func() tea.Msg {
			// Unmount old, then mount new
			_, _ = runMultipassCommand("umount", msg.vmName+":"+msg.oldTarget)
			_, err := runMultipassCommand("mount", msg.newSource, msg.vmName+":"+msg.newTarget)
			return vmOperationResultMsg{vmName: msg.vmName, operation: "mount", err: err}
		})
	}

	// ── Toast expiry (always route to table regardless of view) ──
	if expire, ok := msg.(toastExpireMsg); ok {
		m.table, _ = m.table.Update(expire)
		return m, nil
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
	}

	return m, nil
}

// ─── Key Handling ──────────────────────────────────────────────────────────────

func (m rootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			return m, tea.Quit
		case "esc":
			if m.table.filterVisible {
				m.table.filterText = ""
				m.table.filterInput.SetValue("")
				m.table.filterVisible = false
				m.table.applyFilterAndSort()
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
			return m, tea.Batch(m.loading.Init(), fetchVMListCmd())
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
	}

	return m, nil
}

// ─── View ──────────────────────────────────────────────────────────────────────

func (m rootModel) View() string {
	switch m.currentView {
	case viewTable:
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
	sort.Slice(vms, func(i, j int) bool {
		var less bool
		switch column {
		case 0:
			less = strings.ToLower(vms[i].info.Name) < strings.ToLower(vms[j].info.Name)
		case 1:
			less = strings.ToLower(vms[i].info.State) < strings.ToLower(vms[j].info.State)
		case 2:
			less = vms[i].info.Snapshots < vms[j].info.Snapshots
		case 3:
			less = vms[i].info.IPv4 < vms[j].info.IPv4
		case 4:
			less = vms[i].info.CPUs < vms[j].info.CPUs
		case 5:
			less = vms[i].info.DiskUsage < vms[j].info.DiskUsage
		case 6:
			less = vms[i].info.MemoryUsage < vms[j].info.MemoryUsage
		default:
			less = false
		}
		if !ascending {
			less = !less
		}
		return less
	})
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

	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
