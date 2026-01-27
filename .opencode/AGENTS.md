# 项目编码规范与智能体工作准则

## 项目身份
- **项目性质**：生产级微信小游戏后端服务
- **技术栈**：Go 1.21+ / Fiber v2 / MongoDB / Protobuf / WebSocket
- **目标环境**：高并发、低延迟、7x24 稳定运行
- **代码标准**：资深专家级 Go/C++ 后端工程师标准

---

## 核心编码准则（MANDATORY）

### 1. 代码质量要求

#### **生产级标准**
- 所有代码必须达到 **生产环境可直接部署** 的质量
- 遵循 Go 官方编码规范（Effective Go + Code Review Comments）
- 遵循 C++ Core Guidelines（如涉及 C++ cgo）
- 代码可读性 > 炫技，简洁明了 > 过度抽象

#### **性能与资源管理**
- **内存管理**：
  - 必须避免内存泄漏（goroutine 泄漏、未关闭的连接、循环引用）
  - 大对象使用对象池（sync.Pool）
  - 热路径避免不必要的堆分配
- **并发安全**：
  - 共享数据必须加锁或使用 channel
  - 禁止数据竞争（data race）
  - 优先使用 sync/atomic 而非 mutex（如果适用）
- **错误处理**：
  - 所有错误必须处理，不得忽略
  - 使用 errors.Is/errors.As 判断错误类型
  - 记录足够的上下文信息（使用 fmt.Errorf 包装）

#### **代码结构**
- 函数职责单一，单个函数不超过 50 行（复杂逻辑拆分）
- 包级变量仅用于全局配置，禁止可变全局状态
- 接口设计遵循"接口隔离原则"（小接口优于大接口）
- 依赖注入优先，避免 init() 中的副作用

---

### 2. 禁止事项（NEVER DO）

#### **代码层面**
- ❌ 禁止 panic（除非 init 阶段的配置错误）
- ❌ 禁止忽略 error 返回值（即使是 defer close()）
- ❌ 禁止裸 goroutine（必须有 context 控制生命周期）
- ❌ 禁止使用 `interface{}`（改用泛型或具体类型）
- ❌ 禁止在循环内使用 defer（会导致资源延迟释放）
- ❌ 禁止硬编码配置（IP、端口、密钥等必须从 config 读取）
- ❌ 禁止使用 `time.Sleep` 做流控（改用 rate limiter/ticker）

#### **日志层面**
- ❌ 禁止无上下文的日志（必须包含 traceID、userID、sessionID 等）
- ❌ 禁止在热路径打 Debug 日志（改用条件编译或动态日志级别）
- ❌ 禁止日志中包含敏感信息（密码、token、身份证号等）

#### **性能层面**
- ❌ 禁止在循环内进行字符串拼接（改用 strings.Builder）
- ❌ 禁止在热路径使用反射（除非有性能测试证明影响可忽略）
- ❌ 禁止同步阻塞调用外部服务（必须有超时控制）

---

### 3. 必须事项（MUST DO）

#### **测试要求**
- ✅ 核心业务逻辑必须有单元测试（覆盖率 >= 80%）
- ✅ 并发代码必须有竞态检测测试（`go test -race`）
- ✅ 性能关键路径必须有基准测试（benchmark）
- ✅ 外部依赖必须 mock（使用 gomock/testify）

#### **并发控制**
- ✅ 所有 goroutine 必须通过 context 控制生命周期
- ✅ 使用 errgroup 管理协程组错误
- ✅ 长时间运行的 goroutine 必须有优雅退出机制

#### **数据库操作**
- ✅ 必须使用连接池（已配置：MaxPoolSize=100）
- ✅ 所有查询必须有超时（context.WithTimeout）
- ✅ 批量操作优先使用事务（MongoDB transaction）
- ✅ 索引设计必须评审（避免全表扫描）

#### **WebSocket/网络**
- ✅ 必须有心跳机制（防止连接僵死）
- ✅ 必须有消息大小限制（防止恶意大包）
- ✅ 必须有速率限制（防止 DDoS）
- ✅ 必须有连接数限制（防止资源耗尽）

---

### 4. 架构约定

#### **分层架构**
```
Handler层（handlers.go）
    ↓ 仅处理HTTP/WebSocket协议，参数验证
Logic层（watermelon/logic.go, role/logic.go）
    ↓ 业务逻辑，不关心存储细节
Persistence层（fw/persistence/）
    ↓ 数据持久化，屏蔽存储实现
```

#### **依赖方向**
- 上层可依赖下层，下层禁止依赖上层
- 跨层调用必须通过接口，不得直接耦合实现
- 禁止循环依赖（使用 `go mod graph` 检测）

#### **配置管理**
- 所有配置集中在 `config.yaml`
- 敏感信息使用环境变量覆盖（如 MONGO_PASSWORD）
- 配置变更必须向后兼容（旧版本能启动）

---

### 5. 安全要求

#### **认证与授权**
- ✅ 所有 API 必须验证 session_key（除登录接口）
- ✅ WebSocket 消息必须验证用户身份
- ✅ 禁止信任客户端输入（所有数据必须校验）

#### **数据安全**
- ✅ 密码/密钥禁止明文存储
- ✅ 日志中禁止记录敏感信息
- ✅ SQL/NoSQL 注入防护（使用参数化查询）

#### **速率限制**
- ✅ 登录接口：5次/分钟/IP（已配置）
- ✅ 游戏接口：20次/分钟/IP（已配置）
- ✅ WebSocket 消息：根据业务调整

---

### 6. 代码审查清单

#### **提交前自查**
- [ ] 代码通过 `go vet` 检查
- [ ] 代码通过 `golangci-lint run` 检查
- [ ] 代码通过 `go test -race ./...` 检查
- [ ] 新增代码有对应单元测试
- [ ] 修改已有代码运行回归测试
- [ ] 日志级别正确（Debug/Info/Warn/Error）
- [ ] 错误处理完整（无忽略的 error）
- [ ] 文档/注释已更新（如有 API 变更）

#### **性能自查**
- [ ] 热路径无不必要的堆分配
- [ ] 无 goroutine 泄漏（使用 pprof 验证）
- [ ] 数据库查询有索引支持
- [ ] 大对象使用对象池

---

### 7. 注释规范

#### **何时写注释**
- ✅ 公开的 API/函数必须有 godoc 注释
- ✅ 复杂算法必须解释思路（如排行榜的多维排序）
- ✅ 业务规则必须说明（如西瓜合成的分数计算公式）
- ✅ 性能权衡必须记录（如为什么选择某种数据结构）

#### **何时不写注释**
- ❌ 代码本身已经足够清晰（self-documenting）
- ❌ 注释重复代码逻辑（"这段代码做了 X" → 代码已经说明了）
- ❌ 过时的注释（必须删除或更新）

#### **注释风格**
```go
// Bad: 无用的注释
// 设置用户ID
user.ID = id

// Good: 解释为什么
// 使用雪花算法生成全局唯一ID，避免分布式环境下的ID冲突
user.ID = snowflake.Generate()

// Good: 业务规则
// 西瓜合成分数 = 基础分 * 连击倍数 * 关卡系数
// 连击倍数：1-3次=1.0, 4-6次=1.5, 7+次=2.0
score := baseScore * comboMultiplier * levelCoefficient
```

---

### 8. 错误处理模式

#### **标准模式**
```go
// Bad: 忽略错误
result, _ := doSomething()

// Good: 完整处理
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Good: 分类处理
result, err := doSomething()
if err != nil {
    if errors.Is(err, ErrNotFound) {
        return handleNotFound()
    }
    return fmt.Errorf("unexpected error: %w", err)
}
```

#### **Defer 错误处理**
```go
// Bad: 忽略 Close 错误
defer file.Close()

// Good: 检查 Close 错误
defer func() {
    if err := file.Close(); err != nil {
        logger.Errorf("failed to close file: %v", err)
    }
}()
```

---

### 9. 并发模式

#### **标准 Goroutine 启动**
```go
// Bad: 裸 goroutine
go doWork()

// Good: 带 context 控制
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            logger.Info("Worker stopped")
            return
        case work := <-workChan:
            processWork(work)
        }
    }
}(ctx)
```

#### **Errgroup 模式**
```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(context.Background())

g.Go(func() error {
    return task1(ctx)
})

g.Go(func() error {
    return task2(ctx)
})

if err := g.Wait(); err != nil {
    return fmt.Errorf("tasks failed: %w", err)
}
```

---

### 10. 性能优化原则

#### **过早优化是万恶之源**
- 先写正确的代码，再优化
- 必须有性能测试数据支撑优化决策
- 使用 pprof 定位性能瓶颈

#### **常见优化点**
- 字符串拼接：使用 `strings.Builder`
- JSON 序列化：考虑 `easyjson/jsoniter`
- 正则表达式：预编译 `regexp.MustCompile`
- 对象复用：使用 `sync.Pool`

---

### 11. 项目特定规则

#### **西瓜游戏业务规则**
- 游戏状态必须存储在 MongoDB（可靠性）
- 实时分数通过 WebSocket 推送（低延迟）
- 排行榜每次登录刷新（保证数据一致性）
- 定时落库间隔不超过 5 秒（数据安全性）

#### **消息协议**
- 使用 Protobuf 序列化（性能 + 类型安全）
- 消息 ID 必须在 `msg/msg_id/msg_id.go` 中定义
- 客户端/服务器消息分离（*_msg.pb.go / *_server_msg.pb.go）

#### **配置加载**
- 配置文件使用 YAML 格式
- 支持环境变量覆盖（`MONGO_URL`）
- 配置变更需要重启服务（不支持热加载）

---

## 智能体工作流程约定

### 1. 代码修改流程
1. **分析需求**：理解业务逻辑，识别影响范围
2. **设计方案**：选择合适的设计模式和数据结构
3. **编写代码**：按上述规范实现
4. **自查测试**：运行 lint、test、race 检测
5. **验证诊断**：使用 lsp_diagnostics 确保无类型错误
6. **文档更新**：更新相关注释和文档

### 2. 重构流程
1. **必须先有测试**：确保重构前有足够的测试覆盖
2. **小步快跑**：每次重构一个小模块，立即验证
3. **使用 LSP 工具**：lsp_rename、lsp_find_references 确保安全重构
4. **回归测试**：每次重构后运行完整测试套件

### 3. 委托策略
- 复杂算法实现 → 委托给 `oracle`（GPT 5.2 Medium）
- 性能优化分析 → 委托给 `oracle`
- 快速代码探索 → 委托给 `explore`（Grok Code）
- 外部库文档查询 → 委托给 `librarian`

---

## 质量门禁（Quality Gates）

### 代码提交前必须通过
```bash
# 静态检查
go vet ./...
golangci-lint run

# 单元测试
go test ./... -v

# 竞态检测
go test -race ./...

# 构建验证
go build -o wx_game main.go

# 运行集成测试（如有）
./scripts/integration_test.sh
```

### 性能基准（作为参考）
- 登录接口响应时间：< 100ms (P99)
- WebSocket 消息延迟：< 50ms (P99)
- 排行榜查询：< 200ms (P99)
- 内存占用：< 500MB (稳定运行)
- CPU 占用：< 30% (正常负载)

---

## 特别说明

### 对智能体的期望
- 你是一个**资深专家级 Go/C++ 后端工程师**的助手
- 所有代码必须达到**生产级质量**，可直接部署
- 代码风格必须**简洁、高效、健壮**
- 绝不容忍**技术债务**和**临时方案**
- 优先考虑**可维护性、可测试性、性能**

### 沟通原则
- 遇到不确定的设计决策，**必须先询问**而非自行决定
- 发现潜在的架构问题，**必须指出**而非默默修复
- 提出的方案必须有**权衡分析**（trade-offs）

---

## 参考资源

### Go 编码规范
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)

### 性能优化
- [High Performance Go Workshop](https://dave.cheney.net/high-performance-go-workshop/dotgo-paris.html)
- [Go Memory Management](https://go.dev/blog/ismmkeynote)

### 安全
- [OWASP Go Secure Coding Practices](https://owasp.org/www-project-go-secure-coding-practices-guide/)
- [Go Security Best Practices](https://github.com/Checkmarx/Go-SCP)

---

**版本**: 1.0  
**最后更新**: 2026-01-27  
**维护者**: 资深专家级 Go/C++ 后端工程师
