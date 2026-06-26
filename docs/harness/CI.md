---
title: CI 闸门
status: active
owner: harness
last_updated: 2026-05-29
source_files:
  - ../../.github/workflows/verify.yml
related_docs:
  - VERIFICATION_ROUTING.md
  - HOOKS.md
---

# CI 闸门

`.github/workflows/verify.yml` 在 push / PR 时跑控制面自检 `make verify`。

- CI fail / unknown 时 MUST STOP，先修绿再继续。
- 接入被管工程后，CI 追加按 `VERIFICATION_ROUTING.md` 路由的「affected verify」（只跑跟改动相关的工程测试）。
- 本地等价命令：`make verify`。
