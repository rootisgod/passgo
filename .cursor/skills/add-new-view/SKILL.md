---
name: add-new-view
description: Adds a new view, screen, or modal to the PassGo Bubble Tea TUI. Use when adding a new screen, modal, or sub-view to the application, or when the user asks to add a view, screen, or UI panel.
---

# Add New View (PassGo)

Step-by-step checklist for adding a new view to the PassGo TUI. The root model in `main.go` routes by `viewState`; every new view requires updates in multiple places.

## Checklist

Copy and track progress:

```
- [ ] 1. Add viewState constant
- [ ] 2. Add model field to rootModel
- [ ] 3. Add setChildSizes entry
- [ ] 4. Add Update() handling (async msgs + delegation)
- [ ] 5. Add handleKey() case
- [ ] 6. Add View() switch case
- [ ] 7. Create view_xxx.go
- [ ] 8. (If async) Add msg type and cmd in messages.go
```

---

## Step 1: Add viewState constant

In `main.go`, add to the `viewState` enum:

```go
const (
    viewTable viewState = iota
    // ... existing ...
    viewXxx  // add new
)
```

---

## Step 2: Add model field to rootModel

In `main.go`, add the child model field:

```go
type rootModel struct {
    // ...
    xxxModel xxxModel  // add new
}
```

---

## Step 3: Add setChildSizes entry

In `main.go`, add to `setChildSizes()`:

```go
m.xxx.width = m.width
m.xxx.height = m.height
```

---

## Step 4: Add Update() handling

In `main.go` `Update()`, add:

**A. Async result messages** (if the view fetches data):

```go
case xxxResultMsg:
    if msg.err != nil {
        m.errModal = newErrorModel("Xxx Error", msg.err.Error())
        m.setChildSizes()
        m.currentView = viewError
    } else {
        m.xxx = newXxxModel(msg.data, m.width, m.height)
        m.currentView = viewXxx
    }
    return m, nil
```

**B. Delegation** (forward non-key messages to the active view):

In the `switch m.currentView` block at the end of Update:

```go
case viewXxx:
    var cmd tea.Cmd
    m.xxx, cmd = m.xxx.Update(msg)
    return m, cmd
```

---

## Step 5: Add handleKey() case

In `main.go` `handleKey()`, add a case for `viewXxx`:

```go
case viewXxx:
    var cmd tea.Cmd
    m.xxx, cmd = m.xxx.Update(msg)
    return m, cmd
```

For **simple modals** (help, version, error): handle Esc/Enter to return:

```go
case viewXxx:
    switch msg.String() {
    case "esc", "enter":
        m.currentView = viewTable
    }
    return m, nil
```

---

## Step 6: Add View() switch case

In `main.go` `View()`:

```go
case viewXxx:
    return m.xxx.View()
```

---

## Step 7: Create view_xxx.go

Create `view_xxx.go` with:

```go
// view_xxx.go - Description of the view
package main

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type xxxModel struct {
    width  int
    height int
    // view-specific fields
}

func newXxxModel(width, height int) xxxModel {
    return xxxModel{width: width, height: height}
}

func (m xxxModel) Init() tea.Cmd {
    return nil  // or return fetchXxxCmd() if async
}

func (m xxxModel) Update(msg tea.Msg) (xxxModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc", "q":
            return m, func() tea.Msg { return backToTableMsg{} }
        }
    }
    return m, nil
}

func (m xxxModel) View() string {
    content := modalTitleStyle.Render("Title") + "\n\n" + modalTextStyle.Render("Content")
    box := modalStyle.Render(content)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
```

**Reference implementations:**
- Simple modal: `view_modals.go` (help, version, error, confirm)
- Loading: `view_loading.go`
- Form with inputs: `view_create.go`
- List view: `view_snapshots.go`, `view_mounts.go`

---

## Step 8: Async messages (if needed)

If the view fetches data asynchronously, in `messages.go`:

**Define result message:**

```go
type xxxResultMsg struct {
    data SomeType
    err  error
}
```

**Add cmd factory:**

```go
func fetchXxxCmd() tea.Cmd {
    return func() tea.Msg {
        data, err := doFetchXxx()
        return xxxResultMsg{data: data, err: err}
    }
}
```

**Transition to view:** In `handleKey()` or elsewhere, set loading and dispatch:

```go
m.loading = newLoadingModel("Loading…")
m.setChildSizes()
m.currentView = viewLoading
return m, tea.Batch(m.loading.Init(), fetchXxxCmd())
```

---

## Key patterns

- **Centered modals:** Use `lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)`
- **Styles:** Use `modalStyle`, `modalTitleStyle`, `formLabelStyle`, etc. from `styles.go` (theme-aware)
- **Return to table:** Send `backToTableMsg{}` to go back
- **Error handling:** Use `newErrorModel(title, msg)` and switch to `viewError`
