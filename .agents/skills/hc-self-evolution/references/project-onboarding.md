# 被管工程接入 审查手册

控制面治理被管工程靠三样东西：`projects/<name>/` 里的代码、`workspace/verification.yaml` 的验证路由、随工程下沉的就近 `AGENTS.md`。任何一样靠口头、靠记忆、或登记了但跑不动 = 接入失效。

## 规范（健康长什么样 / 不变量）

- 接入有**成文流程**：`docs/harness/PROJECT_ONBOARDING.md` 是唯一入口，10 步 + 校验清单，不是口头约定。
- 每个被管工程在 `workspace/verification.yaml` 有**一条路由**：`name` / `path` / `verify`（最小收口）/ 按需 `unit`·`api`·`e2e`·`sandbox`。
- 路由登记的命令**从 harness 根原样跑得通**：脚本 CWD 无关，`path` 真存在，`make -C` 的 target 真存在。
- 规则**随工程进 `projects/`**：工程红线在 `projects/<name>/AGENTS.md`（精简 + 指针），细规则下沉到各层 `projects/<name>/<dir>/AGENTS.md`，就近生效——不堆进根 `AGENTS.md`。
- 每个 `AGENTS.md` 有同级 `CLAUDE.md` shim（`@AGENTS.md`），否则 Claude Code 加载不到该层规则。
- 控制面**只管不持有**：业务 unit/api/e2e 测试本体在工程里、与代码同处；控制面只做路由 + 执行 + 评分 + 收证据。

## 怎么检索现状（命令可直接跑，CWD = harness 根）

```bash
# 接入流程在不在、是不是当前态
cat docs/harness/PROJECT_ONBOARDING.md
cat docs/harness/VERIFICATION_ROUTING.md

# 路由表全文 + 每个工程登记了什么
cat workspace/verification.yaml

# 路由里每个 path 真存在吗
grep -E '^\s+path:' workspace/verification.yaml

# 每个被管工程有没有入口 AGENTS.md / 就近下沉了几层
find projects -name AGENTS.md -not -path '*/.git/*'

# 每个 AGENTS.md 有没有 CLAUDE.md shim（控制面自检也查这条）
bash scripts/verify-control-plane.sh   # 看 "AGENTS.md ↔ CLAUDE.md shim" 段

# 路由命令真能跑（最硬的一关）：从 harness 根原样跑
make -C projects/kratos-base verify           # verify 路由
bash projects/kratos-base/test/resilience/run_all.sh   # e2e 路由
```

## 怎么判（逐条可判定）

- **流程齐**：`PROJECT_ONBOARDING.md` 在、status active、引用的 `templates/feature-package.md`·`workspace/verification.yaml`·各 skill 都真存在（被 `make docs-audit` 兜）。缺步骤或指向死文件 = 缺口。
- **每个 `projects/<name>/` 在路由里有一条**：`projects/` 下有目录但 `verification.yaml` 里查不到对应 `name` = 漂移漏登记。
- **路由的 `path` 真存在**：上面 grep 出的每个 path `[ -d ]` 成立，否则 = 死路由。
- **路由命令真跑得通**（不是只读、要亲跑）：从 harness 根跑 `verify`/`e2e` 命令退 0。脚本必须 CWD 无关（开头自 `cd "$(dirname …)/../.."`，如 `run_all.sh` 第 9/15 行）——只在工程目录里能过 = 坏路由（见漏洞模式）。
- **就近规则随工程**：工程根 `AGENTS.md` 精简（红线 + 指针），细规则下沉到层目录（kratos-base 已下沉 `pkg/*`·`app/demo/internal/*`·`test/resilience` 11 层）。所有规则堆根 = 不符合"就近"。
- **shim 完整**：每个 `AGENTS.md` 旁有 `CLAUDE.md` 且含 `@AGENTS.md`；`verify-control-plane.sh` 会判失败。
- **测试归位**：业务测试本体在 `projects/<name>/` 内，不在控制面 `scripts/` 下。

## 常见漏洞模式（本仓真实案例）

- **路由登记的命令"亲跑即挂"——CWD 假设**（`tasks/lessons.md` 2026-06-02：「e2e 脚本隐含 CWD 假设，登记的验证命令实际是坏的」）：弹性脚本假设 CWD=工程根（裸 `make`/`go build`），而 `verification.yaml` 登记的命令从 harness 根调用，亲跑全挂；子代理在工程目录里跑全过，**掩盖了坏路由**。判据：必须从路由的工作目录、用路由里的原始命令亲跑一次。修复后 `run_all.sh` 开头自 `cd "$SCRIPT_DIR/../.."`（第 15 行）做到 CWD 无关——这就是健康态长相。
- **宣称全绿但没跑工程唯一门禁**（`tasks/lessons.md` 2026-06-02：「宣称"全绿"但没跑项目唯一门禁命令」）：接入后声称验证通过，却没跑 `verification.yaml` 登记的 `verify`。撞 rule-0002/0003。判据：路由命令的真实退出码 + 证据，不接受口头。
- **接入靠口头、流程不成文**：没有 `PROJECT_ONBOARDING.md` 沉淀，下一个工程接入全凭记忆 → 漏填路由 / 漏写 shim / 规则堆根。判据：接入步骤是否落在文件里、是否有校验清单。
- **路由漂移**：`projects/` 里有工程但 `verification.yaml` 没登记，或登记的 `path`/命令指向已改名/删除的目录（参 `tasks/lessons.md` 2026-05-29「目录改名后直接写旧逻辑路径会失败」的同类机理）。

## 修复用哪个操作 skill / 脚本

- **接新工程 / 补缺步骤**：照 `docs/harness/PROJECT_ONBOARDING.md` 10 步 + 校验清单走（缺哪步补哪步）。
- **改/补验证路由**：编辑 `workspace/verification.yaml`，按 `docs/harness/VERIFICATION_ROUTING.md` 填 `verify`/`unit`/`e2e`/`sandbox`；**填完从 harness 根原样亲跑一次**。
- **工程级规则落地（就近）**：用 `hc-add-rule` skill（定范围 → 写进就近 `AGENTS.md`/`docs/rules` + 登记 → 挂 hook/eval 执行）。
- **第一个需求包**：用 `hc-dev` skill 深度级（rule-0001）。
- **结构 / shim 体检**：`bash scripts/verify-control-plane.sh`（兜 shim 缺失）+ `make docs-audit`（兜 onboarding 引用的死文件）。注意：当前 `verify-control-plane.sh` 只校验 `verification.yaml` **存在**，**不**校验路由命令真能跑——路由命令亲跑这一关仍要手动做。
