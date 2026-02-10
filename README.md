# Image Copier

借助 GitHub Actions 中转，帮助国内开发者高速拉取海外 Docker 镜像。

## 使用场景

国内网络环境下，直接拉取 Docker Hub、ghcr.io 等海外仓库的镜像往往非常缓慢甚至超时。Image Copier 通过以下方式解决这个问题：

1. 自动触发 GitHub Actions 在海外服务器上拉取目标镜像
2. 推送到你的国内 Registry（如阿里云 ACR）
3. 从国内 Registry 高速下载到本地并导入 Docker

整个过程只需一条命令，支持批量处理和并发。

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

# 从文件读取镜像列表
image-copier pull -f images.txt
```

镜像列表文件格式（每行一个，`#` 开头为注释）：

```txt
nginx:latest
redis:7-alpine
# 数据库
postgres:15
mysql:8
```

### 常用选项

```bash
# 并发数（默认 3）
image-copier pull -j 5 -f images.txt

# 强制重新拉取（即使本地已有）
image-copier pull --force nginx:latest

# 显示详细日志（在进度条上方滚动输出）
image-copier pull -v nginx:latest

# 指定架构
image-copier pull --arch arm64 nginx:latest

# 组合使用
image-copier pull -v -j 5 --force -f images.txt
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
  token: "ghp_xxxxx"                         # Personal Access Token
  workflow_id: "image-copier-v2.yaml"        # Workflow 文件名

registry:
  host: "registry.cn-hangzhou.aliyuncs.com"  # 国内 Registry 地址
  username: "your-username"                  # Registry 用户名
  password: "your-password"                  # Registry 密码
  namespace: "your-namespace"                # 命名空间（可选）
  arch: "amd64"                              # 镜像架构（默认 amd64）
  os: "linux"                                # 操作系统（默认 linux）

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

| 问题 | 排查方向 |
|------|---------|
| Workflow 触发失败 | 检查 `github.token` 是否有 `repo` + `workflow` 权限 |
| skopeo copy 失败 | 检查 Registry 凭证是否正确，`skopeo login` 测试连接 |
| docker load 失败 | 确认 Docker daemon 正在运行 |
| 进度卡在 workflow running | GitHub Actions 排队中，耐心等待或检查 Actions 页面 |

**开启调试日志：**

```bash
# 方式一：命令行 verbose 模式
image-copier pull -v nginx:latest

# 方式二：配置文件设置 debug 级别
# 修改 config.yaml 中 log_level: "debug"
```

## 命令参考

```
image-copier pull [IMAGE...] [flags]

Flags:
      --arch string    镜像架构 (如 amd64, arm64)
  -f, --file string    镜像列表文件路径
      --force          强制重新拉取
  -j, --jobs int       并发数 (默认 3)
      --multi-arch     同步所有架构（实验性）
      --os string      操作系统 (如 linux)
  -v, --verbose        显示详细日志

image-copier config show     显示当前配置
image-copier config init     交互式创建配置
```
