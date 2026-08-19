# U60 NR Cross-Band Probe

这是 U60 Pro / Qualcomm X75 的独立 NR 主动跨频实验工具。它只研究 NR，不接入 WebSSH 前端，也不替换正式按需 QMDL 邻区方案。

工具把一次实验限制为一个事务：保存网络状态、短时开启 QSH ID 96 bit 15、执行 A～F 中的一条控制路径、只等待首个有效目标结果、停止日志、恢复正式 QSH 掩码和网络设置。默认 QMDL 环形上限为 `2 MiB × 2`，并持续检查可用内存和 `/tmp` 空间。

## 当前结论

U60 当前固件的完整 QCRIL 路径已经得到静态证据：

- `startNetworkScan` 的 `ONE_SHOT` 分支会丢弃 Android `RadioAccessSpecifier` 中的 Band 和 channel，只把当前允许的 RAT 写入 NAS `0x0085`。
- 非 `ONE_SHOT` 的 advanced 分支才会把 NGRAN Band 和最多 10 个 NR-ARFCN 写进 NAS `0x0085`，但没有发现额外的前置 QMI 动作。
- `setSystemSelectionChannels` 最终发送 NAS `0x0033`，在本固件只映射 Band 掩码，不映射 channel/NR-ARFCN。

因此，本工具不会把 `ONE_SHOT + n41 + 504990` 描述成真正的单频点扫描。当前最值得实测的是：

```text
临时只允许 n41
→ 触发新的 NR acquisition/no-service acquisition
→ 捕获第一条目标 0xd8f582a8
→ 立即恢复
```

详细证据见 [VALIDATION-U60.md](VALIDATION-U60.md)，A～F 的精确定义见 [EXPERIMENTS.md](EXPERIMENTS.md)。

## 离线检查和构建

```sh
cd tools/u60-nr-crossband-probe
make check
make dry-run
```

`dry-run` 会：

- 强制验证正式 `qtrace.cfg` SHA-256 为 `15b669...c646c58`；
- HDLC 解帧并验证 CRC；
- 只把 QSH ID 96 从 `0x0007` 改为 `0x8007`；
- 重新计算 CRC、HDLC 转义并再次解帧校验；
- 生成手工 advanced `0x0085` 和 QCRIL ONE_SHOT 下沉模型；
- 输出完整 A～F 计划，不接触设备。

生成的主动配置预期 SHA-256：

```text
b1db1e7a4c82bd94513d080a4410b596634b5d546a348e8921c4104485539d54
```

Linux/aarch64 二进制位于：

```text
build/u60-nr-crossband-probe
```

## 设备运行前提

1. 明确停止 WebSSH 的正式邻区采集，并确认 `pidof diag_mdlog` 无输出。
2. 将二进制和正式最小 `qtrace.cfg` 放到设备临时目录。
3. 确认 `/tmp/u60-nr-crossband-probe/state.json` 不存在；若存在，先执行恢复。
4. C～F 会改变网络设置，必须额外传入 `-confirm-network-change`。
5. 工具绝不会自动后台运行或周期扫描。

实验 A（现有手工 `0x0085` 失败基线）：

```sh
/tmp/u60-nr-crossband-probe run \
  -experiment A \
  -config /tmp/qtrace.cfg \
  -target-band 41 \
  -target-arfcn 504990 \
  -window 8s \
  -confirm-active-measurement
```

实验 C（只限目标 Band，不显式重启 acquisition）：

```sh
/tmp/u60-nr-crossband-probe run \
  -experiment C \
  -config /tmp/qtrace.cfg \
  -target-band 41 \
  -target-arfcn 504990 \
  -confirm-active-measurement \
  -confirm-network-change
```

实验 E（短暂移除 NR 服务后，只允许目标 Band 并启动 NR acquisition）：

```sh
/tmp/u60-nr-crossband-probe run \
  -experiment E \
  -config /tmp/qtrace.cfg \
  -target-band 41 \
  -target-arfcn 504990 \
  -acquisition-pause 700ms \
  -window 8s \
  -confirm-active-measurement \
  -confirm-network-change
```

实验 F 需要先从一次成功驻留 n41 的正控制中取得真实 PCI：

```sh
/tmp/u60-nr-crossband-probe run \
  -experiment F \
  -config /tmp/qtrace.cfg \
  -target-band 41 \
  -target-arfcn 504990 \
  -target-pci 123 \
  -confirm-active-measurement \
  -confirm-network-change
```

## QCRIL 实验 B

当前二进制没有硬编码尚未实机验证的 `/dev/socket/qcrild/rild` framing。实验 B 要求提供一个独立的 QCRIL socket/HAL 驱动命令：

```sh
/tmp/u60-nr-crossband-probe run \
  -experiment B \
  -config /tmp/qtrace.cfg \
  -target-band 41 \
  -target-arfcn 504990 \
  -qcril-start-command '/tmp/u60-qcril-scan-driver start' \
  -qcril-stop-command '/tmp/u60-qcril-scan-driver stop' \
  -confirm-active-measurement
```

驱动命令会收到环境变量：

```text
U60_TARGET_RAT=NGRAN
U60_TARGET_BAND=41
U60_TARGET_ARFCN=504990
U60_SCAN_TYPE=ONE_SHOT
```

这样可以先把状态机、QMDL、恢复和报告固定下来，同时避免用猜测的 socket framing 直接写 qcrilNrd。

如果 QCRIL 或厂商 nwinfo 进程已通过 CCI hook 输出 append-only 日志，可以在任意实验上增加：

```sh
-qmi-trace-log /tmp/u60-qmi-trace.log
```

工具会在实验动作前记录文件偏移，只把本轮新增的 QMI request/response/indication 并入 JSON/JSONL。它不会主动注入或重启 qcrilNrd；若本轮没有新 QMI 事件，报告会设置 `qmi_trace_complete=false` 并写明原因，不会伪称已获得完整 wire 证据。

## 异常恢复

启用主动 Trace 前，原始 `net_select`、SA/NSA Band 配置、NR 小区锁和正式 `qtrace.cfg` 字节会写入：

```text
/tmp/u60-nr-crossband-probe/state.json
```

正常完成、SIGINT/SIGTERM、超时、解析失败和低内存熔断都会尝试恢复。进程被 SIGKILL、设备断电或内核崩溃后，执行：

```sh
/tmp/u60-nr-crossband-probe recover \
  -state /tmp/u60-nr-crossband-probe/state.json \
  -confirm-network-change
```

状态文件内嵌了已校验的 196 字节正式 QSH 配置，不依赖运行时传入的配置路径继续存在。状态文件只有在主动 Trace 和网络设置全部恢复成功后才删除。恢复顺序是：恢复 QSH ID 96=`0x0007`、解除实验小区锁、恢复 SA Band、恢复 NSA Band、恢复网络模式、必要时恢复原小区锁。

## 样本与 QMI 差分

只读解析已有 QMDL：

```sh
/tmp/u60-nr-crossband-probe inspect-qmdl \
  -target-band 41 \
  -target-arfcn 504990 \
  /tmp/capture/*.qmdl
```

比较手工 A 和完整 QCRIL B 的 CCI hook 日志：

```sh
/tmp/u60-nr-crossband-probe diff-qmi \
  -manual /tmp/a-qmi-trace.log \
  -qcril /tmp/b-qmi-trace.log \
  -out /tmp/a-vs-b.json
```

判定时同时查看：

- CM 是否收到目标 ARFCN：`0xf9f48e26`；
- NR2NR search 实际启动频点：`0xfa16e120` / `0xfa16e20f` / `0xfa16e581`；
- ML1 主动结果：`0xd8f582a8`；
- `valid=1`、合法 PCI、目标 ARFCN 和合理 RSRP。

## 资源边界

- 默认 QMDL 最大占用约 4 MiB；命令行硬限制每个文件和文件数都不超过 4。
- 默认 `MemAvailable < 32 MiB` 或工作目录可用空间 `< 16 MiB` 立即中止。
- 默认提取报告后删除原始 QMDL；只有显式传入 `-keep-captures` 才保留。
- 活动 Trace 停止后，工具会短时重新应用正式 `qtrace.cfg`，把 ID 96 恢复为 `0x0007`。
- 工具拒绝与任何已运行的 `diag_mdlog` 并发，不会杀死外部日志进程。
