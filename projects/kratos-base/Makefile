SHELL := bash
.PHONY: help init generate proto ent wire build test lint vet verify migrate sandbox-up sandbox-down tidy

# 工具版本（go install，不进 go.mod）。意图见 ADR-0002 版本矩阵 / go.mod 注释。
GO                          ?= go
BIN_DIR                     := bin
SERVICES                    := demo
WIRE_VERSION                ?= v0.7.0
PROTOC_GEN_GO_VERSION       ?= v1.36.11
PROTOC_GEN_GO_GRPC_VERSION  ?= v1.6.1
KRATOS_VERSION              ?= v2.9.2

help: ## 列出可用目标
	@awk 'BEGIN {FS = ":.*##"; print "Available targets:"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

init: ## 安装 codegen / DI 工具到 $(go env GOPATH)/bin（buf CLI 与 atlas 另装，见 README）
	@echo ">> installing protoc/grpc/kratos/wire codegen tools"
	@$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@$(GO) install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/$(KRATOS_VERSION)
	@$(GO) install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/$(KRATOS_VERSION)
	@$(GO) install github.com/google/wire/cmd/wire@$(WIRE_VERSION)
	@echo ">> buf CLI 与 atlas CLI 请见 README（独立安装）"

proto: ## 用 buf 生成 proto Go 代码（go/grpc/http/errors），产物落在 .proto 旁、入库
	@echo ">> buf generate api"
	@buf lint api
	@buf generate api

ent: ## 生成 ent 代码（schema 落在 app/<svc>/internal/data/ent/schema/）
	@for svc in $(SERVICES); do \
		if ls app/$$svc/internal/data/ent/schema/*.go >/dev/null 2>&1; then \
			echo ">> ent generate for $$svc"; \
			cd app/$$svc/internal/data/ent && $(GO) run -mod=mod entgo.io/ent/cmd/ent generate ./schema && cd - > /dev/null; \
		else \
			echo ">> no ent schema for $$svc, skipping"; \
		fi; \
	done

generate: proto ent ## 重新生成全部入库的生成代码（buf + ent）

wire: ## 生成 wire 依赖注入代码（T7b 落地；真实跑 wire CLI）
	@echo ">> wire ./app/demo/cmd/"
	@$(shell go env GOPATH)/bin/wire ./app/demo/cmd/

build: ## 编译所有 service 二进制到 bin/
	@mkdir -p $(BIN_DIR)
	@for svc in $(SERVICES); do \
		echo ">> building $$svc"; \
		$(GO) build -o $(BIN_DIR)/$$svc ./app/$$svc/cmd; \
	done

vet: ## go vet
	@$(GO) vet ./...

lint: ## golangci-lint
	@golangci-lint run ./...

test: ## 单元测试（带 -race）
	@$(GO) test -race ./...

verify: build vet lint test ## 收口质量门：build + vet + lint + test
	@echo ">> verify OK"

# --- 后续任务的桩（不让 verify 失败；真实现见对应任务）---

migrate: ## 生成 atlas 迁移（需要 docker + 非 internal 路径；见 deploy/migrations/README.md）
	@echo ">> atlas migrate diff initial"
	@atlas migrate diff initial \
		--dir "file://deploy/migrations" \
		--to "ent://./app/demo/internal/data/ent/schema" \
		--dev-url "docker://postgres/16-alpine/dev" || \
		(echo "WARN: atlas migrate diff failed (Go internal 包限制或 docker 不可用); 迁移生成留 T9 sandbox 执行" && exit 0)

sandbox-up: ## 起本地 sandbox（postgres + redis + etcd + rabbitmq + nacos + rmqnamesrv + rmqbroker），等待七者都 healthy 后返回
	@echo ">> starting sandbox postgres + redis + etcd + rabbitmq + nacos + rmqnamesrv + rmqbroker"
	@docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml up -d
	@echo ">> waiting for postgres to be healthy (polling docker healthcheck, max 60s)..."
	@for i in $$(seq 1 60); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-postgres-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> postgres is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			echo "ERROR: postgres did not become healthy after 60s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs postgres; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> waiting for redis to be healthy (polling docker healthcheck, max 60s)..."
	@for i in $$(seq 1 60); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-redis-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> redis is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			echo "ERROR: redis did not become healthy after 60s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs redis; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> waiting for etcd to be healthy (polling docker healthcheck, max 60s)..."
	@for i in $$(seq 1 60); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-etcd-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> etcd is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			echo "ERROR: etcd did not become healthy after 60s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs etcd; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> waiting for rabbitmq to be healthy (polling docker healthcheck, max 90s — rabbit starts slower)..."
	@for i in $$(seq 1 90); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-rabbitmq-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> rabbitmq is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 90 ]; then \
			echo "ERROR: rabbitmq did not become healthy after 90s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs rabbitmq; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> waiting for nacos to be healthy (polling docker healthcheck, max 120s — nacos JVM starts slower)..."
	@for i in $$(seq 1 120); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-nacos-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> nacos is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 120 ]; then \
			echo "ERROR: nacos did not become healthy after 120s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs nacos; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> waiting for rmqnamesrv to be healthy (polling docker healthcheck, max 60s)..."
	@for i in $$(seq 1 60); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-rmqnamesrv-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> rmqnamesrv is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			echo "ERROR: rmqnamesrv did not become healthy after 60s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs rmqnamesrv; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> waiting for rmqbroker to be healthy (polling docker healthcheck, max 120s — broker+proxy JVM starts slower)..."
	@for i in $$(seq 1 120); do \
		hc=$$(docker inspect --format='{{.State.Health.Status}}' kratosbase-sandbox-rmqbroker-1 2>/dev/null || echo "unknown"); \
		if [ "$$hc" = "healthy" ]; then \
			echo ">> rmqbroker is healthy (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 120 ]; then \
			echo "ERROR: rmqbroker did not become healthy after 120s"; \
			docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml logs rmqbroker; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> pre-creating rocketmq topic demo-events (retry until DefaultCluster registers in namesrv, max 60s)..."
	@for i in $$(seq 1 60); do \
		out=$$(docker exec kratosbase-sandbox-rmqbroker-1 sh mqadmin updateTopic -n rmqnamesrv:9876 -t demo-events -c DefaultCluster 2>&1); \
		ec=$$?; \
		if echo "$$out" | grep -q "create topic to.*success"; then \
			echo ">> demo-events topic created (attempt $$i)"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			echo "ERROR: could not create demo-events topic after 60s"; \
			echo "$$out"; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo ">> rocketmq sandbox ready"

sandbox-down: ## 停本地 sandbox（postgres + redis + etcd + rabbitmq + nacos + rmqnamesrv + rmqbroker，含卷，保证幂等）
	@echo ">> stopping sandbox postgres + redis + etcd + rabbitmq + nacos + rmqnamesrv + rmqbroker"
	@docker compose -p kratosbase-sandbox -f deploy/sandbox/docker-compose.yaml down -v

tidy: ## go mod tidy
	@$(GO) mod tidy
