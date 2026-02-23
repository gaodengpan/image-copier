# CLAUDE.md

## 构建/测试/运行命令

### 基本命令
```bash
make build              # 构建二进制文件
make test               # 运行所有测试
make test-coverage      # 运行测试并生成覆盖率报告
make test-coverage-html # 生成 HTML 覆盖率报告
make fmt                # 格式化代码 (gofmt + goimports)
make vet                # 运行 go vet
make lint               # 运行 golangci-lint
make check-quality      # 运行所有质量检查
```

## 项目架构
- 采用Clean Architecture + Hexagonal Architecture 混合架构
- 采用DDD思想进行领域建模和代码组织
- 整体代码模块化设计，并遵循SOLID原则

## 研发规范
- 遵循TDD开发模式，严格按照red->green->refactor流程开发

## 代码风格指南

### 导入规范
- 使用标准 Go 导入分组：标准库 → 第三方库 → 本地包
- 本地包使用完整路径：`github.com/gaodengpan/image-copier/internal/...`
- 导入组之间用空行分隔

### 命名约定
- 包名：小写，无下划线（如 `value_objects`, `log_format`）
- 类型/结构体：PascalCase（如 `ImageProgress`, `SyncTask`）
- 接口：以 -er/-or 结尾的 PascalCase（如 `ConfigProvider`, `UseCase`）
- 函数/方法：PascalCase（导出）或 camelCase（私有）
- 常量：PascalCase 或 camelCase
- 错误变量：以 `Error` 或 `Err` 结尾
- 测试函数：`Test<FunctionName>` 或 `Test<FunctionName>_<Scenario>`

### 错误处理
- 使用自定义错误类型（参见 `internal/shared/errors/errors.go`）
- 错误消息应包含上下文信息
- 不要忽略错误，使用 `_` 明确表示有意忽略

### 代码组织
- 结构体字段按重要性排序：关键配置 → 依赖 → 状态 → 辅助字段
- 方法顺序：构造函数 → 公共方法 → 私有方法

### 测试规范
- 测试文件与被测文件同名，加 `_test.go` 后缀
- 使用 `testify/assert` 和 `testify/require` 进行断言
- 测试函数应独立、可重复运行

### Git 提交规范
- 提交消息使用祈使语气（如 "Add feature", "Fix bug"）
- 保持提交消息简洁（50 字符标题，72 字符换行正文）
- 使用 conventional commits 风格（可选）：`feat:`, `fix:`, `docs:`, `test:`

## 其他注意事项
- Go 版本>=1.24.0
- 使用 `CGO_ENABLED=0` 构建静态二进制