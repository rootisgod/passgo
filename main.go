// main.go - Root model, view routing, and program setup for bubbletea TUI
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		fetchVMListCmd(),
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

	// ── Async results ──
	case vmListResultMsg:
		if msg.err != nil {
			m.errModal = newErrorModel("VM List Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.table.setVMs(msg.vms)
			m.currentView = viewTable
		}
		return m, nil

	case vmInfoResultMsg:
		if msg.err != nil {
			m.errModal = newErrorModel("Info Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.info.setContent(msg.info)
		}
		return m, nil

	case vmOperationResultMsg:
		if msg.err != nil {
			m.errModal = newErrorModel("Operation Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
			return m, nil
		}
		// Return to mount/snap manager if that's where we came from
		if m.lastMountVM != "" && (msg.operation == "mount" || msg.operation == "umount") {
			vmName := m.lastMountVM
			m.loading = newLoadingModel("Refreshing mounts…")
			m.setChildSizes()
			m.currentView = viewLoading
			return m, tea.Batch(m.loading.Init(), fetchMountsCmd(vmName))
		}
		if m.lastSnapVM != "" && (msg.operation == "snapshot" || msg.operation == "delete-snapshot" || msg.operation == "restore") {
			vmName := m.lastSnapVM
			m.loading = newLoadingModel("Refreshing snapshots…")
			m.setChildSizes()
			m.currentView = viewLoading
			return m, tea.Batch(m.loading.Init(), fetchSnapshotsCmd(vmName))
		}
		m.loading = newLoadingModel("Refreshing…")
		m.setChildSizes()
		m.currentView = viewLoading
		return m, tea.Batch(m.loading.Init(), fetchVMListCmd())

	case snapshotListResultMsg:
		if msg.err != nil {
			m.errModal = newErrorModel("Snapshot Error", msg.err.Error())
			m.setChildSizes()
			m.currentView = viewError
		} else {
			m.snapManage = newSnapManageModel(msg.vmName, m.width, m.height)
			m.snapManage.snapshots = msg.snapshots
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
		m.loading = newLoadingModel(fmt.Sprintf("Creating %s…", msg.name))
		m.setChildSizes()
		m.currentView = viewLoading
		return m, tea.Batch(
			m.loading.Init(),
			advancedCreateCmd(msg.name, msg.release, msg.cpus, msg.memoryMB, msg.diskGB, msg.cloudInitFile),
		)

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
			runMultipassCommand("umount", msg.vmName+":"+msg.oldTarget)
			_, err := runMultipassCommand("mount", msg.newSource, msg.vmName+":"+msg.newTarget)
			return vmOperationResultMsg{vmName: msg.vmName, operation: "mount", err: err}
		})
	}

	// ── Delegate to active view for non-key messages ──
	switch m.currentView {
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
				return m, fetchVMInfoCmd(vm.Name)
			}
		case "c":
			m.loading = newLoadingModel("Creating VM…")
			m.setChildSizes()
			m.currentView = viewLoading
			return m, tea.Batch(m.loading.Init(), quickCreateCmd())
		case "C":
			m.advCreate = newAdvCreateModel(m.width, m.height)
			m.currentView = viewAdvCreate
			return m, m.advCreate.Init()
		case "[":
			if vm, ok := m.table.selectedVM(); ok {
				m.loading = newLoadingModel("Stopping " + vm.Name + "…")
				m.setChildSizes()
				m.currentView = viewLoading
				return m, tea.Batch(m.loading.Init(), stopVMCmd(vm.Name))
			}
		case "]":
			if vm, ok := m.table.selectedVM(); ok {
				m.loading = newLoadingModel("Starting " + vm.Name + "…")
				m.setChildSizes()
				m.currentView = viewLoading
				return m, tea.Batch(m.loading.Init(), startVMCmd(vm.Name))
			}
		case "p":
			if vm, ok := m.table.selectedVM(); ok {
				m.loading = newLoadingModel("Suspending " + vm.Name + "…")
				m.setChildSizes()
				m.currentView = viewLoading
				return m, tea.Batch(m.loading.Init(), suspendVMCmd(vm.Name))
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
				m.loading = newLoadingModel("Recovering " + vm.Name + "…")
				m.setChildSizes()
				m.currentView = viewLoading
				return m, tea.Batch(m.loading.Init(), recoverVMCmd(vm.Name))
			}
		case "!":
			m.confirm = newConfirmModel("PURGE ALL deleted VMs? This cannot be undone.")
			m.setChildSizes()
			m.pendingCmd = purgeAllVMsCmd()
			m.currentView = viewConfirm
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
				c := exec.Command("multipass", "shell", vm.Name)
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

// ─── Sort (moved from old main.go) ────────────────────────────────────────────

func sortVMs(vms []vmData, column int, ascending bool) {
	n := len(vms)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			var compare bool
			switch column {
			case 0:
				compare = strings.ToLower(vms[j].info.Name) > strings.ToLower(vms[j+1].info.Name)
			case 1:
				compare = strings.ToLower(vms[j].info.State) > strings.ToLower(vms[j+1].info.State)
			case 2:
				compare = vms[j].info.Snapshots > vms[j+1].info.Snapshots
			case 3:
				compare = vms[j].info.IPv4 > vms[j+1].info.IPv4
			case 4:
				compare = vms[j].info.Release > vms[j+1].info.Release
			case 5:
				compare = vms[j].info.CPUs > vms[j+1].info.CPUs
			case 6:
				compare = vms[j].info.DiskUsage > vms[j+1].info.DiskUsage
			case 7:
				compare = vms[j].info.MemoryUsage > vms[j+1].info.MemoryUsage
			case 8:
				compare = vms[j].info.Mounts > vms[j+1].info.Mounts
			default:
				compare = false
			}
			if !ascending {
				compare = !compare
			}
			if compare {
				vms[j], vms[j+1] = vms[j+1], vms[j]
			}
		}
	}
}

// ─── Logger ────────────────────────────────────────────────────────────────────

func initLogger() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(home, ".passgo")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "passgo.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
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

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
