# 安全配置管理功能快速入门指南

## 1. 概述

本指南将帮助您快速了解并使用 image-copier 的新安全配置管理功能。该功能实现了敏感信息（如账户密码、访问令牌等）的加密存储，并在运行时自动解密使用。

## 2. 环境设置

### 2.1 设置加密密钥

首先，您需要设置环境变量 `ENCRYPT_KEY`，它将用于加密和解密敏感配置值：

```bash
export ENCRYPT_KEY="your-very-secure-key-at-least-32-characters-long"
```

**注意**: 请确保密钥至少有 32 个字符，并且足够安全。在生产环境中，请使用安全的密钥管理工具。

### 2.2 验证环境设置

验证密钥是否已正确设置：

```bash
echo $ENCRYPT_KEY
```

## 3. 加密配置值

### 3.1 使用加密工具

要加密配置值，请使用提供的加密工具。例如，加密 GitHub Token：

```bash
# 假设有一个加密工具命令（实现时会提供）
go run cmd/encrypt/main.go "your-github-token"
```

该命令将返回加密后的值，格式如下：
```
encrypted:AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz==
```

### 3.2 更新配置文件

将配置文件中的敏感值替换为加密值。例如，原始配置文件：

```yaml
github:
  owner: "my-org"
  repo: "my-repo"
  token: "ghp_original_token_here"  # 原始明文令牌
  workflow_id: "image-copier-v2.yaml"

registry:
  host: "docker.io"
  username: "myuser"                # 原始明文用户名
  password: "mypassword"            # 原始明文密码
  namespace: "my-namespace"
```

更新为加密配置：

```yaml
github:
  owner: "my-org"
  repo: "my-repo"
  token: "encrypted:AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz=="
  workflow_id: "image-copier-v2.yaml"

registry:
  host: "docker.io"
  username: "encrypted:BbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZzAa=="
  password: "encrypted:CcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZzAaBb=="
  namespace: "my-namespace"
```

**注意**: 非敏感字段（如 owner、repo、host、namespace）不需要加密。

## 4. 运行应用

在设置了 `ENCRYPT_KEY` 环境变量的情况下运行应用：

```bash
# 确保环境变量已设置
export ENCRYPT_KEY="your-very-secure-key-at-least-32-characters-long"

# 运行应用
./image-copier
```

应用将在启动时自动解密配置中的加密字段，并使用解密后的值进行操作。

## 5. 错误处理

### 5.1 解密失败

如果解密失败，应用将显示错误消息并停止运行：

```
Error: Failed to decrypt config field 'github.token': decryption failed, possibly due to incorrect key
```

常见原因：
- `ENCRYPT_KEY` 环境变量未设置或值不正确
- 配置文件中的加密值已损坏
- 加密时使用的密钥与当前使用的密钥不同

### 5.2 配置格式错误

如果配置值格式不正确，应用将显示类似错误：

```
Error: Invalid encrypted value format for field 'registry.password': must start with 'encrypted:'
```

## 6. 开发者集成

### 6.1 使用加密/解密函数

在代码中使用加密/解密功能：

```go
import "path/to/image-copier/internal/encryption"

// 加密值
encryptedValue, err := encryption.Encrypt("my-sensitive-data", keyBytes)
if err != nil {
    // 处理解密错误
}

// 解密值
decryptedValue, err := encryption.Decrypt(encryptedValue, keyBytes)
if err != nil {
    // 处理解密错误
}
```

### 6.2 加载加密配置

使用更新后的配置加载器：

```go
import "path/to/image-copier/internal/config"

// 此函数将自动处理加密字段的解密
cfg, err := config.LoadEncryptedConfig()
if err != nil {
    // 处理加载错误
}
```

## 7. 最佳实践

1. **密钥管理**: 使用安全的密钥管理工具（如 HashiCorp Vault、AWS Secrets Manager）来管理加密密钥
2. **最小权限**: 只加密真正敏感的配置值
3. **备份**: 妥善备份加密密钥，以防丢失导致无法解密配置
4. **监控**: 监控解密失败的情况，及时发现配置问题

## 8. 常见问题

### Q: 我可以混用加密和非加密配置值吗？
A: 可以。非敏感字段可以继续保持明文，只有需要保护的字段才需要加密。

### Q: 加密会影响应用性能吗？
A: 加密/解密操作非常快（通常在毫秒级别），对应用性能影响很小。

### Q: 如何测试加密配置？
A: 可以在测试环境中使用固定的测试密钥，为测试创建加密的配置值。