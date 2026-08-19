# Validated reducer configurations

## `nr-neighbor-min.cfg`

U60 Pro 当前固件上经过独立整机冷启动验证的 NR 帧级最小候选：

```text
0 x 0x7D/0x04
1 x 0x4B subsystem=0x44 subcommand=0x9001
0 x 0x4B subsystem=0x55 subcommand=0x0004
```

```text
bytes   = 196
sha256  = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
```

该文件的 HDLC 封装和 CRC-16/X-25 已由 reducer 重新解帧校验。

“最小”仅指当前帧级实验；`0x9001` 内部 43 组私有 ID/掩码尚未缩减。实测证据见 `../VALIDATION-U60-NR-20260814.md`。

## `lte-neighbor-min.cfg`

U60 Pro 当前固件上经过独立整机冷启动验证的 LTE 帧级最小候选：

```text
0 x 0x7D/0x04
1 x 0x4B subsystem=0x44 subcommand=0x9001
0 x 0x4B subsystem=0x55 subcommand=0x0004
```

```text
bytes   = 196
sha256  = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
```

该文件的 HDLC 封装和 CRC-16/X-25 已由 reducer 重新解帧校验。

“最小”仅指当前帧级实验；`0x9001` 内部 43 组私有 ID/掩码尚未缩减。实测证据见 `../VALIDATION-U60-LTE-20260814.md`。

## `cell-neighbor-combined-min.cfg`

U60 Pro 当前固件上经过 ENDC 冷启动验证的 NR + LTE 合并帧级最小候选：

```text
0 x 0x7D/0x04
1 x 0x4B subsystem=0x44 subcommand=0x9001
0 x 0x4B subsystem=0x55 subcommand=0x0004
```

```text
bytes   = 196
sha256  = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
```

冷状态首轮及同启动两次复验均在同一窗口内同时解析出有效 NR 与 LTE 邻区。

“最小”仅指当前帧级实验；`0x9001` 内部 43 组私有 ID/掩码尚未缩减。实测证据见 `../VALIDATION-U60-COMBINED-20260814.md`。

## 正式接入状态

自 2026-08-14 起，WebSSH 正式内嵌的 `gossh/app/service/embed/qtrace.cfg` 使用 `cell-neighbor-combined-min.cfg` 的字节内容。NR/LTE 两个独立文件继续保留为验证产物；原始 25 帧完整配置保存在 `../testdata/qtrace-original-25frames.cfg`，供 reducer 回归和回退核验使用。
