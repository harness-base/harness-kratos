#!/usr/bin/env bash
# hook-policy 的自测：喂样例，验证该拦的拦、该放的放。改 policy 必须同步改本测试。
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
POLICY="$ROOT/scripts/hook-policy.sh"
pass=0; fail=0

expect_block(){ # $1 desc $2 input
  if printf '%s' "$2" | bash "$POLICY" >/dev/null 2>&1; then
    echo "  ✗ 应拦未拦：$1"; fail=$((fail+1))
  else pass=$((pass+1)); fi
}
expect_ok(){
  if printf '%s' "$2" | bash "$POLICY" >/dev/null 2>&1; then
    pass=$((pass+1))
  else echo "  ✗ 应放行却拦：$1"; fail=$((fail+1)); fi
}

expect_block "bearer token"  'Authorization: Bearer abcdef1234567890XY'
expect_block "api key 赋值"  'api_key = "sk-abcdef1234567890ZZ"'
expect_block "reset --hard"  'git reset --hard origin/main'
expect_block "危险 rm"       'rm -rf /'
expect_ok    "普通代码"      'func main() { println("hi") }'
expect_ok    "普通中文文档"  '本规则禁止泄露密钥与执行危险命令。'

echo "hook-policy.test: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
