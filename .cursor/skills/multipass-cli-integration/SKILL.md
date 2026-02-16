---
name: multipass-cli-integration
description: Integrates with the Multipass CLI for VM operations. Use when adding or modifying multipass commands, parsing multipass output, or implementing VM/snapshot/mount operations. Use when working with multipass.go, parsing.go, or mount_operations.go.
---

# Multipass CLI Integration (PassGo)

PassGo drives Multipass via the CLI. All commands go through `runMultipassCommand` in `multipass.go`. Parsing lives in `parsing.go` and `mount_operations.go`.

## Running commands

Use `runMultipassCommand` for all multipass subprocess calls:

```go
output, err := runMultipassCommand("list")
output, err := runMultipassCommand("stop", vmName)
output, err := runMultipassCommand("info", vmName)
output, err := runMultipassCommand("info", vmName, "--format", "json")
```

- Returns `(string, error)`
- Logs to `appLogger` when non-nil
- Stderr is included in error messages

---

## Adding a new multipass command

### 1. Add wrapper in multipass.go (optional)

For reusable operations:

```go
func DoXxx(vmName string) (string, error) {
    return runMultipassCommand("xxx", vmName)
}
```

### 2. Use from messages.go cmd factory

```go
func doXxxCmd(vmName string) tea.Cmd {
    return func() tea.Msg {
        _, err := runMultipassCommand("xxx", vmName)
        return vmOperationResultMsg{vmName: vmName, operation: "xxx", err: err}
    }
}
```

Never call `runMultipassCommand` directly from Update — use a cmd factory so it runs off the main loop.

---

## Parsing output

### Text output (list, info)

`parsing.go` has patterns for key:value and table output:

```go
// Key:value (multipass info)
parts := strings.SplitN(line, ":", 2)
key := strings.TrimSpace(parts[0])
value := strings.TrimSpace(parts[1])

// Table (multipass list) — skip header/separator
if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
    continue
}
fields := strings.Fields(line)
```

Reference: `parseVMInfo`, `parseVMNames`, `parseSnapshots`.

### JSON output

For structured data (e.g. mounts), use `--format json`:

```go
output, err := runMultipassCommand("info", vmName, "--format", "json")
var response multipassInfoResponse
json.Unmarshal([]byte(output), &response)
```

Reference: `getVMMounts` in `mount_operations.go`.

---

## Logging

Use `appLogger` for multipass-related logging:

```go
if appLogger != nil {
    appLogger.Printf("exec: multipass %s", strings.Join(args, " "))
}
```

`runMultipassCommand` already logs exec and errors. Add logs for config reads, clone operations, and parsing failures.

---

## Error handling

- Return errors from multipass.go helpers; let callers surface them via result msgs
- Include stderr in error messages: `fmt.Errorf("...: %v\nStderr: %s", err, stderr.String())`
- In cmd factories, put `err` in the result msg so main.go can show error modal or toast

---

## Snapshot ID format

Snapshot IDs are `vmName.snapshotName`:

```go
snapshotID := vmName + "." + snapshotName
runMultipassCommand("restore", "--destructive", snapshotID)
runMultipassCommand("delete", "--purge", snapshotID)
```

---

## Interactive shell

For `multipass shell`, do not use `runMultipassCommand` — it would block. Use `tea.ExecProcess` in main.go so the TUI suspends and resumes correctly.
