---
name: hc-git-workflow
description: 做任何 git 写操作（建分支 / 提交 / rebase / 合并 / 解冲突 / 推送 / worktree 清理）前用本 skill：① 安全红线（没授权不写、不强推、别动 main）② 本仓 git 约定（feat/fix 分支从 main 切、本地 rebase main 解冲突、PR 走 merge commit、commit 格式）。打算 commit / push / reset / 合并 / 删分支 时必看。
version: 2
last_reviewed: 2026-06-29
---

# Git Workflow

git 写操作可能不可逆（reset / 删分支 / 强推），未授权的提交 / 推送会污染历史。先授权、走约定。

## 何时用 / 何时不用
- 用：建分支 / 提交 / 合并 / rebase / 解冲突 / 推送 / worktree。
- 不用：只读查询（status / log / diff）随意。

## 安全红线（rule-0006）
- 未经用户许可，不 commit / push / reset / 删分支 / 改 remote。
- 不执行高危命令（强制重置、强推 main 等，hook 会拦）。
- 不直接在 `main` 上干活。

## 分支约定
- **需求分支 `feat/<desc>`、bug 分支 `fix/<desc>`，都从最新 `main` 切。**
- Claude Code 自动开的 worktree 分支（`claude/<auto>`，前缀不可配）**无视它**——在 worktree 里手动切自己的需求/bug 分支：
  ```bash
  git fetch origin main
  git switch -c feat/<desc> origin/main   # bug 修复用 fix/<desc>
  ```

## 改完 → 更新 → 合并
1. 干活、提交都在 `feat/` / `fix/` 分支上。
2. 落后 main 时**本地 `rebase main`**（不是 merge main，保持历史线性）：
   ```bash
   git fetch origin main && git rebase origin/main
   ```
3. **冲突在本地解决**：逐文件看 `<<<<<<<` 标记、留对的、`git add <file>`、`git rebase --continue`；拿不准停下问，别瞎合。
4. push 分支（rebase 后用 `git push --force-with-lease`；`main` 受保护、不可强推）。
5. **开 PR 到 `main`，CI 绿 + review 才合，合并方式 = merge commit。**

## commit 规范
- 格式：`<范围>: <中文主题一句>`（范围如 `docs`、`skills 优化`、`prd 编排`、`fix`）。
- 正文：bullet 说清"做了什么 / 为什么"。

## 演进（rule-0007）
git 约定变化时回顾本 skill，改完同步 `version` / `last_reviewed`、跑 `bash scripts/skills-index.sh`。（约定目前只写进本 skill；将来要机器校验 commit 格式 / 分支名，再加 commit-msg / pre-push hook。）
