# 固定 A～F 实验矩阵

固定 Serving 为 n28 / 152650，Target 为已知成功驻留过的 n41 / 504990。每轮使用 QSH ID 96 mask `0x8007`，以 `0xd8f582a8` 的有效目标 ARFCN 作为成功条件。

| 实验 | 工具动作 | 是否可能断网 | 成功条件 | 当前认识 |
|---|---|---:|---|---|
| A | 直接通过 QRTR 发送 advanced NAS `0x0085`，NGRAN + 单 ARFCN，结束时 `0x00c2` abort | 否 | ML1 返回 504990 | 已知失败基线；请求送达不代表逐频测量 |
| B | 外部驱动完整进入 QCRIL `startNetworkScan` | 否 | QCRIL wire 与 A 有关键差异，并搜到 504990 | ONE_SHOT 静态路径会丢 Band/channel，仍需 hook 实测闭环 |
| C | 临时只允许 n41，不显式触发 acquisition | 可能 | 不重启 acquisition 即搜到 504990 | 对应“设置只在何时生效”的控制组；厂商 Band setter 自身可能重选 |
| D | 临时只允许 n41，然后重新应用自动选网模式 | 是 | 重选后首个有效 504990 | 验证新 acquisition 周期是否足够 |
| E | 临时 Only_LTE，写 n41 Band，再切 Only_5G；首条结果即停止 | 是 | 未等完整注册即得到 504990 PCI/RSRP | 当前最可能形成产品能力的路径 |
| F | 已知 PCI+504990+n41 小区锁，然后 Only_5G | 是 | 已知目标成为 serving 或产生有效 ML1 结果 | 强正控制；必须提供真实 PCI |

## 统一事务边界

```text
拒绝已有 diag_mdlog
→ 检查 RAM 和 /tmp 空间
→ 保存网络模式、SA/NSA Band、NR 小区锁
→ 写恢复 journal
→ 从正式配置生成 ID96=0x8007 临时配置
→ 启动 2 MiB × 2 的短时 QMDL
→ 执行单个实验动作
→ 每 100 ms 解析 QMDL 和检查资源
→ 首个有效目标结果 / 超时 / 低内存 / 信号中断
→ 停止或 abort 主动流程
→ 停止 diag_mdlog
→ 重新应用正式 ID96=0x0007 配置
→ 恢复全部网络设置
→ 写 JSONL、CSV 和单轮 JSON
```

Trace 恢复不只依赖 `defer`：启用 ID96 bit15 前，工具会先将正式 QSH 配置内嵌进 journal 并标记 `trace_restore_required=true`。如果进程被 `SIGKILL`，`recover` 会先恢复低频 Trace，再恢复网络。

## 结果解释

### CM 有目标 ARFCN，但 ML1 仍是 152650

目标列表已经进入 CM，但 acquisition policy 没有为目标 Band 建立真实 search task。不能把本轮标为成功。

### B 与 A 的 `0x0085` 完全一致

QCRIL 没有提供额外关键字段或前置请求，可关闭“完整 QCRIL 路径会更强”的方向。

### C 失败，D 成功

Band/channel selection 只在新的 acquisition 周期生效。可以继续优化“短暂 acquisition → 首条结果 → 恢复”的中断时长。

### D 失败，E/F 成功

固件只允许在 NR 被移除或 no-service 状态下执行跨 Band acquisition。正式能力只能采用短时服务中断。

### 只有 F 成功

本固件没有开放驻网期间的指定跨 Band 测量；只能使用临时锁定/重选作为折中方案。

## 实验 B 的关闭标准

B 必须同时保留 CCI hook 和 QSH：

1. 记录 QCRIL 前后的所有 NAS QMI 请求、响应和 indication。
2. 用 `diff-qmi` 与 A 的 `0x0085` 做 TLV 差分。
3. 检查 QCRIL 是否真的发送 TLV `0x1d`。
4. 即使字节不同，也必须由 `0xd8f582a8` 证明 ML1 搜了 504990。

若 ONE_SHOT 实测只发 `10 01 00 <current RAT mask>`，则与静态反编译一致，B 可直接关闭；没有必要继续逆 qcrild socket 以获得一个必然丢 channel 的请求。

CCI hook 日志通过 `-qmi-trace-log` 绑定到单轮报告。没有该输入时，A 仍能记录自己的 raw QRTR wire，B～F 只记录控制动作和 QSH 证据，不应将 `qmi_trace_complete=false` 解读为“没有发送 QMI”。
