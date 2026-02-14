# 研究文档：配置加密提供者默认实现

## 1. 现状分析

### 1.1 当前配置系统结构
当前项目在 `internal/config/config.go` 中实现了完整的配置管理系统：

- `Config` 结构体定义了所有配置项
- `ConfigProvider` 接口定义了配置加载的通用方法
- `ViperConfigProvider` 作为默认实现
- `EncryptedViperConfigProvider` 提供加密/解密功能，但不是默认实现

### 1.2 加密系统现状
项目已在 `internal/encryption/` 目录下实现了完整的加密系统：

- `crypto.go` 提供底层 AES-256-GCM 加密算法
- `ConfigEncryptor` 和 `ConfigDecryptor` 处理配置值的加密/解密
- 支持使用 "encrypted:" 前缀识别加密值
- 通过环境变量 `ENCRYPT_KEY` 获取加密密钥

### 1.3 CLI 配置向导现状
在 `internal/cli/wizard.go` 中实现了配置向导：

- 支持交互式配置设置
- 会在 `config init` 时提示用户输入敏感信息
- 目前直接将明文保存到配置文件

## 2. 技术决策

### 2.1 决策：默认配置提供者
**选择**: 将 `EncryptedViperConfigProvider` 设置为默认配置提供者
**理由**:
- 安全优先：所有敏感信息默认加密存储
- 透明解密：运行时自动解密，不影响现有业务逻辑
- 向后兼容：仍可读取未加密的旧配置

### 2.2 决策：配置向导加密
**选择**: 在配置向导中自动加密敏感字段
**理由**:
- 用户体验：用户只需输入明文，系统自动加密
- 安全保障：防止敏感信息以明文形式保存
- 一致性：确保所有新创建的配置都是加密的

### 2.3 决策：错误处理策略
**选择**: 在缺少加密密钥时优雅降级
**理由**:
- 可用性：系统仍可在无密钥时运行（尽管安全性较低）
- 渐进迁移：允许逐步迁移到加密配置
- 用户友好：提供清晰错误信息指导用户设置密钥

## 3. 技术实现要点

### 3.1 配置提供者工厂
需要创建工厂函数来返回加密配置提供者：

```go
func NewDefaultConfigProvider(configPath string) ConfigProvider {
    return &EncryptedViperConfigProvider{
        ViperConfigProvider: &ViperConfigProvider{configPath: configPath},
    }
}
```

### 3.2 配置向导加密逻辑
在 `internal/cli/wizard.go` 中需要添加加密逻辑：

```go
func encryptFieldIfNeeded(value string, fieldName string) (string, error) {
    // 检查字段是否属于需要加密的类型
    if isSensitiveField(fieldName) {
        encryptor := encryption.NewConfigEncryptor()
        encryptedValue, err := encryptor.Encrypt(value)
        if err != nil {
            return "", err
        }
        return fmt.Sprintf("encrypted:%s", encryptedValue), nil
    }
    return value, nil
}
```

### 3.3 向后兼容性实现
`EncryptedViperConfigProvider` 需要能够处理混合配置（部分加密、部分明文）：

```go
func (p *EncryptedViperConfigProvider) Load() (*Config, error) {
    // 先加载基础配置
    config, err := p.ViperConfigProvider.Load()
    if err != nil {
        return nil, err
    }

    // 对配置进行解密
    decryptedConfig, err := p.decryptConfig(config)
    if err != nil {
        // 如果解密失败，根据策略决定是否降级
        return p.handleDecryptionFailure(config, err)
    }

    return decryptedConfig, nil
}
```

## 4. 实现挑战与解决方案

### 4.1 挑战：配置向导中的实时加密
**问题**: 如何在配置向导过程中对用户输入的敏感信息进行加密
**解决方案**: 在 `wizard.go` 中添加辅助函数，在写入配置文件前加密敏感字段

### 4.2 挑战：错误处理的一致性
**问题**: 加密密钥缺失时如何优雅地处理
**解决方案**:
- 实现降级机制，允许在没有密钥时使用明文模式
- 提供清晰的错误信息和解决方案指导

### 4.3 挑战：向后兼容性
**问题**: 确保能够读取旧版本的未加密配置
**解决方案**:
- 检测配置值是否已加密（通过 "encrypted:" 前缀）
- 只对加密值进行解密，明文值保持不变

## 5. 测试策略

### 5.1 单元测试重点
- 验证加密配置提供者正确加载并解密配置
- 验证配置向导正确加密敏感字段
- 验证向后兼容性：能正确处理明文配置
- 验证错误处理：密钥缺失时的降级行为

### 5.2 集成测试重点
- 端到端测试：配置初始化 -> 加密保存 -> 加载解密 -> 正常使用
- 回归测试：确保现有功能不受影响

## 6. 部署注意事项

### 6.1 环境变量设置
用户必须设置 `ENCRYPT_KEY` 环境变量才能启用加密功能。

### 6.2 迁移路径
- 新用户：默认使用加密配置
- 现有用户：可以逐步迁移到加密配置，不影响当前运行

### 6.3 安全提醒
在配置文件中明确标记哪些字段已被加密，确保用户了解其配置的安全状态。