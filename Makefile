# Agent Harness — 控制面命令入口
# 脚本以 bash 为主，不依赖 Node 工具链。

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

.PHONY: help verify docs-audit eval verify-eval hooks

help: ## 列出所有命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

verify: ## 控制面自检（结构 + 文档 + hook policy 测试）
	@bash scripts/verify-control-plane.sh

docs-audit: ## 文档自检（frontmatter 依赖文件在不在、链接通不通）
	@bash scripts/docs-audit.sh

eval: ## 跑 task eval review（ARGS 透传，如 ARGS="--task-review --context-level L3 ..."）
	@bash scripts/run-eval.sh $(ARGS)

verify-eval: ## 检查该评的有没有评、评审产物格式对不对
	@bash scripts/verify-eval-materials.sh

hooks: ## 安装 git hooks（core.hooksPath -> .githooks）
	@bash scripts/install-hooks.sh
