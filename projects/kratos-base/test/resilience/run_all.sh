#!/usr/bin/env bash
# run_all.sh — 依次跑 AC1-AC6 + AC-R1~R3 + AC-C2(etcd conf) + AC-D(etcd disc) + AC-M1~M3(MQ) + AC-N1~N2(nacos) + AC-CR1~CR2(runtime-down cc/reg ×{etcd,nacos}) + AC-MR1~MR3(rocketmq e2e) 弹性验收场景，打印通过矩阵。
# Usage: bash test/resilience/run_all.sh
# Must be run from the project root (projects/kratos-base/).
# Each sub-script has its own setup/teardown (trap cleanup), so they
# don't pollute each other.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Sub-scripts use bare `go build` / `make` / relative paths, so they must run
# with CWD = project root. cd there now (derived from this script's location)
# so run_all works from ANY CWD — including the harness root, which is how
# workspace/verification.yaml's e2e command invokes it.
cd "$SCRIPT_DIR/../.." || { echo "cannot cd to project root"; exit 1; }

# Result tracking
AC1_STATUS="FAIL"
AC2_STATUS="FAIL"
AC3_STATUS="FAIL"
AC4_STATUS="FAIL"
AC5_STATUS="FAIL"
AC6_STATUS="FAIL"
ACR1_STATUS="FAIL"
ACR2_STATUS="FAIL"
ACR3_STATUS="FAIL"
ACC2_STATUS="FAIL"
ACD_STATUS="FAIL"
ACM1_STATUS="FAIL"
ACM2_STATUS="FAIL"
ACM3_STATUS="FAIL"
ACN1_STATUS="FAIL"
ACN2_STATUS="FAIL"
ACCR1_ETCD_STATUS="FAIL"
ACCR1_NACOS_STATUS="FAIL"
ACCR2_ETCD_STATUS="FAIL"
ACCR2_NACOS_STATUS="FAIL"
ACCF_STATUS="FAIL"
ACMR1_STATUS="FAIL"
ACMR2_STATUS="FAIL"
ACMR3_STATUS="FAIL"

overall_exit=0

run_scenario() {
    local name="$1"
    local script="$2"
    local ac_label="$3"

    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║  Running: $name"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo ""

    # shellcheck disable=SC2086 — word splitting intentional for scripts with arguments
    if bash $script; then
        echo ""
        echo "── $name: PASSED ──"
        return 0
    else
        local rc=$?
        echo ""
        echo "── $name: FAILED (exit $rc) ──"
        return $rc
    fi
}

# ── AC1: scen_boot_dep_down ───────────────────────────────────────────────────
if run_scenario "AC1: scen_boot_dep_down" "$SCRIPT_DIR/scen_boot_dep_down.sh" "AC1"; then
    AC1_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC2: scen_recover ─────────────────────────────────────────────────────────
if run_scenario "AC2: scen_recover" "$SCRIPT_DIR/scen_recover.sh" "AC2"; then
    AC2_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC3: scen_runtime_drop ────────────────────────────────────────────────────
if run_scenario "AC3: scen_runtime_drop" "$SCRIPT_DIR/scen_runtime_drop.sh" "AC3"; then
    AC3_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC4: scen_config_hot ──────────────────────────────────────────────────────
if run_scenario "AC4: scen_config_hot" "$SCRIPT_DIR/scen_config_hot.sh" "AC4"; then
    AC4_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC5+AC6: scen_observability ───────────────────────────────────────────────
if run_scenario "AC5+AC6: scen_observability" "$SCRIPT_DIR/scen_observability.sh" "AC5+AC6"; then
    AC5_STATUS="PASS"
    AC6_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-R1: scen_redis_boot_down ───────────────────────────────────────────────
if run_scenario "AC-R1: scen_redis_boot_down" "$SCRIPT_DIR/scen_redis_boot_down.sh" "AC-R1"; then
    ACR1_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-R2: scen_redis_recover ─────────────────────────────────────────────────
if run_scenario "AC-R2: scen_redis_recover" "$SCRIPT_DIR/scen_redis_recover.sh" "AC-R2"; then
    ACR2_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-R3: scen_redis_drop ────────────────────────────────────────────────────
if run_scenario "AC-R3: scen_redis_drop" "$SCRIPT_DIR/scen_redis_drop.sh" "AC-R3"; then
    ACR3_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-C2: scen_conf_etcd ─────────────────────────────────────────────────────
if run_scenario "AC-C2: scen_conf_etcd" "$SCRIPT_DIR/scen_conf_etcd.sh" "AC-C2"; then
    ACC2_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-D: scen_disc_etcd ──────────────────────────────────────────────────────
if run_scenario "AC-D: scen_disc_etcd" "$SCRIPT_DIR/scen_disc_etcd.sh" "AC-D"; then
    ACD_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-M1: scen_mq_boot_down ──────────────────────────────────────────────────
if run_scenario "AC-M1: scen_mq_boot_down" "$SCRIPT_DIR/scen_mq_boot_down.sh" "AC-M1"; then
    ACM1_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-M2: scen_mq_recover ────────────────────────────────────────────────────
if run_scenario "AC-M2: scen_mq_recover" "$SCRIPT_DIR/scen_mq_recover.sh" "AC-M2"; then
    ACM2_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-M3: scen_mq_drop ───────────────────────────────────────────────────────
if run_scenario "AC-M3: scen_mq_drop" "$SCRIPT_DIR/scen_mq_drop.sh" "AC-M3"; then
    ACM3_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-N1: scen_conf_nacos ────────────────────────────────────────────────────
if run_scenario "AC-N1: scen_conf_nacos" "$SCRIPT_DIR/scen_conf_nacos.sh" "AC-N1"; then
    ACN1_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-N2: scen_disc_nacos ────────────────────────────────────────────────────
if run_scenario "AC-N2: scen_disc_nacos" "$SCRIPT_DIR/scen_disc_nacos.sh" "AC-N2"; then
    ACN2_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-CR1(etcd): scen_cc_runtime_down etcd ───────────────────────────────────
if run_scenario "AC-CR1(etcd): scen_cc_runtime_down etcd" "$SCRIPT_DIR/scen_cc_runtime_down.sh etcd" "AC-CR1-etcd"; then
    ACCR1_ETCD_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-CR1(nacos): scen_cc_runtime_down nacos ─────────────────────────────────
if run_scenario "AC-CR1(nacos): scen_cc_runtime_down nacos" "$SCRIPT_DIR/scen_cc_runtime_down.sh nacos" "AC-CR1-nacos"; then
    ACCR1_NACOS_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-CR2(etcd): scen_reg_runtime_down etcd ──────────────────────────────────
if run_scenario "AC-CR2(etcd): scen_reg_runtime_down etcd" "$SCRIPT_DIR/scen_reg_runtime_down.sh etcd" "AC-CR2-etcd"; then
    ACCR2_ETCD_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-CR2(nacos): scen_reg_runtime_down nacos ────────────────────────────────
if run_scenario "AC-CR2(nacos): scen_reg_runtime_down nacos" "$SCRIPT_DIR/scen_reg_runtime_down.sh nacos" "AC-CR2-nacos"; then
    ACCR2_NACOS_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-CF: scen_conf_boot_fastfail (etcd 配置源启动 fail-fast) ─────────────────
if run_scenario "AC-CF: scen_conf_boot_fastfail" "$SCRIPT_DIR/scen_conf_boot_fastfail.sh" "AC-CF"; then
    ACCF_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-MR1: scen_mq_rocketmq (rocketmq publish→consume e2e) ──────────────────
if run_scenario "AC-MR1: scen_mq_rocketmq" "$SCRIPT_DIR/scen_mq_rocketmq.sh" "AC-MR1"; then
    ACMR1_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-MR2: scen_mq_rocketmq_boot_down (rocketmq 启动期宕+自愈) ───────────────
if run_scenario "AC-MR2: scen_mq_rocketmq_boot_down" "$SCRIPT_DIR/scen_mq_rocketmq_boot_down.sh" "AC-MR2"; then
    ACMR2_STATUS="PASS"
else
    overall_exit=1
fi

# ── AC-MR3: scen_mq_rocketmq_drop (rocketmq 运行期断+恢复) ───────────────────
if run_scenario "AC-MR3: scen_mq_rocketmq_drop" "$SCRIPT_DIR/scen_mq_rocketmq_drop.sh" "AC-MR3"; then
    ACMR3_STATUS="PASS"
else
    overall_exit=1
fi

# ── Final matrix ─────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        AC1-AC6 + AC-R1~R3 + AC-C2 + AC-D + AC-M1~M3 矩阵    ║"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-6s  启动期依赖宕、服务不崩               %-6s       ║\n" "AC1" "$AC1_STATUS"
printf "║  %-6s  按需连 + 自愈（不重启）               %-6s       ║\n" "AC2" "$AC2_STATUS"
printf "║  %-6s  运行中断连 → 快速失败 → 恢复          %-6s       ║\n" "AC3" "$AC3_STATUS"
printf "║  %-6s  配置热更 + 坏配置回滚                 %-6s       ║\n" "AC4" "$AC4_STATUS"
printf "║  %-6s  可观测：JSON日志+trace_id+metrics+span %-6s       ║\n" "AC5" "$AC5_STATUS"
printf "║  %-6s  网关链路：HTTP→service→ent 正确返回   %-6s       ║\n" "AC6" "$AC6_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-6s  redis启动期宕、readyz/hits/greet正确  %-6s       ║\n" "AC-R1" "$ACR1_STATUS"
printf "║  %-6s  redis恢复自愈、hits计数递增（不重启） %-6s       ║\n" "AC-R2" "$ACR2_STATUS"
printf "║  %-6s  redis运行中断→快速失败→恢复续上       %-6s       ║\n" "AC-R3" "$ACR3_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-6s  etcd配置热更+坏配置回滚+flip闭环      %-6s       ║\n" "AC-C2" "$ACC2_STATUS"
printf "║  %-6s  注册非致命+发现闭环(discoveryprobe)   %-6s       ║\n" "AC-D" "$ACD_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-6s  MQ启动期宕、readyz/events/pg/redis正确%-6s       ║\n" "AC-M1" "$ACM1_STATUS"
printf "║  %-6s  MQ恢复自愈、事件发布+消费（不重启）  %-6s       ║\n" "AC-M2" "$ACM2_STATUS"
printf "║  %-6s  MQ运行中断→快速失败→恢复续消费        %-6s       ║\n" "AC-M3" "$ACM3_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-6s  nacos配置热更+坏配置回滚+flip闭环      %-6s       ║\n" "AC-N1" "$ACN1_STATUS"
printf "║  %-6s  nacos注册非致命+发现闭环(nacosprobe)   %-6s       ║\n" "AC-N2" "$ACN2_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-10s 配置中心运行期宕→不崩→恢复续上(etcd)  %-6s   ║\n" "AC-CR1(e)" "$ACCR1_ETCD_STATUS"
printf "║  %-10s 配置中心运行期宕→不崩→恢复续上(nacos) %-6s   ║\n" "AC-CR1(n)" "$ACCR1_NACOS_STATUS"
printf "║  %-10s 注册中心运行期宕→disc失败→恢复续上(e) %-6s   ║\n" "AC-CR2(e)" "$ACCR2_ETCD_STATUS"
printf "║  %-10s 注册中心运行期宕→disc失败→恢复续上(n) %-6s   ║\n" "AC-CR2(n)" "$ACCR2_NACOS_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-10s etcd配置源启动fail-fast(无配置无缓存) %-6s   ║\n" "AC-CF" "$ACCF_STATUS"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  %-6s  rocketmq publish→consume e2e闭环        %-6s       ║\n" "AC-MR1" "$ACMR1_STATUS"
printf "║  %-6s  rocketmq启动期宕→/ping正常→自愈+消费    %-6s       ║\n" "AC-MR2" "$ACMR2_STATUS"
printf "║  %-6s  rocketmq运行期断→有界失败→恢复续消费    %-6s       ║\n" "AC-MR3" "$ACMR3_STATUS"
echo "╚══════════════════════════════════════════════════════════════╝"

if [ $overall_exit -eq 0 ]; then
    echo ""
    echo "ALL AC1-AC6 + AC-R1~R3 + AC-C2 + AC-D + AC-M1~M3 + AC-N1~N2 + AC-CR1~CR2 + AC-MR1~MR3 PASSED"
else
    echo ""
    echo "SOME ACs FAILED — see output above for diagnosis"
    exit 1
fi
