# U60 固件静态验证与当前进展

日期：2026-08-16

本文件只记录 U60 当前固件，不推广到所有 X75 平台。

## 分析对象

```text
/usr/bin/qcrilNrd
SHA-256 937eede7fe542634eceef59aca6a3df13e2842c02603098823358dcabd433324

/usr/lib/libqcrilNr.so
SHA-256 a6a83345aeddd400d8c9f1e6e9a9821bb889024174d743b19b14b418b38accd5

/usr/lib/libqcrilNrSocketModule.so
SHA-256 b65bc05f61877c63be98cb9444cf14b4c3572f3f6a110cadc4de9acadc3066e0
```

分析辅助脚本位于 `reverse/U60QcrilPathDecompile.java`。地址以下方 ELF 符号虚拟地址为准。

## 1. QCRIL startNetworkScan

### 入口

```text
NasModule::handleStartNetworkScan                         0x20ffd4
qcril_qmi_nas_start_advanced_scan                        0x2a0b44
qcril_qmi_nas_perform_incremental_network_scan           0x2a2190
```

`handleStartNetworkScan` 检查请求对象偏移 `+0xde8`：

```text
值 == 1  → perform_incremental_network_scan
其他值   → start_advanced_scan
```

此处对比的是 socket 模块通过 `Marshal::read<RIL_NetworkScanRequest>` 生成的 legacy RIL 值：`RIL_ONE_SHOT=1`、`RIL_PERIODIC=2`。Android 公共/AIDL `NetworkScanRequest` 的 `ONE_SHOT=0`、`PERIODIC=1` 会在进入该 legacy 结构前转换；不能直接用公共枚举反推这个分支。

### ONE_SHOT / perform_incremental 分支

该分支：

1. 将 576 字节 `nas_perform_incremental_network_scan_req_msg_v01` 全部清零；
2. 调用 `qcril_qmi_nas_retrieve_scan_network_type(..., 1)`；
3. 只根据当前 mode preference 生成 RAT bitmap；
4. 同步发送 NAS `0x0085`，request size `0x240`，response size 8，timeout 30000 ms。

没有读取 `RIL_NetworkScanRequest` 的 RadioAccessSpecifier Band/channel。因此 NR-only 当前模式下，预期编码只有：

```text
TLV 0x10 = 0x10  # NR5G RAT bitmap
```

工具生成的完整 QMI 包模型：

```text
0001008500040010010010
```

这解释了为什么“完整 QCRIL ONE_SHOT + n41 + 504990”不会天然优于手工 `0x0085`：目标字段在到达 NAS 编码前已经丢失。

### advanced 分支

advanced 分支会读取最多 8 个 RadioAccessSpecifier：

- accessNetwork=4 时识别为 NGRAN；
- NGRAN Band 映射到 NR5G band preference；
- channel 最多累计 10 个；
- channel 被复制到 request struct 偏移 `0x1d0`；
- 最终仍同步发送 NAS `0x0085`，request size `0x240`。

没有在该函数中看到 `0x0085` 之前的额外 NAS 请求、system-selection channel 修改或 acquisition-state 切换。

这与既有实测的 TLV `0x1d` 结构一致：

```text
1 byte count + N × little-endian uint32 NR-ARFCN
```

## 2. setSystemSelectionChannels

相关符号：

```text
qcril_qmi_nas_fill_band_info                              0x295680
qcril_qmi_nas_request_set_system_selection_channels       0x2b03b0
NasModule::handleSetSystemSelectionChannels               0x223210
```

请求处理函数会：

1. 清零 792 字节 `nas_set_system_selection_preference_req_msg_v01`；
2. 遍历 `RIL_SysSelChannels` specifier；
3. NGRAN 时仅调用 `qcril_qmi_nas_map_atel_ngran_bands`；
4. 将 Band 写入 NR5G band preference mask；
5. 异步发送 NAS `0x0033`，request size `0x318`，response size 8。

该函数没有读取每个 specifier 的 channel 数组。因此在这版固件上：

```text
setSystemSelectionChannels(NGRAN, n41, 504990)
≈ 设置 NR n41 Band preference
≠ 锁定或白名单化 504990
```

这使方向二仍然有价值，但能力边界必须改写为“临时目标 Band + 新 acquisition”，不能声称是“目标 channel whitelist”。

## 3. 既有动态证据复核

探针已在本地对既有 QMDL 做只读回归：

- n28→n41 失败样本中，`0xf9f48e26` 显示 CM 收到 504990 和 524910；
- 同一轮 `0xfa16e581` 显示 NR2NR search 实际启动在 152650；
- 其它 type-2 样本中，`0xd8f582a8` 和 `0xd8fa712c` 均只报告 152650。

因此已有证据支持：

```text
目标 ARFCN 编码正确且到达 CM
≠
ML1 为每个目标 ARFCN 创建真实搜索任务
```

## 4. 当前交付状态

已完成：

- 正式 qtrace SHA、HDLC、CRC 和 ID96 bit15 临时配置生成；
- 手工 advanced `0x0085` 与 QCRIL ONE_SHOT 下沉模型；
- QSH `0xd8f582a8`、CM ARFCN、NR2NR search-start、found-cell 解析；
- A～F 单轮框架；
- 网络设置事务 journal 和恢复命令；
- journal 内嵌正式 QSH 配置，可在 `SIGKILL` 后恢复 ID96 bit15；
- SIGINT/SIGTERM、超时、解析失败、低内存时的清理路径；
- QMDL 大小限制、内存/空间熔断；
- JSONL、CSV、单轮 JSON；
- QMI CCI hook 日志 `0x0085` TLV 差分；
- Linux/aarch64 静态构建和离线测试。

尚未宣称完成：

- 没有在本轮离线开发阶段执行新的 C～F 设备网络中断实验；
- qcrild socket framing 尚未用 U60 实机客户端验证，因此 B 使用外部驱动接口；
- 尚未证明 D 或 E 能在 1～2 秒内从 n28 搜到 n41；
- 尚未获得“驻网不中断、严格单 ARFCN”的主机接口。

下一步设备顺序应为：

```text
F 已知 n41 正控制
→ A 失败基线复现
→ C Band-only
→ D Band + reselect
→ E NR-off / target Band / NR-on
→ 仅在仍有必要时，用真实 QCRIL driver 完成 B 的 wire hook
```

每批开始和结束都运行 F；若 F 在当前 RF 环境中失败，本批 C/D/E 阴性结果无效。
