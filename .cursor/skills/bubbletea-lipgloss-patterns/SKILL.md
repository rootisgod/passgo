---
name: bubbletea-lipgloss-patterns
description: PassGo-specific Bubble Tea and lipgloss patterns for the TUI. Use when implementing or refactoring TUI components, modals, forms, or views. Use when working with Bubble Tea models, tea.Cmd, lipgloss styles, or view rendering.
---

# Bubble Tea + Lipgloss Patterns (PassGo)

PassGo-specific patterns for the Bubble Tea TUI. Single package `main`, root model routes by `viewState`, async via typed `tea.Msg`.

## Model structure

Every view model implements:

```go
func (m xxxModel) Init() tea.Cmd
func (m xxxModel) Update(msg tea.Msg) (xxxModel, tea.Cmd)
func (m xxxModel) View() string
```

- **Init:** Return `nil` or a `tea.Cmd` for initial async work
- **Update:** Handle messages, return updated model and optional cmd
- **View:** Return the rendered string (no side effects)

---

## Centered modals

Use `lipgloss.Place` for centered overlay modals:

```go
content := modalTitleStyle.Render("Title") + "\n\n" + modalTextStyle.Render("Body")
box := modalStyle.Render(content)
return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
```

Models need `width` and `height`; root calls `setChildSizes()` after creating/replacing them.

---

## Style usage (theme-aware)

Use shared styles from `styles.go` — they are rebuilt when the theme changes:

| Style | Use |
|-------|-----|
| `modalStyle`, `modalTitleStyle`, `modalTextStyle` | Modals, dialogs |
| `formLabelStyle`, `formActiveLabelStyle`, `formValueStyle` | Form labels and values |
| `formButtonStyle`, `formActiveButtonStyle` | Buttons |
| `formHintStyle` | Hints, secondary text |
| `listItemStyle`, `listSelectedItemStyle` | List items |
| `detailKeyStyle`, `detailValStyle` | Key-value panels |
| `errorTitleStyle` | Error modal title |

Do not hardcode colors. Use these styles or theme aliases (`accent`, `subtle`, etc.) in `rebuildStyles()`.

---

## VM state styling

Use helpers from `styles.go`:

```go
stateColor(state string) lipgloss.Color  // Returns theme color for "Running", "Stopped", etc.
stateIcon(state string) string           // Returns colored dot (● ○ ◉ ◌)
```

States: `Running`, `Stopped`, `Suspended`, `Deleted`, `Creating`.

---

## Returning to table

To go back to the main table view:

```go
return m, func() tea.Msg { return backToTableMsg{} }
```

Root handles `backToTableMsg` and clears `lastMountVM` / `lastSnapVM`.

---

## Blocking subprocesses

For interactive shell or other blocking commands:

```go
c := exec.Command("multipass", "shell", vmName)
return m, tea.ExecProcess(c, func(err error) tea.Msg {
    return shellFinishedMsg{err: err}
})
```

TUI suspends; on exit, `shellFinishedMsg` triggers refresh.

---

## Key handling in views

Forward keys to child for complex views:

```go
case viewXxx:
    var cmd tea.Cmd
    m.xxx, cmd = m.xxx.Update(msg)
    return m, cmd
```

Simple modals (help, version, error) handle Esc/Enter directly in root's `handleKey`.

---

## Spinner / loading

Use `bubbles/spinner` with `spinnerStyle`:

```go
s := spinner.New()
s.Spinner = spinner.Dot
s.Style = spinnerStyle
```

Return `m.spinner.Tick` from Init for animation.
