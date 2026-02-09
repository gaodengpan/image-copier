# Image Copier 增强设计文档

**日期**: 2026-02-09
**目标**: 将现有的 Bash 脚本工具重写为 Go CLI 工具，解决国外镜像国内无法拉取的问题

---

## 一、背景

当前仓库是一个 Docker 镜像同步工具，用于将国外镜像（如 Docker Hub）通过 GitHub Actions 同步到国内仓库，然后从国内仓库拉取到本地。现有实现为 Bash 脚本，功能有限。

---

## 二、增强目标

1. **批量模式** - 支持一次同步多个镜像
2. **CLI 工具** - 提供友好的命令行界面
3. **配置化** - 提取关键变量，让用户 fork 后可直接使用
4. **多架构同步** - 支持指定架构进行同步
5. **失败重试** - 网络不稳定时自动重试
6. **环境变量验证** - 启动时检查必需配置
7. **日志级别** - 支持 quiet/normal/verbose 三级输出
8. **Go 重写** - 更好的跨平台支持和维护性

---

## 三、架构概述

工具采用 Go 语言重写，设计为一个单一可执行文件，无需额外依赖。整体采用 CLI 架构，核心分为三层：

### 3.1 配置层
- 从配置文件、环境变量和命令行参数读取配置
- 支持首次运行时的交互式配置向导
- 配置项包括：目标镜像仓库地址、仓库访问凭证、GitHub 仓库信息、GitHub token

### 3.2 业务逻辑层
- 镜像解析和标准化
- GitHub workflow 触发
- 状态轮询
- 镜像拉取
- 失败重试
- 多架构同步
- 批量处理和进度管理

### 3.3 适配层
- GitHub API 客户端
- Skopeo 命令调用
- 本地 Docker 交互

---

## 四、CLI 命令结构

主命令：`imgcp`

### 4.1 核心命令

| 命令 | 说明 |
|------|------|
| `imgcp pull <image>` | 拉取单个镜像 |
| `imgcp pull <image1 image2 ...>` | 批量拉取（命令行参数） |
| `imgcp pull @images.txt` | 批量拉取（从文件读取） |
| `imgcp config init` | 交互式初始化/更新配置 |
| `imgcp config show` | 查看当前配置 |
| `imgcp config set <key> <value>` | 单独设置配置项 |
| `imgcp version` | 显示版本信息 |

### 4.2 全局参数

| 参数 | 说明 |
|------|------|
| `-v/--verbose` | 详细输出 |
| `-q/--quiet` | 静默模式 |
| `-c/--config <path>` | 指定配置文件路径（默认 `~/.imgcp/config.yaml`） |

### 4.3 Pull 命令参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--arch <arch>` | 镜像架构 | amd64 |
| `--os <os>` | 镜像操作系统 | linux |
| `--multi-arch` | 同步所有可用架构 | false |
| `-f/--file <path>` | 从文件读取镜像列表 | - |
| `-j/--jobs <n>` | 并发数 | 3 |

---

## 五、配置管理

### 5.1 配置优先级

环境变量 > 命令行参数 > 配置文件

### 5.2 配置文件结构

```yaml
# imgcp 配置文件

# 目标镜像仓库
registry:
  host: registry.example.com       # 仓库地址
  namespace: my-namespace          # 命名空间（可选）
  username: your-username           # 访问用户名
  password: your-password           # 访问密码

# GitHub 工作流配置
github:
  owner: your-github-username      # 仓库所有者
  repo: image-copier               # 仓库名称
  workflow: image-copier-v2.yaml   # workflow 文件名
  token: gh_xxxxxxxxxxxxxxxxxxxx   # GitHub token

# 重试配置
retry:
  max_attempts: 3                  # 最大重试次数
  initial_interval: 1s             # 初始重试间隔
  max_interval: 30s                # 最大重试间隔

# 默认架构配置
default:
  arch: amd64                      # 默认架构
  os: linux                        # 默认系统
```

### 5.3 交互式向导

首次运行时逐项询问配置信息：
- 提供默认值和示例
- 实时验证（如检查 GitHub token 有效性）
- 支持跳过已有配置的更新

---

## 六、核心业务流程

### 6.1 单个镜像拉取流程

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ 1. 镜像解析 │ -> │ 2. 配置校验 │ -> │ 3. 构建目标 │
└─────────────┘    └─────────────┘    └─────────────┘
                                                     │
┌─────────────┐    ┌─────────────┐    ┌─────────────┘
│ 8. 成功反馈 │ <- │ 7. 镜像拉取 │ <- │ 4. 存在检查 │
└─────────────┘    └─────────────┘    └─────────────┘
                                          │ 不存在
                                          ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ 6. 状态轮询 │ <- │ 5. 触发同步 │ <- │   API调用   │
└─────────────┘    └─────────────┘    └─────────────┘
```

1. **镜像解析** - 标准化镜像 ID，补全前缀
2. **配置校验** - 检查必需配置
3. **构建目标** - 生成目标镜像地址
4. **存在检查** - Skopeo 检查是否已同步
5. **触发同步** - GitHub API 触发 workflow
6. **状态轮询** - 定期查询并显示进度
7. **镜像拉取** - 同步完成后拉取到本地
8. **成功反馈** - 显示完成信息

### 6.2 批量流程

- 支持并发控制（可配置并发数）
- 显示总进度 `[3/10] ████████░░░░░░`
- 单个失败不影响其它镜像
- 最后汇总失败项报告

---

## 七、错误处理和重试

### 7.1 重试策略

所有外部调用支持重试：
- GitHub API 调用：默认 3 次，间隔递增
- 镜像拉取：默认 3 次，间隔递增

可配置参数：
- `max_attempts` - 最大重试次数
- `initial_interval` - 初始重试间隔
- `max_interval` - 最大重试间隔

### 7.2 错误分类

| 类型 | 说明 | 可重试 |
|------|------|--------|
| 网络超时 | 连接超时、DNS 解析失败 | 是 |
| API 限流 | GitHub API 速率限制 | 是 |
| 认证失败 | Token 无效、密码错误 | 否 |
| 镜像不存在 | 源镜像不存在 | 否 |

### 7.3 错误码

每个错误都有对应的错误码和建议修复方式，例如：
- `ERR_CFG_MISSING` - 配置缺失
- `ERR_GITHUB_API` - GitHub API 错误
- `ERR_IMAGE_NOT_FOUND` - 镜像不存在
- `ERR_SYNC_FAILED` - 同步失败

---

## 八、进度显示和用户体验

### 8.1 进度条

批量模式下显示进度：
```
[3/10] ████████░░░░░░ nginx:latest syncing...
      [2/10] ██████░░░░░░░ redis:alpine completed
      [1/10] ████░░░░░░░░░ mysql:8.0 failed
```

### 8.2 多架构模式

- 优先同步用户请求的架构（`--arch amd64`）
- 需要多架构时使用 `--multi-arch` 同步多清单镜像

### 8.3 日志级别

| 级别 | 说明 | 用途 |
|------|------|------|
| quiet | 仅错误和关键信息 | 自动化脚本 |
| normal | 标准输出（默认） | 日常使用 |
| verbose | 详细调试信息 | 问题排查 |

### 8.4 彩色输出

- 绿色 - 成功信息
- 黄色 - 警告信息
- 红色 - 错误信息
- 灰色 - 调试信息（verbose）

---

## 九、技术栈

### 9.1 语言和版本

- Go 1.21+

### 9.2 主要依赖

| 包 | 用途 |
|----|------|
| github.com/spf13/cobra | CLI 框架 |
| github.com/spf13/viper | 配置管理 |
| gopkg.in/yaml.v3 | YAML 解析 |

其余使用标准库，减少第三方依赖。

### 9.3 项目结构

```
imgcp/
├── cmd/
│   └── root.go           # 主程序入口
├── internal/
│   ├── config/           # 配置管理
│   │   └── config.go
│   ├── resolver/         # 镜像名称解析
│   │   └── resolver.go
│   ├── github/           # GitHub API 交互
│   │   └── client.go
│   ├── skopeo/           # Skopeo 调用
│   │   └── executor.go
│   └── pull/             # 拉取业务逻辑
│       └── puller.go
├── pkg/
│   ├── retry/            # 重试机制
│   │   └── retry.go
│   └── logger/           # 日志模块
│       └── logger.go
├── configs/
│   └── config.yaml       # 配置模板
├── go.mod
├── go.sum
├── README.md
└── Makefile              # 构建脚本
```

---

## 十、构建和分发

### 10.1 构建目标

| 平台 | 架构 |
|------|------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |

### 10.2 分发方式

- GitHub Releases 发布预编译二进制文件
- Homebrew tap（可选）
- Docker 镜像（可选）

---

## 十一、后续改进方向

1. 支持本地缓存优化，记录已同步镜像列表
2. 提供 Docker 镜像分发
3. 结构化日志输出（JSON 格式）
4. 支持更多镜像仓库（如 ACR、ECR）