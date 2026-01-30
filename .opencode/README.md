# wx_game 项目 - OpenCode 使用指南

## 已配置的生产级约束

本项目已配置 Oh My OpenCode，确保所有 AI 生成的代码符合**资深专家级 Go/C++ 后端**标准。

### 核心文件

1. **`.opencode/AGENTS.md`** - 项目编码规范（智能体必读）
2. **`.opencode/oh-my-opencode.json`** - 智能体配置
3. **`.opencode/validate.sh`** - Linux/Mac 代码检查脚本
4. **`.opencode/validate.bat`** - Windows 代码检查脚本

---

## 使用方式

### 1. 日常开发

直接在 OpenCode 中与智能体对话，它会自动遵循 `.opencode/AGENTS.md` 中的规范。

**示例提示词：**
```
实现一个新的游戏模式：限时挑战赛
要求：
- 60秒倒计时
- 分数加成机制
- 排行榜集成
```

智能体会自动：
- 遵循 Go 官方编码规范
- 处理所有错误
- 添加并发安全控制
- 编写单元测试
- 使用 LSP 验证代码
- 运行 race 检测

### 2. 代码审查

每次智能体完成代码修改后，运行验证脚本：

**Linux/Mac:**
```bash
bash .opencode/validate.sh
```

**Windows:**
```cmd
.opencode\validate.bat
```

### 3. 触发专家模式

当遇到复杂问题时，使用魔法关键词 `ultrawork` 或 `ulw`：

```
ulw 重构 watermelon/logic.go，优化内存分配和并发性能
```

这会触发：
- 并行代理（explore + librarian + oracle）
- 深度代码探索
- 性能分析
- 直到完成的执行保证

### 4. 架构决策

对于重大架构变更，智能体会主动咨询：

```
Agent: 我发现排行榜可以用两种方式实现：
1. Redis ZSet（性能更好，但需要引入 Redis）
2. MongoDB + 内存缓存（当前方案，无需新依赖）

考虑到你的生产环境约束，建议使用方案2。是否同意？
```

---

## 验证清单（每次提交前）

运行验证脚本会自动检查以下项：

- [x] 代码格式（gofmt）
- [x] 静态分析（go vet）
- [x] 代码质量（golangci-lint）
- [x] 单元测试通过
- [x] 测试覆盖率 >= 80%
- [x] 竞态检测（go test -race）
- [x] 构建成功
- [x] 依赖完整性
- [x] 安全扫描（gosec，可选）

---

## 智能体能力矩阵

| 智能体 | 用途 | 触发场景 |
|--------|------|---------|
| **Sisyphus** | 主编排器 | 所有日常开发 |
| **Oracle** | 架构顾问 | 复杂设计决策、性能优化 |
| **Librarian** | 文档查询 | 查找 Go 最佳实践、开源实现 |
| **Explore** | 代码探索 | 快速查找项目中的实现模式 |
| **Prometheus** | 规划师 | 大型重构、新功能规划 |

---

## 常见场景

### 场景1：修复Bug
```
修复 watermelon/logic.go:245 的并发问题，
测试显示有 data race
```

智能体会：
1. 读取代码和测试
2. 识别竞态条件
3. 添加适当的锁或使用 channel
4. 运行 `go test -race` 验证
5. 确保测试通过

### 场景2：性能优化
```
ulw 优化 rank/manager.go 的查询性能，
目前 P99 延迟超过 500ms
```

智能体会：
1. 咨询 Oracle 分析瓶颈
2. 使用 Librarian 查找优化方案
3. 实现优化（索引、缓存、算法改进）
4. 添加 benchmark 测试
5. 验证 P99 延迟降低

### 场景3：新功能开发
```
添加好友系统：
- 好友列表存储在 MongoDB
- 支持添加/删除/查询
- 每人最多 100 个好友
- WebSocket 实时通知
```

智能体会：
1. 使用 Prometheus 制定实现计划
2. 按计划逐步实现
3. 每个步骤都编写测试
4. 验证并发安全
5. 更新文档

---

## 配置调整

如需调整智能体行为，编辑 `.opencode/oh-my-opencode.json`：

```jsonc
{
  "agents": {
    "Sisyphus (Main)": {
      "temperature": 0.05,  // 更严格（0-1，越低越严格）
      "prompt_append": "额外的项目特定规则..."
    }
  }
}
```

---

## 故障排查

### 问题：智能体生成的代码不符合规范

**解决方案：**
1. 检查 `.opencode/AGENTS.md` 是否存在
2. 确认 `oh-my-opencode` 插件已安装
3. 重启 OpenCode
4. 在提示词中明确引用规范：
   ```
   请严格遵循 .opencode/AGENTS.md 中的规范实现...
   ```

### 问题：验证脚本失败

**解决方案：**
1. 检查是否安装了必要工具：
   ```bash
   go version
   golangci-lint --version
   gosec --version
   ```
2. 安装缺失的工具：
   ```bash
   # golangci-lint
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   
   # gosec
   go install github.com/securego/gosec/v2/cmd/gosec@latest
   ```

### 问题：智能体太保守，拒绝实现某些功能

**解决方案：**
这是正常的！智能体被配置为"安全第一"。
如果你确定某个方案是正确的，可以明确告知：
```
我已评估风险，请使用方案A实现，
理由：[你的技术分析]
```

---

## 最佳实践

1. **明确需求**：提供清晰的输入输出定义
2. **提及约束**：明确性能、并发、安全要求
3. **验证优先**：修改后立即运行验证脚本
4. **渐进式重构**：大型重构分多个小步骤
5. **测试驱动**：要求智能体先写测试再实现

---

**版本**: 1.0  
**维护者**: 资深专家级 Go/C++ 后端工程师
