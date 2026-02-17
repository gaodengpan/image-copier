# Image Copier

借助 GitHub Actions 中转，帮助国内开发者高速拉取海外 Docker 镜像。

## 特性

- **安全加密**：AES-256-GCM 加密保护敏感配置（Token、密码）
- **高速中转**：通过 GitHub Actions 绕过网络限制
- **增量同步**：YAML 清单声明式同步，只拉取缺失镜像
- **并发处理**：多线程并行拉取
- **双重模式**：支持命令行直接拉取和 YAML 清单批量拉取

## 原理

```
用户 → GitHub Actions (海外) → 国内 Registry → 本地 Docker
```

## 快速开始

### 1. 安装

```bash
go install github.com/gaodengpan/image-copier/cmd/image-copier@latest
```

或从 [Releases](https://github.com/gaodengpan/image-copier/releases) 下载二进制。

### 2. 初始化配置

```bash
image-copier config init
```

按提示填写 GitHub Token、国内 Registry 凭证等信息。

### 3. 拉取镜像

```bash
# 单个镜像
image-copier pull nginx:latest

# 批量镜像
image-copier pull nginx:latest redis:7-alpine

# YAML 清单（增量同步）
image-copier pull -f images.yaml
```

## 使用示例

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
# 预览（不执行）
image-copier pull -f images.yaml --dry-run

# 执行增量同步
image-copier pull -f images.yaml

# 强制全量同步 + 并发 5
image-copier pull -f images.yaml --force -j 5

# 带超时的同步（超时后取消未完成的镜像）
image-copier pull -f images.yaml --timeout 5m

# JSON 模式输出（适合脚本处理）
image-copier pull -f images.yaml -o json
```

### 常用选项

| 选项 | 说明 |
|------|------|
| `-j, --jobs` | 并发数（默认 3） |
| `-f, --file` | YAML 清单路径 |
| `--force` | 强制重新拉取 |
| `--dry-run` | 预览模式 |
| `-v, --verbose` | 显示详细日志 |
| `--arch` | 指定架构（如 arm64） |
| `-o, --output` | 输出格式：text 或 json |
| `--timeout` | 批量同步超时时间（如 5m, 1h），默认不超时 |

## 输出说明

### 状态符号

| 符号 | 含义 |
|------|------|
| ✓ | 同步成功 |
| ◦ | 已跳过（本地已存在） |
| ✗ | 同步失败 |
| ⊘ | 同步取消（超时） |
| ~ | 预览模式 |

## 配置

配置文件：`~/.config/image-copier/config.yaml`

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
| `github.owner` | `GITHUB_OWNER` |
| `github.repo` | `GITHUB_REPO` |
| `github.token` | `GITHUB_TOKEN` |
| `registry.host` | `REGISTRY_HOST` |
| `registry.username` | `REGISTRY_USERNAME` |
| `registry.password` | `REGISTRY_PASSWD` |
| `registry.namespace` | `REGISTRY_NAMESPACE` |

## 配置加密

敏感配置（Token、密码）自动使用 AES-256-GCM 加密存储。

### 启用加密

1. **设置加密密钥**（初始化前）：
   ```bash
   export ENCRYPT_KEY="你的加密密钥"
   ```

2. **初始化配置**（自动加密）：
   ```bash
   image-copier config init
   ```

### 运行时的密钥

每次使用 CLI 时需设置相同密钥：
```bash
export ENCRYPT_KEY="你的加密密钥"
image-copier pull nginx:latest
```

或写入 shell 配置文件（如 `~/.bashrc`）永久生效。

### 密钥要求
- 建议使用强密码（至少 16 位）
- 加密和解密必须使用同一密钥
- 密钥丢失后无法解密，需重新初始化配置

## 前置准备

1. **本地依赖**：Docker + Skopeo
2. **国内 Registry**：阿里云 ACR、腾讯云 TCR 等
3. **GitHub 配置**：
   - Fork 本仓库或创建包含 workflow 的仓库
   - 添加 Secret：`DEST_CREDS`（Registry 凭证）
   - 生成 Personal Access Token（需要 `repo` + `workflow` 权限）

## 构建

```bash
make build    # 编译
make test     # 测试
make install  # 安装到 /usr/local/bin
```

## 故障排查

| 问题 | 解决 |
|------|------|
| 配置加密失败 | 确认主密钥未变化，重建配置 |
| Workflow 触发失败 | 检查 Token 权限 |
| skopeo copy 失败 | 验证 Registry 凭证 |
| 进度卡住 | GitHub Actions 排队中，等待或检查 Actions 页面 |

调试：`image-copier pull -v nginx:latest`
