# U60 Pro 邻近小区项目成果总览

更新时间：2026-08-14  
适用设备：U60 Pro / Qualcomm X75 / 当前实测固件

## 一句话结论

项目已经实现可用的 WebSSH 邻区功能：用户点击刷新时短时启动 `diag_mdlog`，Go 后端直接解析 QMDL 中的 `0x9D` QSH Trace，返回 NR/LTE 服务小区和带信号强度的邻区，完成后立即停止采集；前端可以把选中的邻区填入锁小区输入框。

进一步的实机最小化证明，NR、LTE、NR+LTE 三个目标在帧级都只需要一条私有命令：

```text
DIAG command   = 0x4B
subsystem      = 0x44
subcommand     = 0x9001
config bytes   = 196
SHA-256        = 15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58
```

正式 WebSSH 已接入 196-byte NR + LTE 合并最小配置。原始 25 帧完整配置保存在 `tools/u60-qtrace-reducer/testdata/qtrace-original-25frames.cfg`，继续作为 reducer 基线和回退证据；按需 QMDL 的采集方式没有改变。

## 当前完整链路

```text
用户打开网络设置或点击刷新
        ↓
GET /api/neighbor/cells
        ↓
读取 UBUS 服务小区作为身份和兜底数据
        ↓
短时启动 diag_mdlog + qtrace.cfg
        ↓
采集 QMDL（外层主要为 0x9D QSH Trace）
        ↓
Go 原生解析器按当前固件格式哈希提取 NR/LTE 邻区
        ↓
排除当前 Serving，聚合样本并计算 RSRP 中位数
        ↓
返回前端，立即停止本次 diag_mdlog
```

若 QMDL 暂时不可用，后端会回退到 `nwinfo_get_netinfo` 可见的数据，并在响应中携带 warning。不会为了邻区功能常驻后台持续采集。

## 已完成的产品功能

### Go 后端

- 邻区 API：`GET /api/neighbor/cells`。
- 状态和停止接口已保留，可识别自己管理的 `diag_mdlog`，不会误杀其他采集任务。
- QMDL 解析器已由外部 `u60nbrqt_parser` 替换为 Go 原生实现，当前 engine/version 为 `go-native / 1.2.1`。
- 支持 NR、LTE 和 ENDC/NSA 混合场景。
- 支持当前固件以及旧样本中已验证的多组 QShrink 格式哈希。
- NR 会关联候选身份记录与测量记录，恢复 PCI、ARFCN、Band 和 RSRP。
- LTE 支持整数、十分之一 dBm 和 SDX75 分数 RSRP 布局。
- 多条测量按 RAT + PCI + ARFCN 聚合，使用合法 RSRP 样本中位数。
- 服务小区由 UBUS/QMDL 合并，并从邻区结果中排除。
- 内嵌 `qtrace.cfg` 带固定 SHA-256 校验，设备临时目录中的旧外部解析器会被移除。
- 当前请求结束时通过 `defer` 停止本次采集，恢复为按需模式。

主要文件：

- `gossh/app/service/neighbor_cell.go`
- `gossh/app/service/neighbor_qmdl.go`
- `gossh/app/service/neighbor_cell_test.go`
- `gossh/app/service/neighbor_qmdl_test.go`
- `gossh/app/service/embed/qtrace.cfg`

### Web 前端

- 网络设置页展示当前服务小区和检测到的邻区。
- 显示 RAT、PCI、Band、ARFCN、RSRP、样本数等字段。
- 点击整行或“填入”按钮，可将 NR/LTE 邻区写入对应锁小区输入框。
- 缺少 ARFCN/Band 时只填入可用字段并明确提示，不伪造数据。
- 刷新期间禁用按钮并显示“采集中”，另有 1 秒点击冷却，避免重复请求。
- 加载遮罩已改为与深色界面一致的半透明蓝黑背景。
- 打开网络设置时自动执行一次按需刷新。

主要文件：`webssh/src/views/Main.vue`。

## QMDL/QSH 数据结论

原始完整配置与当前正式内嵌配置对比：

| 项目 | 原始配置 | 帧级最小配置 |
|---|---:|---:|
| 总帧数 | 25 | 1 |
| `0x7D/0x04` 消息掩码帧 | 23 | 0 |
| 覆盖 SSID | 466 | 0 |
| 私有 `0x4B` 命令 | 2 | 1 |
| 配置字节数 | 2,345 | 196 |

当前正式 `gossh/app/service/embed/qtrace.cfg` 与 `tools/u60-qtrace-reducer/configs/cell-neighbor-combined-min.cfg` 字节完全一致；单元测试同时校验 196-byte 长度和固定 SHA-256。原始 2,345-byte 配置保存在 reducer 的 `testdata/` 下，不再由正式请求加载。

配置字节数减少约 91.64%，帧数减少 96%。但这不等于运行时 QSH 流量按同样比例下降：同窗口实测中 QSH 输出有明显 RF/调度波动，删除 `0x7D` 帧没有证明能稳定降低流量。当前收益首先是确定了真正的数据启动命令和配置依赖关系。

正确术语和数据模型：

```text
0x44/0x9001 私有 subsystem 命令
        ↓
基带产生 measurement trace
        ↓
0x9D QSH Trace streaming payload
        ↓
QMDL 文件
        ↓
Go 解析器按格式哈希解码
```

当前数据不是标准 `0x10 DIAG_LOG_F / 0xB97F`，不再把它描述为 B97F 日志。

## 最小化实机结果

### 共同控制结论

- 冷状态 `all-off`：无 QSH、无目标邻区。
- 冷状态仅发送 23 个全开 `0x7D/0x04` 帧：无 QSH、无目标邻区。
- 原始完整配置：NR/LTE 正控制稳定命中。
- 23 个消息范围全零、保留两条私有命令：NR/LTE 仍稳定命中。
- 因此 23 个消息帧的 ddmin 结果为空，无需继续做单 SSID/单 bit 缩减。

### 私有命令结论

| 冷状态候选 | NR | LTE | 结论 |
|---|---|---|---|
| 无私有命令 | 不命中 | 不命中 | 无背景 QSH |
| `0x44/0x9001-only` | 命中 | 命中 | 对两个 RAT 均充分 |
| `0x55/0x0004-only` | 不命中 | 不命中 | 不能启动 QSH |
| `0x0004` 后追加 `0x9001` | 立即恢复 | 立即恢复 | 排除 RF 假阴性 |

### NR + LTE 合并验证

ENDC 冷状态下只发送 `0x9001`：

- 首次完整合并命中：487 ms。
- 同一窗口得到 3 个有效 NR 邻区和 3 个有效 LTE 邻区。
- 同启动两次复验分别得到 NR 5 + LTE 3、NR 3 + LTE 2。
- 合并目标达到 3/3。

三个最终帧级配置内容完全相同，但保留独立文件名便于调用方表达目标和后续演进：

- `tools/u60-qtrace-reducer/configs/nr-neighbor-min.cfg`
- `tools/u60-qtrace-reducer/configs/lte-neighbor-min.cfg`
- `tools/u60-qtrace-reducer/configs/cell-neighbor-combined-min.cfg`

详细证据：

- `tools/u60-qtrace-reducer/VALIDATION-U60-NR-20260814.md`
- `tools/u60-qtrace-reducer/VALIDATION-U60-LTE-20260814.md`
- `tools/u60-qtrace-reducer/VALIDATION-U60-COMBINED-20260814.md`

## Reducer 实验工具

`tools/u60-qtrace-reducer` 是独立实验工具，不接入正式请求链路。已实现：

- qtrace.cfg HDLC 解帧和 CRC-16/X-25 校验。
- 原配置无损 round-trip。
- 只针对原 23 个范围生成全零清理帧，不使用全局 `0x7D/0x05`。
- 冷状态控制组、正控制校准和 P95 窗口。
- 保持原始顺序的整帧 ddmin。
- NR、LTE、combined 严格命中判定。
- settle 后按 QMDL 字节偏移切分采集窗口，降低跨轮污染。
- JSONL、CSV、summary 结构化报告。
- 正常退出和 SIGINT/SIGTERM 时再次显式清零。
- 私有命令独立冷象限和同环境顺序正控制。

原始实机结构化报告保存在 reducer 的 `build/validation/` 下；该目录被 `.gitignore` 忽略，避免把大量临时证据混入源码。摘要和结论保存在版本化 Markdown 报告中。

## 已完成但不再继续投入的路径

### DCI / B97F / `0x78`

实机已经证明 DCI 基础链路本身正常：client、stream callback、命令响应、B97F mask 开关均工作。但自然采集与多次 `0x78 7F B9` 抖动请求都没有收到 B97F，`0x78` 也没有有效响应。

严谨结论仅限当前 U60 固件：没有通过 DCI 标准 log code `0xB97F` 发布这份邻区数据，不能用该路径替换 QMDL。失败路线的独立探针不纳入正式项目提交，必要实测结论保留在本文中。

### QMI CellInfo / 主动网络扫描

当前固件的 NAS `0x0043` 在 LTE 下可返回同频邻区，但 NR 下只返回服务小区。`0x0148` 只确认当前 RAT/PCI/Band 等状态，未收到邻区 ARFCN indication。`0x0085` 主动扫描能返回运营商网络，未返回需要的 NR 邻区 PCI/RSRP；实验 scan type 也出现当前固件拒绝。

因此 QMI 路线保留为研究结论，不作为正式邻区数据源，也不再继续逆 Qualcomm 私有 CellInfo 通道。

## 已知限制

1. 当前“最小”是帧级最小；`0x9001` payload 内仍有 43 组未知 ID/掩码，尚未缩减或命名语义。
2. LTE 邻区 measurement scheduling 有长尾：独立冷启动曾在 4,513 ms 才首次命中。短窗口偶尔无结果时应允许再次刷新，不能立即判定配置失效。
3. 异频候选有时只有 PCI/ARFCN、没有 RSRP。这通常表示基带输出了候选身份但尚未调度完整测量；解析器不能安全地补造信号值。
4. 配置变小不代表 QSH 运行时带宽同比下降；正式方案仍坚持短时按需采集，不恢复后台常驻。
5. 格式哈希可能随基带固件重编译变化。升级固件后需要用新样本验证或补充哈希布局。
6. 实机结论只适用于当前 U60 Pro 固件，不推广为所有 X75 平台的通用行为。
7. 普通 DIAG callback collector 仍暂缓；当前没有假定它能与 `diag_mdlog` 并行持有 MODEM logging session。

## 当前验证状态

- Web 前端：TypeScript 检查和 Vite 生产构建通过。
- Go 主程序：Linux/aarch64 静态构建通过。
- Go 邻区 service 测试：在 U60 Pro aarch64 设备上 11 个顶层测试全部通过（另含 7 个 LTE 布局子测试），运行前后均无 `diag_mdlog`。
- QSH reducer：`go test ./...`、`go vet ./...` 和 Linux/aarch64 构建通过。

macOS 不能直接执行 Go 主工程的全量测试，因为仓库中的 PTY、syscall、SFTP 实现是 Linux 专用；因此采用 Linux/aarch64 交叉构建，并把纯 service 测试二进制放到 U60 实机执行。

## 2026-08-14 正式上线验证

- 发布目标：`/data/kano_plugins/kano_web_ssh/webssh`。
- 发布版本标识：`2.4.0-u60-qsh-min`。
- 发布二进制 SHA-256：`007a4d2d3e688f0f428c554dcd5a77b04e233824e114ec3fb50d5ab8df0ee594`。
- 原现网包已保存为 `/data/kano_plugins/kano_web_ssh/webssh.pre-qsh-min-20260814-1648.bak`，SHA-256 为 `35968d8b26a2279d41ca8f887a5ff4be4ee35766da219b97730508bbfc3e32d7`；原有 `webssh.bak` 未被覆盖。
- 脱离升级脚本完成进程切换和本机 HTTP 健康检查，没有触发自动回滚。
- 正式接口在 ENDC 现网环境返回 `原生日志` 数据源，服务小区同时包含 LTE B3 与 NR N41；连续按需采集得到带有效 RSRP 的 LTE 邻区。
- 页面自动采集、刷新按钮禁用态、深色加载遮罩、邻区填入锁小区输入框均通过浏览器回归；填入 PCI 41 / EARFCN 1300 后未执行“应用”，未改变设备锁定状态。
- 正式请求落地的 `/tmp/u60nbrqt_resident/qtrace.cfg` 为 196 bytes，SHA-256 与合并最小配置一致；请求结束状态为 `stopped: request completed`，无 `diag_mdlog` 遗留。
- 浏览器回归最后一分钟没有新增控制台错误。

## 合并状态与下一步

当前成果已经合入工作区，但尚未提交到 Git：

- `gossh/app/service/neighbor_cell.go` 有按请求结束采集的待提交修改。
- `gossh/app/service/embed/qtrace.cfg` 已替换为验证通过的 196-byte 合并最小配置，并增加固定哈希测试。
- `webssh/src/views/Main.vue` 有防重复点击和加载样式的待提交修改。
- `tools/u60-qtrace-reducer/` 下的 reducer、配置和验证报告目前均为未跟踪文件。

建议下一步按优先级选择：

1. 审阅并提交已完成上线回归的现有成果，形成 Git 可回退基线。
2. 若继续研究最小化，下一层是对 `0x9001` 内部 43 组 ID/掩码做具备冷状态边界和异常清理的 reducer，不能直接猜字段语义。
3. 普通 DIAG callback 内存采集器继续放在最小配置稳定之后，不与当前任务混做。
