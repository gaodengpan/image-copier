# 快速入门：配置加密提供者默认实现

## 概述
本指南介绍如何使用 image-copier 的新功能——默认使用加密配置提供者。此功能将自动加密存储敏感信息（如 GitHub Token、注册表用户名密码等），并在运行时自动解密。

## 环境准备

### 1. 设置加密密钥
```bash
export ENCRYPT_KEY="your-32-character-encryption-key-here"
```

密钥长度至少32个字符，用于AES-256-GCM加密算法。

### 2. 验证环境设置
```bash
# 检查加密密钥是否设置
echo $ENCRYPT_KEY
```

## 配置初始化

### 1. 使用交互式向导创建配置
```bash
./image-copier config init
```

向导会自动加密您输入的敏感信息，如：
- GitHub Personal Access Token
- Registry Username
- Registry Password

### 2. 检查生成的配置文件
新生成的配置文件中，敏感字段将以 "encrypted:" 前缀开头：
```yaml
github:
  owner: "your-github-owner"
  token: "encrypted:AQAAAAAAAABCa29uZmlndXJhdGlvbiB0b2tlbg=="
registry:
  username: "encrypted:AQAAAAAAAAC1cGFzc3dvcmQgaGVyZQ=="
  password: "encrypted:AQAAAAAAAABzZWNyZXQgcGFzc3dvcmQ="
  host: "docker.io"
```

## 运行应用

### 1. 正常运行应用
```bash
./image-copier pull [options]
```

应用将自动解密配置中的敏感信息，无需额外操作。

### 2. 查看当前配置
```bash
./image-copier config show
```

## 向后兼容性

### 1. 使用旧配置文件
现有未加密的配置文件将继续正常工作。系统会检测并透明处理明文配置。

### 2. 逐步迁移
当您下次运行 `config init` 或修改配置时，新值将自动加密存储。

## 故障排除

### 1. 缺少加密密钥错误
如果您看到类似以下错误：
```
Error: encryption key not found in environment
```

请设置 `ENCRYPT_KEY` 环境变量：
```bash
export ENCRYPT_KEY="your-32-character-encryption-key-here"
```

### 2. 解密失败
如果遇到解密错误，请检查：
- `ENCRYPT_KEY` 是否正确
- 配置文件是否被意外修改
- 加密数据是否完整

## 开发者指南

### 1. 默认配置提供者
在代码中，默认配置提供者现在是 `EncryptedViperConfigProvider`：

```go
// 这会返回加密配置提供者
provider := config.NewDefaultConfigProvider(configPath)
cfg, err := provider.Load()
```

### 2. 手动加密值
如有需要，可以手动加密单个值：

```go
encryptor := encryption.NewConfigEncryptor()
encryptedValue, err := encryptor.Encrypt("sensitive-data")
if err != nil {
    // 处理解密错误
}
result := fmt.Sprintf("encrypted:%s", encryptedValue)
```

## 安全注意事项

1. **保护加密密钥**：不要将 `ENCRYPT_KEY` 硬编码在代码中
2. **传输安全**：确保配置文件在传输过程中也是安全的
3. **权限控制**：限制对配置文件和加密密钥的访问权限

## 升级路径

对于现有用户：
1. 设置 `ENCRYPT_KEY` 环境变量
2. 重新运行 `config init` 以利用加密功能（可选）
3. 现有配置将继续工作，无需更改