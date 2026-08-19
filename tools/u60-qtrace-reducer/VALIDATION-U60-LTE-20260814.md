# U60 Pro LTE 整帧级最小化实测（2026-08-14）

## 环境

- 设备：U60 Pro / Qualcomm X75
- 冷状态：整机启动约 11 分钟，`/tmp` 已清空，`diag_mdlog` 未运行
- 网络选择：`Only_LTE`
- 网络：CMCC LTE B3
- Serving：PCI 43，EARFCN 1300，20 MHz
- 信号：LTE RSRP 约 -81 dBm，RSRQ 约 -9 dB，SINR 约 12 dB
- 原配置：23 个 `0x7D/0x04` 消息帧，466 个 SSID，2 条私有 `0x4B` 命令

## 冷状态控制组

| 组 | 状态 | LTE 命中 | `0x9D` | 结果 |
|---|---|---:|---:|---|
| all-off | 23 范围全零，不发私有命令 | 0/3 | 0/3 | 符合负控制 |
| message-only | 23 帧全开，不发私有命令 | 0/3 | 0/3 | 冷状态下仅消息掩码不会启动 QSH 流 |
| positive | 23 帧全开 + 两条私有命令 | 3/3 | 86,650–139,252 帧/10s | 正控制稳定 |
| zero-mask-private | 23 范围全零 + 两条私有命令 | 3/3 | 93,401–108,788 帧/10s | 与预期负控制相反 |
| positive-end | 23 帧全开 + 两条私有命令 | 3/3 | 93,960–127,739 帧/10s | 批末正控制稳定 |

正控制每轮解析出 1–5 个有效 LTE 邻区。服务小区 PCI 43 / EARFCN 1300 已排除；观测到的有效邻区包括 PCI 41、42、149、190、240，均有合法 EARFCN 和非缺省 RSRP。

`all-off` 和 `message-only` 均没有 `0x9D`。`message-only` 的第一次采集出现 1 个非目标解析告警，但没有 QSH 帧、目标格式哈希或有效小区，因此不构成命中。

## 整帧 ddmin

- 校准正控制：10/10 命中
- 首个完整 LTE 快照延迟：421, 430, 435, 439, 451, 464, 477, 493, 806, 1,335 ms
- P95：1,335 ms
- 候选窗口：3,335 ms（P95 + 2s）
- 空消息帧集合 + 两条私有命令：3/3 命中
- 空集合独立复验：3/3 命中
- 批末正控制：3/3 命中

整帧级结果：

```text
LTE required 0x7D/0x04 frame IDs = []
LTE required SSIDs at this stage = []
retained private commands         = 0x44/0x9001 + 0x55/0x0004
output frame count                = 2
output bytes                      = 204
output SHA-256                    = e11cb31aee07ac0c626d9e5d203d0589ceab734ef538849d44b5e4ad7defdd4f
```

以上 204-byte 文件只是 LTE 的整帧阶段候选，不是最终 `lte-neighbor-min.cfg`。两条私有命令尚未在 LTE 环境中分别完成独立冷启动象限验证。

## 独立冷启动 `0x44/0x9001-only` 复验

第二次 LTE 整机冷启动后，确认 `/tmp` 已清空、`diag_mdlog` 未运行，设备仍驻留 CMCC Only_LTE B3，Serving PCI 43 / EARFCN 1300，RSRP 约 -81 dBm。本轮唯一冷状态候选为：

```text
0 x 0x7D/0x04 message frame
+ 0x44/0x9001
0 x 0x55/0x0004
```

结果：

```text
first complete hit latency = 4,513 ms
capture window             = 10,000 ms
0x9D frame count           = 116,403
QSH payload bytes          = 3,429,704
target hash count          = 118
parse success count        = 118
parse error count          = 7
valid LTE neighbor cells   = 2
result                     = true
candidate config bytes     = 196
candidate config SHA-256   = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
```

服务小区 PCI 43 已排除。有效邻区为 PCI 41 / 149，EARFCN 均为 1300，RSRP 分别约 -92.23 / -92.19 dBm。采集完成后 `diag_mdlog` 无遗留，23 个原范围已再次显式清零。

本轮 4,513 ms 的首命中延迟明显高于 10 次校准的 1,335 ms P95，说明 LTE 邻区 measurement scheduling 存在长尾；10 秒冷象限窗口仍成功覆盖了本次延迟。

## 独立冷启动 `0x55/0x0004-only` 与同环境正控制

第三次 LTE 整机冷启动后，设备仍驻留 CMCC Only_LTE B3，Serving PCI 43 / EARFCN 1300。本轮唯一独立冷状态候选为：

```text
0 x 0x7D/0x04 message frame
0 x 0x44/0x9001
+ 0x55/0x0004
```

结果：

```text
capture window             = 10,000 ms
captured physical bytes    = 2,985,847
0x9D frame count           = 0
target hash count          = 0
parse success count        = 0
valid LTE neighbor cells   = 0
result                     = false
candidate config bytes     = 8
candidate config SHA-256   = 6db11bd6286dc6cb6fb36a9e82f73d80e48a28204796c0ac32d18e34d9007240
```

为排除当时 RF 环境没有产生邻区测量的假阴性，保留同一次启动中已发送的 `0x0004` 状态，再补发 `0x9001`。该记录明确标记为顺序正控制，不冒充独立冷状态象限：

```text
first complete hit latency = 452 ms
capture window             = 10,000 ms
0x9D frame count           = 149,848
QSH payload bytes          = 4,480,616
target hash count          = 341
parse success count        = 341
parse error count          = 14
valid LTE neighbor cells   = 3
result                     = true
```

有效邻区为 PCI 41、42、149，EARFCN 均为 1300，RSRP 约 -86.54 到 -90.05 dBm。这证明 `0x0004-only` 的失败不是当前 B3 环境无法产生 LTE 邻区造成的；加入 `0x9001` 后 QSH 与完整目标记录立即恢复。

## 同窗口输出量比较

Reducer 使用同为 3,335 ms 的批末正控制和最小集合独立复验进行比较：

| 配置 | 平均 `0x9D` 帧 | 平均 QSH payload 字节 |
|---|---:|---:|
| 23 消息帧 + 两条私有命令 | 40,339 | 1,228,600 |
| 0 消息帧 + 两条私有命令 | 41,247 | 1,259,599 |

本批最小候选的帧数反而高约 2.25%，payload 字节高约 2.52%。这符合轮间 RF/调度波动，说明删除 `0x7D` 消息帧并没有在当前目标 QSH 流上表现出可重复的输出量降幅；主要收益是配置从 23 个消息帧归零，而不是已经证明 QSH 流量降低。

## 当前结论与下一步

1. 本次 LTE 冷状态控制组有效，首尾正控制均为 3/3，负控制均为 0/3。
2. 冷状态下，仅发送原 23 个全开消息掩码不能启动 `0x9D` QSH 流。
3. 在两条私有命令保持发送的条件下，LTE 整帧级最小消息帧集合为空；无需进入单 SSID 或单 mask bit 缩减。
4. `0x44/0x9001` 已证明能从独立冷启动状态单独启动 LTE QSH 邻区流，因此 `0x55/0x0004` 对当前 LTE 目标不是必要条件。
5. `0x55/0x0004-only` 从独立冷状态无法启动 `0x9D`；同环境补发 `0x9001` 后正控制成功。因此它不能替代 `0x9001`，且对当前 LTE 目标可以删除。
6. 当前 LTE 帧级最小配置为单独 `0x44/0x9001`，编码长度 196 bytes，SHA-256 为 `15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58`。该文件保存为 `configs/lte-neighbor-min.cfg`。本报告完成时尚未替换正式配置；随后在 2026-08-14 上线阶段，正式 WebSSH 使用字节内容相同的 combined 配置。
7. 4,513 ms 的冷状态首命中表明 LTE 按需候选窗口不宜直接收紧到 3 秒；后续实机验证仍应保留 10 秒窗口或重新做延迟标定。
8. 本结论仍是“帧级”最小化；`0x9001` 内部 43 组 ID/掩码尚未继续缩减，不应将其内部私有字段命名为已知语义。
9. 本实测只适用于该 U60 Pro 当前固件与当时 LTE B3 RF 环境，不推广为所有 X75 固件结论。
