# 安全配置管理 API 契约

## 1. 加密解密服务接口

### 1.1 加密功能
```
函数: Encrypt(value string, key []byte) (string, error)
```

**描述**: 使用 AES-256-GCM 算法加密给定的字符串值

**参数**:
- value (string): 待加密的明文字符串
- key ([]byte): 加密密钥 (长度必须为 32 字节)

**返回值**:
- string: 格式为 "encrypted:<base64(Nonce+Ciphertext+Tag)>"
- error: 加密失败时返回错误

**前置条件**:
- value 不能为空字符串
- key 必须为 32 字节长

**后置条件**:
- 返回的字符串以 "encrypted:" 开头
- 返回的加密字符串能通过 Decrypt 函数正确解密

### 1.2 解密功能
```
函数: Decrypt(encryptedValue string, key []byte) (string, error)
```

**描述**: 解密由 Encrypt 函数生成的加密字符串

**参数**:
- encryptedValue (string): 待解密的字符串，格式必须为 "encrypted:..."
- key ([]byte): 解密密钥 (长度必须为 32 字节)

**返回值**:
- string: 解密后的明文字符串
- error: 解密失败时返回错误

**前置条件**:
- encryptedValue 必须以 "encrypted:" 开头
- key 必须为 32 字节长

**后置条件**:
- 返回的明文与原加密前的字符串相同
- 如果解密失败，返回错误信息

### 1.3 密钥派生功能
```
函数: DeriveKey(password string, salt []byte) []byte
```

**描述**: 使用 PBKDF2 从密码派生出加密密钥

**参数**:
- password (string): 原始密码字符串
- salt ([]byte): 盐值 (长度至少为 8 字节，推荐 16 字节)

**返回值**:
- []byte: 32 字节长的加密密钥

**前置条件**:
- password 不能为空
- salt 长度至少为 8 字节

**后置条件**:
- 返回的密钥长度为 32 字节
- 相同输入始终产生相同的输出

## 2. 配置处理接口

### 2.1 加密配置值
```
函数: EncryptConfigField(value string, fieldName string) (string, error)
```

**描述**: 根据字段类型决定是否加密配置值

**参数**:
- value (string): 配置字段的值
- fieldName (string): 配置字段的名称

**返回值**:
- string: 如果字段需要加密，则返回加密字符串；否则返回原值
- error: 加密过程中发生错误

**前置条件**:
- fieldName 是合法的配置字段名

**后置条件**:
- 敏感字段被加密（返回 "encrypted:..." 格式）
- 非敏感字段保持不变

### 2.2 解密配置
```
函数: DecryptConfig(config *Config) (*Config, error)
```

**描述**: 解密配置对象中的所有加密字段

**参数**:
- config (*Config): 包含可能加密字段的配置对象

**返回值**:
- *Config: 解密后的配置对象，所有加密字段已被替换为明文
- error: 解密过程中发生错误

**前置条件**:
- config 不为 nil

**后置条件**:
- 所有以 "encrypted:" 开头的字段都被解密
- 非加密字段保持不变
- 如果解密失败，返回错误

## 3. 错误定义

### 3.1 DecryptionError
```
type DecryptionError struct {
    Message        string
    EncryptedValue string
    FieldName      string
}
```

**描述**: 解密失败时抛出的错误类型

### 3.2 InvalidFormatError
```
type InvalidFormatError struct {
    Message       string
    Value         string
    ExpectedFormat string
}
```

**描述**: 输入格式不正确时抛出的错误类型

## 4. 配置验证接口

### 4.1 验证加密配置
```
函数: ValidateEncryptedConfig(config *Config) error
```

**描述**: 验证配置对象中加密字段的格式正确性

**参数**:
- config (*Config): 配置对象

**返回值**:
- error: 如果加密字段格式不正确，返回验证错误

**前置条件**:
- config 不为 nil

**后置条件**:
- 如果配置中包含格式不正确的加密字段，返回错误
- 格式正确的配置不会产生错误