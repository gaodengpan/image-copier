# Homebrew 风格输出设计

## 背景

当前 sync 命令使用 `mpb` 库实现多进度条显示，风格较复杂。用户希望改为类似 Homebrew 的简洁风格。

## 设计目标

- 使用 Spinner 动画显示进度
- 多行列表显示并发任务
- 简洁的阶段状态（Downloading/Uploading）
- 符号状态显示（✓ ✗）
- 静默模式（成功时只显示进度）

---

## 第一部分：输出格式设计

### 1.1 运行时输出

```
⣾ Downloading nginx:latest...
⣷ Downloading redis:alpine...
⣯ Checking python:3.11...
```

**特点**：
- 每个镜像一行
- Braille spinner 动画（`⣾⣷⣯⣟⡿⢿⣻⣽⣾` 循环）
- 简洁阶段名称

### 1.2 阶段映射

| 当前阶段 | Homebrew 风格 |
|---------|--------------|
| checking | Checking |
| sync | Downloading |
| dist | Uploading |

### 1.3 完成状态

```
✓ nginx:latest (5s)
✓ redis:alpine (3s)
✗ python:3.11 (10s): connection timeout
```

**状态符号**：
- `✓` 成功
- `✗` 失败
- `◦` 跳过（已存在）

### 1.4 最终摘要

```
==> Summary
✓ 5 succeeded, 2 skipped, 1 failed in 30s
```

---

## 第二部分：架构设计

### 2.1 文件变更

| 操作 | 文件路径 |
|------|---------|
| 新建 | `pkg/progress/homebrew.go` |
| 修改 | `pkg/progress/progress.go` |
| 删除 | mpb 相关代码 |
| 修改 | `internal/adapters/primary/cli/sync.go` |

### 2.2 新组件：HomebrewProgress

```go
// pkg/progress/homebrew.go

type HomebrewProgress struct {
    mu          sync.Mutex
    tasks       []*TaskLine
    spinner     []rune
    spinnerIdx  int
    interval    time.Duration
    output      io.Writer
    isTerminal  bool
}

type TaskLine struct {
    ImageName   string
    Stage       string      // "Checking", "Downloading", "Uploading"
    Status      TaskStatus  // Pending, Running, Done
    Error       error
    StartTime   time.Time
    EndTime     time.Time
}
```

### 2.3 核心方法

```go
// 添加任务
func (p *HomebrewProgress) AddTask(imageName string) int

// 更新阶段
func (p *HomebrewProgress) UpdateStage(taskID int, stage string)

// 完成任务
func (p *HomebrewProgress) CompleteTask(taskID int, err error)

// 渲染循环
func (p *HomebrewProgress) render()
```

---

## 第三部分：实现细节

### 3.1 Spinner 动画

```go
var brailleSpinner = []rune{'⣾', '⣷', '⣯', '⣟', '⡿', '⢿', '⣻', '⣽'}
```

以 100ms 间隔循环显示。

### 3.2 终端控制

- 使用 ANSI 转义序列控制光标
- `\033[K` 清除行
- `\033[A` 上移一行
- 检测 TTY 自动禁用动画（CI 环境）

### 3.3 并发安全

- 使用 `sync.Mutex` 保护任务列表
- 渲染在单独 goroutine 中运行
- 使用 context 控制停止

### 3.4 阶段简化逻辑

```go
func simplifyStage(stage value_objects.SyncStage) string {
    switch stage {
    case value_objects.SyncStageChecking:
        return "Checking"
    case value_objects.SyncStageSync:
        return "Downloading"
    case value_objects.SyncStageDist:
        return "Uploading"
    default:
        return "Processing"
    }
}
```

---

## 第四部分：验证计划

### 4.1 手动测试

```bash
# 构建并运行
make build
./image-copier sync nginx:latest

# 测试并发
./image-copier sync nginx:latest redis:alpine python:3.11

# 测试失败场景
./image-copier sync invalid-image:v1

# 测试 JSON 输出（应不受影响）
./image-copier sync nginx:latest --output json
```

### 4.2 边界条件

- [ ] 非 TTY 环境（CI）不显示动画
- [ ] JSON 输出模式正常工作
- [ ] Ctrl+C 正确停止渲染
- [ ] 长镜像名称正确截断

---

## 第五部分：依赖变更

### 移除依赖

- `github.com/vbauerster/mpb/v8`
- `github.com/vbauerster/mpb/v8/decor`

### 新增依赖

- 无（纯标准库实现）