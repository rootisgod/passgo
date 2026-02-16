---
name: add-new-theme
description: Adds a new color theme to the PassGo TUI. Use when adding or modifying themes in themes.go, or when the user asks to add a theme, change colors, or customize the TUI appearance.
---

# Add New Theme (PassGo)

Adds a new theme to the PassGo TUI. Themes are defined in `themes.go` and drive all lipgloss styles via `rebuildStyles()` in `styles.go`.

## Checklist

```
- [ ] Add theme to themes slice in themes.go
- [ ] Provide all required color fields
- [ ] (Optional) Update currentThemeIndex default
```

---

## Step 1: Add theme to themes slice

In `themes.go`, append to the `themes` slice. Keys 1–9 map to indices 0–8; key 0 maps to index 9.

```go
// N - Theme Name
{
    Name: "Theme Name", Accent: "#XXXXXX", AccentLight: "#XXXXXX",
    Subtle: "#XXXXXX", Dimmed: "#XXXXXX", Highlight: "#XXXXXX",
    Surface: "#XXXXXX", Text: "#XXXXXX", TextMuted: "#XXXXXX",
    Running: "#XXXXXX", Stopped: "#XXXXXX", Suspended: "#XXXXXX", Deleted: "#XXXXXX",
},
```

---

## Required theme fields

| Field | Purpose |
|-------|---------|
| `Name` | Display name (shown in Help modal) |
| `Accent` | Primary accent (title bar, borders, buttons) |
| `AccentLight` | Lighter accent variant |
| `Subtle` | Muted/secondary text |
| `Dimmed` | Even more muted (dividers, inactive) |
| `Highlight` | Emphasized text (selected, values) |
| `Surface` | Background for buttons, panels |
| `Text` | Normal cell text |
| `TextMuted` | Secondary text (descriptions, values) |
| `Running` | VM state: Running |
| `Stopped` | VM state: Stopped |
| `Suspended` | VM state: Suspended |
| `Deleted` | VM state: Deleted |

Use hex colors (e.g. `#7C3AED`). All fields are `lipgloss.Color`.

---

## Step 2: Optional — change default theme

`currentThemeIndex` in `themes.go` sets the theme at startup. Default is 8 (Solarized). To make a new theme the default, set:

```go
var currentThemeIndex = 9  // or index of your new theme
```

---

## Key bindings

Users switch themes with keys 1–9 and 0. No changes needed when adding a theme — `setTheme()` in `main.go` handles indices 0–9, and `rebuildStyles()` is called automatically.

---

## Reference: existing themes

- 1: Violet, 2: Dracula, 3: Tokyo Night, 4: One Dark, 5: Monokai
- 6: Nord, 7: Catppuccin, 8: Gruvbox, 9: Solarized, 0: Rosé Pine
