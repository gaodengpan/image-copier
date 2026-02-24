# CLAUDE.md

## 基本命令
```bash
make build              # 构建
make test               # 测试
make test-coverage      # 测试 + 覆盖率
make fmt                # 格式化
make vet                # go vet
make check-quality      # 全量检查
```

## 架构原则

### 依赖规则
- UseCase 仅依赖领域层接口，禁止直接依赖基础设施（如 `DockerClient`, `RegistryClient`）
- 基础设施层通过 `domain/ports/output` 接口注入

### 领域建模
- 实体需包含丰富业务行为，避免贫血模型
- 优先使用值对象（`Architecture`, `OperatingSystem`）替代 string 字段
- 领域事件驱动状态变更，UseCase 监听事件而非直接操作

### 职责边界
- UseCase: 业务编排，单一职责（避免 300 行 + 函数）
- CLI 层：仅负责输入解析、结果展示，不包含业务逻辑
- 服务层：仅当需要跨聚合操作时才定义领域服务

## 开发规范

### TDD 流程
1. 编写失败测试（Red）
2. 最小实现通过测试（Green）
3. 重构优化（Refactor）

### 代码组织
- 结构体字段：关键配置 → 依赖 → 状态 → 辅助
- 方法顺序：构造函数 → 公共方法 → 私有方法
- 构造函数参数超过 5 个时，使用参数对象模式

### 错误处理
- 分类：`DomainError`, `ValidationError`, `AdapterError`
- 包装：`fmt.Errorf("context: %w", err)`
- 忽略：显式使用 `_`

### 测试规范
- 文件：`<name>_test.go`
- 断言：`testify/assert` + `testify/require`
- Mock：基础设施依赖必须可模拟

### 命名约定
- 包名：小写（`value_objects`, `log_format`）
- 类型：PascalCase（`ImageProgress`, `SyncTask`）
- 接口：-er/-or 结尾（`ConfigProvider`）
- 错误：`Error`/`Err` 结尾

### Git 提交
- 语气：祈使句（"Add feature"）
- 格式：conventional commits（`feat:`, `fix:`, `docs:`）

## 其他
- Go >= 1.24.0
- `CGO_ENABLED=0` 静态编译