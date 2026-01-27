#!/bin/bash

# ============================================================
# wx_game 代码质量检查脚本
# 用于验证所有代码修改是否符合生产级标准
# ============================================================

set -e  # 遇到错误立即退出

echo "========================================"
echo "wx_game 代码质量检查"
echo "========================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查函数
check_step() {
    echo -e "${YELLOW}► $1${NC}"
}

success_step() {
    echo -e "${GREEN}✓ $1${NC}"
}

error_step() {
    echo -e "${RED}✗ $1${NC}"
    exit 1
}

# 1. 格式检查
check_step "检查代码格式..."
if ! gofmt -l . | grep -q .; then
    success_step "代码格式正确"
else
    error_step "代码格式不符合规范，请运行: gofmt -w ."
fi

# 2. go vet 检查
check_step "运行 go vet..."
if go vet ./...; then
    success_step "go vet 检查通过"
else
    error_step "go vet 检查失败，请修复上述问题"
fi

# 3. 静态分析（如果安装了 golangci-lint）
if command -v golangci-lint &> /dev/null; then
    check_step "运行 golangci-lint..."
    if golangci-lint run --timeout 5m; then
        success_step "golangci-lint 检查通过"
    else
        error_step "golangci-lint 检查失败，请修复上述问题"
    fi
else
    echo -e "${YELLOW}⚠ golangci-lint 未安装，跳过静态分析${NC}"
    echo "  安装方法: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
fi

# 4. 单元测试
check_step "运行单元测试..."
if go test ./... -v -coverprofile=coverage.out; then
    success_step "单元测试通过"
    
    # 显示测试覆盖率
    coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    echo -e "${GREEN}  测试覆盖率: $coverage${NC}"
    
    # 检查覆盖率是否达标（80%）
    coverage_num=$(echo $coverage | sed 's/%//')
    if (( $(echo "$coverage_num < 80" | bc -l) )); then
        echo -e "${YELLOW}  ⚠ 警告: 测试覆盖率低于 80%，请增加测试${NC}"
    fi
else
    error_step "单元测试失败，请修复上述问题"
fi

# 5. 竞态检测
check_step "运行竞态检测..."
if go test -race ./... -timeout 30s; then
    success_step "竞态检测通过（无 data race）"
else
    error_step "检测到数据竞争（data race），请修复并发安全问题"
fi

# 6. 构建检查
check_step "构建验证..."
if go build -o wx_game main.go; then
    success_step "构建成功"
    rm -f wx_game  # 清理构建产物
else
    error_step "构建失败，请修复编译错误"
fi

# 7. 依赖检查
check_step "检查依赖完整性..."
if go mod verify; then
    success_step "依赖验证通过"
else
    error_step "依赖验证失败，请运行: go mod tidy"
fi

# 8. 安全检查（如果安装了 gosec）
if command -v gosec &> /dev/null; then
    check_step "运行安全扫描..."
    if gosec -quiet ./...; then
        success_step "安全扫描通过"
    else
        echo -e "${YELLOW}  ⚠ 发现潜在安全问题，请检查${NC}"
    fi
else
    echo -e "${YELLOW}⚠ gosec 未安装，跳过安全扫描${NC}"
    echo "  安装方法: go install github.com/securego/gosec/v2/cmd/gosec@latest"
fi

echo ""
echo "========================================"
echo -e "${GREEN}✓ 所有检查通过！代码符合生产级标准${NC}"
echo "========================================"
