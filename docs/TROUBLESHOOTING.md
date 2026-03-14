# CI 故障排查指南

本文档记录 image-copier 项目在 CI 环境中常见的问题及解决方案。

## 1. errcheck: 返回值未检查

```
Error: Error return value of `vp.viper.BindEnv` is not checked
```

**修复**：用 `_ =` 显式忽略返回值

```go
// 错误
vp.viper.BindEnv("key", "ENV_KEY")

// 正确
_ = vp.viper.BindEnv("key", "ENV_KEY")
```

## 2. race detected: 竞态条件

```
testing.go:1490: race detected during execution of test
```

**原因**：使用 `time.Sleep` 等待 goroutine 完成

**修复**：使用 `sync.WaitGroup` 同步

```go
// 错误 - 不确定等待时间
for i := 0; i < 10; i++ {
    go doSomething()
}
time.Sleep(100 * time.Millisecond)

// 正确 - 确保所有 goroutine 完成
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        doSomething()
    }()
}
wg.Wait()
```

## 3. 测试缺少环境变量

```
Error: github owner is required / registry host is required
```

**原因**：CI 环境没有预设环境变量

**修复**：在测试中用 `t.Setenv()` 设置

```go
func TestSomething(t *testing.T) {
    t.Setenv("GITHUB_OWNER", "test-owner")
    t.Setenv("GITHUB_REPO", "test-repo")
    t.Setenv("GITHUB_TOKEN", "test-token")
    t.Setenv("REGISTRY_HOST", "ghcr.io")
    t.Setenv("REGISTRY_USERNAME", "test-user")
    t.Setenv("REGISTRY_PASSWD", "test-pass")
    // ...
}
```

## 4. gosimple: 不必要的代码

```
S1039: unnecessary use of fmt.Sprintf
```

**修复**：简化字符串格式化

```go
// 错误
return fmt.Sprintf("docker (local)")

// 正确
return "docker (local)"
```