## Why

修复 puller.go 文件中潜在的命令注入安全漏洞，这些漏洞可能允许恶意用户通过构造特殊的镜像名称或凭据来执行任意命令。此类漏洞可能导致系统被入侵或数据泄露，因此需要立即解决。

## What Changes

- 重构输入验证逻辑，增强 `isValidImageName` 和新增 `isValidCredential` 函数
- 修改命令构建方式，使用更安全的参数传递方法，避免字符串拼接
- 更新 `CheckImageExists` 和 `copyAndImportImage` 函数，加强命令注入防护
- 添加额外的安全层，包括路径遍历检查和更严格的字符白名单
- 优化错误处理机制，提供更好的安全反馈

## Capabilities

### New Capabilities
- `secure-command-execution`: 实现安全的命令执行机制，防止命令注入攻击
- `input-validation-enhancement`: 加强输入验证功能，包括图像名称和用户凭证

### Modified Capabilities

## Impact

- internal/core/puller.go: 主要修改文件，增强安全验证逻辑
- pkg/retry/: 依赖包，可能会因更严格的错误处理而受影响
- 命令行接口: 可能会因为更严格的输入验证而拒绝以前接受的无效输入
- 所有调用 puller 功能的代码: 将受益于更高的安全性