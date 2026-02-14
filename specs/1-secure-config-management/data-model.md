# 安全配置管理功能数据模型

## 1. 加密配置值结构

### 1.1 加密字符串格式
- **格式**: `encrypted:<base64_encoded_data>`
- **组成部分**:
  - 前缀: "encrypted:" (固定字符串，用于识别加密值)
  - 数据: Base64 编码的加密数据

### 1.2 Base64 编码数据内部结构
- **总长度**: 可变长度 (nonce 长度 + 加密内容长度 + 认证标签长度)
- **结构**:
  1. **Nonce (12 字节)**: AES-GCM 标准要求的随机数
  2. **密文 (可变长度)**: 加密后的实际数据
  3. **认证标签 (16 字节)**: 用于验证数据完整性的标签

## 2. 配置对象变化

### 2.1 现有配置结构 (未变化)
```go
type Config struct {
    Github struct {
        Owner      string `mapstructure:"owner"`
        Repo       string `mapstructure:"repo"`
        Token      string `mapstructure:"token"`      // 可能是加密格式
        WorkflowID string `mapstructure:"workflow_id"`
    } `mapstructure:"github"`

    Registry struct {
        Host      string `mapstructure:"host"`
        Username  string `mapstructure:"username"`     // 可能是加密格式
        Password  string `mapstructure:"password"`     // 可能是加密格式
        Namespace string `mapstructure:"namespace"`
        Arch      string `mapstructure:"arch"`
        Os        string `mapstructure:"os"`
    } `mapstructure:"registry"`

    // 其他字段保持不变...
}
```

### 2.2 敏感字段识别
- **Config.Github.Token**: 必须加密
- **Config.Registry.Username**: 可选加密
- **Config.Registry.Password**: 必须加密

## 3. 加密解密流程中的数据转换

### 3.1 加密前数据
- **类型**: string
- **格式**: 明文
- **示例**: "my-secret-token"

### 3.2 加密后数据
- **类型**: string
- **格式**: encrypted:<base64编码的[nonce+ciphertext+tag]>
- **示例**: "encrypted:AbCdEfGhIjKlMnOpQrStUvWxYz123456..."

### 3.3 解密后数据
- **类型**: string
- **格式**: 明文
- **示例**: "my-secret-token"

## 4. 验证规则

### 4.1 加密值验证
- **前缀检查**: 必须以 "encrypted:" 开始
- **Base64 解码**: 编码内容必须能正确解码
- **长度验证**: 解码后长度必须 >= 28 字节 (12 byte nonce + 16 byte tag)

### 4.2 解密验证
- **解密成功**: 密文必须能够成功解密
- **完整性校验**: 认证标签必须匹配，验证数据未被篡改

## 5. 错误状态数据模型

### 5.1 解密错误
- **类型**: *DecryptionError
- **字段**:
  - Message: 错误描述
  - EncryptedValue: 尝试解密的加密值
  - Cause: 具体错误原因

### 5.2 密钥派生错误
- **类型**: *KeyDerivationError
- **字段**:
  - Message: 错误描述
  - KeySource: 密钥源信息（环境变量名）
  - Cause: 具体错误原因