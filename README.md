# Image Copier

借助 GitHub Actions 中转，帮助国内开发者高速拉取海外 Docker 镜像。

## 🚀 核心特性

- **安全配置管理**：采用 AES-256-GCM 加密算法保护敏感信息（令牌、密码等）
- **高效镜像传输**：通过 GitHub Actions 中转，绕过网络限制，高速拉取海外镜像
- **声明式同步**：支持 YAML 清单文件实现增量镜像同步
- **并发处理**：支持多线程并发操作，显著提升处理效率
- **健壮性设计**：内置重试机制和全面的错误处理，增强应用稳定性
- **模块化架构**：基于 Go 语言的现代 CLI 架构，易于维护和扩展

## 🔐 安全特性

Image Copier 采用先进的安全措施来保护用户的敏感配置信息：

### 配置加密
- **加密算法**：使用 AES-256-GCM 对称加密算法，提供行业级安全性
- **自动加密**：配置初始化过程中，敏感字段（如 GitHub Token、Registry 密码）会被自动加密
- **透明解密**：运行时自动解密配置项，无需手动干预

### 安全最佳实践
- **最小权限原则**：严格遵循最小权限访问模型
- **安全存储**：避免在内存和磁盘中明文存储敏感信息
- **配置验证**：启动时验证配置完整性，防止配置被篡改

### 加密工作流程
1. **初始化**：运行 `image-copier config init` 时，输入的敏感信息自动加密
2. **存储**：加密后的数据以 `encrypted:XXXXXXX` 格式存储在配置文件中
3. **运行时**：应用程序加载配置时自动解密，用于实际操作
4. **安全性**：即使配置文件被泄露，也无法直接获取原始敏感信息

## 使用场景

国内网络环境下，直接拉取 Docker Hub、ghcr.io 等海外仓库的镜像往往非常缓慢甚至超时。Image Copier 通过以下方式解决这个问题：

1. 自动触发 GitHub Actions 在海外服务器上拉取目标镜像
2. 推送到你的国内 Registry（如阿里云 ACR）
3. 从国内 Registry 高速下载到本地并导入 Docker

整个过程只需一条命令，支持批量处理、并发和声明式同步。

## 工作原理

```
用户本机                    GitHub Actions                国内 Registry
  │                              │                            │
  │── 触发 workflow ──────────>  │                            │
  │                              │── pull 海外镜像 ────>      │
  │                              │── push 到国内 ──────────>  │
  │<──────── skopeo copy ────────────────────────────────────  │
  │── docker load                │                            │
  │                              │                            │
  ✓ 镜像就绪
```

### 系统架构

```
┌─────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│   CLI 层        │    │   配置管理层     │    │   操作执行层     │
│                │    │                 │    │                 │
│ - Cobra 命令   │◄──►│ - Viper 配置    │◄──►│ - GitHub API    │
│ - 参数解析     │    │ - AES 加密/解密 │    │ - Skopeo 操作   │
│ - 进度显示     │    │ - 环境变量支持  │    │ - Docker 操作   │
└─────────────────┘    └──────────────────┘    └──────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│   核心业务逻辑   │    │   错误处理层     │    │   并发控制层     │
│                │    │                 │    │                 │
│ - 镜像同步      │◄──►│ - 重试机制      │◄──►│ - Worker Pool   │
│ - 清单处理      │    │ - 异常捕获      │    │ - 任务调度      │
│ - 幂等性保证    │    │ - 状态监控      │    │ - 进度跟踪      │
└─────────────────┘    └──────────────────┘    └──────────────────┘
```

## 高级功能

### 重试机制
- **智能重试**：支持指数退避算法，最大重试次数、初始间隔和最大间隔均可配置
- **容错处理**：自动处理网络超时、服务暂时不可用等问题
- **配置选项**：
  ```yaml
  retry:
    max_attempts: "3"              # 最大重试次数
    initial_interval: "2s"         # 初始重试间隔
    max_interval: "30s"            # 最大重试间隔
  ```

### 并发处理
- **多线程支持**：通过 `-j` 或 `--jobs` 参数指定并发数
- **资源优化**：智能任务分配，避免过多并发导致的资源竞争
- **进度可视化**：实时显示各任务进度和总体进度

### 声明式同步
- **YAML 清单**：通过清单文件定义需要同步的镜像列表
- **增量同步**：只同步目标 Registry 中缺失的镜像，提高效率
- **平台过滤**：支持按平台架构（如 linux/amd64）过滤镜像

## 前置准备

使用 Image Copier 前，需要完成以下三项准备工作。

### 1. 本地环境依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| [Docker](https://docs.docker.com/get-docker/) | - | 本地镜像管理 |
| [Skopeo](https://github.com/containers/skopeo) | - | Registry 间镜像复制 |
| [Go](https://go.dev/dl/) | 1.24+ | 仅从源码构建时需要 |

**macOS：**

```bash
brew install docker skopeo
```

**Ubuntu/Debian：**

```bash
sudo apt-get update && sudo apt-get install docker.io skopeo
```

### 2. 国内 Registry 账号

你需要一个国内容器镜像仓库（如阿里云 ACR、腾讯云 TCR 等），准备好以下信息：

- Registry 地址（如 `registry.cn-hangzhou.aliyuncs.com`）
- 用户名和密码
- 命名空间（可选）

### 3. GitHub 仓库与 Actions 配置

Image Copier 依赖你自己的 GitHub 仓库来运行 Actions Workflow，需要完成以下步骤：

1. **Fork 本仓库**（或确保你的仓库中包含 `.github/workflows/image-copier-v2.yaml`）
2. **配置仓库 Secret**：进入仓库 Settings → Secrets and variables → Actions，添加：
   - `DEST_CREDS`：国内 Registry 凭证，格式为 `username:password`（即上一步准备的账号信息）
3. **生成 GitHub Personal Access Token**：
   - 访问 https://github.com/settings/tokens
   - 勾选 `repo` 和 `workflow` 权限
   - 保存生成的 token（后续配置 CLI 时需要填入）

## 安装

### 方式一：Go Install（推荐 Go 开发者）

```bash
go install github.com/gaodengpan/image-copier/cmd/image-copier@latest
```

### 方式二：下载预编译二进制

从 [GitHub Releases](https://github.com/gaodengpan/image-copier/releases) 下载适合你平台的压缩包：

```bash
# 示例：Linux amd64
tar -xzf image-copier_*_linux_amd64.tar.gz
sudo mv image-copier /usr/local/bin/
```

### 方式三：从源码构建

```bash
git clone https://github.com/gaodengpan/image-copier.git
cd image-copier

# 构建
make build

# 安装到 /usr/local/bin（可选）
make install
```

### 验证安装

```bash
image-copier --version
```

## 快速开始

### 1. 初始化配置

```bash
image-copier config init
```

交互式向导会引导你填写 GitHub 和 Registry 信息（即前置准备中获取的 Token、凭证等），配置文件保存在 `~/.config/image-copier/config.yaml`。

### 2. 拉取镜像

```bash
image-copier pull nginx:latest
```

你会看到进度条实时展示拉取进度：

```
[0/1] ████████████████████████░░░░░░░░  75% pulling
  ◐ nginx:latest        [ 75%] workflow running (32s)
```

## 使用示例

### 拉取单个镜像

```bash
image-copier pull nginx:latest
image-copier pull ghcr.io/owner/repo:tag
image-copier pull redis:7-alpine
```

镜像名称会自动补全——`nginx` 等价于 `docker.io/library/nginx:latest`。

### 批量拉取

```bash
# 命令行传入多个镜像
image-copier pull nginx:latest redis:7-alpine postgres:15
```

### 声明式批量拉取（YAML manifest）

通过 `-f` 指定 YAML manifest 文件，实现增量镜像同步——只拉取目标 Registry 中缺失的镜像。

**创建 manifest 文件** `images.yaml`：

```yaml
images:
  - source: ghcr.io/tektoncd/pipeline/controller:v1.1.0
    platforms: [linux/amd64, linux/arm64]    # 可选，默认使用配置中的 arch/os
  - source: ghcr.io/nginx/nginx-gateway-fabric:2.0.1
  - source: docker.io/library/nginx:1.25
    platforms: [linux/amd64]
```

**执行同步**：

```bash
# 预览同步计划（不实际执行）
image-copier pull -f images.yaml --dry-run

# 执行增量同步
image-copier pull -f images.yaml

# 强制全量重新同步 + 并发 5
image-copier pull -f images.yaml --force -j 5 -v
```

`-f` 模式分两阶段运行：
1. **Diff 阶段**：并发检查所有镜像在目标 Registry 的存在性，输出差异报告
2. **Sync 阶段**：仅拉取缺失的镜像，带进度条显示

### 常用选项

```bash
# 并发数（默认 3）
image-copier pull -j 5 -f images.yaml

# 强制重新拉取（即使本地已有）
image-copier pull --force nginx:latest

# 显示详细日志（在进度条上方滚动输出）
image-copier pull -v nginx:latest

# 指定架构
image-copier pull --arch arm64 nginx:latest

# 组合使用
image-copier pull -v -j 5 --force -f images.yaml

# 预览模式（不实际执行任何操作）
image-copier pull --dry-run nginx:latest redis:alpine
```

### 查看配置

```bash
image-copier config show
```

## 配置参考

配置文件位于 `~/.config/image-copier/config.yaml`，也可通过环境变量覆盖。

```yaml
github:
  owner: "your-github-username"              # GitHub 用户名或组织
  repo: "image-copier"                       # 仓库名
  token: "encrypted:XXXXXXX"                 # 加密后的 Personal Access Token
  workflow_id: "image-copier-v2.yaml"        # Workflow 文件名

registry:
  host: "registry.cn-hangzhou.aliyuncs.com"  # 国内 Registry 地址
  username: "your-username"                  # Registry 用户名
  password: "encrypted:YYYYYYY"              # 加密后的 Registry 密码
  namespace: "your-namespace"                # 命名空间（可选）
  arch: "amd64"                              # 镜像架构（默认 amd64）
  os: "linux"                                # 操作系统（默认 linux）

retry:
  max_attempts: "3"                          # 最大重试次数
  initial_interval: "2s"                     # 初始重试间隔
  max_interval: "30s"                        # 最大重试间隔

log_level: "info"                            # 日志级别：debug/info/warn/error
```

**环境变量映射：**

| 配置项 | 环境变量 |
|--------|---------|
| `github.owner` | `GITHUB_OWNER` |
| `github.repo` | `GITHUB_REPO` |
| `github.token` | `GITHUB_TOKEN` |
| `registry.host` | `REGISTRY_HOST` |
| `registry.username` | `REGISTRY_USERNAME` |
| `registry.password` | `REGISTRY_PASSWD` |
| `registry.namespace` | `REGISTRY_NAMESPACE` |

## 故障排查

| 问题 | 解决方案 |
|------|----------|
| 配置加密/解密失败 | 检查主密钥是否正确，确保配置文件未被手动编辑破坏加密格式 |
| Workflow 触发失败 | 检查 `github.token` 是否有 `repo` + `workflow` 权限 |
| skopeo copy 失败 | 检查 Registry 凭证是否正确，`skopeo login` 测试连接 |
| docker load 失败 | 确认 Docker daemon 正在运行 |
| 进度卡在 workflow running | GitHub Actions 排队中，耐心等待或检查 Actions 页面 |
| pull -f 报 "invalid platform format" | 检查 manifest 中 platforms 格式是否为 `os/arch`（如 `linux/amd64`） |
| 加密配置无法读取 | 确认加密密钥未发生变化，如果重装请重新初始化配置 |

**开启调试日志：**

```bash
# 方式一：命令行 verbose 模式
image-copier pull -v nginx:latest

# 方式二：配置文件设置 debug 级别
# 修改 config.yaml 中 log_level: "debug"
```

## 开发贡献

欢迎社区贡献！以下是参与项目开发的相关信息：

### 项目结构
```
image-copier/
├── cmd/                    # CLI 入口点
│   └── image-copier/
├── internal/               # 内部包
│   ├── config/             # 配置管理和加密
│   ├── github/             # GitHub API 操作
│   ├── registry/           # Registry 相关操作
│   └── utils/              # 工具函数
├── pkg/                    # 公共包
├── test/                   # 测试文件
└── .github/workflows/      # CI/CD 配置
```

### 测试策略
- **单元测试**：覆盖核心业务逻辑和工具函数
- **集成测试**：验证端到端功能
- **安全测试**：定期扫描依赖包漏洞和加密功能
- **运行命令**：
  ```bash
  # 运行所有测试
  make test

  # 运行覆盖率测试
  make coverage

  # 运行安全扫描
  make security-check
  ```

### 开发环境
1. **安装依赖**：
   ```bash
   # 安装 Go 1.24+
   brew install go  # macOS

   # 安装 Docker 和 Skopeo
   brew install docker skopeo  # macOS
   ```

2. **构建项目**：
   ```bash
   # 构建可执行文件
   make build

   # 运行开发版本
   make run
   ```

## 命令参考

```
image-copier pull [IMAGE...] [flags]

Flags:
      --arch string    Image architecture (e.g., amd64, arm64)
  -f, --file string    Path to YAML manifest file
      --force          Force re-pull even if image exists locally
  -j, --jobs int       Number of concurrent workers (default 3)
      --os string      Operating system (e.g., linux)
  -v, --verbose        Show detailed logs
      --dry-run        Dry run mode, no actual execution

image-copier config show     Show current configuration
image-copier config init     Interactive configuration creation
```
