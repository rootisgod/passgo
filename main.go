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

	// Set table styling
	vmTable.SetCell(0, 0, tview.NewTableCell("Name").SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))
	vmTable.SetCell(0, 1, tview.NewTableCell("State").SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))
	vmTable.SetCell(0, 2, tview.NewTableCell("Image").SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))

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

	// Function to populate the table with VMs
	populateVMTable := func() {
		clearVMRows()

		output, err := ListVMs()
		if err != nil {
			vmTable.SetCell(1, 0, tview.NewTableCell("Error fetching VMs").SetTextColor(tview.Styles.PrimaryTextColor))
			vmTable.SetCell(1, 1, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			vmTable.SetCell(1, 2, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
		} else {
			// Parse multipass output and add each VM as a table row
			lines := strings.Split(output, "\n")
			row := 1
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
					continue // Skip header and separator lines
				}

				// Parse VM line (format: Name State IPv4 Image Release)
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					vmName := fields[0]
					vmState := fields[1]
					vmImage := fields[3] // Image is field[3], IPv4 is field[2] (which we skip)

					vmTable.SetCell(row, 0, tview.NewTableCell(vmName).SetTextColor(tview.Styles.PrimaryTextColor))
					vmTable.SetCell(row, 1, tview.NewTableCell(vmState).SetTextColor(tview.Styles.PrimaryTextColor))
					vmTable.SetCell(row, 2, tview.NewTableCell(vmImage).SetTextColor(tview.Styles.PrimaryTextColor))
					row++
				}
			}

			if row == 1 {
				vmTable.SetCell(1, 0, tview.NewTableCell("No VMs found").SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(1, 1, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(1, 2, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
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
			vmTable.SetCell(1, 1, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			vmTable.SetCell(1, 2, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
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
