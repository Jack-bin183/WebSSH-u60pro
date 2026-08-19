# U60 Pro NR + LTE 合并帧级最小配置实测（2026-08-14）

## 环境

- 设备：U60 Pro / Qualcomm X75
- 冷状态：整机启动约 4 分钟，`/tmp` 已清空，`diag_mdlog` 未运行
- 网络选择：`LTE_AND_5G`
- 网络类型：ENDC（LTE-NSA）
- LTE Serving：B3，PCI 43，EARFCN 1300，20 MHz，RSRP 约 -84 dBm
- NR Serving：n41，PCI 509，NR-ARFCN 504990，100 MHz，RSRP 约 -78 dBm

NR 与 LTE 的独立整帧实验已经分别证明：

- 23 个 `0x7D/0x04` 消息帧的必要集合为空；
- `0x44/0x9001-only` 可从独立冷启动状态启动目标 QSH 流；
- `0x55/0x0004-only` 不能启动目标 QSH 流；
- 最小帧级候选对两个 RAT 都是同一个 196-byte 配置。

因此合并阶段不再重复对相同 23 帧做无信息增益的 ddmin，而是在真实 ENDC 环境中直接验证该集合的并集，并要求同一采集窗口同时出现完整 NR 与 LTE 邻区。

## 冷状态合并验证

本轮从新的整机冷启动状态仅发送：

```text
0 x 0x7D/0x04 message frame
1 x 0x4B subsystem=0x44 subcommand=0x9001
0 x 0x4B subsystem=0x55 subcommand=0x0004
```

结果：

```text
first complete combined hit = 487 ms
capture window              = 10,000 ms
0x9D frame count            = 156,308
QSH payload bytes           = 5,068,792
target hash count           = 858
parse success count         = 858
parse error count           = 10
valid NR neighbor cells     = 3
valid LTE neighbor cells    = 3
result                      = true
```

有效 NR 邻区均位于 NR-ARFCN 504990 / n41，PCI 为 41、120、567，RSRP 约 -87.74 到 -103.01 dBm。有效 LTE 邻区均位于 EARFCN 1300 / B3，PCI 为 41、42、240，RSRP 约 -92.68 到 -98.45 dBm。NR Serving PCI 509 和 LTE Serving PCI 43 均已排除。

## 同启动重复性

冷状态首轮后，在同一 ENDC 启动中显式清零 23 个消息范围、重发 `0x9001`，再做两次 10 秒顺序复验。它们用于验证合并目标的重复性，不冒充额外的独立冷启动样本。

| 轮次 | 冷状态 | 首命中 | `0x9D` 帧 | 解析成功 | NR 邻区 | LTE 邻区 | 结果 |
|---|---|---:|---:|---:|---:|---:|---|
| 1 | 是 | 487 ms | 156,308 | 858 | 3 | 3 | 成功 |
| 2 | 否，同启动复验 | 789 ms | 146,142 | 938 | 5 | 3 | 成功 |
| 3 | 否，同启动复验 | 874 ms | 305,896 | 942 | 3 | 2 | 成功 |

三轮均满足严格合并判定：同一采集窗口内既有完整、合法、非 Serving 的 NR 邻区，也有完整、合法、非 Serving 的 LTE 邻区。仅出现格式哈希而字段解析失败的记录不计入命中。

## 最终帧级配置

```text
file    = configs/cell-neighbor-combined-min.cfg
frames  = 1
bytes   = 196
sha256  = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
command = 0x4B subsystem=0x44 subcommand=0x9001
```

## 结论与边界

1. NR、LTE、NR + LTE 合并三个目标的帧级最小配置完全相同：只保留 `0x44/0x9001`。
2. 合并配置从独立 ENDC 冷状态启动，并在 487 ms 内同时产生两种 RAT 的完整邻区；同启动复验达到 3/3。
3. 23 个 `0x7D/0x04` 消息帧和 `0x55/0x0004` 均可从当前三个帧级配置中删除。
4. 本轮结束后 `diag_mdlog` 无遗留，23 个原消息范围已再次显式清零。
5. 三个最小文件当前字节内容完全相同，但分别保留文件名，便于调用方表达目标和后续独立演进。
6. “最小”仍只指帧级；`0x9001` 内部 43 组私有 ID/掩码尚未缩减。
7. 当前结果只适用于该 U60 Pro 当前固件与本次 ENDC RF 环境，不推广为所有 X75 固件结论。
8. 本报告完成时尚未替换正式配置；随后在 2026-08-14 上线阶段，正式 WebSSH `qtrace.cfg` 已接入该 196-byte 合并配置，按需 QMDL 采集方式保持不变。
