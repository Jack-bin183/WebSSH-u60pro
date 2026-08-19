# U60 Pro NR 整帧级最小化实测（2026-08-14）

## 环境

- 设备：U60 Pro / Qualcomm X75
- 冷状态：整机启动约 3 分钟，`/tmp` 已清空，`diag_mdlog` 未运行
- 网络：CMCC SA n41
- Serving：PCI 509，NR-ARFCN 504990
- 信号：NR RSRP 约 -79 dBm，RSRQ 约 -11 dB
- 原配置：23 个 `0x7D/0x04` 消息帧，466 个 SSID，2 条私有 `0x4B` 命令

## 冷状态控制组

| 组 | 状态 | NR 命中 | `0x9D` | 结果 |
|---|---|---:|---:|---|
| all-off | 23 范围全零，不发私有命令 | 0/3 | 0/3 | 符合负控制 |
| message-only | 23 帧全开，不发私有命令 | 0/3 | 0/3 | 冷状态下仅消息掩码不会启动 QSH 流 |
| positive | 23 帧全开 + 两条私有命令 | 3/3 | 199,073–269,992 帧/10s | 正控制稳定 |
| zero-mask-private | 23 范围全零 + 两条私有命令 | 3/3 | 145,973–271,861 帧/10s | 与预期负控制相反 |
| positive-end | 23 帧全开 + 两条私有命令 | 3/3 | 211,766–276,151 帧/10s | 批末正控制稳定 |

正控制首个完整 NR 快照延迟为 410–493 ms。`zero-mask-private` 的延迟为 431–475 ms，且每轮都有 7–9 个有效 NR 邻区，不符合短暂残留帧的表现。

## 整帧 ddmin

- 校准正控制：10/10 命中
- 首个完整快照延迟：430, 431, 437, 450, 456, 472, 520, 521, 557, 911 ms
- P95：911 ms
- 候选窗口：3,000 ms（P95 + 2s，受 3s 下限约束）
- 空消息帧集合 + 两条私有命令：3/3 命中
- 空集合独立复验：3/3 命中
- 批末正控制：3/3 命中

整帧级结果：

```text
NR required 0x7D/0x04 frame IDs = []
NR required SSIDs at this stage  = []
retained private commands        = 0x44/0x9001 + 0x55/0x0004
output frame count               = 2
output bytes                     = 204
output SHA-256                   = e11cb31aee07ac0c626d9e5d203d0589ceab734ef538849d44b5e4ad7defdd4f
```

## 独立冷启动 private-only 复验

在下一次整机冷启动后，确认 `/tmp` 已清空、`diag_mdlog` 未运行且仍在 SA n41，执行唯一一次冷状态候选：

```text
0 x 0x7D/0x04 message frame
+ 0x44/0x9001
+ 0x55/0x0004
```

结果：

```text
first complete hit latency = 486 ms
capture window             = 10,000 ms
0x9D frame count           = 156,249
QSH payload bytes          = 5,653,880
target hash count          = 3,557
parse success count        = 3,557
parse error count          = 19
valid NR neighbor cells    = 9
result                     = true
candidate config bytes     = 204
candidate config SHA-256   = e11cb31aee07ac0c626d9e5d203d0589ceab734ef538849d44b5e4ad7defdd4f
```

服务小区 PCI 509 / NR-ARFCN 504990 已被排除。有效邻区覆盖 NR-ARFCN 504990 和 524910，RSRP 约 -92.45 到 -114.91 dBm。采集完成后 `diag_mdlog` 无遗留，23 个原范围已再次显式清零。

## 独立冷启动 `0x44/0x9001-only` 复验

第三次整机冷启动后，设备驻留在 CMCC SA n28，Serving PCI 266 / NR-ARFCN 152650，RSRP 约 -64 到 -71 dBm。约 100 MB 下行负载未使设备切回 n41。

本轮唯一冷状态候选：

```text
0 x 0x7D/0x04 message frame
+ 0x44/0x9001
0 x 0x55/0x0004
```

结果：

```text
first complete hit latency = 506 ms
capture window             = 10,000 ms
0x9D frame count           = 102,432
QSH payload bytes          = 3,610,776
target hash count          = 910
parse success count        = 910
parse error count          = 17
valid NR neighbor cells    = 9
result                     = true
candidate config bytes     = 196
candidate config SHA-256   = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
```

服务小区 PCI 266 已排除。有效邻区均位于 NR-ARFCN 152650 / n28，RSRP 约 -71.22 到 -137.52 dBm。采集结束后 `diag_mdlog` 无遗留，子进程本轮正常 `exit=0`。

## 独立冷启动 `0x55/0x0004-only` 与同环境正控制

第四次整机冷启动后，设备驻留在 CMCC SA n41，Serving PCI 509 / NR-ARFCN 504990，RSRP 约 -76 dBm。本轮唯一冷状态 C 象限：

```text
0 x 0x7D/0x04 message frame
0 x 0x44/0x9001
+ 0x55/0x0004
```

结果：

```text
capture window             = 10,000 ms
captured physical bytes    = 3,412,336
0x9D frame count           = 0
target hash count          = 0
valid NR neighbor cells    = 0
result                     = false
candidate config bytes     = 8
candidate config SHA-256   = 6db11bd6286dc6cb6fb36a9e82f73d80e48a28204796c0ac32d18e34d9007240
```

为排除当时 RF 环境未产生邻区的假阴性，保留同一启动中已发送的 `0x0004` 状态，再发 `0x9001` 形成顺序 D 组合正控制。该顺序控制不冒充为独立冷状态象限：

```text
first complete hit latency = 548 ms
capture window             = 10,000 ms
0x9D frame count           = 202,544
QSH payload bytes          = 7,356,220
target hash count          = 6,763
parse success count        = 6,763
valid NR neighbor cells    = 12
result                     = true
```

这证明 `0x0004-only` 的失败不是当时 n41 RF 环境不能产生目标数据造成的。同环境加入 `0x9001` 后 QSH 和完整 NR 邻区立即恢复。

## 同窗口输出量比较

原始生成的 summary 曾把 10 秒校准窗口与 3 秒最小集合窗口直接比较，得到的 68% 降幅是时长混杂造成的假象，不应使用。Reducer 已修正为只比较同样候选窗口的批末正控制与最小集合复验。

3 秒窗口、3 次重复的本批数据：

| 配置 | 平均 `0x9D` 帧 | 平均 QSH payload 字节 |
|---|---:|---:|
| 23 消息帧 + 两条私有命令 | 76,065 | 2,767,379 |
| 0 消息帧 + 两条私有命令 | 66,095 | 2,375,511 |

本批观测降幅约为帧数 13.11%、payload 字节 14.16%。但 QSH 流量的轮间波动很大，三次样本不足以证明这个降幅具有稳定统计意义。

## 严谨结论与未决状态

1. 本批已证明：在冷状态下，单独发送原 23 个全开消息掩码不会产生 `0x9D` QSH 流。
2. 从独立整机冷启动状态开始，不发任何 `0x7D/0x04` 帧，仅发两条私有命令，NR QSH 流可在 486 ms 内稳定产生。因此不再存在“必须先发一次完整消息掩码”的状态歧义。
3. 因此，对当前 NR 整帧阶段，23 个 `0x7D/0x04` 帧的最小集合为空；无需继续做 NR 的单 SSID 或单 bit 缩减。
4. 两条私有命令组合的 204-byte 配置已证明能从冷状态独立启动 NR QSH 邻区流。
5. `0x44/0x9001` 已证明能从独立冷启动状态单独启动 NR QSH 邻区流，是当前已测私有帧中对 NR 充分的那一帧。
6. `0x55/0x0004-only` 从独立冷状态无法启动 `0x9D`；同环境补发 `0x9001` 后正控制成功。因此它不能替代 `0x9001`，且对当前 NR 目标可删除。
7. 当前 NR 帧级最小配置为单独 `0x44/0x9001`，编码长度 196 bytes，SHA-256 为 `15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58`。
8. 本结论仍是“帧级”最小化；`0x9001` 内部 43 组 ID/掩码尚未继续缩减，不应将其内部私有字段命名为已知语义。
9. 本实测只适用于该 U60 Pro 当前固件与当时 SA n41/n28 RF 环境，不推广为所有 X75 固件结论。
