# Image Copier

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.24-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub Actions](https://img.shields.io/badge/GitHub-Actions-2088FF?style=flat&logo=github-actions)](https://github.com/features/actions)

通过 GitHub Actions 中转，帮助国内开发者高速拉取海外 Docker 镜像。

## 核心特性

- **两阶段同步**：Phase 1 同步到中转 Registry，Phase 2 分发到多个目标
- **多目标分发**：支持同时分发到 Docker 和多个私有 Registry
- **并发控制**：自适应 Worker 数量，信号量模式限制并发
- **增量同步**：智能检测已存在镜像，避免重复拉取
- **安全加密**：AES-256-GCM 加密保护敏感配置
- **实时进度**：可视化 Worker 状态和进度跟踪
- **声明式管理**：YAML 清单批量管理镜像

## 架构设计

### 分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI (Primary Adapter)                    │
│                          ↓                                  │
│                    UseCase Layer                            │
│                      ↓   ↘                                  │
│                Domain Layer    ←── Ports (Output)           │
│           (Entities, Services)          ↑                   │
│                                      Gateways               │
│                                   (Secondary Adapters)      │
└─────────────────────────────────────────────────────────────┘
```

**核心原则**：
- **依赖倒置**：UseCase 依赖接口，不依赖实现
- **单一职责**：每层专注单一职责
- **实体行为**：实体包含业务方法（如 `SyncTask.Start()`, `SyncTask.Complete()`）

### 同步流程

```
┌──────────────┐     ┌───────────────────┐     ┌─────────────────┐
│   用户 CLI   │ ──→ │  GitHub Actions   │ ──→ │  中转 Registry  │
└──────────────┘     │   (海外服务器)     │     │   (国内镜像源)  │
                     └───────────────────┘     └─────────────────┘
                                                          │
                                                          ↓
                     ┌───────────────────┐     ┌─────────────────┐
                     │   本地 Docker     │ ←── │   分发策略      │
                     │   私有 Registry   │     │   (多目标并行)  │
                     └───────────────────┘     └─────────────────┘
```

## 快速开始

### 安装

```bash
# Go install
go install github.com/gaodengpan/image-copier/cmd/image-copier@latest

# 或从 Releases 下载
# https://github.com/gaodengpan/image-copier/releases
```

### 初始化配置

```bash
image-copier config init
```

按提示填写 GitHub Token、国内 Registry 凭证等信息。

### 基本用法

```bash
# 同步单个镜像（默认分发到 Docker）
image-copier sync nginx:latest

# 批量同步
image-copier sync nginx:latest redis:7-alpine

# YAML 清单模式
image-copier sync -f images.yaml
```

## 高级功能

### 两阶段同步

```bash
# 完整流程：同步 + 分发
image-copier sync nginx:latest

# 仅同步到中转 Registry
image-copier sync nginx:latest --skip-distribute

# 仅分发（跳过同步阶段）
image-copier sync nginx:latest --skip-sync
```

### 多目标分发

```bash
# 分发到多个目标
image-copier sync nginx:latest --target docker --target my-registry

# 仅分发到私有 Registry
image-copier sync nginx:latest --target my-registry --skip-sync
```

### YAML 清单模式

创建 `images.yaml`：

```yaml
images:
  - source: ghcr.io/tektoncd/pipeline/controller:v1.1.0
    platforms: [linux/amd64, linux/arm64]
  - source: docker.io/library/nginx:1.25
```

执行同步：

```bash
# 预览模式（不执行）
image-copier sync -f images.yaml --dry-run

# 增量同步（跳过已存在镜像）
image-copier sync -f images.yaml

# 强制全量同步
image-copier sync -f images.yaml --force

# 并发控制
image-copier sync -f images.yaml -j 5

# 超时控制
image-copier sync -f images.yaml --timeout 5m

# JSON 输出（适合脚本处理）
image-copier sync -f images.yaml -o json
```

## 命令行选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `-j, --jobs` | 并发 Worker 数量 | 3（自适应） |
| `-f, --file` | YAML 清单路径 | - |
| `--force` | 强制重新拉取 | false |
| `--dry-run` | 预览模式 | false |
| `-v, --verbose` | 详细日志 | false |
| `--arch` | 指定架构 | 配置值 |
| `--os` | 指定操作系统 | 配置值 |
| `-o, --output` | 输出格式（text/json） | text |
| `--timeout` | 超时时间（如 5m, 1h） | 无限制 |
| `--target` | 分发目标（可多次指定） | docker |
| `--skip-sync` | 跳过同步阶段 | false |
| `--skip-distribute` | 跳过分发阶段 | false |

## 输出状态

| 符号 | 含义 |
|------|------|
| ✓ | 同步成功 |
| ◦ | 已跳过（镜像已存在） |
| ✗ | 同步失败 |
| ⊘ | 同步取消（超时） |
| ~ | 预览模式 |

## 配置

### 配置文件

位置：`~/.config/image-copier/config.yaml`

```yaml
github:
  owner: "your-github-username"
  repo: "image-copier"
  token: "encrypted:XXXXX"
  workflow_id: "image-copier-v2.yaml"

registry:
  host: "registry.cn-hangzhou.aliyuncs.com"
  username: "your-username"
  password: "encrypted:YYYYY"
  namespace: "your-namespace"
  arch: "amd64"
  os: "linux"

log_level: "info"
```

### 环境变量

| 配置项 | 环境变量 |
|--------|----------|
| `github.owner` | `IMAGE_COPIER_GITHUB_OWNER` |
| `github.repo` | `IMAGE_COPIER_GITHUB_REPO` |
| `github.token` | `IMAGE_COPIER_GITHUB_TOKEN` |
| `registry.host` | `IMAGE_COPIER_REGISTRY_HOST` |
| `registry.username` | `IMAGE_COPIER_REGISTRY_USERNAME` |
| `registry.password` | `IMAGE_COPIER_REGISTRY_PASSWD` |
| `registry.namespace` | `IMAGE_COPIER_REGISTRY_NAMESPACE` |

### 配置加密

敏感配置自动使用 AES-256-GCM 加密存储。

```bash
# 设置加密密钥
export IMAGE_COPIER_ENCRYPT_KEY="your-encryption-key"

# 初始化配置（自动加密）
image-copier config init

# 使用时设置相同密钥
export IMAGE_COPIER_ENCRYPT_KEY="your-encryption-key"
image-copier sync nginx:latest
```

**密钥要求**：
- 建议至少 16 位强密码
- 加密和解密必须使用同一密钥
- 密钥丢失后无法恢复，需重新初始化

## 前置准备

1. **本地依赖**：
   - Docker
   - Skopeo

2. **国内 Registry**：
   - 阿里云 ACR
   - 腾讯云 TCR
   - 其他支持 Docker Registry API 的服务

3. **GitHub 配置**：
   - Fork 本仓库或创建包含 workflow 的仓库
   - 添加 Secret：`DEST_CREDS`（Registry 凭证）
   - 生成 Personal Access Token（需要 `repo` + `workflow` 权限）

## 开发

### 构建

```bash
make build              # 编译
make test               # 运行测试
make test-coverage      # 测试 + 覆盖率
make fmt                # 格式化代码
make vet                # go vet
make check-quality      # 全量检查
```

### 项目结构

```
cmd/
└── image-copier/           # 入口点

internal/
├── adapters/               # 适配层
│   ├── primary/cli/        # 输入适配器（CLI → UseCase）
│   └── secondary/gateways/ # 输出适配器（实现 Output Ports）
├── application/usecases/   # 应用层（编排业务流程）
├── domain/                 # 领域层（核心业务逻辑）
│   ├── entities/           # 实体：有行为的数据模型
│   ├── value_objects/      # 值对象：不可变概念
│   ├── ports/              # 端口：抽象接口
│   ├── services/           # 领域服务
│   └── validators/         # 验证器
├── infrastructure/         # 基础设施（配置、加密）
├── shared/                 # 共享组件
└── utils/                  # 工具函数

pkg/                        # 可复用公共包
```

### 架构原则

| 原则 | 说明 |
|------|------|
| 依赖倒置 | UseCase → Output 接口，禁止直接依赖实现 |
| 单一职责 | 每层只做一件事 |
| 接口隔离 | 端口接口小而专注 |
| 实体行为 | 实体包含业务方法 |

## 故障排查

| 问题 | 解决方案 |
|------|----------|
| 配置加密失败 | 确认 `IMAGE_COPIER_ENCRYPT_KEY` 未变化，重建配置 |
| Workflow 触发失败 | 检查 Token 权限（需要 `repo` + `workflow`） |
| skopeo copy 失败 | 验证 Registry 凭证正确性 |
| 进度卡住 | GitHub Actions 排队中，等待或检查 Actions 页面 |
| 分发失败 | 检查目标 Registry 是否可达、凭证是否正确 |

**调试模式**：

```bash
image-copier sync -v nginx:latest
```

## 迁移指南

### 从 pull 命令迁移

`pull` 命令已被移除，请使用功能更完整的 `sync` 命令：

| pull 命令 | sync 等效命令 |
|-----------|--------------|
| `pull nginx:latest` | `sync nginx:latest` |
| `pull -f images.yaml` | `sync -f images.yaml` |
| `pull --target docker` | `sync`（默认分发到 Docker） |
| `pull --target registry --registry myreg` | `sync --target myreg` |

**sync 命令优势**：
- 两阶段同步：先同步到中转 Registry，再分发到多个目标
- 多目标分发：`--target` 可多次指定
- 灵活控制：`--skip-sync`、`--skip-distribute` 可单独执行某阶段

## 技术栈

- **语言**：Go >= 1.24.0
- **CLI 框架**：cobra + viper
- **日志**：logrus
- **进度条**：mpb
- **加密**：AES-256-GCM
- **静态编译**：`CGO_ENABLED=0`

## 贡献

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

[MIT License](LICENSE)