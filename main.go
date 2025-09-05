// main.go (for Option 1, same package)
package main

import (
	"fmt"
	"log"

	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Display VM list
	textView := tview.NewTextView().SetText("Fetching VMs...")

	// Fetch and display VMs at startup
	go func() {
		output, err := ListVMs()
		app.QueueUpdateDraw(func() {
			if err != nil {
				textView.SetText(fmt.Sprintf("Error: %v", err))
			} else {
				textView.SetText(fmt.Sprintf("VM List:\n%s", output))
			}
		})
	}()

	flex.AddItem(textView, 0, 1, false)

	// Button to refresh VM list
	refreshButton := tview.NewButton("Refresh VMs").SetSelectedFunc(func() {
		output, err := ListVMs()
		if err != nil {
			textView.SetText(fmt.Sprintf("Error: %v", err))
		} else {
			textView.SetText(fmt.Sprintf("VM List:\n%s", output))
		}
	})
	flex.AddItem(refreshButton, 1, 1, true)

	// Button to launch a new VM
	launchButton := tview.NewButton("Launch VM").SetSelectedFunc(func() {
		output, err := LaunchVM("test-vm", "22.04")
		if err != nil {
			textView.SetText(fmt.Sprintf("Launch error: %v", err))
		} else {
			textView.SetText(fmt.Sprintf("Launched:\n%s", output))
		}
	})
	flex.AddItem(launchButton, 1, 1, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatalf("tview error: %v", err)
	}
}
