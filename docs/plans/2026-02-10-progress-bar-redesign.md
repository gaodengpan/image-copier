# Progress Bar Redesign: Replace Custom Implementation with mpb

## Problem

The batch command's progress bar suffers from cursor position drift and flickering, even in single-line mode. Root causes:

- Multiple goroutines calling `Display()` concurrently, causing rapid terminal writes
- Using raw `\r` (carriage return) without proper line clearing, leaving residual characters
- No output buffering or refresh rate limiting

## Decision

Replace the custom progress bar implementation in `pkg/progress/progress.go` with [`vbauerster/mpb/v8`](https://github.com/vbauerster/mpb), a mature multi-progress-bar library designed for concurrent scenarios.

### Why mpb

- Built for multi-goroutine concurrent progress updates (exactly our batch worker pool scenario)
- Handles terminal rendering internally with proper buffering and refresh rate
- Auto-detects terminal width
- Well-maintained, ~2.3k GitHub stars

### Why not self-fix

Fixing the custom implementation requires adding output buffering, refresh rate limiting, and ANSI escape codes for line clearing. This effectively means rewriting a progress bar library while also handling terminal compatibility (especially Windows `cmd.exe`).

## Display Design

### Default (and only) mode

The `-v` / `--verbose` flag is removed. There is one unified display:

```
[4/10] ████████████████████░░░░░░░░░░ 40.0% [pulling images]
  ◐ redis:alpine
  ◐ postgres:15
  ◐ mysql:8.0
```

- **Top line**: total progress bar managed by `mpb.Bar`
- **Below**: one spinner bar per currently running image, created on `StatusRunning`, removed on `StatusCompleted` / `StatusFailed`

### Completion summary

```
[10/10] ██████████████████████████████ 100.0% [done]

Summary: 8 succeeded, 2 failed
  ✗ redis:alpine: connection timeout
  ✗ mysql:8.0: unauthorized
```

## API Design

### Public types (unchanged)

```go
type ImageStatus int
const (
    StatusPending   ImageStatus = iota
    StatusRunning
    StatusCompleted
    StatusFailed
)
```

### Public methods

| Method | Description |
|--------|-------------|
| `NewProgress(total int) *Progress` | Create progress manager, start mpb render loop |
| `AddImage(index int, image string)` | Register an image at the given index |
| `UpdateStatus(index int, status ImageStatus, err error)` | Update image status; Running creates a spinner bar, Completed/Failed removes it |
| `Increment()` | Increment the total progress bar by 1 |
| `Wait()` | Wait for mpb render to finish, print completion summary |

### Removed API

| Removed | Reason |
|---------|--------|
| `Display()` | mpb handles rendering automatically |
| `SetWidth(int)` | mpb auto-detects terminal width |
| `BatchProgress` struct | Unnecessary wrapper, use `Progress` directly |
| `NewProgress` `showDetails` param | Only one display mode now |

## Files Changed

| File | Change |
|------|--------|
| `go.mod` | Add `github.com/vbauerster/mpb/v8` |
| `pkg/progress/progress.go` | Rewrite using mpb |
| `pkg/progress/progress_test.go` | Update tests for new API |
| `internal/cli/batch.go` | Remove `-v` flag, adapt to new Progress API |

## Calling Flow

```
NewProgress(total)
AddImage() x N
                        ┌─ worker goroutines ──────────────┐
                        │ UpdateStatus(idx, Running, nil)   │ → spinner bar appears
                        │ PullSingle(ctx, image)            │
                        │ UpdateStatus(idx, Done/Fail, err) │ → spinner bar removed
                        │ Increment()                       │ → main bar +1
                        └───────────────────────────────────┘
Wait()  → mpb stops render loop, print summary
```

Key improvement: rendering is fully handled by mpb's internal goroutine with proper buffering and refresh rate. Worker goroutines only update data, never write to terminal directly.
