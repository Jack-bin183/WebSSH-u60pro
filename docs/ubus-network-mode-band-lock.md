# U60Pro 网络制式 / 4G·5G 锁频 ubus 接口文档

本文整理 WebSSH-u60pro 中"网络制式切换"与"4G/5G 锁频"所用到的 ubus 接口，
包含**查询**与**设定**两类，列出入参与出参。

所有制式 / 锁频相关接口都挂在同一个 ubus object 下：

```
zte_nwinfo_api
```

底层执行形式（后端 `gossh/app/utils/ubus.go`）：

```sh
ubus call <object> <method> '<JSON 入参>'
```

---

## 一、调用链路

项目里有两条不同的调用路径，**查询走前端批量代理，设定走后端封装接口**。

### 1. 查询：前端 JSON-RPC 批量 → `/api/ubus`

前端把多条 ubus 调用打包成 JSON-RPC 数组，POST 到 `/api/ubus`，后端
（`ZteUbusBatchHandler` → `CallUbusBatchAsync`）并发执行后按 `id` 排序返回。

JSON-RPC 单条结构（`webssh/src/views/Main.vue`）：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "call",
  "params": [
    "00000000000000000000000000000000",  // params[0] SESSION_ID，后端忽略
    "zte_nwinfo_api",                     // params[1] object
    "nwinfo_get_netinfo",                 // params[2] method
    {}                                    // params[3] 入参
  ]
}
```

> 后端只取 `params[1]/[2]/[3]`，`params[0]`（SESSION_ID）仅为兼容官方格式，实际不参与鉴权。
> 返回里 `Result = [code, data]`，`code==0` 为成功，`data` 即 ubus 原始 JSON。

### 2. 设定：前端 → `/api/network/*` → 后端封装 → ubus

设定类不直接暴露 ubus，由后端 `gossh/app/service/network_settings.go` 做参数校验后再
`ubus call`。路由（`gossh/main.go`）：

| 前端接口 | 后端 Handler | ubus object.method |
|---|---|---|
| `POST /api/network/mode` | `NetworkModeSetHandler` | `zte_nwinfo_api nwinfo_set_netselect` |
| `POST /api/network/band/lte` | `NetworkLTEBandLockHandler` | `zte_nwinfo_api nwinfo_set_gwl_bandlock` |
| `POST /api/network/band/nr` | `NetworkNRBandLockHandler` | `zte_nwinfo_api nwinfo_set_nrbandlock` |

后端统一返回：`{ "code": 0, "msg": "ok", "data": <ubus 返回> }`，失败 `code != 0`。

---

## 二、查询接口

### `nwinfo_get_netinfo` —— 网络信息总查询

当前制式、4G/5G 当前锁频状态都从这一条查询的返回字段里读取（前端 `id=1`）。

**ubus 调用**

```sh
ubus call zte_nwinfo_api nwinfo_get_netinfo '{}'
```

**入参**

| 字段 | 类型 | 说明 |
|---|---|---|
| （无） | `{}` | 空对象 |

**出参（与制式/锁频相关的字段）**

| 字段 | 类型 | 含义 | 前端用途 |
|---|---|---|---|
| `net_select` | string | 当前网络制式（取值见下方制式映射表） | 制式下拉框回填 |
| `network_type` | string | 当前组网类型，如 `SA` / `NSA` | 决定读 SA 还是 NSA 的锁频字段 |
| `lte_band` | string | 当前 4G 锁频列表，逗号分隔（如 `"1,3,41"`） | 4G 锁频勾选回填 |
| `nr5g_sa_band_lock` | string | SA 模式下 5G 锁频列表，逗号分隔 | 5G 锁频勾选回填（SA 优先） |
| `nr5g_nsa_band_lock` | string | NSA 模式下 5G 锁频列表，逗号分隔 | 5G 锁频勾选回填（NSA / 兜底） |
| `nr5g_action_band` | string | 5G 当前驻留主频段（如 `n78`） | "锁当前 5G 频段"取值 |
| `lteca` | string | 4G 载波聚合信息（分号/逗号分隔，含频段、频点、带宽） | 解析当前 4G 频段做"锁当前" |
| `nrca` | string | 5G 载波聚合信息（分号/逗号分隔） | 解析当前 5G 频段做"锁当前" |

> 前端读取逻辑：`network_type == "NSA"` 时取 `nr5g_nsa_band_lock`，否则取
> `nr5g_sa_band_lock || nr5g_nsa_band_lock`（见 `currentNRBandLockValue()`）。

---

## 三、设定接口

### 1. `nwinfo_set_netselect` —— 设置网络制式

**ubus 调用**

```sh
ubus call zte_nwinfo_api nwinfo_set_netselect '{"net_select":"WL_AND_5G"}'
```

**入参**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `net_select` | string | 是 | 制式标识，仅允许下表取值（后端 `validNetSelect` 白名单校验） |

**`net_select` 制式映射表**

| net_select | 含义（UI 标签） |
|---|---|
| `WL_AND_5G` | 5G/4G/3G（自动） |
| `Only_5G` | 5G SA |
| `LTE_AND_5G` | 5G NSA |
| `WCDMA_AND_LTE` | 4G/3G |
| `Only_LTE` | 4G LTE |
| `Only_WCDMA` | 3G |

**HTTP 层（前端实际走的路径）**

```http
POST /api/network/mode
Content-Type: application/json

{ "net_select": "WL_AND_5G" }
```

成功：

```json
{ "code": 0, "msg": "ok", "data": { /* ubus 原始返回 */ } }
```

**HTTP 层入参**

| 字段 | 类型 | 必填 | 校验 |
|---|---|---|---|
| `net_select` | string | 是 | `binding:"required"` + `validNetSelect` 白名单 |

**ubus 层入参**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `net_select` | string | 是 | 制式标识，仅允许下表取值 |

**`net_select` 制式映射表**（前端 `netSelectOptions` 与后端 `validNetSelect` 一一对应）

| net_select | UI 标签 | 含义 |
|---|---|---|
| `WL_AND_5G` | 5G/4G/3G | 全制式自动（默认/推荐） |
| `Only_5G` | 5G SA | 仅 5G 独立组网 |
| `LTE_AND_5G` | 5G NSA | 5G 非独立组网（依附 4G 锚点） |
| `WCDMA_AND_LTE` | 4G/3G | 4G + 3G，不驻留 5G |
| `Only_LTE` | 4G LTE | 仅 4G |
| `Only_WCDMA` | 3G | 仅 3G |

**出参**：ubus 原始返回（一般为空对象或状态对象），后端包装为
`{"code":0,"msg":"ok","data":<ubus 返回>}`。

**错误码**

| code | 触发条件 | msg |
|---|---|---|
| 1 | JSON 解析失败 / `net_select` 不在白名单 | `网络模式不合法` |
| 1 | ubus 调用失败（`writeUbusResult` 透传） | ubus 错误文本 |

**行为细节**

- UI 上既有"应用"按钮（`applyNetworkMode`），首页快捷下拉变更也会触发
  `netSelectChange` → 立即 `applyNetworkMode`，**无需二次确认**。
- 切换成功后前端 `setTimeout(fetchAllData, 3000)`，**延迟约 3 秒**再刷新，因为
  模组重新选网/驻留需要时间，立即查会读到旧状态。
- 制式切换会触发模组重新搜网，期间数据连接可能短暂中断。

---

### 2. `nwinfo_set_gwl_bandlock` —— 4G 锁频

4G 锁频用**位掩码**表达频段集合：band N 对应 bit (N-1)，掩码 = Σ 2^(band-1)，
以十进制字符串传入（后端 `lteBandMask()`，用 `math/big.Int` 计算，**支持任意大整数**）。

**HTTP 层（前端实际走的路径）**

```http
POST /api/network/band/lte
Content-Type: application/json

{ "bands": [1, 3] }
```

前端只传频段数组，**掩码由后端计算**，再拼成 ubus 入参。

**ubus 调用**

```sh
# 例：锁 B1 + B3  ->  2^0 + 2^2 = 1 + 4 = 5
ubus call zte_nwinfo_api nwinfo_set_gwl_bandlock \
  '{"is_gw_band":"0","gw_band_mask":"0","is_lte_band":"1","lte_band_mask":"5"}'
```

**ubus 入参**

| 字段 | 类型 | 值 | 说明 |
|---|---|---|---|
| `is_gw_band` | string | 固定 `"0"` | 是否设置 2G/3G(GSM/WCDMA) 频段，本项目恒为 0（不动） |
| `gw_band_mask` | string | 固定 `"0"` | 2G/3G 频段掩码，本项目未使用 |
| `is_lte_band` | string | 固定 `"1"` | 启用 4G 频段设置 |
| `lte_band_mask` | string | 十进制掩码 | 4G 锁频掩码，`Σ 2^(band-1)`；`"0"` 表示空集 |

**支持的 4G 频段（后端 `allowed4GBands` 白名单，前端 `lteBandOptions` 一致）**

```
1 2 3 4 5 7 8 18 19 20 26 28 29 32 34 38 39 40 41 42 43 48 66 71
```

**频段 → bit → 掩码增量 速查表**

| Band | bit (=N-1) | 2^(N-1) | Band | bit | 2^(N-1) |
|---|---|---|---|---|---|
| B1 | 0 | 1 | B32 | 31 | 2147483648 |
| B2 | 1 | 2 | B34 | 33 | 8589934592 |
| B3 | 2 | 4 | B38 | 37 | 137438953472 |
| B4 | 3 | 8 | B39 | 38 | 274877906944 |
| B5 | 4 | 16 | B40 | 39 | 549755813888 |
| B7 | 6 | 64 | B41 | 40 | 1099511627776 |
| B8 | 7 | 128 | B42 | 41 | 2199023255552 |
| B18 | 17 | 131072 | B43 | 42 | 4398046511104 |
| B19 | 18 | 262144 | B48 | 47 | 140737488355328 |
| B20 | 19 | 524288 | B66 | 65 | 36893488147419103232 |
| B26 | 25 | 33554432 | B71 | 70 | 1180591620717411303424 |
| B28 | 27 | 134217728 | | | |
| B29 | 28 | 268435456 | | | |

> 高频段（B66=2^65、B71=2^70）已远超 64 位整数，所以后端用 `big.Int` 生成十进制串，
> `lte_band_mask` 可能是 20+ 位长数字，不能当普通整型处理。

**掩码计算示例**

| 选择 | 计算 | lte_band_mask |
|---|---|---|
| B1 + B3 | 1 + 4 | `5` |
| B3 + B41 | 4 + 1099511627776 | `1099511627780` |
| B1 + B3 + B40 + B41 | 1 + 4 + 549755813888 + 1099511627776 | `1649267441669` |
| 全部 24 个频段 | Σ 全部 | `1217485258272147374303` |

**边界语义**

- **空数组 `[]`** → mask `"0"`，下发空集（UI 的"全不选"）。
- **全选 = 自动**：UI 里"自动"按钮等价于"全选"（`selectAllLTEBands`），勾满所有频段
  即不做实际限制，等于放开自动选频。前端据此显示"4G 已切换为自动频段"提示，但
  **实际仍下发全频段掩码，不是 mask 0**。
- 后端去重：重复频段只计一次（`seen` map）。

**错误码**

| code | 触发条件 | msg |
|---|---|---|
| 1 | JSON 解析失败 | `输入数据不合法` |
| 2 | 含白名单外频段 | `不支持的 4G 频段: Bxx` |
| 1 | ubus 调用失败 | ubus 错误文本 |

**出参**：ubus 原始返回，后端包装为统一结构。成功后前端 `fetchAllData()` 立即刷新。

---

### 3. `nwinfo_set_nrbandlock` —— 5G 锁频

5G 锁频用**逗号分隔的频段编号字符串**（后端 `nrBandString()`，不带 `n` 前缀）。

**HTTP 层（前端实际走的路径）**

```http
POST /api/network/band/nr
Content-Type: application/json

{ "bands": [41, 78] }
```

前端传频段数组，**后端拼成逗号串**再下发。

**ubus 调用**

```sh
# 例：锁 n41 + n78
ubus call zte_nwinfo_api nwinfo_set_nrbandlock '{"nr5g_type":"SA","nr5g_band":"41,78"}'
```

**ubus 入参**

| 字段 | 类型 | 值 | 说明 |
|---|---|---|---|
| `nr5g_type` | string | 固定 `"SA"` | NR 类型，本项目恒为 SA（见下方注意） |
| `nr5g_band` | string | 逗号分隔频段号 | 如 `"41,78"`；空串代表空集/自动 |

**支持的 5G 频段（后端 `allowed5GBands` 白名单，前端 `nrBandOptions` 一致）**

```
1 2 3 5 7 8 18 20 26 28 29 38 40 41 48 66 71 75 77 78 79
```

**边界语义**

- **空数组 `[]`** → `nr5g_band` 为空串（UI 的"全不选"）。
- **全选 = 自动**：同 4G，"自动"按钮等价"全选"（`selectAllNRBands`），勾满即不限制。
  前端显示"5G 已切换为自动频段"，但实际下发的是全部频段串。
- 后端去重并**保留入参数组顺序**（按 `bands` 数组遍历，非排序）。

**注意（SA-only 局限）**

- 写入时 `nr5g_type` **恒为 `SA`**，即便当前制式是 NSA，锁频也写到 SA 维度。
- 而查询/回填时（`currentNRBandLockValue()`）会按 `network_type` 区分读
  `nr5g_sa_band_lock` 或 `nr5g_nsa_band_lock`。因此 **NSA 场景下"读到的锁频"与
  "写下去的锁频"维度可能不一致**，使用时需注意。

**错误码**

| code | 触发条件 | msg |
|---|---|---|
| 1 | JSON 解析失败 | `输入数据不合法` |
| 2 | 含白名单外频段 | `不支持的 5G 频段: Nxx` |
| 1 | ubus 调用失败 | ubus 错误文本 |

**出参**：ubus 原始返回，后端包装为统一结构。成功后前端 `fetchAllData()` 立即刷新。

---

## 四、速查表

| 功能 | 类型 | object.method | 关键入参 |
|---|---|---|---|
| 查制式/锁频现状 | 查询 | `zte_nwinfo_api nwinfo_get_netinfo` | `{}` |
| 切换网络制式 | 设定 | `zte_nwinfo_api nwinfo_set_netselect` | `net_select` |
| 4G 锁频 | 设定 | `zte_nwinfo_api nwinfo_set_gwl_bandlock` | `is_lte_band="1"`, `lte_band_mask`(掩码) |
| 5G 锁频 | 设定 | `zte_nwinfo_api nwinfo_set_nrbandlock` | `nr5g_type="SA"`, `nr5g_band`(逗号串) |

**代码位置**

- 设定（后端）：`gossh/app/service/network_settings.go`
- 批量查询代理：`gossh/app/service/zwrt_ubus.go`、`gossh/app/utils/ubus.go`
- 路由注册：`gossh/main.go`（`/api/network/*`、`/api/ubus`）
- 前端调用与字段解析：`webssh/src/views/Main.vue`
