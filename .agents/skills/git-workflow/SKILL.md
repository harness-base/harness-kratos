---
name: git-workflow
description: 做任何 git 写操作（分支/提交/合并/rebase/推送/worktree 清理）前用本 skill，约束安全习惯。尤其当你打算 commit、push、reset、合并或删分支时必须先看——防止未授权的 git 写操作和不可逆破坏。
version: 1
last_reviewed: 2026-05-29
---

# Git Workflow

git 写操作可能不可逆（reset、删分支、强推），未经允许的提交/推送还会污染历史。所以这些动作要先授权、走安全习惯。

## 何时用 / 何时不用
- 用：分支、提交、合并、rebase、推送、worktree。
- 不用：只读查询（status / log / diff）随意。

## 硬规则
- 未经用户许可，不 commit / push / reset / 删分支 / 改 remote（rule-0006）。
- 不执行高危命令（强制重置、强推等，hook 会拦）。
- 默认在新分支工作，不直接改主干（除非用户指定）。

## 步骤
1. 动手前确认当前分支与目标分支。
2. 提交信息讲清"做了什么、为什么"。
3. 推送 / PR 后等 CI（见 `docs/harness/CI.md`）；fail / unknown MUST STOP。

## 演进（rule-0007）
团队 git 约定变化时回顾本 skill。
