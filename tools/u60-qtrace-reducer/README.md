# U60 QSH Trace Reducer

独立实验工具，用现有 `qtrace.cfg` 和短时 `diag_mdlog` QMDL 采集寻找 NR/LTE 邻小区所需的最小 `0x7D/0x04` 消息掩码集合。

它不会接入或替换 WebSSH 的正式按需 QMDL 路径，也不使用 DCI/B97F、`0x78`、QMI CellInfo 或 MPSS 内存。

当前版本完成：

- 解析、HDLC 解码并校验每一帧 CRC-16/X-25；
- 按原范围生成 23 个全零 `0x7D/0x04` 清理帧；
- 生成和编排四个控制组；
- 正控制重复校准、P95 窗口估算；
- 保持原帧顺序的整帧级 ddmin；
- NR、LTE、NR+LTE 三种严格判定；
- 每轮 JSONL、CSV 和阶段总结；
- SIGINT/SIGTERM/正常退出时再次显式清零。

当前 U60 Pro 实机验证已经完成整帧 ddmin、NR/LTE 私有命令冷启动象限，以及 ENDC 合并复验。三个目标的帧级最小配置均为单独 `0x44/0x9001`，保存在 `configs/`。因为 23 个 `0x7D/0x04` 消息帧的必要集合为空，不再存在需要继续拆分的单 SSID 或单 mask bit；尚未缩减的是 `0x9001` payload 内部的 43 组私有 ID/掩码。

项目当前成果总览见 [`../../docs/u60-neighbor-cell-results.md`](../../docs/u60-neighbor-cell-results.md)，实机明细见三份 `VALIDATION-U60-*.md`。

## 1. 离线 dry-run

在任何设备实验前必须先运行：

```sh
cd tools/u60-qtrace-reducer
make check
make dry-run
```

也可以指定路径：

```sh
go run . dry-run \
  -config testdata/qtrace-original-25frames.cfg \
  -out /tmp/u60-qtrace-dry-run
```

应确认 `manifest.json` 至少包含：

```text
frame_count                 25
message_frame_count         23
private_frame_count          2
ssid_count                 466
all_masks_ffffffff        true
round_trip_exact          true
contains_global_0x7d_0x05 false
```

生成文件：

```text
original-roundtrip.cfg
cleanup-zero.cfg
controls/positive.cfg
controls/zero-mask-private.cfg
controls/message-only.cfg
controls/all-off.cfg
controls/private-both-only.cfg
controls/private-9001-only.cfg
controls/private-0004-only.cfg
manifest.json
```

`cleanup-zero.cfg` 只包含原 23 个范围对应的零掩码帧，不包含全局 `0x7D/0x05`。

原始 25 帧配置在正式 WebSSH 切换到最小配置后固定保存在 `testdata/qtrace-original-25frames.cfg`，用于复现实验和回归 reducer；不要改用当前 1 帧的正式内嵌配置作为 ddmin 基线。

已有 QMDL 可以只读检查：

```sh
u60-qtrace-reducer inspect-qmdl -target lte sample.qmdl
```

## 2. 构建设备二进制

```sh
make build
```

输出：

```text
build/u60-qtrace-reducer
```

该文件是 Linux/aarch64、无 CGO 的独立二进制。

## 3. 设备安全前提

执行 `controls` 或 `ddmin` 前：

1. 确认设备 RF 状态适合产生目标邻区；
2. 通过 WebSSH 停止正式邻区采集；
3. 确认 `pidof diag_mdlog` 无输出；
4. 确保 `/tmp` 空间足够；
5. 先把 `qtrace.cfg` 和 reducer 放到设备临时目录。

Reducer **不会杀死已存在的 `diag_mdlog`**。发现任何实例都会中止，避免误停其他日志任务。

Reducer 启动 `diag_mdlog` 时故意不使用 `-c`：该参数的公开说明是退出时执行 mask cleanup，但其具体清理范围不能证明只限于当前 23 个范围。本工具始终通过重新计算 CRC 的显式 `0x7D/0x04` 零掩码帧清理。

异常退出可处理 SIGINT/SIGTERM；SIGKILL、断电和内核崩溃无法由用户态进程补救。设备恢复且没有 `diag_mdlog` 运行后，执行显式恢复：

```sh
/tmp/u60-qtrace-reducer cleanup \
  -config /tmp/qtrace.cfg \
  -work-dir /tmp/u60-qtrace-recovery \
  -confirm-device
```

## 4. 四个控制组

“不发送私有命令”只有在已知冷状态下才有意义。重启基带或设备，并且重启后不要运行正式邻区采集，然后执行：

```sh
/tmp/u60-qtrace-reducer controls \
  -config /tmp/qtrace.cfg \
  -work-dir /tmp/u60-qtrace-controls \
  -target combined \
  -repeats 3 \
  -window 10s \
  -confirm-device \
  -confirm-cold-private-state
```

为了不先污染冷状态，实际顺序是：

```text
all-off
message-only
positive
zero-mask-private
positive-end
```

前两个冷状态组不能被正控制包围，因为正控制本身会发送两条私有命令。第一次正控制之后，工具会在后续记录中将 `positive_control_status` 标为真；批末正控制失败会令整批结论无效。

工具目前无法关闭或证明关闭两条私有命令，控制组结束后会明确报告：

```text
private state at exit = both private commands sent; persistence unknown
```

## 5. 整帧 ddmin

分别运行三个目标，或一次运行全部目标：

```sh
/tmp/u60-qtrace-reducer ddmin \
  -config /tmp/qtrace.cfg \
  -work-dir /tmp/u60-qtrace-ddmin \
  -target all \
  -controls-report /tmp/u60-qtrace-controls/reports/controls-combined-summary.json \
  -calibration-runs 10 \
  -candidate-repeats 3 \
  -candidate-min-hits 2 \
  -maximum-window 15s \
  -window-margin 2s \
  -confirm-device
```

`ddmin` 会拒绝在没有完整冷状态控制报告时运行；报告必须包含四个控制组、批末正控制，并且正控制成功。零掩码组出现背景记录本身不会令报告无效，它是后续解释“空消息集合仍命中”的依据。

每个目标的边界为：

```text
正控制校准
→ 每轮前运行 23 范围全零配置并排空
→ 按原顺序发送候选配置和两条私有命令
→ settle
→ 仅统计 settle 结束后的 QMDL 字节
→ 固定窗口采集与严格解析
→ 每轮后再次清零并排空
→ 最小候选重复验证
→ 批末正控制
```

窗口默认按十次正控制成功样本的 P95 加 2 秒计算。无法在当前 RF 环境中稳定命中的目标会中止，不会被当成候选失败继续削减。

整帧输出：

```text
nr-neighbor-frame-min.cfg
lte-neighbor-frame-min.cfg
cell-neighbor-combined-frame-min.cfg
```

上述文件是每批 ddmin 的阶段输出。完成私有命令象限后确认的配置位于：

```text
configs/nr-neighbor-min.cfg
configs/lte-neighbor-min.cfg
configs/cell-neighbor-combined-min.cfg
```

## 6. 冷状态私有命令象限

当某个 RAT 的整帧级结果已缩减到空消息集合后，可以从新的基带/设备冷启动状态验证私有命令。每次冷启动只运行一个象限：

```sh
/tmp/u60-qtrace-reducer private-probe \
  -config /tmp/qtrace.cfg \
  -work-dir /tmp/u60-private-both-nr \
  -target nr \
  -mode both \
  -label nr-cold-both-boot1 \
  -window 10s \
  -confirm-device \
  -confirm-cold-private-state
```

`-mode` 可选：

```text
both  = 0x44/0x9001 + 0x55/0x0004
9001  = 仅 0x44/0x9001
0004  = 仅 0x55/0x0004
```

该命令只采集一次，不把同一次启动中的后续重复冒充为独立冷状态样本。A 象限（两条都不发）由冷状态 `all-off` 控制提供；B、C、D 象限必须分别重启后采集。

如果单命令冷象限失败，可以在同一启动和 RF 环境中补发另一条命令，建立顺序 D 组合正控制：

```sh
/tmp/u60-qtrace-reducer private-probe \
  -config /tmp/qtrace.cfg \
  -work-dir /tmp/u60-private-followup \
  -target nr \
  -mode 9001 \
  -prior-private-state 0x55/0x0004 \
  -label nr-post-0004-add-9001 \
  -window 10s \
  -confirm-device
```

这种记录会标记 `cold_state_confirmed=false` 和已存在的私有状态，只用于排除 RF 假阴性，不会被计为另一个独立冷象限。

## 7. 命中标准

`0x9D` 中只有目标格式哈希不算成功。至少需要：

- 帧 CRC、长度和 word count 有效；
- 目标格式按现有 WebSSH QMDL 字段布局成功解析；
- NR 报告能与同一轮、同一 QMDL 文件中的候选记录关联出合法 ARFCN；
- PCI、ARFCN 和信号值合法；
- RSRP 存在且在 `-140..-30 dBm`；
- 已确认携带 RSRQ 字段的 LTE 格式还要求 RSRQ 非零且不是明显无效值；没有已验证 RSRQ 位置的格式不猜测字段语义；
- 当前 UBUS serving PCI/ARFCN 不计入邻区；
- `combined` 要求同一轮窗口内 NR 和 LTE 都完整命中。

Reducer 在 settle 结束时记录每个 QMDL 文件的字节偏移；分析时忽略此前字节。如果偏移位于一个尚未写完的 HDLC 帧中，会丢弃该残帧直到下一个 `0x7E`，避免上一阶段数据污染本轮。

## 8. 报告

`reports/runs.jsonl` 和 `reports/runs.csv` 每轮包含：

```text
run_id, target, candidate_frame_ids, candidate_ssids,
private_command_state, positive_control_status,
capture_duration_ms, first_hit_latency_ms,
qsh_frame_count, qsh_total_bytes, target_hash_count,
parse_success_count, parse_error_count,
nr_cell_count, lte_cell_count, result, failure_reason
```

`qsh_total_bytes` 是本轮窗口内通过 CRC 的 `0x9D` 解码 payload 字节数，不包含 HDLC 转义、CRC 和分隔符；`captured_bytes` 是窗口内 QMDL 文件新增的物理字节数。

阶段总结会给出原始/最小消息帧数、平均 QSH 帧数和字节数，以及下降比例。输出量只比较同样候选窗口的“批末正控制”与“最小集合复验”；长窗口校准数据不参与原始字节数降幅计算。若窗口时长不一致，降幅字段将保持为零并将 `windows_comparable` 设为 `false`。若没有使用 `-keep-captures`，QMDL 在提取结构化指标后删除，防止 `/tmp` 被多轮测试占满。
