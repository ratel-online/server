.PHONY: help build run stop clean logs shell test prod-build prod-up prod-down

# 检测 Docker Compose 命令
DOCKER_COMPOSE := $(shell which docker-compose 2>/dev/null)
ifeq ($(DOCKER_COMPOSE),)
    DOCKER_COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "")
endif
ifeq ($(DOCKER_COMPOSE),)
    $(error "Docker Compose 未安装。请安装 docker-compose 或升级到支持 'docker compose' 的 Docker 版本")
endif

# 默认目标
help:
	@echo "Ratel游戏服务器 Docker 管理命令"
	@echo ""
	@echo "当前使用的 Docker Compose 命令: $(DOCKER_COMPOSE)"
	@echo ""
	@echo "开发环境命令:"
	@echo "  make build       - 构建Docker镜像"
	@echo "  make run         - 启动开发环境"
	@echo "  make stop        - 停止开发环境"
	@echo "  make logs        - 查看日志"
	@echo "  make shell       - 进入容器shell"
	@echo "  make clean       - 清理容器和镜像"
	@echo ""
	@echo "生产环境命令:"
	@echo "  make prod-build  - 构建生产镜像"
	@echo "  make prod-up     - 启动生产环境"
	@echo "  make prod-down   - 停止生产环境"
	@echo ""
	@echo "测试命令:"
	@echo "  make test        - 运行测试"

# 开发环境命令
build:
	$(DOCKER_COMPOSE) build

run:
	$(DOCKER_COMPOSE) up -d
	@echo "服务已启动："
	@echo "  WebSocket: http://localhost:9998"
	@echo "  TCP: localhost:9999"

stop:
	$(DOCKER_COMPOSE) down

logs:
	$(DOCKER_COMPOSE) logs -f

shell:
	$(DOCKER_COMPOSE) exec ratel-server sh

clean:
	$(DOCKER_COMPOSE) down -v
	docker rmi ratel-server:latest || true

# 生产环境命令
prod-build:
	docker build -t ratel-server:latest .

prod-up:
	$(DOCKER_COMPOSE) -f docker-compose.prod.yaml up -d
	@echo "生产环境已启动："
	@echo "  WebSocket: http://localhost:9998"
	@echo "  TCP: localhost:9999"
	@echo "  HTTP: http://localhost:80"

prod-down:
	$(DOCKER_COMPOSE) -f docker-compose.prod.yaml down

# 测试命令
test:
	@echo "测试WebSocket连接..."
	@curl -s -o /dev/null -w "HTTP状态码: %{http_code}\n" http://localhost:9998 || echo "WebSocket端口未响应"
	@echo ""
	@echo "测试TCP连接..."
	@nc -zv localhost 9999 2>&1 || echo "TCP端口未响应"
	@echo ""
	@echo "测试Nginx健康检查..."
	@curl -s http://localhost/health || echo "Nginx未运行" 