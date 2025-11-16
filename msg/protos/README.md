# Protobuf 协议定义

## 生成 Go 代码

### 1. 安装依赖

```bash
# 安装 Protocol Buffers 编译器（protoc）
# Windows: 下载 https://github.com/protocolbuffers/protobuf/releases
# 或使用包管理器：
#   choco install protoc
#   scoop install protoc

# 安装 Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### 2. 生成代码

在项目根目录执行：

```bash
protoc --go_out=. --go_opt=paths=source_relative msg/websocket.msg
```

生成的代码将位于：`proto/websocket.pb.go`

### 3. 添加依赖到 go.mod

```bash
go get google.golang.org/protobuf/msg
go mod tidy
```
