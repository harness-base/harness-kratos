level: green
prompts: ["010", "003", "002", "012", "001", "011"]
task: kratos-base-s6
generated_at: 20260623T150902Z
verdicts:
  "012": pass
  "003": pass
  "002": pass
  "010": pass
  "001": pass
  "011": n/a
blockers: []
warnings: []
note: >
  评委独立读码 + 真跑核实。make verify 绿；亲跑 AC-MR1 / AC-MR3 / CR1-b(etcd) 三条
  e2e 均 PASS（EXIT=0），断言命中行确为消费方/confcenter 产出方证据；AC-MR3 bounded-fail
  1.007s、recovery 续消费 total_received 1→2 同 pid；CR1-b retaining count 0→1。
  结束无残留容器、无 stray 进程。被遗弃 Send goroutine 在持续高流量 outage 下有界累积
  （≈速率×40s），已诚实标注，非 blocker。
