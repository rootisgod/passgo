# PassGo Architecture

High-level overview for developers and LLMs working with the codebase.

## Data Flow

```mermaid
flowchart TB
    subgraph UserInput [User Input]
        Key[Key Press]
        Mouse[Mouse]
    end

    subgraph Root [rootModel]
        handleKey[handleKey]
        Update[Update]
        View[View]
    end

    subgraph Cmds [tea.Cmd]
        fetchVM[fetchVMListCmd]
        stopVM[stopVMCmd]
        fetchSnap[fetchSnapshotsCmd]
    end

    subgraph Msgs [Result Messages]
        vmList[vmListResultMsg]
        vmOp[vmOperationResultMsg]
        snapList[snapshotListResultMsg]
    end

    Key --> handleKey
    handleKey --> Cmds
    Cmds --> Msgs
    Msgs --> Update
    Update --> View
```

1. User presses a key → `handleKey` in main.go
2. Handler returns a `tea.Cmd` (e.g. `fetchVMListCmd`, `stopVMCmd`)
3. Bubble Tea runs the cmd asynchronously; it eventually produces a typed `*Msg`
4. `rootModel.Update(msg)` receives the message and updates state
5. On result messages, `currentView` may change; `View()` renders the active child model

## File Map

| File | Purpose |
|------|---------|
| main.go | Root model, view routing, handleKey, setChildSizes, Init, Update, View |
| messages.go | All tea.Msg types and tea.Cmd factories for async operations |
| view_table.go | Main VM table, filter, sorting, toasts, busy indicators |
| view_info.go | VM detail view with CPU/memory charts |
| view_create.go | Advanced VM creation form (cloud-init, resources) |
| view_modals.go | Help, version, error, and confirm modals |
| view_loading.go | Loading spinner overlay |
| view_snapshots.go | Snapshot create and manage views |
| view_mounts.go | Mount manage, add, and modify views |
| styles.go | Lipgloss styles; rebuildStyles() when theme changes |
| themes.go | Theme definitions, currentTheme(), setTheme() |
| multipass.go | Multipass CLI wrapper, cloud-init scanning, repo cloning |
| parsing.go | VMInfo, SnapshotInfo, parseVMInfo, parseSnapshots, parseVMNames |
| mount_operations.go | Mount JSON parsing (getVMMounts) for multipass info --format json |
| constants.go | VM defaults, limits, naming config, Ubuntu releases |
| utils.go | truncateToRunes, randomString |
| version.go | GetVersion() for build info |
| vm_operations.go | (Stub; VM logic in multipass.go and messages.go) |
| snapshot_operations.go | (Stub; snapshot logic in multipass.go and messages.go) |
| llm.go | OpenAI-compatible LLM client (ChatMessage, ToolCall, ToolDef types) |
| agent.go | ReAct agent loop: LLM ↔ MCP tool execution with live p.Send() streaming |
| mcp_client.go | MCP client: spawns multipass-mcp subprocess, JSON-RPC over stdio |
| mcp_install.go | Auto-detect/download multipass-mcp binary from GitHub releases |
| view_chat.go | Chat panel (split view alongside table), viewport + text input |
| view_llm_settings.go | LLM settings form (base-url, api-key, model) |
| chat_messages.go | Chat-specific tea.Msg types (tool start/done, agent result, MCP ready) |
| config_llm.go | Config loading/saving for ~/.passgo/llm.conf |

## Message Flow

All messages are handled in `main.go`'s `Update()`. Who produces them:

| Message | Produced By | Handled In |
|---------|-------------|------------|
| vmListResultMsg | fetchVMListCmd, fetchVMListBackgroundCmd | main.Update |
| vmOperationResultMsg | stop/start/suspend/delete/recover/create/mount/umount cmds | main.Update |
| vmInfoResultMsg | fetchVMInfoCmd | main.Update (delegates to infoModel when on viewInfo) |
| snapshotListResultMsg | fetchSnapshotsCmd | main.Update |
| mountListResultMsg | fetchMountsCmd | main.Update |
| shellFinishedMsg | tea.ExecProcess callback (shell exit) | main.Update |
| confirmResultMsg | confirmModel (y/n, Enter) | main.Update |
| backToTableMsg | view_info, view_create, view_snapshots, view_mounts | main.Update |
| advCreateMsg | view_create (form submit) | main.Update |
| mountAddRequestMsg | view_mounts (mountManageModel) | main.Update |
| mountModifyRequestMsg | view_mounts (mountManageModel) | main.Update |
| mountModifySubmitMsg | view_mounts (mountModifyModel) | main.Update |
| chatToolStartMsg | agent.go (p.Send during tool exec) | main.Update → chatModel |
| chatToolDoneMsg | agent.go (p.Send after tool exec) | main.Update → chatModel |
| chatAgentResultMsg | agent goroutine (final response) | main.Update → chatModel |
| chatMCPReadyMsg | initMCPAndRunCmd goroutine | main.Update → chatModel |
| chatMCPInitDoneMsg | initMCPAndRunCmd goroutine | main.Update → chatModel |
| llmSettingsSavedMsg | llmSettingsModel save | main.Update |
| toastExpireMsg | tableModel (toast timer) | main.Update (always routes to table) |
| autoRefreshTickMsg | autoRefreshTickCmd (tea.Tick) | main.Update |
| infoRefreshTickMsg | infoRefreshTickCmd (tea.Tick) | main.Update (when on viewInfo) |

## View State Machine

`viewState` determines which child model is active and rendered. Call `setChildSizes()` when creating a child or switching views.

| viewState | Model | Keys | Notes |
|-----------|-------|------|-------|
| viewTable | tableModel | All shortcuts (h, c, C, [, ], p, d, r, s, n, m, M, ?, L, etc.) | Main VM list |
| viewHelp | helpModel | esc, enter, q | Read-only |
| viewVersion | versionModel | esc, enter, q | Read-only |
| viewInfo | infoModel | esc, i (refresh) | VM detail, live charts |
| viewLoading | loadingModel | (none) | Spinner; transitions on result msg |
| viewError | errorModel | esc, enter | Modal overlay |
| viewConfirm | confirmModel | y/n, left/right, enter | Yes/No for destructive ops |
| viewAdvCreate | advCreateModel | Form navigation, Enter, Esc | Advanced create form |
| viewSnapCreate | snapCreateModel | Form navigation | Create snapshot |
| viewSnapManage | snapManageModel | n (create), e (restore), d (delete), Esc | Snapshot tree |
| viewMountManage | mountManageModel | a (add), e (modify), d (remove), Esc | Mount list |
| viewMountAdd | mountAddModel | Form navigation | Add mount |
| viewMountModify | mountModifyModel | Form navigation | Modify mount |
| viewLLMSettings | llmSettingsModel | Form navigation | Edit LLM config |

## Key Conventions

- **Async ops**: Define Msg type in messages.go; return tea.Cmd that produces it. Root handles in Update.
- **Child models**: Receive width/height; call `setChildSizes()` when creating or on WindowSizeMsg.
- **Inline ops**: Set `busyVMs[name]` before cmd; clear on `vmOperationResultMsg`. User stays on table.
- **Context return**: `lastMountVM` and `lastSnapVM` track where to return after mount/snapshot ops complete.

## LLM Chat Integration

Split-view chat panel (? to toggle, Tab to switch focus, L for settings).

```
Table (60%) │ Chat Panel (40%)
             │ viewport (scrollable messages)
             │ text input
             │
             │ Agent Loop → LLM API (OpenAI-compatible)
             │     ↕
             │ MCP Client ──stdio──→ multipass-mcp binary
```

**Key design decisions:**
- **No `list_instances` tool** in multipass-mcp — the system prompt injects current VM state from PassGo's table so the LLM answers informational questions without tool calls
- **Single OpenAI-compatible client** — works with any endpoint (OpenRouter, Ollama, OpenAI, LiteLLM) via base-url swap
- **No interface abstraction** — just `LLMClient` struct with configurable base URL
- **Split view via `chatOpen bool`** — not a new viewState, just conditional `JoinHorizontal` in View()
- **Goroutine safety** — agent goroutines capture values upfront, communicate state back via messages only, never mutate model fields directly
- **MCP binary auto-download** — GitHub releases are .tar.gz/.zip archives that must be extracted (not raw binaries)
- **Config**: `~/.passgo/llm.conf` with 4 fields: base-url, api-key, model, mcp-binary

**Guardrails:**
- Tool name whitelisting against MCP tool list
- Context cancellation checked between iterations
- Conversation history capped at 50 messages (sliding window)
- LLM response body limited to 10MB
- MCP calls timeout after 60s, init after 15s
- MCP subprocess force-killed after 5s on Close()
- Chat entries capped at 200
