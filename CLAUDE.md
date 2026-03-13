# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

image-copier 是一个通过 GitHub Actions 中转拉取海外 Docker 镜像到国内 Registry 的 CLI 工具。

核心流程：`用户 → GitHub Actions (海外) → 国内 Registry → 本地 Docker`

## 基本命令

```bash
make build              # 构建
make test               # 测试（本地开发）
make test-ci            # CI 模式测试（含 race 检测 + 覆盖率）
make test-coverage      # 测试 + 覆盖率
go test -v ./path/to/pkg -run TestName   # 单个测试
make fmt                # 格式化
make vet                # go vet
make lint               # golangci-lint 检查
make check-quality      # 全量检查（CI 前必跑）
```

**重要**：提交代码前必须运行 `make check-quality`，它会执行：
1. `gofmt` + `goimports` 格式化
2. `go vet` 静态检查
3. `golangci-lint` 代码质量检查
4. `-race` 竞态条件检测

## 架构原则

### 分层架构（Clean + Hexagonal）

```
cmd/                          # 入口点
internal/
├── adapters/                 # 适配层
│   ├── primary/cli/          # 输入适配器（CLI → UseCase）
│   └── secondary/gateways/   # 输出适配器（实现 output ports）
├── application/usecases/     # 应用层（编排业务流程）
├── domain/                   # 领域层（核心业务逻辑）
│   ├── entities/             # 实体：有行为的数据模型
│   ├── value_objects/        # 值对象：不可变概念
│   ├── ports/                # 端口：抽象接口
│   │   ├── input/            #   UseCase 接口定义
│   │   └── output/           #   基础设施接口
│   ├── services/             # 领域服务：跨实体逻辑
│   └── validators/           # 验证器
├── infrastructure/           # 基础设施（配置、加密）
└── shared/errors/            # 共享错误类型
pkg/                          # 可复用公共包
```

### 核心原则

| 原则 | 规则 | 示例 |
|------|------|------|
| **依赖倒置** | UseCase → `output` 接口，禁止直接依赖实现 | `sync_images.go` 仅依赖 `output.DockerClient` |
| **单一职责** | 每层只做一件事 | CLI 解析参数，UseCase 编排流程，Gateway 调外部服务 |
| **接口隔离** | 端口接口小而专注 | `DockerClient` 仅 3 个方法 |
| **实体行为** | 实体包含业务方法 | `SyncTask.Start()`, `SyncTask.Complete()` |

### 依赖流向

```
┌─────────────────────────────────────────────────────┐
│                    CLI (primary)                    │
│                        ↓                            │
│                   UseCase                           │
│                    ↓   ↘                            │
│              Domain Layer    ←── Ports (output)     │
│           (entities, services)        ↑             │
│                                      Gateways       │
│                                   (secondary)       │
└─────────────────────────────────────────────────────┘
```

**关键规则**：
- 外层可依赖内层，内层禁止依赖外层
- UseCase 通过 `ports/output` 接口与外部交互
- 所有具体实现通过 `AdapterFactory` 创建

### 层职责速查

| 层 | 职责 | 禁止 |
|---|------|------|
| CLI | 参数解析、调用 UseCase、输出格式化 | 业务逻辑、直接调用 Gateway |
| UseCase | 编排流程、业务规则校验 | 直接操作外部服务 |
| Domain | 核心业务、实体行为、领域规则 | 依赖框架、I/O 操作 |
| Gateway | 封装外部服务调用 | 业务逻辑 |

## 开发规范

### 领域建模
- 实体包含业务行为（如 `SyncTask.Start()`, `SyncTask.Complete()`）
- 值对象封装概念（`Architecture`, `OperatingSystem`）

### 错误分类
- `AdapterError` - 基础设施层错误（Docker/Registry/GitHub）
- `ValidationError` - 配置验证错误
- `DomainError` - 领域层错误

### 测试
- 文件：`<name>_test.go`
- 断言：`testify/assert` + `testify/require`
- Mock：位于 `internal/application/usecases/mocks/`
- **提交前检查**：`make check-quality` 必须通过
- **CI 环境**：使用 `make test-ci`，包含 `-race` 竞态检测

### CI 常见问题排查

#### 1. errcheck: 返回值未检查
```
Error: Error return value of `vp.viper.BindEnv` is not checked
```
**修复**：用 `_ =` 显式忽略返回值
```go
// 错误
vp.viper.BindEnv("key", "ENV_KEY")

// 正确
_ = vp.viper.BindEnv("key", "ENV_KEY")
```

#### 2. race detected: 竞态条件
```
testing.go:1490: race detected during execution of test
```
**原因**：使用 `time.Sleep` 等待 goroutine 完成
**修复**：使用 `sync.WaitGroup` 同步
```go
// 错误 - 不确定等待时间
for i := 0; i < 10; i++ {
    go doSomething()
}
time.Sleep(100 * time.Millisecond)

// 正确 - 确保所有 goroutine 完成
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        doSomething()
    }()
}
wg.Wait()
```

#### 3. 测试缺少环境变量
```
Error: github owner is required / registry host is required
```
**原因**：CI 环境没有预设环境变量
**修复**：在测试中用 `t.Setenv()` 设置
```go
func TestSomething(t *testing.T) {
    t.Setenv("GITHUB_OWNER", "test-owner")
    t.Setenv("GITHUB_REPO", "test-repo")
    t.Setenv("GITHUB_TOKEN", "test-token")
    t.Setenv("REGISTRY_HOST", "ghcr.io")
    t.Setenv("REGISTRY_USERNAME", "test-user")
    t.Setenv("REGISTRY_PASSWD", "test-pass")
    // ...
}
```

#### 4. gosimple: 不必要的代码
```
S1039: unnecessary use of fmt.Sprintf
```
**修复**：简化字符串格式化
```go
// 错误
return fmt.Sprintf("docker (local)")

// 正确
return "docker (local)"
```

### Git 提交
- 格式：conventional commits（`feat:`, `fix:`, `docs:`）
- 语气：祈使句

## 技术栈

- Go >= 1.24.0
- CLI: cobra + viper
- 日志: logrus
- 进度条: mpb
- 静态编译: `CGO_ENABLED=0`