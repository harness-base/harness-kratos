# sandbox 审查手册

被管工程的**可复现本地验证环境**这一环：起得来、销得净、CWD 无关，且 `workspace/verification.yaml` 路由登记的命令**真能从路由的工作目录跑通**。

## 规范（健康长什么样 / 不变量）

- **一键起 / 一键销**：有 `sandbox-up` / `sandbox-down`（或等价），销要**幂等 + 清卷**（`down -v`），重复跑不报错、不残留。
- **CWD 无关**：任何脚本不假设 `CWD=工程根`。两种合法形态：① 脚本开头自 `cd` 到根（`cd "$(dirname "$0")/../.."`）；② 命令用 `make -C <projectdir>` 把 CWD 钉死。裸 `make` / `go build` / 相对 `-f compose.yaml` 必须配套其一。
- **起后等 healthy 才返回**：用 docker healthcheck 轮询，**绝不 `sleep` 凑数**；超时打容器日志再 `exit 1`。
- **路由对得上**：`workspace/verification.yaml` 的 `sandbox` / `e2e` 命令，**逐字从其工作目录亲跑能过**——登记 ≠ 能跑。
- **依赖固定项目名**：compose 用固定 `-p <name>`（如 `-p kratosbase-sandbox`），保证 up/down 操作同一组容器、teardown 确定。

## 怎么检索现状（索引 / 文件 / 机器检查入口）

```bash
# 1. 列 sandbox 资产 + 路由事实源
ls projects/*/deploy/sandbox/                              # compose / initdb / broker 配置
cat workspace/verification.yaml                            # sandbox / e2e / unit / verify 路由
cat projects/kratos-base/deploy/sandbox/README.md          # sandbox 静态索引（怎么用 / 谁驱动）

# 2. 核 CWD 假设：scen/run_all 是否自 cd；Makefile compose 是否相对路径
grep -nE 'cd "\$|dirname|BASH_SOURCE' projects/kratos-base/test/resilience/run_all.sh   # 应有 cd "$SCRIPT_DIR/../.."
grep -nE 'sandbox-(up|down)|-f deploy/sandbox|-p ' projects/kratos-base/Makefile        # 相对 -f → 必须靠 -C 或自 cd
grep -nE 'make sandbox|trap|cleanup' projects/kratos-base/test/resilience/scen_*.sh     # 每 scen 自起自销

# 3. 亲跑路由（唯一可信的"能跑"证据）——用路由原文、从路由工作目录
make -C projects/kratos-base sandbox-up && make -C projects/kratos-base sandbox-down
bash projects/kratos-base/test/resilience/run_all.sh       # e2e 路由原文，从 harness 根跑
```

注意：`make verify`（`scripts/verify-control-plane.sh:11`）只校验 `workspace/verification.yaml` **文件在不在**，**不**验证路由命令可跑——sandbox 维度没有机器兜底，必须**亲跑**。

## 怎么判（逐条可判定）

- **能一键起 / 销？** 有 `sandbox-up` / `sandbox-down`，down 含 `-v` 且幂等（连跑两次第二次不报错）→ 符合；只起不销、或销不清卷 → 缺口。
- **CWD 假设？** 脚本里裸 `make` / `go build` / 相对 `-f`，又**没有**自 `cd` 也**没有** `-C` 钉死 → 漏洞（lessons 2026-06-02 原型）。形态①或②满足其一 → 符合。
- **路由命令真能跑？** 从路由登记的工作目录、用路由原文逐字跑：过 → 符合；任一非零退出 / blocked / skipped → 漏洞（`blocked/skipped ≠ pass`，rule-0002）。
- **起后真就绪？** 等待靠 healthcheck 轮询 → 符合；靠固定 `sleep N` → 缺口（竞态假绿）。
- **README 与现状一致？** `deploy/sandbox/README.md` 列的文件 / 驱动方式与实际目录、与 `verification.yaml` 谁驱动谁对得上 → 符合；漂移 → 缺口。

## 常见漏洞模式（本仓真实案例）

- **脚本隐含 CWD 假设，登记的验证命令实际是坏的**（`tasks/lessons.md` 2026-06-02）：弹性脚本假设 `CWD=工程根`（裸 `make`/`go build`），而 `verification.yaml` 登记的命令从 harness 根调用——**从路由工作目录亲跑即全挂**；子代理恰好在工程目录跑全过，**掩盖了问题**。教训沉淀为 `test/resilience/AGENTS.md` 的 `kratos/test-cwd-invariant`（warn），修法见 `run_all.sh:12-13`。
- **"在工程目录跑过了"≠ 路由能跑**：同上案例的根因——验证位置和路由位置不一致就会假绿。登记路由前**必须用路由原文、从路由的工作目录**亲跑一次。
- **teardown 不幂等 / 不清卷**：down 不带 `-v` 或项目名不固定 → 卷 / 旧容器残留，下次 up 撞脏数据假绿/假红。本仓正解：`-p kratosbase-sandbox ... down -v`（Makefile `sandbox-down`），且每个 `scen_*.sh` 在 `trap cleanup` 里 `make sandbox-down ... || true`。
- **sleep 凑就绪**：用固定睡眠代替 healthcheck 轮询 → 慢机 / 冷启动竞态假失败。本仓正解：`sandbox-up` 逐容器轮询 `docker inspect ... Health.Status`（nacos/broker 给到 120s）。

## 修复用哪个操作 skill / 脚本

- **改脚本 CWD 无关**：脚本开头加 `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; cd "$SCRIPT_DIR/../.."`，或把路由命令改成 `make -C <projectdir> ...`。范式：`projects/kratos-base/test/resilience/run_all.sh:9-15`。
- **亲跑路由收口**：`make -C projects/kratos-base sandbox-up && ... sandbox-down`；`bash projects/kratos-base/test/resilience/run_all.sh`（路由原文，从 harness 根）。
- **路由 / CWD 红线没沉淀就补**：把"CWD 无关 + 登记前亲跑"写成就近规则 → 走 `hc-add-rule`（落 `test/resilience/AGENTS.md` 一类就近位，带 `<!-- rule: -->` 标记）。
- **改 sandbox 环境本身**（加依赖容器 / initdb）：改 `projects/<name>/deploy/sandbox/docker-compose.yaml` + 同步该目录 `README.md` 索引 + `Makefile` 的 up/down 等待逻辑。
- **路由登记 / 文档对账**：事实源 `workspace/verification.yaml`；路由规约见 `docs/harness/VERIFICATION_ROUTING.md`；改完跑 `make docs-audit`。
