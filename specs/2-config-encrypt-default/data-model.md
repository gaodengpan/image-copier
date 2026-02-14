# 数据模型：配置加密提供者默认实现

## 1. 配置数据模型

### 1.1 Config 结构体
```go
type Config struct {
    Github  GithubConfig  `mapstructure:"github"`
    Registry RegistryConfig `mapstructure:"registry"`
    Retry   RetryConfig   `mapstructure:"retry"`
    // ... 其他配置项
}
```

### 1.2 敏感字段识别
- `Github.Token`: GitHub 个人访问令牌
- `Registry.Username`: 注册表用户名
- `Registry.Password`: 注册表密码
- 这些字段需要自动加密存储

## 2. 配置提供者模型

### 2.1 接口定义
```go
type ConfigProvider interface {
    Load() (*Config, error)
    GetConfigPath() string
}
```

### 2.2 实现模型
- `ViperConfigProvider`: 基础实现，支持从文件和环境变量加载
- `EncryptedViperConfigProvider`: 扩展实现，提供自动加密/解密功能

## 3. 加密数据格式

### 3.1 加密值格式
```go
// 加密后的值格式
"encrypted:<base64_encoded_encrypted_data>"
```

### 3.2 验证规则
- 所有以 "encrypted:" 开头的值被视为已加密
- 加密值必须能成功解密，否则视为配置错误
- 非加密值保持原样处理

## 4. 状态转换

### 4.1 配置加载状态
```
配置文件 -> 读取 -> 检测加密字段 -> 解密 -> 应用使用
```

### 4.2 配置保存状态
```
用户输入 -> 检测敏感字段 -> 加密 -> 保存到文件
```

## 5. 数据验证规则

### 5.1 加密字段验证
- 敏感字段（Token、Username、Password）必须以 "encrypted:" 开头（如果是加密的）
- 加密字段必须能够成功解密
- 解密后的值必须符合原有验证规则

### 5.2 非加密字段验证
- 非敏感字段保持原有验证规则
- 不受加密/解密过程影响

## 6. 错误处理数据流

### 6.1 解密失败处理
- 捕获解密错误
- 根据配置决定是否降级到明文模式
- 提供详细的错误信息

### 6.2 密钥缺失处理
- 检测 ENCRYPT_KEY 环境变量是否存在
- 如果缺失，根据策略决定是否允许降级
- 提供清晰的错误信息指导用户设置密钥

## 7. 数据模型变更

### 7.1 向后兼容性
- 旧格式（明文）配置文件继续支持
- 新格式（加密）配置文件自动处理
- 混合格式（部分加密、部分明文）支持

### 7.2 数据格式升级
- 新创建的配置默认使用加密格式
- 现有配置可通过迁移工具升级（非必需）
- 数据结构保持不变，仅值的存储格式变化