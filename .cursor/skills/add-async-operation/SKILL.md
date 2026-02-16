---
name: add-async-operation
description: Adds a new asynchronous operation to the PassGo Bubble Tea TUI. Use when adding background fetches, VM operations, or any tea.Cmd that returns a result message. Use when the user asks to add async work, background operations, or new multipass commands.
---

# Add Async Operation (PassGo)

Step-by-step guide for adding async operations to PassGo. All async work uses typed `tea.Msg` and `tea.Cmd` factories in `messages.go`, with handling in `main.go` Update.

## Operation Types

| Type | Message | When to use |
|------|---------|-------------|
| **Fetch/list** | Custom `xxxResultMsg` | Loading data for a view (snapshots, mounts, VM info) |
| **VM operation** | `vmOperationResultMsg` | Start, stop, create, delete, snapshot, mount, etc. |
| **Confirm-triggered** | Via `confirmResultMsg` + `pendingCmd` | User confirms, then run cmd |

---

## Type A: Fetch/List Operation

For loading data that drives a view (e.g. snapshots, mounts).

### 1. Define result message in messages.go

```go
type xxxResultMsg struct {
    vmName string  // or other context
    data   SomeType
    err    error
}
```

### 2. Add cmd factory in messages.go

```go
func fetchXxxCmd(vmName string) tea.Cmd {
    return func() tea.Msg {
        data, err := doFetchXxx(vmName)
        return xxxResultMsg{vmName: vmName, data: data, err: err}
    }
}
```

### 3. Handle in main.go Update()

```go
case xxxResultMsg:
    if msg.err != nil {
        m.errModal = newErrorModel("Xxx Error", msg.err.Error())
        m.setChildSizes()
        m.currentView = viewError
    } else {
        m.xxxView = newXxxModel(msg.vmName, msg.data, m.width, m.height)
        m.currentView = viewXxx
    }
    return m, nil
```

### 4. Trigger from handleKey or elsewhere

```go
m.loading = newLoadingModel("Loading…")
m.setChildSizes()
m.currentView = viewLoading
return m, tea.Batch(m.loading.Init(), fetchXxxCmd(vmName))
```

**Reference:** `fetchSnapshotsCmd`, `fetchMountsCmd`, `snapshotListResultMsg`, `mountListResultMsg`

---

## Type B: VM Operation (reuse vmOperationResultMsg)

For start, stop, create, delete, snapshot, mount, etc. Use existing `vmOperationResultMsg` — no new message type.

### 1. Add cmd factory in messages.go

```go
func doXxxCmd(vmName string) tea.Cmd {
    return func() tea.Msg {
        _, err := runMultipassCommand("xxx", vmName)  // or call multipass.go helper
        return vmOperationResultMsg{
            vmName:    vmName,
            operation: "xxx",
            err:       err,
            inline:    true,  // true = stay on table, false = show loading then refresh
        }
    }
}
```

### 2. Inline vs non-inline

| `inline` | Behavior |
|----------|----------|
| `true` | Stay on table, show toast, refresh VM list in background. Use for start, stop, suspend, recover, quick create. |
| `false` | Show loading, then refresh VM list (or return to snap/mount manager). Use for delete, create (advanced), snapshot, restore, mount. |

### 3. For inline: set busy state before dispatching

```go
m.table.busyVMs[vm.Name] = busyInfo{operation: "Stopping", startTime: time.Now()}
return m, stopVMCmd(vm.Name)
```

### 4. Add toast message (if new operation type)

In `main.go`, add to `operationToastMessage()` switch:

```go
case "xxx":
    return fmt.Sprintf("✓ %s xxx'd%s", vmName, timeStr)
```

**Reference:** `stopVMCmd`, `startVMCmd`, `createSnapshotCmd`, `mountCmd`

---

## Type C: Confirm-triggered operation

For operations that need user confirmation (delete, purge).

### 1. Add cmd factory in messages.go

```go
func doXxxCmd(args ...) tea.Cmd {
    return func() tea.Msg {
        _, err := doTheThing(args...)
        return vmOperationResultMsg{operation: "xxx", err: err}
    }
}
```

### 2. In handleKey: show confirm, set pendingCmd

```go
m.confirm = newConfirmModel("Really do xxx?")
m.setChildSizes()
m.pendingCmd = doXxxCmd(args...)
m.currentView = viewConfirm
return m, nil
```

### 3. confirmResultMsg handler (already in main.go)

When user confirms, `pendingCmd` is run. No changes needed if using existing pattern.

**Reference:** delete (`d`), purge (`!`), stop-all (`<`), start-all (`>`)

---

## Checklist for new async operation

```
- [ ] Define msg type (or reuse vmOperationResultMsg)
- [ ] Add cmd factory in messages.go
- [ ] Add case in main.go Update() for the result msg
- [ ] Add trigger (handleKey, or from view)
- [ ] (VM op) Add to operationToastMessage if new operation name
- [ ] (Inline VM op) Set busyVMs before dispatching
```

---

## Key patterns

- **Cmd shape:** `func() tea.Msg { ...; return xxxMsg{...} }`
- **Error handling:** Always include `err` in result msg; main.go shows error modal or toast
- **Loading transition:** `m.loading = newLoadingModel("…"); m.currentView = viewLoading; return m, tea.Batch(m.loading.Init(), yourCmd)`
- **Batch cmds:** `tea.Batch(cmd1, cmd2)` to run multiple
- **Blocking subprocess:** Use `tea.ExecProcess(cmd, func(err error) tea.Msg { return shellFinishedMsg{err} })` for shell, etc.
