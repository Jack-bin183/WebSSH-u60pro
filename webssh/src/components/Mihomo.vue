<template>
  <el-tab-pane label="Mihomo 代理" name="mihomo">
    <el-card style="margin-bottom: 16px">
      <!-- 顶部：进程状态 + 控制按钮 -->
      <el-row :gutter="16" align="middle">
        <el-col :span="12">
          <div style="display: flex; align-items: center; gap: 12px">
            <el-tag :type="status.running ? 'success' : 'info'" size="large">
              {{ status.running ? '运行中' : '已停止' }}
            </el-tag>
            <span v-if="status.running" style="color: #909399; font-size: 13px">
              PID {{ status.pid }}
            </span>
            <span style="color: #909399; font-size: 13px">
              {{ status.mihomo_dir }}
            </span>
          </div>
        </el-col>
        <el-col :span="12" style="text-align: right">
          <el-button-group>
            <el-button type="success" :loading="controlling === 'start'" @click="control('start')">启动</el-button>
            <el-button type="warning" :loading="controlling === 'restart'" @click="control('restart')">重启</el-button>
            <el-button type="danger" :loading="controlling === 'stop'" @click="control('stop')">停止</el-button>
            <el-button :loading="controlling === 'reload-ipset'" @click="control('reload-ipset')">重载 ipset</el-button>
          </el-button-group>
          <el-button style="margin-left: 8px" :icon="Refresh" @click="loadStatus" :loading="loadingStatus">刷新</el-button>
        </el-col>
      </el-row>

      <!-- 控制命令输出 -->
      <el-collapse-transition>
        <div v-if="controlOutput" style="margin-top: 12px">
          <pre class="output-box">{{ controlOutput }}</pre>
        </div>
      </el-collapse-transition>
    </el-card>

    <!-- 数据文件列表 -->
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span>数据文件</span>
          <div style="display: flex; align-items: center; gap: 10px">
            <span v-if="versionInfo.remote_version" style="font-size: 13px; color: #909399">
              远端：{{ versionInfo.remote_version }}
              &nbsp;|&nbsp;
              本地：{{ status.local_version || '未知' }}
            </span>
            <el-tag v-if="versionInfo.has_update" type="warning" size="small">有更新</el-tag>
            <el-tag v-else-if="versionInfo.remote_version && !versionInfo.has_update" type="success" size="small">已是最新</el-tag>

            <el-button size="small" @click="checkVersion" :loading="checkingVersion">检查更新</el-button>
            <el-button
              size="small"
              type="primary"
              @click="startUpdate"
              :loading="updateStatus.state === 'downloading'"
              :disabled="updateStatus.state === 'downloading'">
              一键更新
            </el-button>
            <el-button
              v-if="updateStatus.state === 'downloading'"
              size="small"
              type="danger"
              @click="cancelUpdate">
              取消
            </el-button>
          </div>
        </div>
      </template>

      <!-- 下载进度 -->
      <div v-if="updateStatus.state === 'downloading' || updateStatus.state === 'done' || updateStatus.state === 'failed' || updateStatus.state === 'canceled'"
           style="margin-bottom: 16px">
        <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 6px">
          <el-tag :type="updateStateTagType" size="small">{{ updateStateLabel }}</el-tag>
          <span style="font-size: 13px; color: #606266">{{ updateStatus.msg }}</span>
        </div>
        <el-progress
          v-if="updateStatus.state === 'downloading'"
          :percentage="updateStatus.percent"
          :format="(p: number) => `${p}%  ${updateStatus.file_name} [${updateStatus.file_index}/${updateStatus.file_total}]`"
          striped
          striped-flow
          :duration="10" />
      </div>

      <!-- 文件列表 -->
      <el-table :data="status.files" style="width: 100%">
        <el-table-column prop="name" label="文件名" width="160" />
        <el-table-column prop="desc" label="说明" />
        <el-table-column label="状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.exists ? 'success' : 'danger'" size="small">
              {{ scope.row.exists ? '存在' : '缺失' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="100">
          <template #default="scope">
            {{ scope.row.exists ? formatSize(scope.row.size) : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="mod_time" label="修改时间" width="180">
          <template #default="scope">
            {{ scope.row.exists ? scope.row.mod_time : '-' }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </el-tab-pane>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

// ─── 类型定义 ───

interface FileInfo {
  name: string
  desc: string
  exists: boolean
  size: number
  mod_time: string
}

interface MihomoStatus {
  running: boolean
  pid: number
  mihomo_dir: string
  dir_exists: boolean
  local_version: string
  files: FileInfo[]
}

interface VersionInfo {
  remote_version: string
  local_version: string
  has_update: boolean
}

interface UpdateStatus {
  state: string
  msg: string
  file_name: string
  file_index: number
  file_total: number
  downloaded: number
  total: number
  percent: number
  started_at: string
  updated_at: string
}

// ─── 状态 ───

const status = reactive<MihomoStatus>({
  running: false,
  pid: 0,
  mihomo_dir: '',
  dir_exists: false,
  local_version: '',
  files: [],
})

const versionInfo = reactive<VersionInfo>({
  remote_version: '',
  local_version: '',
  has_update: false,
})

const updateStatus = reactive<UpdateStatus>({
  state: 'idle',
  msg: '',
  file_name: '',
  file_index: 0,
  file_total: 0,
  downloaded: 0,
  total: 0,
  percent: 0,
  started_at: '',
  updated_at: '',
})

const loadingStatus = ref(false)
const controlling = ref('')
const controlOutput = ref('')
const checkingVersion = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

// ─── 计算属性 ───

const updateStateLabel = computed(() => {
  const map: Record<string, string> = {
    downloading: '下载中',
    done: '已完成',
    failed: '失败',
    canceled: '已取消',
    idle: '空闲',
  }
  return map[updateStatus.state] ?? updateStatus.state
})

const updateStateTagType = computed(() => {
  const map: Record<string, string> = {
    downloading: 'primary',
    done: 'success',
    failed: 'danger',
    canceled: 'info',
  }
  return (map[updateStatus.state] ?? 'info') as any
})

// ─── 方法 ───

async function loadStatus() {
  loadingStatus.value = true
  try {
    const res = await axios.get('/api/mihomo/status')
    if (res.data.code === 0) {
      Object.assign(status, res.data.data)
    }
  } catch (e: any) {
    ElMessage.error('获取状态失败: ' + (e.message ?? e))
  } finally {
    loadingStatus.value = false
  }
}

async function control(action: string) {
  controlling.value = action
  controlOutput.value = ''
  try {
    const res = await axios.post('/api/mihomo/control', { action })
    if (res.data.code === 0) {
      ElMessage.success(`${action} 执行成功`)
      controlOutput.value = res.data.output ?? ''
    } else {
      ElMessage.error(res.data.msg)
      controlOutput.value = res.data.output ?? res.data.msg
    }
    await loadStatus()
  } catch (e: any) {
    ElMessage.error('请求失败: ' + (e.message ?? e))
  } finally {
    controlling.value = ''
  }
}

async function checkVersion() {
  checkingVersion.value = true
  try {
    const res = await axios.get('/api/mihomo/data/version')
    if (res.data.code === 0) {
      Object.assign(versionInfo, res.data.data)
      if (versionInfo.has_update) {
        ElMessage.warning(`有新版本：${versionInfo.remote_version}`)
      } else {
        ElMessage.success('已是最新版本')
      }
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('检查失败: ' + (e.message ?? e))
  } finally {
    checkingVersion.value = false
  }
}

async function startUpdate() {
  try {
    const res = await axios.post('/api/mihomo/data/update')
    if (res.data.code === 0) {
      ElMessage.success('开始下载更新...')
      Object.assign(updateStatus, res.data.data)
      startPollUpdate()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('启动更新失败: ' + (e.message ?? e))
  }
}

async function cancelUpdate() {
  try {
    const res = await axios.post('/api/mihomo/data/update/cancel')
    if (res.data.code === 0) {
      ElMessage.info('已取消下载')
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error(e.message ?? e)
  }
}

async function pollUpdateStatus() {
  try {
    const res = await axios.get('/api/mihomo/data/update/status')
    if (res.data.code === 0) {
      Object.assign(updateStatus, res.data.data)
      if (updateStatus.state === 'done') {
        ElMessage.success('所有数据文件更新完成！')
        stopPollUpdate()
        await loadStatus()
      } else if (updateStatus.state === 'failed') {
        ElMessage.error('更新失败：' + updateStatus.msg)
        stopPollUpdate()
      } else if (updateStatus.state === 'canceled') {
        stopPollUpdate()
      }
    }
  } catch {
    // 忽略轮询错误
  }
}

function startPollUpdate() {
  if (pollTimer) return
  pollTimer = setInterval(pollUpdateStatus, 1000)
}

function stopPollUpdate() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

// ─── 生命周期 ───

onMounted(async () => {
  await loadStatus()
  // 如果已有下载任务在跑，恢复轮询
  const res = await axios.get('/api/mihomo/data/update/status').catch(() => null)
  if (res?.data?.code === 0) {
    Object.assign(updateStatus, res.data.data)
    if (updateStatus.state === 'downloading') {
      startPollUpdate()
    }
  }
})

onUnmounted(() => {
  stopPollUpdate()
})
</script>

<style scoped>
.output-box {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 12px 16px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
</style>
