# API 契约：配置加密提供者默认实现

## 1. 配置提供者接口

### 1.1 ConfigProvider 接口
```go
// 定义配置提供者的通用接口
type ConfigProvider interface {
    // Load 加载配置并返回配置对象
    // 返回: (*Config, error)
    // 错误情况: 配置文件不存在、格式错误、解密失败等
    Load() (*Config, error)

    // GetConfigPath 获取配置文件路径
    // 返回: 配置文件的路径字符串
    GetConfigPath() string
}
```

### 1.2 配置提供者工厂函数
```go
// NewDefaultConfigProvider 创建默认配置提供者实例（返回加密提供者）
// 参数: configPath - 配置文件路径
// 返回: ConfigProvider - 加密配置提供者实例
// 作用: 创建新的默认配置提供者实例，该实例将使用加密功能
func NewDefaultConfigProvider(configPath string) ConfigProvider
```

## 2. 加密相关接口

### 2.1 敏感字段加密函数
```go
// encryptSensitiveFields 对配置数据中的敏感字段进行加密
// 参数: *ConfigData - 待处理的配置数据指针
// 返回: error - 处理结果或错误
// 作用: 加密配置数据中的敏感字段
func encryptSensitiveFields(configData *ConfigData) error
```

### 2.2 加密状态检测函数
```go
// needsEncryption 检测字段值是否需要加密
// 参数: string - 字段值
// 返回: bool - 是否需要加密
// 作用: 检测字段是否为加密格式
func needsEncryption(value string) bool
```

## 3. 配置验证接口

### 3.1 配置验证器接口
```go
// ConfigValidator 定义配置验证接口
type ConfigValidator interface {
    // Validate 在读取时验证加密配置字段的有效性和完整性
    // 参数: *Config - 待验证的配置对象
    // 返回: error - 验证结果或错误
    Validate(config *Config) error
}
```

### 3.2 具体验证函数
```go
// validateEncryptedFields 验证加密字段的有效性
// 参数: *Config - 配置对象
// 返回: error - 验证错误（如果有）
// 作用: 在读取配置文件时验证加密字段的有效性和完整性
func validateEncryptedFields(config *Config) error
```

## 4. CLI 命令接口

### 4.1 配置初始化命令
```go
// configInitCmd 配置初始化命令，会自动加密敏感字段
// 参数: 无（通过交互式向导获取输入）
// 返回: 无（创建加密配置文件）
// 作用: 初始化配置文件并自动加密敏感字段
var configInitCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize configuration file with encrypted sensitive fields",
    Long:  "Interactively create a configuration file, automatically encrypting sensitive fields like passwords and tokens.",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 实现加密配置初始化逻辑
    },
}
```

## 5. 错误处理接口

### 5.1 解密错误处理
```go
// handleDecryptionFailure 处理解密失败情况
// 参数: *Config - 原始配置，error - 解密错误
// 返回: *Config, error - 处理后的配置和可能的错误
// 作用: 在解密失败时根据策略决定是否降级到明文模式
func handleDecryptionFailure(originalConfig *Config, decryptErr error) (*Config, error)
```

### 5.2 密钥缺失处理
```go
// checkEncryptionKey 检查加密密钥是否可用
// 参数: 无
// 返回: bool, error - 是否有密钥，错误信息
// 作用: 检查 ENCRYPT_KEY 环境变量是否存在且有效
func checkEncryptionKey() (bool, error)
```

## 6. 向导界面扩展

### 6.1 加密向导助手
```go
// encryptFieldIfNeeded 如果字段需要则进行加密
// 参数: value string - 字段值, fieldName string - 字段名
// 返回: string, error - 加密后的值或错误
// 作用: 在配置向导过程中对敏感字段进行加密
func encryptFieldIfNeeded(value string, fieldName string) (string, error)
```

## 7. 向后兼容接口

### 7.1 混合配置处理器
```go
// processMixedConfig 处理混合配置（部分加密、部分明文）
// 参数: *Config - 配置对象
// 返回: *Config, error - 处理后的配置和可能的错误
// 作用: 透明处理混合格式的配置，只解密已加密的字段
func processMixedConfig(config *Config) (*Config, error)
```

## 8. 安全接口

### 8.1 实时解密接口
```go
// decryptOnDemand 按需解密配置值（不缓存）
// 参数: encryptedValue string - 加密值
// 返回: string, error - 解密后的值和错误
// 作用: 实现不缓存的实时解密，以最大化安全性
func decryptOnDemand(encryptedValue string) (string, error)
```

## 9. 数据模型契约

### 9.1 配置数据结构
```go
type Config struct {
    Github  GithubConfig  `mapstructure:"github"`  // GitHub 相关配置
    Registry RegistryConfig `mapstructure:"registry"` // 注册表相关配置
    Retry   RetryConfig   `mapstructure:"retry"`   // 重试相关配置
}

type GithubConfig struct {
    Owner      string `mapstructure:"owner"`       // GitHub 仓库所有者
    Token      string `mapstructure:"token"`       // GitHub 个人访问令牌（敏感）
}

type RegistryConfig struct {
    Username   string `mapstructure:"username"`    // 注册表用户名（敏感）
    Password   string `mapstructure:"password"`    // 注册表密码（敏感）
    Host       string `mapstructure:"host"`        // 注册表主机
}
```