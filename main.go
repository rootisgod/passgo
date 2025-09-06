// main.go (for Option 1, same package)
package main

import (
	"log"
	"strings"

	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Create selectable VM table
	vmTable := tview.NewTable()
	vmTable.SetBorder(true).SetTitle("Multipass VMs")
	vmTable.SetSelectable(true, false) // Allow row selection, not column selection
	vmTable.SetSelectedFunc(func(row, column int) {
		// Only allow selection of VM rows (skip header row 0)
		if row > 0 {
			// Handle VM selection - you can add actions here later
			app.Stop()
		}
	})

	// Set table styling with all fields
	headers := []string{"Name", "State", "Snapshots", "IPv4", "Release", "CPUs", "Disk Usage", "Memory Usage", "Mounts"}
	for i, header := range headers {
		vmTable.SetCell(0, i, tview.NewTableCell(header).SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))
	}

	// Set initial selection to first VM row (row 1) instead of header (row 0)
	vmTable.SetSelectionChangedFunc(func(row, column int) {
		// If we're on the header row, move to the first VM row
		if row == 0 && vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	})

	// Helper function to clear VM rows (keep header)
	clearVMRows := func() {
		for i := vmTable.GetRowCount() - 1; i > 0; i-- {
			vmTable.RemoveRow(i)
		}
	}

	// VMInfo represents detailed VM information
	type VMInfo struct {
		Name        string
		State       string
		Snapshots   string
		IPv4        string
		Release     string
		CPUs        string
		DiskUsage   string
		MemoryUsage string
		Mounts      string
	}

	// Parse detailed VM info from multipass info output
	parseVMInfo := func(info string) VMInfo {
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

	// Function to populate the table with detailed VM info
	populateVMTable := func() {
		clearVMRows()

		// First get the list of VMs
		listOutput, err := ListVMs()
		if err != nil {
			vmTable.SetCell(1, 0, tview.NewTableCell("Error fetching VMs").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
			return
		}

		// Parse VM names from list output
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

		if len(vmNames) == 0 {
			vmTable.SetCell(1, 0, tview.NewTableCell("No VMs found").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
			return
		}

		// Get detailed info for each VM
		row := 1
		for _, vmName := range vmNames {
			infoOutput, err := GetVMInfo(vmName)
			if err != nil {
				// If we can't get detailed info, show basic info
				vmTable.SetCell(row, 0, tview.NewTableCell(vmName).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 1, tview.NewTableCell("Error").SetTextColor(tview.Styles.PrimaryTextColor))
				for i := 2; i < 9; i++ {
					vmTable.SetCell(row, i, tview.NewTableCell("--").SetTextColor(tview.Styles.PrimaryTextColor))
				}
			} else {
				vm := parseVMInfo(infoOutput)
				vmTable.SetCell(row, 0, tview.NewTableCell(vm.Name).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 1, tview.NewTableCell(vm.State).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 2, tview.NewTableCell(vm.Snapshots).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 3, tview.NewTableCell(vm.IPv4).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 4, tview.NewTableCell(vm.Release).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 5, tview.NewTableCell(vm.CPUs).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 6, tview.NewTableCell(vm.DiskUsage).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 7, tview.NewTableCell(vm.MemoryUsage).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 8, tview.NewTableCell(vm.Mounts).SetTextColor(tview.Styles.PrimaryTextColor))
			}
			row++
		}

		// Set selection to first VM row (row 1) after populating
		if vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	}

	// Fetch and display VMs at startup
	go func() {
		app.QueueUpdateDraw(populateVMTable)
	}()

	flex.AddItem(vmTable, 0, 1, true) // Make the table focusable

	// Button to refresh VM list
	refreshButton := tview.NewButton("Refresh VMs").SetSelectedFunc(func() {
		populateVMTable()
	})
	flex.AddItem(refreshButton, 1, 1, true)

	// Button to launch a new VM
	launchButton := tview.NewButton("Launch VM").SetSelectedFunc(func() {
		_, err := LaunchVM("test-vm", "22.04")
		if err != nil {
			// Show error in table temporarily
			clearVMRows()
			vmTable.SetCell(1, 0, tview.NewTableCell("Launch Error").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
		} else {
			// Refresh the table to show the new VM
			populateVMTable()
		}
	})
	flex.AddItem(launchButton, 1, 1, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatalf("tview error: %v", err)
	}
}
