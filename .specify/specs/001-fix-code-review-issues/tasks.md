# Tasks: 001-fix-code-review-issues

## Phase 1: Fix Code Review Issues

### 1.1 Fix DistributeTask unused state methods
- [x] Done
- **Do**:
  1. Read `internal/domain/entities/distribute_task.go`
  2. Read `internal/application/usecases/sync_command_usecase.go`
  3. Integrate entity state machine into UseCase flow OR remove unused methods
  4. If integrating: call `Start()`, `Complete()`, `Fail()` at appropriate points
  5. If removing: delete unused methods and simplify entity
- **Files**: `internal/domain/entities/distribute_task.go`, `internal/application/usecases/sync_command_usecase.go`
- **Done when**: Entity methods are either used or removed
- **Verify**: `go build ./...` and `go test ./...`
- **Commit**: `refactor(domain): integrate or remove DistributeTask state methods`

### 1.2 Fix potential race condition in removeTask
- [x] Done
  1. Read `internal/application/usecases/sync_command_usecase.go:207-210`
  2. Change `removeTask` function to use `Source` field comparison instead of pointer
  3. Update function signature: `removeTask(tasks []*entities.SyncTask, source string) []*entities.SyncTask`
  4. Update all call sites to pass `t.Source` instead of `t`
- **Files**: `internal/application/usecases/sync_command_usecase.go`
- **Done when**: Task removal uses source identifier, not pointer
- **Verify**: `go test -v ./internal/application/usecases/... -run TestSync`
- **Commit**: `fix(usecase): use source field for task removal comparison`

### 1.3 Add nil checks in Presenter
- [x] Done
  1. Read `internal/adapters/primary/cli/sync_presenter.go:241-242`
  2. Add nil checks before calling `PresentSyncPhaseResult` and `PresentDistributePhaseResult`
  3. Handle nil case gracefully (skip or log warning)
- **Files**: `internal/adapters/primary/cli/sync_presenter.go`
- **Done when**: No panic on nil phase results
- **Verify**: `go build ./...` and `go test ./...`
- **Commit**: `fix(cli): add nil checks for phase results in presenter`

### 1.4 [VERIFY] Run tests and verify fixes
- **Do**:
  1. Run `go build ./...` to verify compilation
  2. Run `go test ./...` to verify all tests pass
  3. Run `make test-coverage` to check coverage
- **Files**: N/A
- **Done when**: All tests pass, coverage maintained
- **Verify**: All commands exit with 0
- **Commit**: N/A (verification only)