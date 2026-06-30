# candidate — prd-orchestration（提交 5e22c2d..7630519，6 commit）

任务：把 prd-elicitation 从线性交互 skill 重构成编排式——产品总监（主 agent）调度专职 worker，
带 必选/可选·权重 + 确认门 + 并行 + review loop。依据 ADR-0010。

## commit
- 5e22c2d (T1) ADR-0010 + 重写 SKILL 总谱
- a33c349 (T2) prd-reviewer 子 agent 双栈
- fe442c7 (T3) 5 个产出 worker 子 agent 双栈
- 7564fa7 (T4) Workflow 编排模板
- 7bbef49 (T5) doc-sync（CURRENT_STATUS + docs 路由）
- 7630519 (T6) 对抗挑刺修平 4 类

## 产物（diff --stat 关键项）
- docs/decisions/0010-prd-orchestration.md（ADR，受影响 skill 栏已填）+ index.yaml 登记（ADR-0010）
- .agents/skills/hc-prd/SKILL.md（version 3，编排总谱）
- .agents/skills/hc-prd/references/orchestration-workflow.js（109 行，Workflow 编排模板）
- 6 worker 双栈：.claude/agents/{prd-reviewer,requirements-gatherer,user-story-writer,prd-writer,
  feature-point-writer,prototype-builder}.md + .codex/agents/*.toml + .codex/config.toml 注册 + README 登记
- doc-sync：docs/context/CURRENT_STATUS.md、docs/README.md
- 设计稿/计划：docs/superpowers/{specs,plans}/2026-06-29-prd-orchestration*
- tasks/todo.md（level: L4，T1-T6 勾稽）

## 对抗挑刺修平 4 类（dogfood code-reviewer，11 agent/3 视角/独立证伪）
1. deep-research「复用」措辞如实化（可用 plugin skill，非 repo 资产/非第 7 subagent，走 Skill 工具调）
2. workflow 原型 opt-out（用户跳过原型不再误重跑 prototype-builder：ran 集合过滤 + 条件审稿提示）
3. 重审轮数上限（workflow MAX_ROUNDS=4，到顶交总监人工裁，防审稿员持续挑刺不收敛）
4. Workflow 运行时注释（说明 --check 报"顶层 return 非法"是误报，非真语法错）

详见各 commit；本副本为评审输入快照，权威以仓内文件为准。
