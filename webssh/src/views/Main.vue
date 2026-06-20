<template>
  <div class="page">
    <div class="child">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="title-section">
        <h1 class="title">
          <div class="uptime-value" @click="handleUptimeSecretClick">
            已运行{{ formatUptime(deviceInfo.device_uptime as any) }}
          </div>
        </h1>
        <div class="status-indicator">
          <div :class="['status-dot', connectionStatusClass]"></div>
          <span class="status-text">{{ connectionStatusText }}</span>
        </div>
      </div>

      <div class="controls">
        <div class="top-status-controls">
          <div style="display: flex; position: relative">
            <span class="uptime-label">{{ netWorkProvider }} {{ networkType }}{{ is5GA ? 'A' : '' }}</span>
          </div>

          <div style="display: flex; align-items: center">
            <div
              :class="[
                'battery',
                batteryStateClass,
                {
                  charging:
                    deviceInfo?.bat_charger_connect &&
                    deviceInfo.bat_charger_connect == '1',
                },
              ]">
              <div
                class="battery-level"
                :style="{ width: `${deviceInfo.bat_percent || '0'}%` }"></div>
              <div class="battery-head"></div>
              <!-- 充电状态 -->
              <div class="battery-charging">⚡</div>
            </div>
            <span class="battery-percent" v-if="deviceInfo.bat_percent"
              >{{ deviceInfo.bat_percent }}%</span
            >
          </div>
        </div>

        <div class="quick-actions-grid">
          <button
            class="quick-action-button"
            :class="{ active: autoRefresh }"
            @click="toggleAutoRefresh"
          >
            <span class="quick-action-icon">{{ autoRefresh ? 'ON' : 'OFF' }}</span>
            <span class="quick-action-copy">
              <span class="quick-action-title">{{ autoRefresh ? '自动刷新中' : '自动刷新' }}</span>
              <span class="quick-action-subtitle">{{ autoRefresh ? '点击停止刷新' : '点击开始刷新' }}</span>
            </span>
          </button>

          <button
            class="quick-action-button adb-action-button"
            :class="{ active: usbStatus?.connect == 1 }"
            @click="handleOpenAdbClick"
          >
            <span class="quick-action-icon">ADB</span>
            <span class="quick-action-copy">
              <span class="quick-action-title">开启 ADB</span>
              <span class="quick-action-subtitle">{{ usbStatus?.connect == 1 ? 'USB 已连接' : '等待 USB 连接' }}</span>
            </span>
          </button>

          <button class="quick-action-button" @click="openNetworkSettingsDialog">
            <span class="quick-action-icon">Net</span>
            <span class="quick-action-copy">
              <span class="quick-action-title">网络设置</span>
              <span class="quick-action-subtitle">{{ networkSettingsSummary }}</span>
            </span>
          </button>

          <button
            class="quick-action-button speedtest-action-button"
            :class="{ active: localSpeedTest.running || smsForward.running }"
            @click="() => openSystemToolsDialog()"
          >
            <span class="quick-action-icon">SYS</span>
            <span class="quick-action-copy">
              <span class="quick-action-title">系统工具</span>
              <span class="quick-action-subtitle">{{ systemToolsSummary }}</span>
            </span>
          </button>

          <button
            v-if="mmEntryUnlocked"
            class="quick-action-button mm-action-button"
            :class="{ active: mmStatus.running }"
            @click="openMmDialog"
          >
            <span class="quick-action-icon">MH</span>
            <span class="quick-action-copy">
              <span class="quick-action-title">Mihomo</span>
              <span class="quick-action-subtitle">{{ mmStatus.running ? '代理运行中' : '代理已停止' }}</span>
            </span>
          </button>

          <button
            class="wifi-mode-button"
            :class="{ active: wifiInfo.highPerformance, saving: wifiSettingsSaving }"
            :disabled="wifiSettingsSaving !== ''"
            @click="openWifiSettingsDialog"
          >
            <span class="wifi-mode-icon">{{ wifiInfo.highPerformance ? 'HP' : 'PS' }}</span>
            <span class="wifi-mode-copy">
              <span class="wifi-mode-title">WiFi 设置</span>
              <span class="wifi-mode-action">{{ wifiSettingsSummary }}</span>
            </span>
          </button>

        </div>

      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading && !dataReady" class="loading">
      <div class="loading-spinner"></div>
      <p>正在加载数据...</p>
    </div>

    <!-- 错误状态 -->
    <div v-else-if="error" class="error">
      <div class="error-icon">⚠️</div>
      <h3>加载失败</h3>
      <p style="margin: 10px 0">{{ error }}</p>
    </div>

    <!-- 数据展示 -->
    <div v-else-if="dataReady" class="content">
      <div class="top-cards">
        <!-- NR 5G 信号卡片 -->
        <div class="card" v-if="networkType === '5G'">
          <div class="card-header">
            <h3 class="hd">
              <img style="width: 24px" :src="NetworkIcon" alt="" />5G 信号
            </h3>
            <div class="card-tags">
              <span :class="['tag', 'conn-status', connectStatusTag.className]">{{ connectStatusTag.text }}</span>
              <span :class="['tag', getNetworkSignalStatus('nr').className]">
                信号{{ getNetworkSignalStatus('nr').text }}
              </span>
            </div>
          </div>
          <div class="card-content">
            <div class="signal-grid">
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">RSRP</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('nr', 'rsrp')"
                      @click="toggleSignalHelp('nr', 'rsrp')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('nr', 'rsrp', d.nr5g_rsrp).className]">
                    {{ getSignalDisplayStatus('nr', 'rsrp', d.nr5g_rsrp).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('nr', 'rsrp', d.nr5g_rsrp).className"
                    :style="{ width: getRsrpPercent(d.nr5g_rsrp) + '%' }"></div>
                  <span class="progress-text">{{ formatDbm(d.nr5g_rsrp) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('nr', 'rsrp')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.rsrp.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.rsrp.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.rsrp.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">RSRQ</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('nr', 'rsrq')"
                      @click="toggleSignalHelp('nr', 'rsrq')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('nr', 'rsrq', d.nr5g_rsrq).className]">
                    {{ getSignalDisplayStatus('nr', 'rsrq', d.nr5g_rsrq).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('nr', 'rsrq', d.nr5g_rsrq).className"
                    :style="{ width: getRsrqPercent(d.nr5g_rsrq) + '%' }"></div>
                  <span class="progress-text">{{ formatDb(d.nr5g_rsrq) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('nr', 'rsrq')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.rsrq.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.rsrq.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.rsrq.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">SINR</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('nr', 'sinr')"
                      @click="toggleSignalHelp('nr', 'sinr')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('nr', 'sinr', d.nr5g_snr).className]">
                    {{ getSignalDisplayStatus('nr', 'sinr', d.nr5g_snr).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('nr', 'sinr', d.nr5g_snr).className"
                    :style="{ width: getSnrPercent(d.nr5g_snr) + '%' }"></div>
                  <span class="progress-text">{{ formatSnr(d.nr5g_snr) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('nr', 'sinr')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.sinr.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.sinr.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.sinr.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">RSSI</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('nr', 'rssi')"
                      @click="toggleSignalHelp('nr', 'rssi')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('nr', 'rssi', d.nr5g_rssi).className]">
                    {{ getSignalDisplayStatus('nr', 'rssi', d.nr5g_rssi).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('nr', 'rssi', d.nr5g_rssi).className"
                    :style="{ width: getRssiPercent(d.nr5g_rssi) + '%' }"></div>
                  <span class="progress-text">{{ formatDbm(d.nr5g_rssi) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('nr', 'rssi')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.rssi.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.rssi.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.rssi.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <span class="label">PCI</span>
                <span class="value">{{ d.nr5g_pci ?? '-' }}</span>
              </div>

              <div class="signal-item" width="100%">
                <span class="label">Cell ID</span>
                <span class="value">{{ d.nr5g_cell_id ?? '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- NR 5G 载波 -->
        <div class="card"  v-if="networkType === '5G'">
          <div class="card-header">
            <h3 class="hd">
              <img style="width: 24px" :src="NetworkIcon" alt="" />5G 载波信息
            </h3>
            <span class="tag success">
              {{ networkType }}{{ is5GA ? 'A' : '' }}

              ({{ d.nr5g_action_band?.toUpperCase() ?? '-' }}{{
                  formatNrca(d.nrca,'',0,3) != '-' ? ', N' + formatNrca(d.nrca,'',0,3) : '' }}{{
                  formatNrca(d.nrca,'',1,3) != '-' ? ', N' + formatNrca(d.nrca,'',1,3) : '' }})
              </span>
          </div>
          <div class="card-content">
            <div class="signal-grid">
              <table class="mytable" width="100%">
                <tr>
                  <td width="13%"></td>
                  <td width="9%">PCI</td>
                  <td width="11%">频段</td>
                  <td width="16%">频点</td>
                  <td width="11%">带宽</td>
                  <td width="10%">RSRP</td>
                  <td width="10%">RSRQ</td>
                  <td width="10%">SINR</td>
                  <td width="10%">RSSI</td>
                </tr>
                <tr>
                  <td>PCC</td>
                  <td>{{ d.nr5g_pci ?? '-' }}</td>
                  <td>{{ d.nr5g_action_band?.toUpperCase() ?? '-' }}</td>
                  <td>{{ d.nr5g_action_channel ?? '-' }}</td>
                  <td>{{ d.nr5g_bandwidth ? d.nr5g_bandwidth + 'Mhz' : '-' }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', d.nr5g_rsrp)]">{{ d.nr5g_rsrp }}</td>
                  <td :class="getSignalValueClass('rsrq', d.nr5g_rsrq)">{{ d.nr5g_rsrq }}</td>
                  <td :class="getSignalValueClass('sinr', d.nr5g_snr)">{{ d.nr5g_snr }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', d.nr5g_rssi)]">{{ d.nr5g_rssi }}</td>
                </tr>
                <tr>
                  <td>SCC0</td>
                  <td>{{ formatNrca(d.nrca,'',0,1) }}</td>
                  <td>{{ formatNrca(d.nrca,'N',0,3) }}</td>
                  <td>{{ formatNrca(d.nrca,'',0,4) }}</td>
                  <td>
                    {{
                      formatNrca(d.nrca, '', 0, 5) != '-'
                        ? formatNrca(d.nrca, '', 0, 5) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.nrca,'',0,7))]">{{ formatNrca(d.nrca,'',0,7) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.nrca,'',0,8))">{{ formatNrca(d.nrca,'',0,8) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.nrca,'',0,9))">{{ formatNrca(d.nrca,'',0,9) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.nrca,'',0,10))]">{{ formatNrca(d.nrca,'',0,10) }}</td>
                </tr>
                <tr>
                  <td>SCC1</td>
                  <td>{{ formatNrca(d.nrca,'',1,1) }}</td>
                  <td>{{ formatNrca(d.nrca,'N',1,3) }}</td>
                  <td>{{ formatNrca(d.nrca,'',1,4) }}</td>
                  <td>
                    {{
                      formatNrca(d.nrca, '', 1, 5) != '-'
                        ? formatNrca(d.nrca, '', 1, 5) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.nrca,'',1,7))]">{{ formatNrca(d.nrca,'',1,7) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.nrca,'',1,8))">{{ formatNrca(d.nrca,'',1,8) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.nrca,'',1,9))">{{ formatNrca(d.nrca,'',1,9) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.nrca,'',1,10))]">{{ formatNrca(d.nrca,'',1,10) }}</td>
                </tr>
                <tr>
                  <td>SCC2</td>
                  <td>{{ formatNrca(d.nrca,'',2,1) }}</td>
                  <td>{{ formatNrca(d.nrca,'N',2,3) }}</td>
                  <td>{{ formatNrca(d.nrca,'',2,4) }}</td>
                  <td>
                    {{
                      formatNrca(d.nrca, '', 2, 5) != '-'
                        ? formatNrca(d.nrca, '', 2, 5) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.nrca,'',2,7))]">{{ formatNrca(d.nrca,'',2,7) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.nrca,'',2,8))">{{ formatNrca(d.nrca,'',2,8) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.nrca,'',2,9))">{{ formatNrca(d.nrca,'',2,9) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.nrca,'',2,10))]">{{ formatNrca(d.nrca,'',2,10) }}</td>
                </tr>
              </table>
            </div>
          </div>
        </div>

        <!-- LTE 4G 信号卡片 -->
        <div class="card" v-if="networkType === '4G'">
          <div class="card-header">
            <h3 class="hd">
              <img style="width: 24px" :src="NetworkIcon" alt="" />4G 信号
            </h3>
            <div class="card-tags">
              <span :class="['tag', 'conn-status', connectStatusTag.className]">{{ connectStatusTag.text }}</span>
              <span :class="['tag', getNetworkSignalStatus('lte').className]">
                信号{{ getNetworkSignalStatus('lte').text }}
              </span>
            </div>
          </div>
          <div class="card-content">
            <div class="signal-grid">

              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">RSRP</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('lte', 'rsrp')"
                      @click="toggleSignalHelp('lte', 'rsrp')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('lte', 'rsrp', d.lte_rsrp).className]">
                    {{ getSignalDisplayStatus('lte', 'rsrp', d.lte_rsrp).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('lte', 'rsrp', d.lte_rsrp).className"
                    :style="{ width: getRsrpPercent(d.lte_rsrp) + '%' }"></div>
                  <span class="progress-text">{{ formatDbm(d.lte_rsrp) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('lte', 'rsrp')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.rsrp.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.rsrp.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.rsrp.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">RSRQ</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('lte', 'rsrq')"
                      @click="toggleSignalHelp('lte', 'rsrq')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('lte', 'rsrq', d.lte_rsrq).className]">
                    {{ getSignalDisplayStatus('lte', 'rsrq', d.lte_rsrq).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('lte', 'rsrq', d.lte_rsrq).className"
                    :style="{ width: getRsrqPercent(d.lte_rsrq) + '%' }"></div>
                  <span class="progress-text">{{ formatDb(d.lte_rsrq) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('lte', 'rsrq')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.rsrq.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.rsrq.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.rsrq.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">SINR</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('lte', 'sinr')"
                      @click="toggleSignalHelp('lte', 'sinr')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('lte', 'sinr', d.lte_snr).className]">
                    {{ getSignalDisplayStatus('lte', 'sinr', d.lte_snr).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('lte', 'sinr', d.lte_snr).className"
                    :style="{ width: getSnrPercent(d.lte_snr) + '%' }"></div>
                  <span class="progress-text">{{ formatSnr(d.lte_snr) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('lte', 'sinr')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.sinr.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.sinr.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.sinr.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <div class="signal-label-row">
                  <span class="signal-label-help">
                    <span class="label">RSSI</span>
                    <button
                      type="button"
                      class="signal-help-trigger"
                      :aria-expanded="isSignalHelpOpen('lte', 'rssi')"
                      @click="toggleSignalHelp('lte', 'rssi')">*</button>
                  </span>
                  <span :class="['signal-status', getSignalDisplayStatus('lte', 'rssi', d.lte_rssi).className]">
                    {{ getSignalDisplayStatus('lte', 'rssi', d.lte_rssi).text }}
                  </span>
                </div>
                <div class="progress-bar">
                  <div
                    class="progress-fill"
                    :class="getSignalDisplayStatus('lte', 'rssi', d.lte_rssi).className"
                    :style="{ width: getRssiPercent(d.lte_rssi) + '%' }"></div>
                  <span class="progress-text">{{ formatDbm(d.lte_rssi) }}</span>
                </div>
                <div v-if="isSignalHelpOpen('lte', 'rssi')" class="signal-help-panel">
                  <div class="signal-help-title">{{ signalHelpMap.rssi.title }}</div>
                  <div class="signal-help-desc">{{ signalHelpMap.rssi.description }}</div>
                  <div class="signal-help-ranges">
                    <div v-for="item in signalHelpMap.rssi.ranges" :key="item.label">
                      <span :class="['signal-help-dot', item.className]"></span>
                      <span>{{ item.label }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="signal-item">
                <span class="label">PCI</span>
                <span class="value">{{ d.lte_pci ?? '-' }}</span>
              </div>
              <div class="signal-item">
                <span class="label">Cell ID</span>
                <span class="value">{{ d.cell_id ?? '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- LTE 4G 载波 -->
        <div class="card" v-if="networkType === '4G'">
          <div class="card-header">
            <h3 class="hd">
              <img style="width: 24px" :src="NetworkIcon" alt="" />4G 载波信息
            </h3>
            <span class="tag success">
              {{ networkType }}{{ is5GA ? '+' : '' }}
              ({{ formatNrca(d.lteca,'B',0,1) }}{{
                  formatNrca(d.lteca,'',1,1) != '-' ? ', B' + formatNrca(d.lteca,'',1,1) : '' }}{{
                  formatNrca(d.lteca,'',2,1) != '-' ? ', B' + formatNrca(d.lteca,'',2,1) : '' }}{{
                  formatNrca(d.lteca,'',3,1) != '-' ? ', B' + formatNrca(d.lteca,'',3,1) : '' }})
            </span>
          </div>
          <div class="card-content">
            <div class="signal-grid">
              <table class="mytable" width="100%">
                <tr>
                  <td width="13%"></td>
                  <td width="9%">PCI</td>
                  <td width="11%">频段</td>
                  <td width="16%">信道</td>
                  <td width="11%">带宽</td>
                  <td width="10%">RSRP</td>
                  <td width="10%">RSRQ</td>
                  <td width="10%">SINR</td>
                  <td width="10%">RSSI</td>
                </tr>
                <tr>
                  <td>PCC</td>
                  <td>{{ d.lte_pci ?? '-' }}</td>
                  <td>{{ formatNrca(d.lteca,'B',0,1) }}</td>
                  <td>{{ d.wan_active_channel ?? '-' }}</td>
                  <td>
                    {{
                      formatNrca(d.lteca, '', 0, 4) != '-'
                        ? formatNrca(d.lteca, '', 0, 4) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', d.lte_rsrp)]">{{ d.lte_rsrp }}</td>
                  <td :class="getSignalValueClass('rsrq', d.lte_rsrq)">{{ d.lte_rsrq }}</td>
                  <td :class="getSignalValueClass('sinr', d.lte_snr)">{{ d.lte_snr }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', d.lte_rssi)]">{{ d.lte_rssi }}</td>
                </tr>
                <tr>
                  <td>SCC0</td>
                  <td>{{ formatNrca(d.lteca,'',1,0) }}</td>
                  <td>{{ formatNrca(d.lteca,'B',1,1) }}</td>
                  <td>{{ formatNrca(d.lteca,'',1,3) }}</td>
                  <td>{{
                      formatNrca(d.lteca, '', 1, 4) != '-'
                        ? formatNrca(d.lteca, '', 1, 4) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.ltecasig,'',0,0))]">{{ formatNrca(d.ltecasig,'',0,0) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.ltecasig,'',0,1))">{{ formatNrca(d.ltecasig,'',0,1) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.ltecasig,'',0,2))">{{ formatNrca(d.ltecasig,'',0,2) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.ltecasig,'',0,3))]">{{ formatNrca(d.ltecasig,'',0,3) }}</td>
                </tr>
                <tr>
                  <td>SCC1</td>
                  <td>{{ formatNrca(d.lteca,'',2,0) }}</td>
                  <td>{{ formatNrca(d.lteca,'B',2,1) }}</td>
                  <td>{{ formatNrca(d.lteca,'',2,3) }}</td>
                  <td>{{
                      formatNrca(d.lteca, '', 2, 4) != '-'
                        ? formatNrca(d.lteca, '', 2, 4) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.ltecasig,'',1,0))]">{{ formatNrca(d.ltecasig,'',1,0) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.ltecasig,'',1,1))">{{ formatNrca(d.ltecasig,'',1,1) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.ltecasig,'',1,2))">{{ formatNrca(d.ltecasig,'',1,2) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.ltecasig,'',1,3))]">{{ formatNrca(d.ltecasig,'',1,3) }}</td>
                </tr>
                <tr>
                  <td>SCC2</td>
                  <td>{{ formatNrca(d.lteca,'',3,0) }}</td>
                  <td>{{ formatNrca(d.lteca,'B',3,1) }}</td>
                  <td>{{ formatNrca(d.lteca,'',3,3) }}</td>
                  <td>{{
                      formatNrca(d.lteca, '', 3, 4) != '-'
                        ? formatNrca(d.lteca, '', 3, 4) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.ltecasig,'',2,0))]">{{ formatNrca(d.ltecasig,'',2,0) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.ltecasig,'',2,1))">{{ formatNrca(d.ltecasig,'',2,1) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.ltecasig,'',2,2))">{{ formatNrca(d.ltecasig,'',2,2) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.ltecasig,'',2,3))]">{{ formatNrca(d.ltecasig,'',2,3) }}</td>
                </tr>
                <tr>
                  <td>SCC3</td>
                  <td>{{ formatNrca(d.lteca,'',4,0) }}</td>
                  <td>{{ formatNrca(d.lteca,'B',4,1) }}</td>
                  <td>{{ formatNrca(d.lteca,'',4,3) }}</td>
                  <td>{{
                      formatNrca(d.lteca, '', 4, 4) != '-'
                        ? formatNrca(d.lteca, '', 4, 4) + 'Mhz'
                        : '-'
                    }}
                  </td>
                  <td :class="['dbmstyle', getSignalValueClass('rsrp', formatNrca(d.ltecasig,'',4,0))]">{{ formatNrca(d.ltecasig,'',4,0) }}</td>
                  <td :class="getSignalValueClass('rsrq', formatNrca(d.ltecasig,'',4,1))">{{ formatNrca(d.ltecasig,'',4,1) }}</td>
                  <td :class="getSignalValueClass('sinr', formatNrca(d.ltecasig,'',4,2))">{{ formatNrca(d.ltecasig,'',4,2) }}</td>
                  <td :class="['dbmstyle', getSignalValueClass('rssi', formatNrca(d.ltecasig,'',4,3))]">{{ formatNrca(d.ltecasig,'',4,3) }}</td>
                </tr>
              </table>
            </div>
          </div>
        </div>
      </div>
      

      <!-- 设备信息卡片 -->
      <div class="card device-info-card">
        <div class="card-header">
          <h3 class="hd">
            <img style="width: 24px" :src="DashboardIcon" alt="" />设备信息
          </h3>
        </div>
        <div class="card-content">
          <div class="device-stats">

            <div class="device-item">
              <div class="health-card cpu-health-card">
                <div class="health-title">CPU 负载</div>

                <div class="cpu-health-layout">
                  <div class="cpu-pie-box">
                    <div class="cpu-pie" :style="cpuPieStyle">
                      <div class="cpu-pie-inner">
                        <div class="cpu-pie-value">{{ totalCpuLoad.toFixed(0) }}%</div>
                      </div>
                    </div>
                  </div>

                  <div class="cpu-core-grid">
                    <div
                      class="cpu-core-card"
                      v-for="item in cpuCoreLoads"
                      :key="item.name"
                    >
                      <div class="cpu-core-header">
                        <!-- <span class="cpu-core-name">{{ item.name }}</span> -->
                        <span class="cpu-core-value">{{ item.value.toFixed(0) }}%</span>
                      </div>

                      <div class="cpu-core-bar">
                        <div
                          class="cpu-core-fill"
                          :style="{ width: item.value + '%' }"
                        ></div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>


            <div class="device-item">
              <div class="health-card temp-health-card">
                <div class="health-title">温度状态</div>

                <div class="temp-gauges">
                  <div class="temp-gauge">
                    <div
                      class="temp-ring"
                      :class="getTempClass(cpuTemp.cpuss_temp)"
                      :style="{ '--percent': getTempPercent(cpuTemp.cpuss_temp) + '%' }"
                    >
                      <div class="temp-ring-inner">
                        <strong>{{ cpuTemp.cpuss_temp || '-' }}°</strong>
                        <span>CPU</span>
                      </div>
                    </div>
                    <div class="temp-state" :class="getTempClass(cpuTemp.cpuss_temp)">
                      {{ getTempText(cpuTemp.cpuss_temp) }}
                    </div>
                  </div>

                  <div class="temp-gauge">
                    <div
                      class="temp-ring"
                      :class="getTempClass(Number(deviceInfo.bat_temperature))"
                      :style="{ '--percent': getTempPercent(Number(deviceInfo.bat_temperature)) + '%' }"
                    >
                      <div class="temp-ring-inner">
                        <strong>{{ deviceInfo.bat_temperature || '-' }}°</strong>
                        <span>电池</span>
                      </div>
                    </div>
                    <div
                      class="temp-state"
                      :class="getTempClass(Number(deviceInfo.bat_temperature))"
                    >
                      {{ getTempText(Number(deviceInfo.bat_temperature)) }}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="device-item">
              <div class="health-card memory-health-card">
                <div class="health-title">内存使用</div>

                <div class="memory-big">
                  {{
                    formatMemoryPercent(
                      ((deviceInfo.meminfo?.total as any) || 0) -
                        ((deviceInfo.meminfo?.avaliable as any) || 0),
                      (deviceInfo.meminfo?.total as any) || 1
                    )
                  }}%
                </div>

                <div class="memory-detail">
                  {{
                    formatMemory(
                      ((deviceInfo.meminfo?.total || 0) as any) -
                        ((deviceInfo.meminfo?.avaliable as any) || 0)
                    )
                  }}
                  <span>/ {{ formatMemory((deviceInfo.meminfo?.total as any) || 0) }}</span>
                </div>

                <div class="memory-stack">
                  <div
                    class="memory-stack-fill"
                    :style="{
                      width:
                        formatMemoryPercent(
                          ((deviceInfo.meminfo?.total as any) || 0) -
                            ((deviceInfo.meminfo?.avaliable as any) || 0),
                          (deviceInfo.meminfo?.total as any) || 1
                        ) + '%',
                    }"
                  ></div>
                </div>

                <div class="memory-caption">已用 / 总量</div>
              </div>
            </div>

          </div>
        </div>
      </div>

      <!-- 网络信息卡片 -->
      <div class="card">
        <div class="card-header">
          <h3 class="hd">
            <img style="width: 24px" :src="InternetIcon" alt="" />网络信息
          </h3>
        </div>
        <div class="card-content">
          <div class="info-grid">
            <div class="info-item">
                <span class="label">运营商</span>
                <span class="value">{{ netWorkProvider }}</span>
            </div>
            <div class="info-item">
                <span class="label">QCI</span>
                <span class="value">{{ netAmbr.qci2 || netAmbr.qci1 }}</span>
            </div>
            

            <div class="info-item">
              <span class="label">网络类型</span>
              <span class="value">{{ networkType }}{{ is5GA ? 'A' : '' }}</span>
            </div>

            <div class="info-item">
              <span class="label">签约速率</span>
              <span class="value">⬇️ {{ netAmbr.dl.value }} {{ formatSpeedUnit(netAmbr.dl.unit) }} 
               ⬆️ {{ netAmbr.ul.value }} {{ formatSpeedUnit(netAmbr.dl.unit) }}</span>
            </div>

            <div class="info-item">
              <span class="label">信号强度</span>
              <div :class="['signal-bars', signalLevelClass]">
                <div
                  v-for="n in 5"
                  :key="n"
                  :class="['bar', { active: n <= signalBars }]"></div>
              </div>
            </div>
            <div class="info-item">
              <span class="label">接入设备</span>
              <span class="value">
                <button class="device-count-link" @click="openDeviceDialog">无线: {{ lanUserList?.wireless_num || '0' }}</button>
                &nbsp;/&nbsp;
                <button class="device-count-link" @click="openDeviceDialog">有线: {{ lanUserList?.lan_num || '0' }}</button>
              </span>
            </div>
            <div class="info-item">
              <span class="label">主载波</span>
              <span class="value">{{
                d.wan_active_band?.toUpperCase() || '-'
              }}</span>
            </div>
            <div class="info-item">
              <span class="label">当前载波</span>
              <span class="value"
                >{{
                  d.wan_active_band?.toUpperCase()
                    ? d.wan_active_band.toUpperCase() + ', '
                    : ''
                }}{{ currentActiveBands || '-' }}</span
              >
            </div>
            <div class="info-item">
              <span class="label">频道</span>
              <span class="value">{{ d.nr5g_action_channel ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">带宽</span>
              <span class="value">{{
                d.nr5g_bandwidth ? d.nr5g_bandwidth + ' Mhz' : '-'
              }}</span>
            </div>
            <div class="info-item">
              <span class="label">ICCID</span>
              <span class="value">{{ simInfo2.sim_iccid ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">SIM 卡号</span>
              <span class="value">{{simInfo2?.msisdn ?? '-'}}</span>
            </div>
            <div class="info-item">
              <span class="label">IMEI</span>
              <span class="value">{{ simInfo?.values?.imei ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">IMSI</span>
              <span class="value">{{ simInfo2.sim_imsi ?? '-' }}</span>
            </div>

            <div class="info-item">
              <span class="label">Modem MSN</span>
              <span class="value">{{ simInfo?.values?.modem_msn ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">系统版本</span>
              <span class="value">{{
                sysVersion?.wa_inner_version ?? '-'
              }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 流量统计卡片 -->
      <div class="card">
        <div class="card-header">
          <h3 class="hd">
            <img style="width: 24px" :src="ChartIcon" alt="" />流量统计
          </h3>
        </div>
        <div class="card-content">
          <div class="traffic-stats">
            <div class="traffic-item">
              <div class="traffic-label">上传速度</div>
              <div class="traffic-value upload">
                {{ formatSpeed(trafficData.real_tx_speed) }}
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">下载速度</div>
              <div class="traffic-value download">
                {{ formatSpeed(trafficData.real_rx_speed) }}
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">上传用量</div>
              <div class="traffic-value" style="font-size: 12px">
                <!-- {{ formatBytes(trafficData.real_tx_bytes) }} -->
                <div>当日：{{ formatBytes(trafficData.day_tx_bytes) }}</div>
                <div>本月：{{ formatBytes(trafficData.month_tx_bytes) }}</div>
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">下载用量</div>
              <div class="traffic-value"></div>
              <div class="traffic-value" style="font-size: 12px">
                <!-- {{ formatBytes(trafficData.real_rx_bytes) }} -->
                <div>当日：{{ formatBytes(trafficData.day_rx_bytes) }}</div>
                <div>本月：{{ formatBytes(trafficData.month_rx_bytes) }}</div>
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">总上传</div>
              <div class="traffic-value">
                {{ formatBytes(trafficData.total_tx_bytes as any) }}
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">总下载</div>
              <div class="traffic-value">
                {{ formatBytes(trafficData.total_rx_bytes as any) }}
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">最大上传速度</div>
              <div class="traffic-value">
                {{ formatSpeed(trafficData.real_max_tx_speed) }}
              </div>
            </div>
            <div class="traffic-item">
              <div class="traffic-label">最大下载速度</div>
              <div class="traffic-value">
                {{ formatSpeed(trafficData.real_max_rx_speed) }}
              </div>
            </div>
          </div>
        </div>
      </div>

            <!-- 接口状态卡片 -->
      <div class="card">
        <div class="card-header">
          <h3 class="hd">
            <img style="width: 24px" :src="InterfaceIcon" alt="" />接口状态
          </h3>
        </div>
        <div class="card-content">
          <div class="interface-grid">

            <div class="interface-section" v-if="wwanInfo?.ipv4_address">
              <h4>WAN IPv4</h4>
              <div class="info-grid-compact">
                <div class="info-item">
                  <span class="label">IP 地址</span>
                  <span class="value">{{ wwanInfo?.ipv4_address || '-' }}</span>
                </div>
                <div class="info-item">
                  <span class="label">网关</span>
                  <span class="value">{{ wwanInfo?.ipv4_gateway || '-' }}</span>
                </div>
                <div class="info-item">
                  <span class="label">DNS 服务器</span>
                  <span class="value">{{
                    wanData['dns-server']?.join('\n') || '-'
                  }}</span>
                </div>
                <div class="info-item">
                  <span class="label">运行时间</span>
                  <span class="value">{{ formatUptime(wanData.uptime) }}</span>
                </div>
              </div>
            </div>

            <div class="interface-section" v-if="wwanInfo?.ipv6_address !== '0'">
              <h4>WAN IPv6</h4>
              <div class="info-grid-compact">
                <div class="info-item">
                  <span class="label">IPv6 地址</span>
                  <span class="value">{{ wwanInfo?.ipv6_address || '-' }}</span>
                </div>
                <div class="info-item">
                  <span class="label">网关</span>
                  <span class="value">{{ wwanInfo?.ipv6_gateway || '-' }}</span>
                </div>
                <div class="info-item">
                  <span class="label">DNS 服务器</span>
                  <span class="value">{{
                    wan6Data['dns-server']?.join('\n') || '-'
                  }}</span>
                </div>
                <div class="info-item">
                  <span class="label">运行时间</span>
                  <span class="value">{{ formatUptime(wan6Data.uptime) }}</span>
                </div>
              </div>
            </div>
            
            <div class="interface-section" v-if="lanData?.ipv4_address">
              <h4>LAN 接口</h4>
              <div class="info-grid-compact">
                <div class="info-item">
                  <span class="label">IP 地址</span>
                  <span class="value">{{
                    lanData.ipv4_address?.[0]?.address || '-'
                  }}</span>
                </div>
                <div class="info-item">
                  <span class="label">网关</span>
                  <span class="value">{{
                    lanData.route?.[0]?.nexthop || '-'
                  }}</span>
                </div>
                <div class="info-item">
                  <span class="label">DNS 服务器</span>
                  <span class="value">{{
                    lanData['dns-server']?.join('\n') || '-'
                  }}</span>
                </div>
                <div class="info-item">
                  <span class="label">运行时间</span>
                  <span class="value">{{ formatUptime(lanData.uptime) }}</span>
                </div>
              </div>
            </div>

          </div>
        </div>
      </div>

    </div>

    <!-- 空状态 -->
    <div v-else class="empty">
      <div class="empty-icon">📱</div>
      <h3>暂无数据</h3>
      <p>请点击刷新按钮获取设备信息</p>
    </div>
    </div>
  </div>

  <!-- ───────── Mihomo 管理弹窗 ───────── -->
  <el-dialog
    v-model="mmDialogVisible"
    title="Mihomo 代理管理"
    width="min(760px, 96vw)"
    :close-on-click-modal="true"
    destroy-on-close
    class="mm-dialog">

    <el-tabs v-model="mmActiveTab" type="border-card" class="mm-tabs">

      <!-- ── Tab 1: 总览 ── -->
      <el-tab-pane label="总览" name="overview">
        <!-- 状态卡片 -->
        <div class="mh-status-card">
          <div class="mh-status-row">
            <el-tag :type="mmStatus.running ? 'success' : 'info'" size="large" effect="dark">
              {{ mmStatus.running ? '● 运行中' : '○ 已停止' }}
            </el-tag>
            <span v-if="mmStatus.running" class="mh-meta">PID {{ mmStatus.pid }}</span>
            <span v-if="mmStatus.running && mmStatus.start_time" class="mh-meta">启动于 {{ mmStatus.start_time }}</span>
            <span class="mh-meta mh-dir">路径: {{ mmStatus.mihomo_dir }}</span>
          </div>
          <div class="mh-info-grid">
            <div class="mh-info-item">
              <span class="mh-info-label">内核版本</span>
              <span class="mh-info-value">{{ mmStatus.binary_version || '未知' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">API 状态</span>
              <el-tag v-if="mmStatus.running" :type="mmStatus.api_reachable ? 'success' : 'warning'" size="small">
                {{ mmStatus.api_reachable ? '正常' : '异常' }}
              </el-tag>
              <span v-else class="mh-info-value">—</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">API 版本</span>
              <span class="mh-info-value">{{ mmStatus.api_version || '—' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">控制面板</span>
              <a
                v-if="mmStatus.api_version"
                :href="'http://' + mmStatus.external_controller"
                target="_blank"
                class="mh-info-value"
              >
                {{ mmStatus.external_controller }}
              </a>
              <span v-else class="mh-info-value">—</span>
              
            </div>
          </div>
        </div>

        <!-- 控制按钮 -->
        <div class="mh-control-row">
          <el-button-group>
            <el-button type="success" :loading="mmControlling==='start'" @click="mmControl('start')">启动</el-button>
            <el-button type="warning" :loading="mmControlling==='restart'" @click="mmControl('restart')">重启</el-button>
            <el-button type="danger" :loading="mmControlling==='stop'" @click="mmControl('stop')">停止</el-button>
            <el-button :loading="mmControlling==='reload-ipset'" @click="mmControl('reload-ipset')">重载 ipset</el-button>
          </el-button-group>
          <el-button :icon="RefreshIcon" @click="loadMmStatus" :loading="mmLoadingStatus">刷新</el-button>
        </div>
        <transition name="el-fade-in">
          <pre v-if="mmControlOutput" class="mh-output">{{ mmControlOutput }}</pre>
        </transition>

        <!-- 开机自启 -->
        <div v-if="mmStatus.binary_version" class="mh-autostart-row">
          <span class="mh-info-label">开机自启</span>
          <el-switch
              v-model="mmStatus.autostart_enabled"
              :loading="mmAutostartChanging"
              active-text="已开启"
              inactive-text="已关闭"
              @change="(v: boolean) => setMmAutostart(v)"
            />
          <span class="mh-meta">webssh 启动时自动运行 mihomo</span>
        </div>
      </el-tab-pane>

      <!-- ── Tab 2: 数据更新 ── -->
      <el-tab-pane label="数据更新" name="data">
        <div class="mh-data-header">
          <div class="mh-version-row">
            <span v-if="mmVersionInfo.remote_version" class="mh-meta">
              远端：{{ mmVersionInfo.remote_version }}&nbsp;|&nbsp;本地：{{ mmStatus.local_version || '未知' }}
            </span>
            <el-tag v-if="mmVersionInfo.has_update" type="warning" size="small">有更新</el-tag>
            <el-tag v-else-if="mmVersionInfo.remote_version && !mmVersionInfo.has_update" type="success" size="small">已最新</el-tag>
          </div>
          <div style="display:flex;gap:6px;flex-wrap:wrap">
            <el-button size="small" @click="checkMmVersion" :loading="mmCheckingVersion">检查更新</el-button>
            <el-button size="small" type="primary"
              :loading="mmUpdateStatus.state==='downloading'"
              :disabled="mmUpdateStatus.state==='downloading'"
              @click="startMmUpdate">一键更新</el-button>
            <el-button v-if="mmUpdateStatus.state==='downloading'" size="small" type="danger" @click="cancelMmUpdate">取消</el-button>
          </div>
        </div>

        <div v-if="['downloading','done','failed','canceled'].includes(mmUpdateStatus.state)" class="mh-progress-area">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px">
            <el-tag :type="mmUpdateTagType" size="small">{{ mmUpdateLabel }}</el-tag>
            <span style="font-size:12px;color:#606266">{{ mmUpdateStatus.msg }}</span>
          </div>
          <el-progress
            v-if="mmUpdateStatus.state==='downloading'"
            :percentage="mmUpdateStatus.percent"
            :format="() => `${mmUpdateStatus.percent}%  ${mmUpdateStatus.file_name} [${mmUpdateStatus.file_index}/${mmUpdateStatus.file_total}]`"
            striped striped-flow :duration="10" />
        </div>

        <div class="mh-table-wrap">
          <el-table :data="mmStatus.files" size="small" style="width:100%;margin-top:10px">
            <el-table-column prop="name" label="文件" width="140" />
            <el-table-column prop="desc" label="说明" min-width="80" />
            <el-table-column label="状态" width="64">
              <template #default="scope">
                <el-tag :type="scope.row.exists ? 'success' : 'danger'" size="small">{{ scope.row.exists ? '存在' : '缺失' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="76">
              <template #default="scope">{{ scope.row.exists ? formatMmSize(scope.row.size) : '—' }}</template>
            </el-table-column>
            <el-table-column label="修改时间" width="150" class-name="mh-col-time">
              <template #default="scope">
                <span style="font-size:12px">{{ scope.row.exists ? scope.row.mod_time : '—' }}</span>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- ── Tab 3: 配置文件 ── -->
      <el-tab-pane label="配置文件" name="config">
        <div class="mh-config-toolbar">
          <span class="mh-meta">{{ mmStatus.mihomo_dir }}/config.yaml</span>
          <div style="display:flex;gap:6px">
            <el-button size="small" :icon="RefreshIcon" @click="loadMmConfig" :loading="mmConfigLoading">重新加载</el-button>
            <el-button size="small" @click="checkMmConfig" :loading="mmConfigChecking" title="调用 mihomo -t 校验磁盘上的 config.yaml（不会包含未保存的改动）">测试配置</el-button>
            <el-button size="small" type="primary" @click="saveMmConfig" :loading="mmConfigSaving">保存</el-button>
          </div>
        </div>
        <div v-if="mmConfigError" class="mh-config-error">{{ mmConfigError }}</div>
        <pre v-if="mmConfigCheckOutput" class="mh-output">{{ mmConfigCheckOutput }}</pre>
        <textarea
          v-model="mmConfigText"
          class="mh-config-editor"
          spellcheck="false"
          placeholder="配置文件内容将在此显示..." />
      </el-tab-pane>

      <!-- ── Tab 4: 安装管理 ── -->
      <el-tab-pane label="安装管理" name="install">
        <!-- 版本信息 -->
        <div class="mh-install-version-card">
          <div class="mh-info-grid">
            <div class="mh-info-item">
              <span class="mh-info-label">已安装版本</span>
              <span class="mh-info-value">{{ mmStatus.binary_version || '未知' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">远端最新版本</span>
              <span class="mh-info-value">{{ mmBinaryVersionInfo.remote_version || '—' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">安装目录</span>
              <span class="mh-info-value">/data/kano_plugins/mihomo</span>
            </div>
            <div class="mh-info-item">
              <el-tag v-if="mmBinaryVersionInfo.remote_version && !mmBinaryVersionInfo.installed" type="info" size="small">未安装</el-tag>
              <el-tag v-else-if="mmBinaryVersionInfo.has_update" type="warning" size="small">有新版本</el-tag>
              <el-tag v-else-if="mmBinaryVersionInfo.remote_version && !mmBinaryVersionInfo.has_update" type="success" size="small">已是最新</el-tag>
            </div>
          </div>
          <div style="display:flex;gap:8px;margin-top:12px;flex-wrap:wrap">
            <el-button size="small" @click="checkMmBinaryVersion" :loading="mmBinaryChecking">检查版本</el-button>
            <el-button size="small" type="primary"
              :loading="mmInstallStatus.state==='downloading'"
              :disabled="mmInstallStatus.state==='downloading'"
              @click="startMmInstall">
              {{ mmStatus.binary_version ? '更新内核' : '安装 Mihomo' }}
            </el-button>
            <el-button v-if="mmInstallStatus.state==='downloading'" size="small" type="danger" @click="cancelMmInstall">取消</el-button>
          </div>
        </div>

        <!-- 安装进度 -->
        <div v-if="['downloading','done','failed','canceled'].includes(mmInstallStatus.state)" class="mh-progress-area" style="margin-top:12px">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px">
            <el-tag :type="mmInstallTagType" size="small">{{ mmInstallLabel }}</el-tag>
            <span style="font-size:12px;color:#606266">{{ mmInstallStatus.msg }}</span>
          </div>
          <el-progress
            v-if="mmInstallStatus.state==='downloading'"
            :percentage="mmInstallStatus.percent"
            striped striped-flow :duration="10" />
        </div>

        <!-- 卸载区 -->
        <el-divider style="margin:16px 0 10px">卸载</el-divider>
        <div style="display:flex;gap:8px;flex-wrap:wrap">
          <el-button size="small" type="warning" @click="uninstallMm('soft')" :loading="mmUninstalling==='soft'">
            仅删除内核
          </el-button>
          <el-button size="small" type="danger" @click="uninstallMm('full')" :loading="mmUninstalling==='full'">
            完全卸载
          </el-button>
        </div>
      </el-tab-pane>

    </el-tabs>

    <template #footer>
      <el-button @click="mmDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>

  <!-- ───────── 网络设置弹窗 ───────── -->
  <el-dialog
    v-model="networkSettingsDialogVisible"
    title="网络设置"
    width="min(760px, 96vw)"
    :close-on-click-modal="true"
    class="wireless-dialog">
    <div class="settings-panel">
      <section class="settings-section">
        <div class="settings-section-title">网络制式</div>
        <div class="settings-inline network-mode-row">
          <el-select
            v-model="networkForm.net_select"
            class="net-select settings-select"
            popper-class="net-select-popper"
            placeholder="未知">
            <el-option v-for="opt in netSelectOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
          <el-button class="network-mode-apply" type="primary" :loading="networkApplying === 'mode'" @click="applyNetworkMode">应用</el-button>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-title">4G 锁频段</div>
        <el-checkbox-group v-model="networkForm.lte_bands" class="band-checkbox-grid">
          <el-checkbox-button v-for="band in lteBandOptions" :key="band" :label="band">B{{ band }}</el-checkbox-button>
        </el-checkbox-group>
        <div class="settings-actions">
          <el-button size="small" @click="lockCurrentLTEBands">锁当前频段</el-button>
          <el-button size="small" @click="selectAllLTEBands">全选</el-button>
          <el-button size="small" @click="clearAllLTEBands">全不选</el-button>
          <el-button size="small" @click="selectAllLTEBands">自动</el-button>
          <el-button size="small" type="primary" :loading="networkApplying === 'lteBand'" @click="applyLTEBandLock">应用 4G 锁频</el-button>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-title">5G 锁频段</div>
        <el-checkbox-group v-model="networkForm.nr_bands" class="band-checkbox-grid">
          <el-checkbox-button v-for="band in nrBandOptions" :key="band" :label="band">N{{ band }}</el-checkbox-button>
        </el-checkbox-group>
        <div class="settings-actions">
          <el-button size="small" @click="lockCurrentNRBands">锁当前频段</el-button>
          <el-button size="small" @click="selectAllNRBands">全选</el-button>
          <el-button size="small" @click="clearAllNRBands">全不选</el-button>
          <el-button size="small" @click="selectAllNRBands">自动</el-button>
          <el-button size="small" type="primary" :loading="networkApplying === 'nrBand'" @click="applyNRBandLock">应用 5G 锁频</el-button>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-title">小区锁定</div>
        <div class="cell-lock-grid">
          <div class="cell-lock-block">
            <div class="settings-small-title">4G 小区</div>
            <el-input v-model="networkForm.lock_lte_pci" placeholder="PCI" />
            <el-input v-model="networkForm.lock_lte_earfcn" placeholder="EARFCN" />
            <div class="settings-actions">
              <el-button size="small" @click="fillCurrentLTECell">填入当前小区</el-button>
              <el-button size="small" @click="clearLTECell">解锁</el-button>
              <el-button size="small" type="primary" :loading="networkApplying === 'lteCell'" @click="applyLTECellLock">应用</el-button>
            </div>
          </div>
          <div class="cell-lock-block">
            <div class="settings-small-title">5G 小区</div>
            <el-input v-model="networkForm.lock_nr_pci" placeholder="PCI" />
            <el-input v-model="networkForm.lock_nr_earfcn" placeholder="ARFCN" />
            <el-input v-model="networkForm.lock_nr_band" placeholder="Band，例如 78" />
            <div class="settings-actions">
              <el-button size="small" @click="fillCurrentNRCell">填入当前小区</el-button>
              <el-button size="small" @click="clearNRCell">解锁</el-button>
              <el-button size="small" type="primary" :loading="networkApplying === 'nrCell'" @click="applyNRCellLock">应用</el-button>
            </div>
          </div>
        </div>
      </section>
    </div>
    <template #footer>
      <el-button @click="networkSettingsDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>

  <!-- ───────── WiFi 设置弹窗 ───────── -->
  <el-dialog
    v-model="wifiSettingsDialogVisible"
    title="WiFi 设置"
    width="min(720px, 96vw)"
    :close-on-click-modal="true"
    class="wireless-dialog">
    <div class="settings-panel">

      <div class="wifi-settings-grid">
        <section class="settings-section wifi-radio-card">
          <div>
            <div class="settings-section-title">2.4G WiFi</div>
            <div class="wifi-radio-subtitle">{{ wifiForm.wifi24_enabled ? '当前已开启' : '当前已关闭' }}，应用后立即生效</div>
          </div>
          <div class="wifi-card-control">
            <el-switch
              v-model="wifiForm.wifi24_enabled"
              active-text="开启"
              inactive-text="关闭" />
            <el-button size="small" type="primary" :loading="wifiSettingsSaving === 'radio24'" @click="applyWifi24State">应用</el-button>
          </div>
        </section>
        <section class="settings-section wifi-radio-card">
          <div>
            <div class="settings-section-title">5G WiFi</div>
            <div class="wifi-radio-subtitle">{{ wifiForm.wifi5_enabled ? '当前已开启' : '当前已关闭' }}，应用后立即生效</div>
          </div>
          <div class="wifi-card-control">
            <el-switch
              v-model="wifiForm.wifi5_enabled"
              active-text="开启"
              inactive-text="关闭" />
            <el-button size="small" type="primary" :loading="wifiSettingsSaving === 'radio5'" @click="applyWifi5State">应用</el-button>
          </div>
        </section>
      </div>
      <section class="settings-section">
        <div class="wifi-tuning-grid">
          <div class="wifi-tuning-item">
            <div class="settings-title-row">
              <span class="settings-section-title">性能模式</span>
              <el-tooltip
                content="高性能会关闭 WiFi 省电策略，提升无线响应和稳定性，但耗电和发热可能增加。"
                placement="top">
                <span class="settings-help-icon">!</span>
              </el-tooltip>
            </div>
            <el-switch
              v-model="wifiForm.high_performance"
              active-text="高性能"
              inactive-text="省电" />
            <div class="settings-actions wifi-setting-actions">
              <el-button size="small" type="primary" :loading="wifiSettingsSaving === 'psm'" @click="applyWifiPerformanceSetting">应用</el-button>
            </div>
          </div>
          <div class="wifi-tuning-item wifi-power-control">
            <div class="settings-title-row">
              <span class="settings-section-title">发射功率</span>
              <span class="wifi-value-pill">{{ wifiForm.txpower }}%</span>
            </div>
            <div class="wifi-distance-options">
              <button
                v-for="opt in wifiTxPowerOptions"
                :key="opt.value"
                class="wifi-distance-option"
                :class="{ active: wifiForm.txpower === opt.value }"
                type="button"
                @click="wifiForm.txpower = opt.value">
                <span>{{ opt.label }}</span>
                <small>{{ opt.value }}%</small>
              </button>
            </div>
            <div class="settings-actions wifi-setting-actions">
              <el-button size="small" type="primary" :loading="wifiSettingsSaving === 'txpower'" @click="applyWifiTxPowerSetting">应用</el-button>
            </div>
          </div>
          <div class="wifi-tuning-item">
            <div class="settings-title-row">
              <span class="settings-section-title">国家码</span>
              <el-tooltip
                content="国家码会同时应用到 2.4G 和 5G WiFi，重启后生效，并会使 2.4G 和 5G 都处于开启状态。它会影响可用信道和发射限制，通常中国大陆填写 CN。"
                placement="top">
                <span class="settings-help-icon">!</span>
              </el-tooltip>
            </div>
            <el-input v-model="wifiForm.country" maxlength="2" placeholder="CN" />
            <div class="settings-actions wifi-setting-actions">
              <el-button size="small" type="primary" :loading="wifiSettingsSaving === 'country'" @click="applyWifiCountrySetting">应用</el-button>
            </div>
          </div>
        </div>
      </section>
    </div>
    <template #footer>
      <el-button @click="wifiSettingsDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>

  <!-- ───────── 系统工具弹窗 ───────── -->
  <el-dialog
    v-model="systemToolsDialogVisible"
    title="系统工具"
    width="min(760px, 96vw)"
    :close-on-click-modal="!localSpeedTest.running"
    class="wireless-dialog">
    <el-tabs v-model="systemToolsActiveTab" class="system-tools-tabs">
      <el-tab-pane label="流量测速" name="speedtest">
        <div class="local-speedtest-panel">
      <div class="local-speedtest-header">
        <div>
          <div class="settings-section-title">测速地址
            <el-tooltip
                content="通过设备后端代理下载测速源，避免浏览器跨域限制。默认原神 PC 包下载源；自定义请填写 http/https 直链。"
                placement="top">
                <span class="settings-help-icon" size="small" style="margin-left: 4px;">!</span>
              </el-tooltip>
          </div>
          
        </div>
        <div class="local-speedtest-url-field">
          <el-input
            v-model="localSpeedTest.url"
            class="local-speedtest-url"
            :disabled="localSpeedTest.running"
            placeholder="https://..."
            clearable
            @blur="fillDefaultUrlIfEmpty" />
          
        </div>
      </div>

      <div class="local-speedtest-options">
        <div class="local-speedtest-option">
          <span>多线程</span>
          <el-input-number
            v-model="localSpeedTest.threads"
            :min="1"
            :max="8"
            :step="1"
            :disabled="localSpeedTest.running"
            controls-position="right" />
        </div>
        <div class="local-speedtest-option">
          <span>循环</span>
          <el-switch
            v-model="localSpeedTest.loop"
            :disabled="localSpeedTest.running"
            active-text="开"
            inactive-text="关" />
        </div>
      </div>

      <div class="local-speedtest-meter">
        <div class="local-speedtest-value">{{ localSpeedTest.currentSpeed }}</div>
        <div class="local-speedtest-label">当前速度</div>
        <el-progress
          :percentage="localSpeedTest.progress"
          :stroke-width="10"
          :show-text="false"
          striped
          :striped-flow="localSpeedTest.running" />
      </div>

      <div v-if="localSpeedTest.running || speedChart.hasData" class="local-speedtest-chart">
        <div class="local-speedtest-chart-head">
          <span>速度曲线</span>
          <span v-if="speedChart.max > 0" class="lst-peak">峰值 {{ speedChart.max.toFixed(1) }} Mbps</span>
        </div>
        <div class="local-speedtest-plot">
          <div class="lst-ylabels">
            <span v-for="g in speedChart.ygrid" :key="'yl-' + g.y" :style="{ top: g.y + 'px' }">{{ g.label }}</span>
          </div>
          <div class="lst-main">
            <svg class="local-speedtest-chart-svg" :viewBox="`0 0 ${SPEED_CHART_W} ${SPEED_CHART_H}`" preserveAspectRatio="none">
              <g class="lst-grid">
                <line v-for="g in speedChart.xgrid" :key="'vx-' + g.x" :x1="g.x" y1="0" :x2="g.x" :y2="SPEED_CHART_H" />
                <line v-for="g in speedChart.ygrid" :key="'hy-' + g.y" x1="0" :y1="g.y" :x2="SPEED_CHART_W" :y2="g.y" />
              </g>
              <polyline v-if="speedChart.area" :points="speedChart.area" class="lst-area" />
              <polyline :points="speedChart.points" class="lst-line" vector-effect="non-scaling-stroke" />
            </svg>
            <div class="lst-xlabels">
              <span v-for="g in speedChart.xgrid" :key="'xl-' + g.x" :style="{ left: g.xPct + '%' }">{{ g.label }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="local-speedtest-stats">
        <div>
          <span>已下载</span>
          <strong>{{ localSpeedTest.downloaded }}</strong>
        </div>
        <div>
          <span>平均速度</span>
          <strong>{{ localSpeedTest.avgSpeed }}</strong>
        </div>
        <div>
          <span>耗时</span>
          <strong>{{ localSpeedTest.elapsed }}</strong>
        </div>
        <div>
          <span>线程</span>
          <strong>{{ localSpeedTest.activeThreads || localSpeedTest.threads }}</strong>
        </div>
      </div>

      <div v-if="localSpeedTest.message" class="local-speedtest-message">{{ localSpeedTest.message }}</div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="短信转发" name="sms">
        <div class="system-tool-panel">
          <div class="settings-section-title">短信转发</div>
          <div class="sms-forward-grid">
            <section class="system-tool-section">
              <div class="system-tool-section-title">Bark</div>
              <el-switch v-model="smsForward.barkEnabled" active-text="启用" inactive-text="关闭" />
              <el-input v-model="smsForward.barkUrl" placeholder="https://api.day.app/你的Key?icon=https://..." clearable />
              <el-input v-model="smsForward.barkGroup" placeholder="分组 group，可选" clearable />
            </section>
            <section class="system-tool-section">
              <div class="system-tool-section-title">TG Bot</div>
              <el-switch v-model="smsForward.tgEnabled" active-text="启用" inactive-text="关闭" />
              <el-input v-model="smsForward.tgBotToken" placeholder="Bot Token" clearable show-password />
              <el-input v-model="smsForward.tgChatId" placeholder="Chat ID" clearable />
            </section>
          </div>
          <div class="system-tool-actions">
            <el-button size="small" :loading="smsForward.loading" @click="loadSmsMessages">刷新短信</el-button>
            <el-button size="small" :loading="smsForward.configSaving" @click="saveSmsForwardConfig">保存配置</el-button>
            <el-button size="small" type="primary" :loading="smsForward.forwarding" @click="forwardLatestSms">发送最新一条</el-button>
          </div>
          <div class="sms-forward-switches">
            <div class="local-speedtest-option">
              <span>后台监听</span>
              <el-switch
                :model-value="smsForward.running"
                :loading="smsForward.controlChanging"
                active-text="开"
                inactive-text="关"
                @change="(val: string | number | boolean) => setSmsForwardRunning(Boolean(val))" />
            </div>
            <div class="local-speedtest-option">
              <span>开机自启</span>
              <el-switch
                :model-value="smsForward.autostartEnabled"
                :loading="smsForward.autostartChanging"
                active-text="开"
                inactive-text="关"
                @change="(val: string | number | boolean) => setSmsForwardAutostart(Boolean(val))" />
            </div>
          </div>
          <div class="sms-forward-hint">后台每 {{ smsForward.pollInterval }} 秒轮询新短信；开机自启会在 webssh 启动时自动开启监听。</div>
          <div v-if="smsForward.status" class="local-speedtest-message">{{ smsForward.status }}</div>
          <div class="sms-message-list">
            <div v-for="msg in smsMessages" :key="msg.id" class="sms-message-item">
              <div class="sms-message-meta">
                <span>#{{ msg.id }}</span>
                <strong>{{ msg.number || '未知号码' }}</strong>
                <span>{{ msg.date }}</span>
              </div>
              <div class="sms-message-content">{{ msg.content }}</div>
            </div>
            <div v-if="!smsForward.loading && smsMessages.length === 0" class="system-tool-empty">暂无短信</div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="屏幕更新" name="devui">
        <div class="system-tool-panel">
          <div class="system-tool-header">
            <div>
              <div class="settings-section-title">屏幕更新</div>
              <div class="system-tool-hint">启动或开启自启前会自动下载 devui 补丁文件到 /data/kano_plugins/devui。</div>
            </div>
            <el-button size="small" :loading="devui.loading" @click="loadDevuiStatus">刷新</el-button>
          </div>
          <div class="sms-forward-switches">
            <div class="local-speedtest-option">
              <span>启动</span>
              <el-switch
                :model-value="devui.running"
                :loading="devui.controlChanging"
                active-text="开"
                inactive-text="关"
                @change="(val: string | number | boolean) => setDevuiRunning(Boolean(val))" />
            </div>
            <div class="local-speedtest-option">
              <span>开机自启</span>
              <el-switch
                :model-value="devui.autostartEnabled"
                :loading="devui.autostartChanging"
                active-text="开"
                inactive-text="关"
                @change="(val: string | number | boolean) => setDevuiAutostart(Boolean(val))" />
            </div>
          </div>
          <div class="devui-status-grid">
            <div class="devui-patch-status">
              <span>补丁文件</span>
              <strong>{{ devui.binaryExists ? '已下载' : '未下载' }}</strong>
              <el-button size="small" :loading="devui.downloading" @click="downloadDevuiBinary">
                {{ devui.binaryExists ? '重新下载' : '下载' }}
              </el-button>
              <div v-if="devui.downloadState === 'downloading' || devui.downloadState === 'done' || devui.downloadState === 'failed'" class="devui-download-progress">
                <el-progress
                  :percentage="devui.downloadPercent"
                  :status="devui.downloadState === 'failed' ? 'exception' : (devui.downloadState === 'done' ? 'success' : undefined)"
                  :stroke-width="8" />
                <div>{{ devui.downloadMsg || '正在下载...' }} · {{ formatSpeedTestBytes(devui.downloaded) }} / {{ devui.downloadTotal > 0 ? formatSpeedTestBytes(devui.downloadTotal) : '未知大小' }}</div>
              </div>
            </div>
            <div><span>挂载状态</span><strong>{{ devui.mounted ? '已挂载' : '未挂载' }}</strong></div>
            <div class="devui-data-status">
              <span>数据接口</span>
              <strong>{{ devui.dataReady ? '就绪' : '异常' }}</strong>
              <em v-if="!devui.dataReady && devui.dataError">{{ devui.dataError }}</em>
            </div>
          </div>
          <section class="system-tool-section">
            <div class="system-tool-section-title">锁屏壁纸替换</div>
            <div class="sms-forward-hint">最好上传 320x456 的 jpg/png，其他尺寸会自动缩放。</div>
            <div class="devui-wallpaper-row">
              <div class="devui-wallpaper-preview">
                <img v-if="devui.wallpaperPreview" :src="devui.wallpaperPreview" alt="壁纸预览" />
                <span v-else>预览</span>
              </div>
              <div class="devui-wallpaper-actions">
                <input
                  ref="devuiWallpaperInput"
                  class="devui-file-input"
                  type="file"
                  accept="image/png,image/jpeg"
                  @change="selectDevuiWallpaper" />
                <el-button size="small" @click="pickDevuiWallpaper">选择图片</el-button>
                <el-button
                  size="small"
                  type="primary"
                  :disabled="!devui.wallpaperFile"
                  :loading="devui.wallpaperUploading"
                  @click="uploadDevuiWallpaper">替换壁纸</el-button>
                <div v-if="devui.wallpaperName" class="devui-wallpaper-name">{{ devui.wallpaperName }}</div>
              </div>
            </div>
          </section>
          <div v-if="devui.status" class="local-speedtest-message">{{ devui.status }}</div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="rc.local" name="rcLocal">
        <div class="system-tool-panel">
          <div class="system-tool-header">
            <div>
              <div class="settings-section-title">/etc/rc.local</div>
              <div class="system-tool-hint">保存后会写入设备本机文件，并设置为可执行权限。</div>
            </div>
            <el-button size="small" :loading="rcLocal.loading" @click="loadRcLocal">刷新</el-button>
          </div>
          <el-input
            v-model="rcLocal.content"
            type="textarea"
            :autosize="{ minRows: 14, maxRows: 22 }"
            spellcheck="false"
            class="rc-local-editor"
            placeholder="#!/bin/sh" />
          <div v-if="rcLocal.status" class="local-speedtest-message">{{ rcLocal.status }}</div>
        </div>
      </el-tab-pane>
    </el-tabs>
    <template #footer>
      <el-button @click="systemToolsDialogVisible = false" :disabled="localSpeedTest.running">关闭</el-button>
      <template v-if="systemToolsActiveTab === 'speedtest'">
        <el-button v-if="localSpeedTest.running" type="danger" @click="() => stopLocalSpeedTest()">停止</el-button>
        <el-button v-else type="primary" @click="startLocalSpeedTest">开始测速</el-button>
      </template>
      <el-button v-else-if="systemToolsActiveTab === 'rcLocal'" type="primary" :loading="rcLocal.saving" @click="saveRcLocal">保存</el-button>
    </template>
  </el-dialog>

  <!-- ───────── 已连接设备弹窗（无线 + 有线） ───────── -->
  <el-dialog
    v-model="deviceDialogVisible"
    title="已连接设备"
    width="min(680px, 96vw)"
    :close-on-click-modal="true"
    destroy-on-close
    class="wireless-dialog">

    <div v-if="deviceListLoading" style="text-align:center;padding:32px 0;">
      <div class="loading-spinner" style="width:32px;height:32px;border-width:3px;margin:0 auto 12px;"></div>
      <p style="color:rgba(255,255,255,0.7);margin:0">正在获取设备列表...</p>
    </div>

    <div v-else-if="wirelessDeviceList.length === 0 && wiredDeviceList.length === 0"
         style="text-align:center;padding:32px 0;color:rgba(255,255,255,0.6);font-size:14px;">
      暂无连接设备
    </div>

    <template v-else>
      <!-- 无线段 -->
      <div v-if="wirelessDeviceList.length > 0" class="device-section">
        <h4 class="device-section-title">
          <span>无线</span>
          <span class="device-section-count">{{ wirelessDeviceList.length }}</span>
        </h4>
        <div class="wireless-device-list">
          <div v-for="device in wirelessDeviceList" :key="'w-' + device.mac_address" class="wireless-device-item">
            <div class="wireless-device-header">
              <span class="wireless-device-name">{{ device.hostname || '未知设备' }}</span>
              <span class="wireless-device-type-tag" :class="getDeviceTagClass(device.interface_type)">
                {{ device.interface_type }}
              </span>
            </div>
            <div class="wireless-device-info">
              <div class="wireless-info-row">
                <span class="wireless-info-label">IP 地址</span>
                <span class="wireless-info-value">{{ device.ip_address || '-' }}</span>
              </div>
              <div v-if="device.signal != null" class="wireless-info-row">
                <span class="wireless-info-label">信号</span>
                <span class="wireless-info-value" :class="signalClass(device.signal)">{{ device.signal }} dBm</span>
              </div>
              <div v-if="device.tx_rate" class="wireless-info-row">
                <span class="wireless-info-label">协商速率</span>
                <span class="wireless-info-value">{{ formatRate(device.tx_rate) }} Mbps</span>
              </div>
              <div class="wireless-info-row">
                <span class="wireless-info-label">连接时间</span>
                <span class="wireless-info-value">{{ device.access_time || '-' }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 有线段 -->
      <div v-if="wiredDeviceList.length > 0" class="device-section" :class="{ 'device-section-spaced': wirelessDeviceList.length > 0 }">
        <h4 class="device-section-title">
          <span>有线</span>
          <span class="device-section-count">{{ wiredDeviceList.length }}</span>
        </h4>
        <div class="wireless-device-list">
          <div v-for="device in wiredDeviceList" :key="'l-' + device.mac_address" class="wireless-device-item">
            <div class="wireless-device-header">
              <span class="wireless-device-name">{{ device.hostname || '未知设备' }}</span>
              <span class="wireless-device-type-tag" :class="getDeviceTagClass(device.interface_type)">
                {{ device.interface_type }}
              </span>
            </div>
            <div class="wireless-device-info">
              <div class="wireless-info-row">
                <span class="wireless-info-label">IP 地址</span>
                <span class="wireless-info-value">{{ device.ip_address || '-' }}</span>
              </div>
              <div class="wireless-info-row">
                <span class="wireless-info-label">连接时间</span>
                <span class="wireless-info-value">{{ device.access_time || '-' }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <el-button @click="deviceDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>

</template>

<script setup lang="ts">
import ChartIcon from '@/assets/svgs/chart.svg';
import DashboardIcon from '@/assets/svgs/dashboard.svg';
import InterfaceIcon from '@/assets/svgs/interface.svg';
import InternetIcon from '@/assets/svgs/internet.svg';
import NetworkIcon from '@/assets/svgs/network.svg';
import { Refresh as RefreshIcon } from '@element-plus/icons-vue';
import axios from 'axios';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue';

// interface UbusResponse<T = any> {
//   code: number;
//   msg: string;
//   result?: T;
// }

interface ZteRpcResponse {
  jsonrpc: string
  id: number
  result: [number, any]
}

interface NetInfoResult {
  [key: string]: any;
}

interface NetworkInterface {
  up: boolean;
  device?: string;
  proto?: string;
  uptime?: number;
  ipv4_address?: Array<{ address: string; mask: number }>;
  ipv6_address?: Array<{ address: string; mask: number }>;
  route?: Array<{ nexthop: string }>;
  'dns-server'?: string[];
}

interface TrafficData {
  real_tx_speed: number;
  real_rx_speed: number;
  real_tx_bytes: number;
  real_rx_bytes: number;
  real_max_tx_speed: number;
  real_max_rx_speed: number;
  total_tx_bytes: number;
  total_rx_bytes: number;
  day_tx_bytes: number;
  day_rx_bytes: number;
  month_tx_bytes: number;
  month_rx_bytes: number;
}

interface SystemInfo {
  localtime: number;
  uptime: number;
  load: number[];
  memory: {
    total: number;
    free: number;
    shared: number;
    buffered: number;
    available: number;
    cached: number;
  };
  root: {
    total: number;
    free: number;
    used: number;
    avail: number;
  };
  tmp: {
    total: number;
    free: number;
    used: number;
    avail: number;
  };
  swap: {
    total: number;
    free: number;
  };
}

interface DeviceInfo {
  hightemp_datalimit_status: string;
  quicken_power_on: string;
  bat_online: string;
  bat_health: string;
  bat_mode: string;
  bat_low_power: string;
  bat_percent: string;
  bat_level: string;
  bat_temperature: string;
  bat_charger_connect: string;
  bat_charger_type: string;
  bat_charger_status: string;
  bat_ui_charger_type: string;
  bat_temperature_level: string;
  external_charging_flag: string;
  bat_time_to_full: string;
  bat_time_to_empty: string;
  power_adapter: string;
  device_uptime: string;
  cpuinfo: {
    name: string;
    idle: string;
    gnice?: string;
  }[];
  meminfo: {
    total: string;
    free: string;
    avaliable: string;
  };
  flashinfo: {
    filesystem: string;
    size: string;
    used: string;
    avail: string;
    use: string;
    mounted_on: string;
  }[];
}

interface CpuTemp {
  cpuss_temp: number;
}

interface SimInfo {
  values: {
    digitalcode: string;
    imei: string;
    imei2: string;
    lock_status: string;
    modem_msn: string;
    msisdn: string;
    wlan_mac_address: string;
  };
}

interface SimInfo2 {
  sim_iccid: string;
  sim_imsi: string;
  msisdn: string;
  Operator: string;
}

interface WwanInfo {
  connect_fail_count: 0;
  connect_status: string;
  ipv4_address: string;
  ipv4_dev_name: string;
  ipv4_dns_prefer: string;
  ipv4_dns_standby: string;
  ipv4_gateway: string;
  ipv4_netmask: string;
  ipv6_address: string;
  ipv6_dev_name: string;
  ipv6_dns_prefer: string;
  ipv6_dns_standby: string;
  ipv6_gateway: string;
  roam_enable: number;
}

interface LanUserList {
  access_total_num: number;
  lan_num: number;
  wireless_num: number;
  offline_num: number;
  guest_num_24g: number;
  guest_num_5g: number;
  guest_num_6g: number;
}

// WiFi状态
interface WifiInfo {
  wlan0: string
  wlan1: string
  wlan2: string
  wlan3: string
  wifiStatus24: boolean
  wifiStatus5: boolean
  highPerformance: boolean
}
const wifiInfo = ref<WifiInfo>({} as WifiInfo);
const wifiModeText = computed(() => wifiInfo.value.highPerformance ? '性能模式' : '省电模式')
const wifiButtonText = computed(() => wifiInfo.value.highPerformance ? '点击切换省电' : '点击切换高性能')
const wifiPsmSaving = ref(false)

interface WifiStatus {
  dfs_status: string,
  lbd_enable: string,
  load_status: string,
  main2g_authmode: string,
  main2g_ssid: string,
  main5g_authmode: string,
  main5g_ssid: string,
  mesh_deployed: string,
  mesh_deploying_status: string,
  mesh_set_status: string,
  mlo_enable: string,
  radio2: string,
  radio2_disabled: string,
  radio5: string,
  radio5_disabled: string,
  wifi_onoff: string,
  wifi_start_mode: string,
}
const wifiStatus = ref<WifiStatus>({} as WifiStatus);

interface SysVersion {
  ".anonymous": boolean,
  ".type": string,
  ".name": string,
  manufacturer: string,
  hardware_version: string,
  wa_inner_version: string,
  model_name: string,
  integrate_version: string,
  device_alias_name: string,
  imei_sv: string,
  device_market_name: string,
}
const sysVersion = ref<SysVersion>({} as SysVersion);

// USB状态
interface USBStatus {
  connect: number,
  mode: string,
  typec_cc: string,
  usb2rj45: number,
}

const usbStatus = ref<USBStatus>({} as USBStatus);

// 连接状态
interface NetAmbr {
  raw: string;
  dl: {
    value:    number;
    unit:     string;
    unit_num: number;
    unit_raw: string;
  };
  ul: {
    value:    number;
    unit:     string;
    unit_num: number;
    unit_raw: string;
  };
  qci1: number;
  qci2: number;
}
const netAmbr = ref<NetAmbr>({
  raw: '',
  dl: {
    value: 0,
    unit: '',
    unit_num: 0,
    unit_raw: ''
  },
  ul: {
    value: 0,
    unit: '',
    unit_num: 0,
    unit_raw: ''
  },
  qci1: 0,
  qci2: 0
});



// 响应式数据
const loading = ref(false);
const error = ref<string | null>(null);
const data = ref<NetInfoResult | null>(null);
const connectionOk = ref(false);
const connectionLoaded = ref(false);
const lanData = ref<NetworkInterface>({} as NetworkInterface);
const wanData = ref<NetworkInterface>({} as NetworkInterface);
const wan6Data = ref<NetworkInterface>({} as NetworkInterface);
const trafficData = ref<TrafficData>({} as TrafficData);
const deviceInfo = ref<DeviceInfo>({} as DeviceInfo);
const cpuTemp = ref<CpuTemp>({} as CpuTemp);
const simInfo = ref<SimInfo>({} as SimInfo);
const simInfo2 = ref<SimInfo2>({} as SimInfo2);
const wwanInfo = ref<WwanInfo>({} as WwanInfo);
const lanUserList = ref<LanUserList>({} as LanUserList);
const networkType = computed(() => {
  const val = d.value?.network_type;
  if (
    val?.includes('NR') ||
    val?.includes('5G') ||
    val?.includes('SA') ||
    val?.includes('NSA') ||
    val?.includes('ENDC')
  )
    return '5G';
  if (val?.includes('4G') || val?.includes('LTE')) return '4G';
  if (val?.includes('HSPA')) return 'H+';
  if (val?.includes('3G')) return '3G';
  return '';
});
const currentActiveBands = computed(() => {
  const val = networkType.value != '5G' ? data.value?.lteca : data.value?.nrca;
  if (!val) return null;
  const list = (val as string).split(';').filter((el) => el);
  if (list.length == 0) return null;
  if (networkType.value != '5G' && list.length == 1) return null;

  const res = list
    .map((item) =>
      networkType.value != '5G'
        ? item[1]
          ? item.split(',')[1]
          : ''
        : item[3]
        ? 'N' + item.split(',')[3]
        : ''
    )
    .join(', ')
    .replace(/,/g, ',');
  return res;
});

function formatSpeedUnit(unit?: string) {
  if (!unit) return '';

  const u = unit.toLowerCase();

  if (u.includes('mbps')) return 'M';
  if (u.includes('kbps')) return 'K';
  if (u.includes('bps')) return 'B';

  return unit;
}

const is5GA = computed(() => {
  if (!currentActiveBands.value) return false;
  return currentActiveBands.value.split(',').length >= 2;
});

// 自动刷新控制
const autoRefresh = ref(true);
const refreshInterval = ref(1000);
const refreshInterval2 = ref(5000);
let refreshTimer: number | null = null;
let refreshTimer2: number | null = null;

// session 固定值（未登录）
const SESSION_ID = '00000000000000000000000000000000'

// 网络制式选择（5G/4G、5G SA、5G NSA、4G）
const netSelectOptions = [
  { value: 'WL_AND_5G',  label: '5G/4G/3G' },
  { value: 'Only_5G',    label: '5G SA' },
  { value: 'LTE_AND_5G', label: '5G NSA' },
  { value: 'WCDMA_AND_LTE', label: '4G/3G' },
  { value: 'Only_LTE',   label: '4G LTE' },
  { value: 'Only_WCDMA', label: '3G' },
]
const lteBandOptions = [1,2,3,4,5,7,8,18,19,20,26,28,29,32,34,38,39,40,41,42,43,48,66,71];
const nrBandOptions = [1,2,3,5,7,8,18,20,26,28,29,38,40,41,48,66,71,75,77,78,79];
const wifiTxPowerOptions = [
  { value: 40, label: '近距离' },
  { value: 80, label: '中距离' },
  { value: 100, label: '远距离' },
];

type NetworkApplyTarget = '' | 'mode' | 'lteBand' | 'nrBand' | 'lteCell' | 'nrCell';
type WifiApplyTarget = '' | 'radio24' | 'radio5' | 'psm' | 'txpower' | 'country' | 'settings' | 'all';

interface DeviceSettings {
  wifi24_enabled?: boolean;
  wifi5_enabled?: boolean;
  wifi_txpower: string;
  wifi24_txpower: string;
  wifi5_txpower: string;
  wifi_country: string;
  wifi24_country: string;
  wifi5_country: string;
  wifi_performance: string;
}

const networkSettingsDialogVisible = ref(false);
const wifiSettingsDialogVisible = ref(false);
const networkApplying = ref<NetworkApplyTarget>('');
const wifiSettingsSaving = ref<WifiApplyTarget>('');

const networkForm = reactive({
  net_select: '',
  lte_bands: [] as number[],
  nr_bands: [] as number[],
  lock_lte_pci: '',
  lock_lte_earfcn: '',
  lock_nr_pci: '',
  lock_nr_earfcn: '',
  lock_nr_band: '',
});

const wifiForm = reactive({
  high_performance: false,
  wifi24_enabled: false,
  wifi5_enabled: false,
  txpower: 100,
  country: '',
});

function normalizeWifiTxPower(value: unknown): number {
  const n = Number(value);
  return wifiTxPowerOptions.some(item => item.value === n) ? n : 100;
}

const networkSettingsSummary = computed(() => {
  const opt = netSelectOptions.find(item => item.value === (networkForm.net_select || d.value?.net_select));
  return opt?.label || '点击配置';
});

const wifiSettingsSummary = computed(() => {
  return wifiSettingsSaving.value ? '应用中...' : `${wifiInfo.value.wifiStatus24 ? '2.4G开' : '2.4G关'} / ${wifiInfo.value.wifiStatus5 ? '5G开' : '5G关'}`;
});

type SystemToolsTab = 'speedtest' | 'sms' | 'devui' | 'rcLocal';

interface SmsMessage {
  id: number;
  number: string;
  date: string;
  content: string;
  raw_hex: string;
  tag: string;
  mem_store: string;
}

const systemToolsDialogVisible = ref(false);
const systemToolsActiveTab = ref<SystemToolsTab>('speedtest');
let localSpeedTestWorkers: Worker[] = [];
// 当前速度按固定节拍采样的定时器（见 startLocalSpeedTest 里的 sampleTick）
let localSpeedTestSampleTimer: number | null = null;
function clearSpeedSampleTimer() {
  if (localSpeedTestSampleTimer != null) {
    clearInterval(localSpeedTestSampleTimer);
    localSpeedTestSampleTimer = null;
  }
}
// 默认测速源：原神 PC 包下载直链；由后端代理下载以绕开浏览器 CORS。
const TRAFFIC_SPEEDTEST_DEFAULT_URL = 'https://autopatchcn.yuanshen.com/client_app/download/pc_zip/20211117173857_8JkfDHNPmqKi67qR/YuanShen_2.3.0.zip';
const TRAFFIC_SPEEDTEST_URL_STORAGE_KEY = 'trafficSpeedTestUrl';
const TRAFFIC_SPEEDTEST_THREADS_STORAGE_KEY = 'trafficSpeedTestThreads';
const TRAFFIC_SPEEDTEST_LOOP_STORAGE_KEY = 'trafficSpeedTestLoop';
const localSpeedTest = reactive({
  running: false,
  url: localStorage.getItem(TRAFFIC_SPEEDTEST_URL_STORAGE_KEY) || TRAFFIC_SPEEDTEST_DEFAULT_URL,
  threads: normalizeSpeedTestThreads(localStorage.getItem(TRAFFIC_SPEEDTEST_THREADS_STORAGE_KEY)),
  loop: localStorage.getItem(TRAFFIC_SPEEDTEST_LOOP_STORAGE_KEY) === '1',
  activeThreads: 0,
  progress: 0,
  currentSpeed: '-- Mbps',
  avgSpeed: '-- Mbps',
  downloaded: '0 MB',
  elapsed: '0.00 秒',
  message: '',
});

const localSpeedTestSummary = computed(() => {
  return localSpeedTest.avgSpeed !== '-- Mbps' ? localSpeedTest.avgSpeed : '未配置';
});

const smsForward = reactive({
  barkEnabled: false,
  barkUrl: '',
  barkGroup: '',
  tgEnabled: false,
  tgBotToken: '',
  tgChatId: '',
  lastId: 0,
  running: false,
  autostartEnabled: false,
  pollInterval: 3,
  loading: false,
  forwarding: false,
  configSaving: false,
  controlChanging: false,
  autostartChanging: false,
  sentCount: 0,
  lastError: '',
  status: '',
});
const smsMessages = ref<SmsMessage[]>([]);

const devui = reactive({
  loading: false,
  running: false,
  autostartEnabled: false,
  binaryExists: false,
  mounted: false,
  dataReady: false,
  dataError: '',
  downloading: false,
  downloadState: 'idle',
  downloadMsg: '',
  downloaded: 0,
  downloadTotal: 0,
  downloadPercent: 0,
  controlChanging: false,
  autostartChanging: false,
  wallpaperUploading: false,
  wallpaperFile: null as File | null,
  wallpaperPreview: '',
  wallpaperName: '',
  status: '',
});
const devuiWallpaperInput = ref<HTMLInputElement | null>(null);
let devuiDownloadPollTimer: ReturnType<typeof setInterval> | null = null;
let devuiDownloadClearTimer: ReturnType<typeof setTimeout> | null = null;

const rcLocal = reactive({
  loading: false,
  saving: false,
  loaded: false,
  content: '',
  status: '',
});

const systemToolsSummary = computed(() => {
  if (localSpeedTest.running) return '测速中...';
  if (smsForward.running) return '短信监听中';
  return localSpeedTestSummary.value;
});

// 速度曲线采样点：t=首字节起的经过秒数（横轴），v=瞬时速度 Mbps（纵轴）
const speedCurve = ref<{ t: number; v: number }[]>([]);
const SPEED_CHART_W = 300;
const SPEED_CHART_H = 90;
const SPEED_CURVE_MAX_POINTS = 600;   // 循环测速时只保留最近这么多点（约 150s）
const speedChart = computed(() => {
  const data = speedCurve.value;
  const n = data.length;
  // ── 纵轴（速度）：漂亮刻度，随峰值变化 ──
  let max = 0;
  for (const p of data) if (p.v > max) max = p.v;
  const yStep = niceStep(max > 0 ? max : 1, 4);
  const niceMax = Math.max(yStep, Math.ceil((max > 0 ? max : 1) / yStep) * yStep);
  const ygrid: { y: number; label: string }[] = [];
  for (let v = 0; v <= niceMax + yStep * 1e-6; v += yStep) {
    ygrid.push({ y: +(SPEED_CHART_H - (v / niceMax) * SPEED_CHART_H).toFixed(2), label: fmtAxis(v) });
  }
  if (n === 0) return { points: '', area: '', max: 0, niceMax, ygrid, xgrid: [], hasData: false };

  // ── 横轴（时间）：用样本实际经过秒数，漂亮时间刻度随时长增长/滚动 ──
  const tMin = data[0].t;
  const tMax = data[n - 1].t;
  const span = tMax - tMin;
  const xOf = (t: number) => (span > 0 ? ((t - tMin) / span) * SPEED_CHART_W : 0);
  const xgrid: { x: number; xPct: number; label: string }[] = [];
  if (span > 0) {
    const xStep = niceStep(span, 6);
    for (let t = Math.ceil(tMin / xStep) * xStep; t <= tMax + xStep * 1e-6; t += xStep) {
      const x = xOf(t);
      xgrid.push({ x: +x.toFixed(2), xPct: +((x / SPEED_CHART_W) * 100).toFixed(2), label: fmtTime(t) });
    }
  }

  // ── 曲线 ──
  let points = '';
  for (let i = 0; i < n; i++) {
    points += (i ? ' ' : '') + xOf(data[i].t).toFixed(1) + ',' + (SPEED_CHART_H - (data[i].v / niceMax) * SPEED_CHART_H).toFixed(1);
  }
  const area = n > 1 ? `0,${SPEED_CHART_H} ${points} ${SPEED_CHART_W},${SPEED_CHART_H}` : '';
  return { points, area, max, niceMax, ygrid, xgrid, hasData: n > 1 };
});

// 坐标轴“漂亮”步进：把范围分成约 ticks 段，步进取 1/2/5×10^n
function niceStep(range: number, ticks: number): number {
  const raw = range / ticks;
  if (!Number.isFinite(raw) || raw <= 0) return 1;
  const base = Math.pow(10, Math.floor(Math.log10(raw)));
  const f = raw / base;
  const nf = f < 1.5 ? 1 : f < 3 ? 2 : f < 7 ? 5 : 10;
  return nf * base;
}
function fmtAxis(v: number): string {
  if (v <= 0) return '0';
  return Number.isInteger(v) ? String(v) : v.toFixed(1);
}
function fmtTime(v: number): string {
  return (Number.isInteger(v) ? String(v) : v.toFixed(1)) + 's';
}

function openSystemToolsDialog(tab: SystemToolsTab = 'speedtest') {
  systemToolsActiveTab.value = tab;
  systemToolsDialogVisible.value = true;
  if (tab === 'sms') {
    loadSmsForwardStatus();
  }
  if (tab === 'sms' && smsMessages.value.length === 0) {
    loadSmsMessages();
  }
  if (tab === 'devui') {
    loadDevuiStatus();
    loadDevuiDownloadStatus().then(() => {
      if (devui.downloadState === 'downloading') startDevuiDownloadPoll();
    });
  }
  if (tab === 'rcLocal' && !rcLocal.loaded) {
    loadRcLocal();
  }
}

async function loadSmsMessages() {
  smsForward.loading = true;
  try {
    const res = await axios.get('/api/system/sms');
    if (res.data.code !== 0) {
      ElMessage.error(res.data.msg || '读取短信失败');
      return;
    }
    smsMessages.value = res.data.data?.messages || [];
  } catch (e: any) {
    ElMessage.error('读取短信失败: ' + (e?.message ?? e));
  } finally {
    smsForward.loading = false;
  }
}

function validateSmsForwardTarget() {
  if (!smsForward.barkEnabled && !smsForward.tgEnabled) {
    ElMessage.warning('请至少启用 Bark 或 TG Bot');
    return false;
  }
  if (smsForward.barkEnabled && !String(smsForward.barkUrl || '').trim()) {
    ElMessage.warning('请填写 Bark 地址');
    return false;
  }
  if (smsForward.tgEnabled && (!String(smsForward.tgBotToken || '').trim() || !String(smsForward.tgChatId || '').trim())) {
    ElMessage.warning('请填写 TG Bot Token 和 Chat ID');
    return false;
  }
  return true;
}

async function loadSmsForwardStatus() {
  try {
    const res = await axios.get('/api/system/sms-forward/status');
    if (res.data.code !== 0) {
      smsForward.status = res.data.msg || '读取短信转发状态失败';
      return;
    }
    const data = res.data.data || {};
    const config = data.config || {};
    smsForward.barkEnabled = !!config.bark_enabled;
    smsForward.barkUrl = config.bark_url || '';
    smsForward.barkGroup = config.bark_group || '';
    smsForward.tgEnabled = !!config.tg_enabled;
    smsForward.tgBotToken = config.tg_bot_token || '';
    smsForward.tgChatId = config.tg_chat_id || '';
    smsForward.lastId = Number(data.last_id || config.last_id || 0);
    smsForward.running = !!data.running;
    smsForward.autostartEnabled = !!data.autostart_enabled;
    smsForward.pollInterval = Number(data.poll_interval || 3);
    smsForward.sentCount = Number(data.sent_count || 0);
    smsForward.lastError = data.last_error || '';
    smsForward.status = smsForward.lastError
      ? `后台监听异常：${smsForward.lastError}`
      : (smsForward.running ? `后台监听中，已推送 ${smsForward.sentCount} 次` : '后台监听未开启');
  } catch (e: any) {
    smsForward.status = '读取短信转发状态失败: ' + (e?.message ?? e);
  }
}

async function saveSmsForwardConfig(showMessage = true) {
  if (!validateSmsForwardTarget()) return false;
  smsForward.configSaving = true;
  try {
    const res = await axios.put('/api/system/sms-forward/config', {
      bark_enabled: smsForward.barkEnabled,
      bark_url: smsForward.barkUrl,
      bark_group: smsForward.barkGroup,
      tg_enabled: smsForward.tgEnabled,
      tg_bot_token: smsForward.tgBotToken,
      tg_chat_id: smsForward.tgChatId,
      last_id: smsForward.lastId,
    });
    if (res.data.code !== 0) {
      smsForward.status = res.data.msg || '保存短信转发配置失败';
      ElMessage.error(smsForward.status);
      return false;
    }
    if (showMessage) ElMessage.success('短信转发配置已保存');
    await loadSmsForwardStatus();
    return true;
  } catch (e: any) {
    smsForward.status = '保存短信转发配置失败: ' + (e?.message ?? e);
    ElMessage.error(smsForward.status);
    return false;
  } finally {
    smsForward.configSaving = false;
  }
}

async function forwardSms(onlyLatest: boolean) {
  if (!validateSmsForwardTarget()) return;
  smsForward.forwarding = true;
  smsForward.status = onlyLatest ? '正在发送最新短信...' : '正在检查新短信...';
  try {
    const res = await axios.post('/api/system/sms/forward', {
      bark_enabled: smsForward.barkEnabled,
      bark_url: smsForward.barkUrl,
      bark_group: smsForward.barkGroup,
      tg_enabled: smsForward.tgEnabled,
      tg_bot_token: smsForward.tgBotToken,
      tg_chat_id: smsForward.tgChatId,
      last_id: onlyLatest ? 0 : smsForward.lastId,
      only_latest: onlyLatest,
    });
    const data = res.data.data || {};
    if (Array.isArray(data.messages) && data.messages.length > 0) {
      smsMessages.value = data.messages.concat(smsMessages.value.filter(existing => !data.messages.some((m: SmsMessage) => m.id === existing.id)));
    }
    if (typeof data.latest_id === 'number') {
      smsForward.lastId = data.latest_id;
    }
    if (res.data.code !== 0) {
      smsForward.status = res.data.msg || '短信转发失败';
      ElMessage.error(smsForward.status);
      return;
    }
    smsForward.status = `已发送 ${data.sent || 0} 次推送`;
  } catch (e: any) {
    smsForward.status = '短信转发失败: ' + (e?.message ?? e);
    ElMessage.error(smsForward.status);
  } finally {
    smsForward.forwarding = false;
  }
}

function forwardLatestSms() {
  return forwardSms(true);
}

function applyDevuiStatus(data: any) {
  devui.running = !!data.running;
  devui.autostartEnabled = !!data.autostart_enabled;
  devui.binaryExists = !!data.binary_exists;
  devui.mounted = !!data.mounted;
  devui.dataReady = !!data.data_ready;
  devui.dataError = data.data_error || '';
  devui.status = data.last_error
    ? `屏幕更新异常：${data.last_error}`
    : (devui.running ? '屏幕更新已启动' : '屏幕更新未启动');
}

async function loadDevuiStatus() {
  devui.loading = true;
  try {
    const res = await axios.get('/api/system/devui/status');
    if (res.data.code !== 0) {
      devui.status = res.data.msg || '读取屏幕更新状态失败';
      ElMessage.error(devui.status);
      return;
    }
    applyDevuiStatus(res.data.data || {});
  } catch (e: any) {
    devui.status = '读取屏幕更新状态失败: ' + (e?.message ?? e);
    ElMessage.error(devui.status);
  } finally {
    devui.loading = false;
  }
}

function applyDevuiDownloadStatus(data: any) {
  devui.downloadState = data.state || 'idle';
  devui.downloadMsg = data.msg || '';
  devui.downloaded = Number(data.downloaded || 0);
  devui.downloadTotal = Number(data.total || 0);
  devui.downloadPercent = Number(data.percent || 0);
  devui.downloading = devui.downloadState === 'downloading';
}

function clearDevuiDownloadProgress(delayMs = 0) {
  if (devuiDownloadClearTimer) {
    clearTimeout(devuiDownloadClearTimer);
    devuiDownloadClearTimer = null;
  }
  const clear = () => applyDevuiDownloadStatus({ state: 'idle', msg: '', downloaded: 0, total: 0, percent: 0 });
  if (delayMs > 0) {
    devuiDownloadClearTimer = setTimeout(() => {
      clear();
      devuiDownloadClearTimer = null;
    }, delayMs);
  } else {
    clear();
  }
}

async function loadDevuiDownloadStatus() {
  try {
    const res = await axios.get('/api/system/devui/download/status');
    if (res.data.code === 0) {
      applyDevuiDownloadStatus(res.data.data || {});
      if (devui.downloadState === 'done') clearDevuiDownloadProgress(2000);
    }
  } catch {
    // ignore transient polling errors
  }
}

async function downloadDevuiBinary() {
  clearDevuiDownloadProgress();
  devui.downloading = true;
  try {
    const res = await axios.post('/api/system/devui/download');
    applyDevuiDownloadStatus(res.data.data || {});
    if (res.data.code !== 0) {
      ElMessage.error(res.data.msg || '下载 devui 补丁文件失败');
      return;
    }
    startDevuiDownloadPoll();
  } catch (e: any) {
    devui.status = '下载 devui 补丁文件失败: ' + (e?.message ?? e);
    ElMessage.error(devui.status);
    devui.downloading = false;
  }
}

function startDevuiDownloadPoll() {
  if (devuiDownloadPollTimer) return;
  devuiDownloadPollTimer = setInterval(async () => {
    await loadDevuiDownloadStatus();
    if (devui.downloadState === 'done') {
      stopDevuiDownloadPoll();
      devui.status = devui.downloadMsg || 'devui 补丁文件已下载';
      ElMessage.success(devui.status);
      await loadDevuiStatus();
      clearDevuiDownloadProgress(2000);
    } else if (devui.downloadState === 'failed') {
      stopDevuiDownloadPoll();
      devui.status = devui.downloadMsg || '下载 devui 补丁文件失败';
      ElMessage.error(devui.status);
      await loadDevuiStatus();
    }
  }, 1000);
}

function stopDevuiDownloadPoll() {
  if (devuiDownloadPollTimer) {
    clearInterval(devuiDownloadPollTimer);
    devuiDownloadPollTimer = null;
  }
}

async function setDevuiRunning(enabled: boolean) {
  devui.controlChanging = true;
  devui.running = enabled;
  devui.status = enabled ? '正在启动屏幕更新...' : '正在停止屏幕更新...';
  try {
    const res = await axios.post('/api/system/devui/control', { enabled });
    applyDevuiStatus(res.data.data || {});
    if (res.data.code !== 0) {
      ElMessage.error(res.data.msg || '设置屏幕更新失败');
      return;
    }
    ElMessage.success(enabled ? '屏幕更新已启动' : '屏幕更新已停止');
  } catch (e: any) {
    devui.running = !enabled;
    devui.status = '设置屏幕更新失败: ' + (e?.message ?? e);
    ElMessage.error(devui.status);
  } finally {
    devui.controlChanging = false;
    await loadDevuiStatus();
  }
}

async function setDevuiAutostart(enabled: boolean) {
  devui.autostartChanging = true;
  devui.autostartEnabled = enabled;
  devui.status = enabled ? '正在开启屏幕更新自启...' : '正在关闭屏幕更新自启...';
  try {
    const res = await axios.post('/api/system/devui/autostart', { enabled });
    applyDevuiStatus(res.data.data || {});
    if (res.data.code !== 0) {
      ElMessage.error(res.data.msg || '设置屏幕更新自启失败');
      return;
    }
    ElMessage.success(enabled ? '屏幕更新已设置开机自启' : '屏幕更新已关闭开机自启');
  } catch (e: any) {
    devui.autostartEnabled = !enabled;
    devui.status = '设置屏幕更新自启失败: ' + (e?.message ?? e);
    ElMessage.error(devui.status);
  } finally {
    devui.autostartChanging = false;
    await loadDevuiStatus();
  }
}

function pickDevuiWallpaper() {
  devuiWallpaperInput.value?.click();
}

function selectDevuiWallpaper(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0] || null;
  if (devui.wallpaperPreview) {
    URL.revokeObjectURL(devui.wallpaperPreview);
  }
  devui.wallpaperFile = file;
  devui.wallpaperName = file?.name || '';
  devui.wallpaperPreview = file ? URL.createObjectURL(file) : '';
}

async function uploadDevuiWallpaper() {
  if (!devui.wallpaperFile) {
    ElMessage.warning('请先选择壁纸图片');
    return;
  }
  if (devui.running || devui.mounted) {
    ElMessage.warning('请先停止屏幕更新后再替换壁纸');
    return;
  }
  devui.wallpaperUploading = true;
  devui.status = '正在替换锁屏壁纸...';
  try {
    const form = new FormData();
    form.append('file', devui.wallpaperFile);
    const res = await axios.post('/api/system/devui/wallpaper', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    applyDevuiStatus(res.data.data || {});
    if (res.data.code !== 0) {
      ElMessage.error(res.data.msg || '替换壁纸失败');
      return;
    }
    devui.status = res.data.msg || '壁纸已替换';
    ElMessage.success(devui.status);
  } catch (e: any) {
    devui.status = '替换壁纸失败: ' + (e?.message ?? e);
    ElMessage.error(devui.status);
  } finally {
    devui.wallpaperUploading = false;
    await loadDevuiStatus();
  }
}

async function setSmsForwardRunning(enabled: boolean) {
  smsForward.controlChanging = true;
  try {
    if (enabled) {
      const saved = await saveSmsForwardConfig(false);
      if (!saved) {
        smsForward.running = false;
        return;
      }
    }
    const res = await axios.post('/api/system/sms-forward/control', { action: enabled ? 'start' : 'stop' });
    if (res.data.code !== 0) {
      smsForward.running = !enabled;
      smsForward.status = res.data.msg || '设置后台监听失败';
      ElMessage.error(smsForward.status);
      return;
    }
    smsForward.running = enabled;
    ElMessage.success(enabled ? '已开启后台短信监听' : '已停止后台短信监听');
    await loadSmsForwardStatus();
  } catch (e: any) {
    smsForward.running = !enabled;
    smsForward.status = '设置后台监听失败: ' + (e?.message ?? e);
    ElMessage.error(smsForward.status);
  } finally {
    smsForward.controlChanging = false;
  }
}

async function setSmsForwardAutostart(enabled: boolean) {
  smsForward.autostartChanging = true;
  try {
    if (enabled) {
      const saved = await saveSmsForwardConfig(false);
      if (!saved) {
        smsForward.autostartEnabled = false;
        return;
      }
    }
    const res = await axios.post('/api/system/sms-forward/autostart', { enabled });
    if (res.data.code !== 0) {
      smsForward.autostartEnabled = !enabled;
      smsForward.status = res.data.msg || '设置开机自启失败';
      ElMessage.error(smsForward.status);
      return;
    }
    smsForward.autostartEnabled = enabled;
    ElMessage.success(enabled ? '已开启开机自启' : '已关闭开机自启');
    await loadSmsForwardStatus();
  } catch (e: any) {
    smsForward.autostartEnabled = !enabled;
    smsForward.status = '设置开机自启失败: ' + (e?.message ?? e);
    ElMessage.error(smsForward.status);
  } finally {
    smsForward.autostartChanging = false;
  }
}

async function loadRcLocal() {
  rcLocal.loading = true;
  rcLocal.status = '';
  try {
    const res = await axios.get('/api/system/rc-local');
    if (res.data.code !== 0) {
      rcLocal.status = res.data.msg || '读取 rc.local 失败';
      ElMessage.error(rcLocal.status);
      return;
    }
    rcLocal.content = res.data.data?.content || '';
    rcLocal.loaded = true;
  } catch (e: any) {
    rcLocal.status = '读取 rc.local 失败: ' + (e?.message ?? e);
    ElMessage.error(rcLocal.status);
  } finally {
    rcLocal.loading = false;
  }
}

async function saveRcLocal() {
  rcLocal.saving = true;
  rcLocal.status = '正在保存...';
  try {
    const res = await axios.put('/api/system/rc-local', { content: rcLocal.content });
    if (res.data.code !== 0) {
      rcLocal.status = res.data.msg || '保存失败';
      ElMessage.error(rcLocal.status);
      return;
    }
    rcLocal.status = '保存成功';
    ElMessage.success('rc.local 已保存');
  } catch (e: any) {
    rcLocal.status = '保存失败: ' + (e?.message ?? e);
    ElMessage.error(rcLocal.status);
  } finally {
    rcLocal.saving = false;
  }
}

function normalizeSpeedTestThreads(value: unknown) {
  const n = Number(value);
  if (!Number.isFinite(n)) return 5;
  return Math.max(1, Math.min(8, Math.round(n)));
}

function formatSpeedMbps(bytesPerSecond: number) {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) return '0.00 Mbps';
  return `${(bytesPerSecond * 8 / 1024 / 1024).toFixed(2)} Mbps`;
}

function formatSpeedTestBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 MB';
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}

function resetLocalSpeedTestResult() {
  localSpeedTest.progress = 0;
  localSpeedTest.currentSpeed = '-- Mbps';
  localSpeedTest.avgSpeed = '-- Mbps';
  localSpeedTest.downloaded = '0 MB';
  localSpeedTest.elapsed = '0.00 秒';
  localSpeedTest.activeThreads = 0;
  localSpeedTest.message = '';
  speedCurve.value = [];
}

function stopLocalSpeedTest(message: unknown = '测速已停止') {
  clearSpeedSampleTimer();
  if (!localSpeedTest.running) return;
  const finalMessage = typeof message === 'string' ? message : '测速已停止';
  localSpeedTest.message = '正在停止测速...';
  localSpeedTestWorkers.forEach(worker => {
    try {
      worker.postMessage({ type: 'stop' });
    } catch {
      // ignore
    }
    worker.terminate();
  });
  localSpeedTestWorkers = [];
  localSpeedTest.running = false;
  localSpeedTest.activeThreads = 0;
  localSpeedTest.message = finalMessage;
}

// 预热时长（毫秒）：忽略 TCP 慢启动爬坡阶段，平均速度只统计进入稳态后的数据，提升准确度。
const SPEEDTEST_GRACE_MS = 800;

// 测速链接为空时回填默认链接（输入框失焦、或开始测速时触发）
function fillDefaultUrlIfEmpty() {
  if (!String(localSpeedTest.url || '').trim()) {
    localSpeedTest.url = TRAFFIC_SPEEDTEST_DEFAULT_URL;
  }
}

async function startLocalSpeedTest() {
  if (localSpeedTest.running) return;
  let testUrl = String(localSpeedTest.url || '').trim();
  if (!testUrl) testUrl = TRAFFIC_SPEEDTEST_DEFAULT_URL;   // 链接为空则回退默认链接
  if (!/^https?:\/\//i.test(testUrl)) {
    ElMessage.error('请输入 http/https 测速地址');
    return;
  }
  localSpeedTest.url = testUrl;
  localSpeedTest.threads = normalizeSpeedTestThreads(localSpeedTest.threads);
  localStorage.setItem(TRAFFIC_SPEEDTEST_URL_STORAGE_KEY, testUrl);
  localStorage.setItem(TRAFFIC_SPEEDTEST_THREADS_STORAGE_KEY, String(localSpeedTest.threads));
  localStorage.setItem(TRAFFIC_SPEEDTEST_LOOP_STORAGE_KEY, localSpeedTest.loop ? '1' : '0');
  resetLocalSpeedTestResult();
  localSpeedTest.running = true;
  localSpeedTest.message = '正在准备测速...';

  const token = localStorage.getItem('token') || '';
  const streams = localSpeedTest.threads;

  let totalBytes = 0;
  let targetBytes = 0;
  let firstByteTime = 0;       // 首字节到达时间：从这里开始计时，排除连接建立耗时
  let measureStartTime = 0;    // 预热结束、计入平均速度的起点
  let measuredBaseBytes = 0;   // measureStartTime 时刻已下载的字节数
  let finishedWorkers = 0;
  let failed = false;

  // 「当前速度」改为：固定节拍采样 + 最近 1s 滑动平均，消除原来单个 200ms 瞬时窗口
  // 在重发空窗 / 主线程抖动时砸出的「闪现极低值」。采样在定时器里做，不再被 chunk 事件驱动。
  const SAMPLE_INTERVAL_MS = 250;   // 固定采样节拍
  const SPEED_WINDOW_MS = 1000;     // 当前速度 = 最近 1s 平均
  const speedSamples: { t: number; bytes: number }[] = [];

  const sampleTick = () => {
    if (firstByteTime === 0) return;            // 还没首字节，不显示
    const now = performance.now();
    speedSamples.push({ t: now, bytes: totalBytes });
    // 丢掉窗口外的旧样本，但保留一个跨过窗口起点的样本，使窗口稳定覆盖 ~1s
    while (speedSamples.length >= 2 && speedSamples[1].t <= now - SPEED_WINDOW_MS) {
      speedSamples.shift();
    }
    const oldest = speedSamples[0];
    const dt = (now - oldest.t) / 1000;
    const bps = dt > 0 ? (totalBytes - oldest.bytes) / dt : 0;
    localSpeedTest.currentSpeed = formatSpeedMbps(bps);

    // 速度曲线：记录这一拍的 经过秒数 + 瞬时速度（Mbps）
    speedCurve.value.push({ t: (now - firstByteTime) / 1000, v: bps * 8 / 1024 / 1024 });
    if (speedCurve.value.length > SPEED_CURVE_MAX_POINTS) speedCurve.value.shift();

    // 平均速度实时更新（与最终口径一致：预热结束后累计），随时间往稳态爬升
    if (measureStartTime > 0) {
      const avgSec = (now - measureStartTime) / 1000;
      if (avgSec > 0) localSpeedTest.avgSpeed = formatSpeedMbps((totalBytes - measuredBaseBytes) / avgSec);
    }

    localSpeedTest.downloaded = formatSpeedTestBytes(totalBytes);
    localSpeedTest.elapsed = `${((now - firstByteTime) / 1000).toFixed(2)} 秒`;
    localSpeedTest.progress = targetBytes > 0 ? Math.min(100, Math.round((totalBytes / targetBytes) * 100)) : 0;
  };

  const finishSpeedTest = (message = '测速完成') => {
    if (!localSpeedTest.running) return;
    clearSpeedSampleTimer();
    const endTime = performance.now();
    let avgBytes = totalBytes;
    let avgSeconds = Math.max((endTime - (firstByteTime || endTime)) / 1000, 0.001);
    if (measureStartTime > 0 && endTime - measureStartTime >= 300) {
      avgBytes = totalBytes - measuredBaseBytes;
      avgSeconds = (endTime - measureStartTime) / 1000;
    }
    const avg = avgBytes / avgSeconds;
    localSpeedTest.currentSpeed = formatSpeedMbps(avg);
    localSpeedTest.avgSpeed = formatSpeedMbps(avg);
    localSpeedTest.downloaded = formatSpeedTestBytes(totalBytes);
    localSpeedTest.elapsed = `${((endTime - (firstByteTime || endTime)) / 1000).toFixed(2)} 秒`;
    localSpeedTest.progress = targetBytes > 0 ? 100 : localSpeedTest.progress;
    localSpeedTest.message = message;
    localSpeedTest.running = false;
    localSpeedTest.activeThreads = 0;
    localSpeedTestWorkers.forEach(worker => worker.terminate());
    localSpeedTestWorkers = [];
  };

  const handleWorkerMessage = (event: MessageEvent<any>) => {
    const data = event.data || {};
    if (data.type === 'length') {
      const bytes = Number(data.bytes);
      if (Number.isFinite(bytes) && bytes > 0) targetBytes += bytes;
      return;
    }
    if (data.type === 'progress') {
      const bytes = Number(data.bytes);
      if (!Number.isFinite(bytes) || bytes <= 0) return;
      const now = performance.now();
      if (firstByteTime === 0) {
        firstByteTime = now;
        localSpeedTest.message = localSpeedTest.loop ? '循环测速中...' : '测速中...';
      }
      totalBytes += bytes;
      if (measureStartTime === 0 && now - firstByteTime >= SPEEDTEST_GRACE_MS) {
        measureStartTime = now;
        measuredBaseBytes = totalBytes;
      }
      // 速度/进度的 UI 更新交给 sampleTick 定时器，这里只累加字节
      return;
    }
    if (data.type === 'done' || data.type === 'stopped') {
      finishedWorkers += 1;
      localSpeedTest.activeThreads = Math.max(0, streams - finishedWorkers);
      if (!localSpeedTest.loop && finishedWorkers >= streams) finishSpeedTest('测速完成');
      return;
    }
    if (data.type === 'error') {
      failed = true;
      const msg = data.message || '测速失败';
      localSpeedTest.message = msg;
      ElMessage.error(msg);
      stopLocalSpeedTest(msg);
    }
  };

  clearSpeedSampleTimer();
  localSpeedTestSampleTimer = window.setInterval(sampleTick, SAMPLE_INTERVAL_MS);
  localSpeedTest.activeThreads = streams;
  localSpeedTestWorkers = Array.from({ length: streams }, (_, idx) => {
    const worker = new Worker(new URL('../workers/trafficSpeedtest.worker.ts', import.meta.url), { type: 'module' });
    worker.onmessage = handleWorkerMessage;
    worker.onerror = (event) => {
      if (failed) return;
      failed = true;
      const msg = event.message || '测速线程异常';
      localSpeedTest.message = msg;
      ElMessage.error(msg);
      stopLocalSpeedTest(msg);
    };
    worker.postMessage({
      type: 'start',
      id: idx,
      url: testUrl,
      token,
      loop: localSpeedTest.loop,
    });
    return worker;
  });
}
// 1.网络信息
const netInfoRequest = {
  jsonrpc: '2.0',
  id: 1,
  method: 'call',
  params: [
    SESSION_ID,
    'zte_nwinfo_api',
    'nwinfo_get_netinfo',
    {},
  ]
};
// 2.LAN 状态
const lanRequest = {
  jsonrpc: '2.0',
  id: 2,
  method: 'call',
  params: [
    SESSION_ID,
    'network.interface.lan',
    'status',
    {},
  ],
}
// 3.WAN IPv4
const wanRequest = {
  jsonrpc: '2.0',
  id: 3,
  method: 'call',
  params: [
    SESSION_ID,
    'network.interface.zte_wan',
    'status',
    {},
  ],
}
// 4.WAN IPv6
const wan6Request = {
  jsonrpc: '2.0',
  id: 4,
  method: 'call',
  params: [
    SESSION_ID,
    'network.interface.zte_wan6',
    'status',
    {},
  ],
}
// 5.流量统计
const trafficRequest = {
  jsonrpc: '2.0',
  id: 5,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_data',
    'get_wwandst',
    { source_module: 'web', cid: 1, type: 4 },
  ],
}
// 6.设备信息
const deviceInfoRequest = {
  jsonrpc: '2.0',
  id: 6,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_mc.device.manager',
    'get_device_info',
    {},
  ],
}
// 7.CPU 温度
const cpuTempRequest = {
  jsonrpc: '2.0',
  id: 7,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_bsp.thermal',
    'get_cpu_temp',
    {},
  ],
}
// 8.SIM 信息（uci）
const simInfoRequest = {
  jsonrpc: '2.0',
  id: 8,
  method: 'call',
  params: [
    SESSION_ID,
    'uci',
    'get',
    {
      config: 'zwrt_zte_mdm',
      section: 'device_info',
    },
  ],
}
// 9.SIM 信息 2
const simInfo2Request = {
  jsonrpc: '2.0',
  id: 9,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_zte_mdm.api',
    'get_sim_info',
    {},
  ],
}
// 10.WWAN 接口信息
const wwanRequest = {
  jsonrpc: '2.0',
  id: 10,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_data',
    'get_wwaniface',
    {
      source_module: 'web',
      cid: 1,
      connect_status: '',
    },
  ],
}
// 11.LAN 用户数
const lanUserListRequest = {
  jsonrpc: '2.0',
  id: 11,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_router.api',
    'router_get_user_list_num',
    {},
  ],
}

// wifi 状态
const wifiStatusRequest = {
  jsonrpc: '2.0',
  id: 14,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_wlan',
    'report',
    {},
  ],
}

// 系统版本信息
const sysVersionRequest = {
  jsonrpc: '2.0',
  id: 15,
  method: 'call',
  params: [
    SESSION_ID,
    'uci',
    'get',
    {
      config: 'zwrt_common_info',
      section: 'common_config',
    },
  ],
};

const usbStatusRequest = {
  jsonrpc: '2.0',
  id: 16,
  method: 'call',
  params: [
    SESSION_ID,
    'zwrt_bsp.usb',
    'list',
    {},
  ],
};

// 1.网络信息 => netInfoRequest
// 2.LAN 状态 => lanRequest
// 3.WAN IPv4 => wanRequest
// 4.WAN IPv6 => wan6Request
// 5.流量统计 => trafficRequest
// 6.设备信息 => deviceInfoRequest
// 7.CPU 温度 => cpuTempRequest
// 8.SIM 信息（uci） => simInfoRequest
// 9.SIM 信息 2 => simInfo2Request
// 10.WWAN 接口信息 => wwanRequest
// 11.LAN 用户数 => lanUserListRequest
const batchRequests = [
  netInfoRequest,
  lanRequest,
  wanRequest,
  wan6Request,
  trafficRequest,
  deviceInfoRequest,
  cpuTempRequest,
  simInfoRequest,
  simInfo2Request,
  wifiStatusRequest,
  sysVersionRequest,
  usbStatusRequest,
  wwanRequest,
  lanUserListRequest,
]


// 计算属性
const dataReady = computed(() => !!data.value);
const connectionStatusClass = computed(() => {
  if (connectionOk.value) return 'online';
  if (loading.value || !connectionLoaded.value) return 'pending';
  return 'offline';
});
const connectionStatusText = computed(() => {
  if (connectionOk.value) return '已连接';
  if (loading.value && !connectionLoaded.value) return '连接中';
  if (connectionLoaded.value) return '连接失败';
  return '未加载';
});

// WAN 拨号连接状态（wwanInfo.connect_status）→ 信号卡片上的 tag 文案与配色，
// 替换原先固定的“已激活”，真实反映 IPv4/IPv6 的连接情况。
const connectStatusTag = computed<{ text: string; className: string }>(() => {
  switch (wwanInfo.value.connect_status) {
    case 'ipv4_ipv6_connected': return { text: 'IPv4/IPv6', className: 'success' };
    case 'ipv4_connected':      return { text: 'IPv4',      className: 'success' };
    case 'ipv6_connected':      return { text: 'IPv6',      className: 'success' };
    case 'connecting':          return { text: '连接中',    className: 'warning' };
    case 'disconnected':        return { text: '未连接',    className: 'danger'  };
    default:                    return { text: '未知',      className: 'unknown' };
  }
});
const d = computed(() => data.value || {});

const signalBars = computed(() => {
  const bars = Number(d.value.signalbar || 0);
  if (Number.isNaN(bars)) return 0;
  return Math.max(0, Math.min(5, bars));
});

// 信号强度分级，用于给信号条整体着色（弱=红 / 中=橙 / 强=绿）
const signalLevelClass = computed(() => {
  const b = signalBars.value;
  if (b <= 0) return 'level-none';
  if (b <= 2) return 'level-weak';
  if (b === 3) return 'level-medium';
  return 'level-strong';
});

// 格式化函数
function formatDbm(v: unknown): string {
  const n = Number(v);
  if (Number.isNaN(n)) return '-';
  return `${n} dBm`;
}

function formatDb(v: unknown): string {
  const n = Number(v);
  if (Number.isNaN(n)) return '-';
  return `${n} dB`;
}

function formatSnr(v: unknown): string {
  const n = Number(v);
  if (Number.isNaN(n)) return '-';
  return `${n.toFixed(1)} dB`;
}

function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return '-';

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);

  const parts: string[] = [];
  if (days) parts.push(`${days}天`);
  parts.push(`${hours}小时`);
  parts.push(`${minutes}分`);
  // if (secs || parts.length === 0) parts.push(`${secs}秒`);
  return parts.join('');
}

function formatSpeed(bytesPerSecond: number): string {
  if (!bytesPerSecond) return '0 B/s';
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
  let size = bytesPerSecond;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatBytes(bytes: number): string {
  if (!bytes) return '-';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = bytes;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatNumber(num: number): string {
  if (!num) return '-';
  return num.toLocaleString();
}

// 系统信息格式化函数
function formatLoad(load: number): string {
  return (load / 1000).toFixed(2);
}

function formatMemory(KB: number): string {
  if (!KB) return '-';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = KB * 1024;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatMemoryPercent(used: number, total: number): number {
  if (!total) return 0;
  return Math.round((used / total) * 100);
}

function formatCpuTemp(temp: number): string {
  if (!temp) return '-';
  return `${temp}°C`;
}

// 信号强度百分比计算函数
function getRssiPercent(rssi: number): number {
  if (!rssi) return 0;
  // RSSI: -120dBm (0%) 到 -30dBm (100%)
  const min = -120;
  const max = -30;
  const percent = Math.max(
    0,
    Math.min(100, ((rssi - min) / (max - min)) * 100)
  );
  return Math.round(percent);
}

function getRsrpPercent(rsrp: number): number {
  if (!rsrp) return 0;
  // RSRP: -140dBm (0%) 到 -70dBm (100%)
  const min = -140;
  const max = -70;
  const percent = Math.max(
    0,
    Math.min(100, ((rsrp - min) / (max - min)) * 100)
  );
  return Math.round(percent);
}

function getRsrqPercent(rsrq: number): number {
  if (!rsrq) return 0;
  // RSRQ: -20dB (0%) 到 -3dB (100%)
  const min = -20;
  const max = -3;
  const percent = Math.max(
    0,
    Math.min(100, ((rsrq - min) / (max - min)) * 100)
  );
  return Math.round(percent);
}

function getSnrPercent(snr: number): number {
  if (!snr) return 0;
  // SNR: 0dB (0%) 到 30dB (100%)
  const min = 0;
  const max = 30;
  const percent = Math.max(0, Math.min(100, ((snr - min) / (max - min)) * 100));
  return Math.round(percent);
}

type SignalMetric = 'rsrp' | 'rsrq' | 'sinr' | 'rssi';
type SignalGrade = 'excellent' | 'good' | 'fair' | 'poor' | 'unknown';
type SignalType = 'nr' | 'lte';
type SignalHelpKey = `${SignalType}-${SignalMetric}`;

interface SignalStatus {
  text: string;
  className: SignalGrade;
  score: number;
}

interface SignalHelpRange {
  label: string;
  className: SignalGrade;
}

interface SignalHelp {
  title: string;
  description: string;
  ranges: SignalHelpRange[];
}

const signalStatusMap: Record<SignalGrade, SignalStatus> = {
  excellent: { text: '优秀', className: 'excellent', score: 4 },
  good: { text: '良好', className: 'good', score: 3 },
  fair: { text: '一般', className: 'fair', score: 2 },
  poor: { text: '较差', className: 'poor', score: 1 },
  unknown: { text: '未知', className: 'unknown', score: 0 },
};

const signalHelpMap: Record<SignalMetric, SignalHelp> = {
  rsrp: {
    title: 'RSRP：信号覆盖强度',
    description: '主要看基站信号到设备这里有多强。数值是负数，越接近 0 越好。',
    ranges: [
      { label: '≥ -85 dBm：优秀', className: 'excellent' },
      { label: '-95 到 -86 dBm：良好', className: 'good' },
      { label: '-105 到 -96 dBm：一般', className: 'fair' },
      { label: '< -105 dBm：较差', className: 'poor' },
    ],
  },
  rsrq: {
    title: 'RSRQ：信号质量',
    description: '主要看信号是否干净、是否拥挤。数值也是负数，越接近 0 越好。',
    ranges: [
      { label: '≥ -8 dB：优秀', className: 'excellent' },
      { label: '-11 到 -9 dB：良好', className: 'good' },
      { label: '-15 到 -12 dB：一般', className: 'fair' },
      { label: '< -15 dB：较差', className: 'poor' },
    ],
  },
  sinr: {
    title: 'SINR：信噪比',
    description: '主要看有用信号比干扰和噪声强多少。数值越大越好。',
    ranges: [
      { label: '≥ 20 dB：优秀', className: 'excellent' },
      { label: '13 到 19.9 dB：良好', className: 'good' },
      { label: '> 0 到 12.9 dB：一般', className: 'fair' },
      { label: '≤ 0 dB：较差', className: 'poor' },
    ],
  },
  rssi: {
    title: 'RSSI：接收总强度',
    description: '包含有用信号、干扰和噪声，只适合作辅助参考。数值越接近 0 越好。',
    ranges: [
      { label: '≥ -65 dBm：优秀', className: 'excellent' },
      { label: '-75 到 -66 dBm：良好', className: 'good' },
      { label: '-85 到 -76 dBm：一般', className: 'fair' },
      { label: '< -85 dBm：较差', className: 'poor' },
    ],
  },
};

const openedSignalHelp = ref<SignalHelpKey | null>(null);

function getSignalHelpKey(type: SignalType, metric: SignalMetric): SignalHelpKey {
  return `${type}-${metric}`;
}

function isSignalHelpOpen(type: SignalType, metric: SignalMetric): boolean {
  return openedSignalHelp.value === getSignalHelpKey(type, metric);
}

function toggleSignalHelp(type: SignalType, metric: SignalMetric) {
  const key = getSignalHelpKey(type, metric);
  openedSignalHelp.value = openedSignalHelp.value === key ? null : key;
}

async function netSelectChange(value: string) {
  networkForm.net_select = value;
  await applyNetworkMode();
}

async function openNetworkSettingsDialog() {
  await fetchAllData();
  syncNetworkFormFromCurrent();
  networkSettingsDialogVisible.value = true;
}

function openWifiSettingsDialog() {
  syncWifiFormFromCurrent();
  wifiSettingsDialogVisible.value = true;
  loadWifiSettings();
}

function syncNetworkFormFromCurrent() {
  networkForm.net_select = d.value?.net_select || '';
  networkForm.lte_bands = parseBandList(d.value?.lte_band, lteBandOptions);
  networkForm.nr_bands = parseBandList(currentNRBandLockValue(), nrBandOptions);
  syncLTECellLockFromCurrent();
  syncNRCellLockFromCurrent();
}

function syncWifiFormFromCurrent() {
  wifiForm.high_performance = !!wifiInfo.value.highPerformance;
  wifiForm.wifi24_enabled = !!wifiInfo.value.wifiStatus24;
  wifiForm.wifi5_enabled = !!wifiInfo.value.wifiStatus5;
}

async function applyNetworkMode() {
  if (!networkForm.net_select) return;
  networkApplying.value = 'mode';
  try {
    const res = await axios.post('/api/network/mode', { net_select: networkForm.net_select });
    if (res.data.code !== 0) throw new Error(res.data.msg || '模式切换失败');
    ElMessage.success('模式切换成功');
    setTimeout(fetchAllData, 3000);
  } catch (err: any) {
    console.error('模式切换失败', err);
    ElMessage.error(err.message || '模式切换失败');
  } finally {
    networkApplying.value = '';
  }
}

function parseBandsFromCarrierString(raw: unknown, bandIndex: number): number[] {
  if (!raw || typeof raw !== 'string') return [];
  const bands = raw
    .split(';')
    .map(item => Number(item.split(',')[bandIndex]))
    .filter(band => Number.isFinite(band));
  return [...new Set(bands)];
}

function parseBandList(raw: unknown, allowed: number[]): number[] {
  if (!raw || typeof raw !== 'string') return [];
  return [...new Set(raw
    .split(',')
    .map(item => Number(String(item).trim().replace(/^[bn]/i, '')))
    .filter(band => Number.isFinite(band) && allowed.includes(band)))];
}

function currentNRBandLockValue(): unknown {
  const type = String(d.value?.network_type || '').toUpperCase();
  if (type === 'NSA') return d.value?.nr5g_nsa_band_lock;
  return d.value?.nr5g_sa_band_lock || d.value?.nr5g_nsa_band_lock;
}

function getCurrentLTEBands(): number[] {
  return parseBandsFromCarrierString(d.value?.lteca, 1)
    .filter(band => lteBandOptions.includes(band));
}

function getCurrentNRBands(): number[] {
  const bands = parseBandsFromCarrierString(d.value?.nrca, 3);
  const primary = Number(String(d.value?.nr5g_action_band || '').replace(/^n/i, ''));
  if (Number.isFinite(primary)) bands.unshift(primary);
  return [...new Set(bands)].filter(band => nrBandOptions.includes(band));
}

function lockCurrentLTEBands() {
  const bands = getCurrentLTEBands();
  if (!bands.length) {
    ElMessage.warning('未读取到当前 4G 频段');
    return;
  }
  networkForm.lte_bands = bands;
}

function selectAllLTEBands() {
  networkForm.lte_bands = [...lteBandOptions];
}

function clearAllLTEBands() {
  networkForm.lte_bands = [];
}

function lockCurrentNRBands() {
  const bands = getCurrentNRBands();
  if (!bands.length) {
    ElMessage.warning('未读取到当前 5G 频段');
    return;
  }
  networkForm.nr_bands = bands;
}

function selectAllNRBands() {
  networkForm.nr_bands = [...nrBandOptions];
}

function clearAllNRBands() {
  networkForm.nr_bands = [];
}

async function applyLTEBandLock() {
  networkApplying.value = 'lteBand';
  try {
    const res = await axios.post('/api/network/band/lte', { bands: networkForm.lte_bands });
    if (res.data.code !== 0) throw new Error(res.data.msg || '4G 锁频失败');
    ElMessage.success(networkForm.lte_bands.length === lteBandOptions.length ? '4G 已切换为自动频段' : '4G 锁频已应用');
    fetchAllData();
  } catch (err: any) {
    ElMessage.error(err.message || '4G 锁频失败');
  } finally {
    networkApplying.value = '';
  }
}

async function applyNRBandLock() {
  networkApplying.value = 'nrBand';
  try {
    const res = await axios.post('/api/network/band/nr', { bands: networkForm.nr_bands });
    if (res.data.code !== 0) throw new Error(res.data.msg || '5G 锁频失败');
    ElMessage.success(networkForm.nr_bands.length === nrBandOptions.length ? '5G 已切换为自动频段' : '5G 锁频已应用');
    fetchAllData();
  } catch (err: any) {
    ElMessage.error(err.message || '5G 锁频失败');
  } finally {
    networkApplying.value = '';
  }
}

function fillCurrentLTECell(showMessage = true) {
  networkForm.lock_lte_pci = String(d.value?.lte_pci || formatNrca(d.value?.lteca, '', 0, 0) || '').replace('-', '');
  networkForm.lock_lte_earfcn = String(d.value?.lte_action_channel || formatNrca(d.value?.lteca, '', 0, 3) || '').replace('-', '');
  if (showMessage) ElMessage.success('已填入当前 4G 小区');
}

function syncLTECellLockFromCurrent() {
  const values = parseCellLockValue(d.value?.lock_lte_cell);
  networkForm.lock_lte_pci = values[0] || '';
  networkForm.lock_lte_earfcn = values[1] || '';
}

function fillCurrentNRCell(showMessage = true) {
  networkForm.lock_nr_pci = String(d.value?.nr5g_pci || formatNrca(d.value?.nrca, '', 0, 1) || '').replace('-', '');
  networkForm.lock_nr_earfcn = String(d.value?.nr5g_action_channel || formatNrca(d.value?.nrca, '', 0, 4) || '').replace('-', '');
  networkForm.lock_nr_band = String(d.value?.nr5g_action_band || formatNrca(d.value?.nrca, '', 0, 3) || '').replace(/^n/i, '').replace('-', '');
  if (showMessage) ElMessage.success('已填入当前 5G 小区');
}

function syncNRCellLockFromCurrent() {
  const values = parseCellLockValue(d.value?.lock_nr_cell);
  networkForm.lock_nr_pci = values[0] || '';
  networkForm.lock_nr_earfcn = values[1] || '';
  networkForm.lock_nr_band = values[2] || '';
}

function parseCellLockValue(raw: unknown): string[] {
  if (!raw || typeof raw !== 'string') return [];
  return raw.match(/\d+/g) || [];
}

function clearLTECell() {
  networkForm.lock_lte_pci = '0';
  networkForm.lock_lte_earfcn = '0';
}

function clearNRCell() {
  networkForm.lock_nr_pci = '0';
  networkForm.lock_nr_earfcn = '0';
  networkForm.lock_nr_band = '0';
}

async function applyLTECellLock() {
  networkApplying.value = 'lteCell';
  try {
    const res = await axios.post('/api/network/cell/lte', {
      pci: networkForm.lock_lte_pci,
      earfcn: networkForm.lock_lte_earfcn,
    });
    if (res.data.code !== 0) throw new Error(res.data.msg || '4G 小区锁定失败');
    ElMessage.success(networkForm.lock_lte_pci === '0' ? '4G 小区已解锁' : '4G 小区已锁定');
    fetchAllData();
  } catch (err: any) {
    ElMessage.error(err.message || '4G 小区锁定失败');
  } finally {
    networkApplying.value = '';
  }
}

async function applyNRCellLock() {
  networkApplying.value = 'nrCell';
  try {
    const res = await axios.post('/api/network/cell/nr', {
      pci: networkForm.lock_nr_pci,
      earfcn: networkForm.lock_nr_earfcn,
      band: networkForm.lock_nr_band,
    });
    if (res.data.code !== 0) throw new Error(res.data.msg || '5G 小区锁定失败');
    ElMessage.success(networkForm.lock_nr_pci === '0' ? '5G 小区已解锁' : '5G 小区已锁定');
    fetchAllData();
  } catch (err: any) {
    ElMessage.error(err.message || '5G 小区锁定失败');
  } finally {
    networkApplying.value = '';
  }
}

function valuesFromUci(payload: any): Record<string, any> {
  return payload?.values || payload?.data?.values || payload || {};
}

async function loadWifiSettings() {
  try {
    const res = await axios.get('/api/wifi/settings');
    if (res.data.code !== 0) return;
    const wifi0 = valuesFromUci(res.data.data?.wifi0);
    const wifi1 = valuesFromUci(res.data.data?.wifi1);
    wifiForm.txpower = normalizeWifiTxPower(wifi0.txpowerpercent || wifi1.txpowerpercent || wifiForm.txpower || 100);
    wifiForm.country = wifi0.country || wifi1.country || wifiForm.country;
  } catch {
    // U60Pro 外的开发环境可能没有 ubus，这里保持静默。
  }
}

async function applyWifiPerformance() {
  await psmSetHandler(wifiForm.high_performance, false);
}

function wifiAttrs(txpower: number, country: string) {
  const attrs: Record<string, string> = {};
  if (txpower) attrs.txpowerpercent = String(txpower);
  if (country) attrs.country = country;
  return attrs;
}

async function applyWifiSettings() {
  wifiSettingsSaving.value = 'settings';
  try {
    const res = await axios.post('/api/wifi/settings', {
      wifi0: wifiAttrs(wifiForm.txpower, wifiForm.country),
      wifi1: wifiAttrs(wifiForm.txpower, wifiForm.country),
    });
    if (res.data.code !== 0) throw new Error(res.data.msg || 'WiFi 参数应用失败');
    ElMessage.success('WiFi 参数已应用');
    loadWifiSettings();
  } catch (err: any) {
    ElMessage.error(err.message || 'WiFi 参数应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

async function setWifiPerformance(highPerformance: boolean) {
  const res = await axios.post('/api/wifi/psm/set', {
    ifaces: ['wlan0', 'wlan1', 'wlan2', 'wlan3'],
    mode: highPerformance ? 'off' : 'on',
  });
  if (res.data.code !== 0) throw new Error(res.data.msg || 'WiFi 性能模式应用失败');
}

async function setWifiRadioState(iface: string, up: boolean) {
  const res = await axios.post('/api/wifi/state/set', {
    ifaces: [iface],
    up,
  });
  if (res.data.code !== 0) throw new Error(res.data.msg || 'WiFi 开关应用失败');
}

async function setWifiUciSettings() {
  const res = await axios.post('/api/wifi/settings', {
    wifi0: wifiAttrs(wifiForm.txpower, wifiForm.country),
    wifi1: wifiAttrs(wifiForm.txpower, wifiForm.country),
  });
  if (res.data.code !== 0) throw new Error(res.data.msg || 'WiFi 参数应用失败');
}

async function setWifiTxPowerSettings() {
  const res = await axios.post('/api/wifi/settings', {
    wifi0: wifiAttrs(wifiForm.txpower, ''),
    wifi1: wifiAttrs(wifiForm.txpower, ''),
  });
  if (res.data.code !== 0) throw new Error(res.data.msg || '发射功率应用失败');
}

async function setWifiCountrySettings() {
  const country = String(wifiForm.country || '').trim().toUpperCase();
  if (!/^[A-Za-z]{2}$/.test(country)) throw new Error('国家码必须是 2 位字母');
  wifiForm.country = country;
  const res = await axios.post('/api/wifi/settings', {
    wifi0: wifiAttrs(0, country),
    wifi1: wifiAttrs(0, country),
  });
  if (res.data.code !== 0) throw new Error(res.data.msg || '国家码应用失败');
}

async function applyWifi24State() {
  wifiSettingsSaving.value = 'radio24';
  try {
    await setWifiRadioState('wlan0', wifiForm.wifi24_enabled);
    await persistDeviceSettings();
    setTimeout(psmGetHandler, 2500);
    ElMessage.success('2.4G WiFi 设置已立即生效');
  } catch (err: any) {
    ElMessage.error(err.message || '2.4G WiFi 设置应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

async function applyWifi5State() {
  wifiSettingsSaving.value = 'radio5';
  try {
    await setWifiRadioState('wlan2', wifiForm.wifi5_enabled);
    await persistDeviceSettings();
    setTimeout(psmGetHandler, 2500);
    ElMessage.success('5G WiFi 设置已立即生效');
  } catch (err: any) {
    ElMessage.error(err.message || '5G WiFi 设置应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

async function applyWifiPerformanceSetting() {
  wifiSettingsSaving.value = 'psm';
  try {
    await setWifiPerformance(wifiForm.high_performance);
    await persistDeviceSettings();
    psmGetHandler();
    ElMessage.success('WiFi 性能模式已应用');
  } catch (err: any) {
    ElMessage.error(err.message || 'WiFi 性能模式应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

async function applyWifiTxPowerSetting() {
  wifiSettingsSaving.value = 'txpower';
  try {
    await setWifiTxPowerSettings();
    await persistDeviceSettings();
    loadWifiSettings();
    ElMessage.success('WiFi 发射功率已应用');
  } catch (err: any) {
    ElMessage.error(err.message || 'WiFi 发射功率应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

async function applyWifiCountrySetting() {
  wifiSettingsSaving.value = 'country';
  try {
    await setWifiCountrySettings();
    wifiForm.wifi24_enabled = true;
    wifiForm.wifi5_enabled = true;
    await persistDeviceSettings();
    setTimeout(() => {
      psmGetHandler();
      loadWifiSettings();
    }, 3000);
    ElMessage.success('国家码已应用，2.4G 和 5G 会自动开启');
  } catch (err: any) {
    ElMessage.error(err.message || '国家码应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

async function persistDeviceSettings() {
  const res = await axios.put('/api/device/settings', buildDeviceSettings());
  if (res.data.code !== 0) throw new Error(res.data.msg || '保存用户设置失败');
}

async function saveAndApplyWifiSettings() {
  wifiSettingsSaving.value = 'all';
  try {
    await setWifiPerformance(wifiForm.high_performance);
    await setWifiRadioState('wlan0', wifiForm.wifi24_enabled);
    await setWifiRadioState('wlan2', wifiForm.wifi5_enabled);
    await setWifiUciSettings();
    await persistDeviceSettings();
    psmGetHandler();
    loadWifiSettings();
    ElMessage.success('WiFi 设置已保存并应用');
  } catch (err: any) {
    ElMessage.error(err.message || 'WiFi 设置应用失败');
  } finally {
    wifiSettingsSaving.value = '';
  }
}

function buildDeviceSettings(): DeviceSettings {
  return {
    wifi24_enabled: wifiForm.wifi24_enabled,
    wifi5_enabled: wifiForm.wifi5_enabled,
    wifi_txpower: String(wifiForm.txpower),
    wifi24_txpower: String(wifiForm.txpower),
    wifi5_txpower: String(wifiForm.txpower),
    wifi_country: wifiForm.country,
    wifi24_country: wifiForm.country,
    wifi5_country: wifiForm.country,
    wifi_performance: wifiForm.high_performance ? 'high' : 'power_save',
  };
}

async function loadDeviceSettings() {
  try {
    const res = await axios.get('/api/device/settings');
    if (res.data.code !== 0 || !res.data.data) return;
    const saved = res.data.data as Partial<DeviceSettings>;
    wifiForm.wifi24_enabled = saved.wifi24_enabled ?? wifiForm.wifi24_enabled;
    wifiForm.wifi5_enabled = saved.wifi5_enabled ?? wifiForm.wifi5_enabled;
    wifiForm.txpower = normalizeWifiTxPower(saved.wifi_txpower || saved.wifi24_txpower || saved.wifi5_txpower || wifiForm.txpower || 100);
    wifiForm.country = saved.wifi_country || saved.wifi24_country || saved.wifi5_country || '';
    wifiForm.high_performance = saved.wifi_performance === 'high';
  } catch {
    // ignore
  }
}

function getSignalStatus(metric: SignalMetric, rawValue: unknown): SignalStatus {
  if (rawValue === null || rawValue === undefined || rawValue === '') {
    return signalStatusMap.unknown;
  }
  const value = Number(rawValue);
  if (!Number.isFinite(value)) return signalStatusMap.unknown;

  if (metric === 'rsrp') {
    if (value === 0) return signalStatusMap.unknown;
    if (value >= -85) return signalStatusMap.excellent;
    if (value >= -95) return signalStatusMap.good;
    if (value >= -105) return signalStatusMap.fair;
    return signalStatusMap.poor;
  }

  if (metric === 'rsrq') {
    if (value === 0) return signalStatusMap.unknown;
    if (value >= -8) return signalStatusMap.excellent;
    if (value >= -11) return signalStatusMap.good;
    if (value >= -15) return signalStatusMap.fair;
    return signalStatusMap.poor;
  }

  if (metric === 'sinr') {
    if (value >= 20) return signalStatusMap.excellent;
    if (value >= 13) return signalStatusMap.good;
    if (value > 0) return signalStatusMap.fair;
    return signalStatusMap.poor;
  }

  if (value === 0) return signalStatusMap.unknown;
  if (value >= -65) return signalStatusMap.excellent;
  if (value >= -75) return signalStatusMap.good;
  if (value >= -85) return signalStatusMap.fair;
  return signalStatusMap.poor;
}

function hasUsableSignalValue(rawValue: unknown): boolean {
  if (rawValue === null || rawValue === undefined || rawValue === '') {
    return false;
  }
  const value = Number(rawValue);
  return Number.isFinite(value) && value !== 0;
}

function isSignalActive(type: 'nr' | 'lte'): boolean {
  if (type === 'nr') {
    return networkType.value === '5G' && hasUsableSignalValue(d.value.nr5g_rsrp);
  }
  return networkType.value === '4G' && hasUsableSignalValue(d.value.lte_rsrp);
}

function getSignalDisplayStatus(
  type: 'nr' | 'lte',
  metric: SignalMetric,
  rawValue: unknown
): SignalStatus {
  if (!isSignalActive(type)) return signalStatusMap.unknown;
  return getSignalStatus(metric, rawValue);
}

function getSignalValueClass(metric: SignalMetric, rawValue: unknown): string {
  return `signal-value ${getSignalStatus(metric, rawValue).className}`;
}

function getAverageSignalStatus(statuses: SignalStatus[]): SignalStatus {
  const validStatuses = statuses.filter(item => item.className !== 'unknown');
  if (!validStatuses.length) return signalStatusMap.unknown;

  const averageScore =
    validStatuses.reduce((total, item) => total + item.score, 0) /
    validStatuses.length;

  let status: SignalStatus;
  if (averageScore >= 3.5) {
    status = signalStatusMap.excellent;
  } else if (averageScore >= 2.5) {
    status = signalStatusMap.good;
  } else if (averageScore >= 1.5) {
    status = signalStatusMap.fair;
  } else {
    status = signalStatusMap.poor;
  }

  const weakestScore = Math.min(...validStatuses.map(item => item.score));
  if (weakestScore <= 1 && status.score > signalStatusMap.fair.score) {
    return signalStatusMap.fair;
  }
  if (weakestScore <= 2 && status.score > signalStatusMap.good.score) {
    return signalStatusMap.good;
  }
  return status;
}

function getNetworkSignalStatus(type: 'nr' | 'lte'): SignalStatus {
  if (!isSignalActive(type)) return signalStatusMap.unknown;

  if (type === 'nr') {
    return getAverageSignalStatus([
      getSignalStatus('rsrp', d.value.nr5g_rsrp),
      getSignalStatus('rsrq', d.value.nr5g_rsrq),
      getSignalStatus('sinr', d.value.nr5g_snr),
      getSignalStatus('rssi', d.value.nr5g_rssi),
    ]);
  }

  return getAverageSignalStatus([
    getSignalStatus('rsrp', d.value.lte_rsrp),
    getSignalStatus('rsrq', d.value.lte_rsrq),
    getSignalStatus('sinr', d.value.lte_snr),
    getSignalStatus('rssi', d.value.lte_rssi),
  ]);
}

// 获取载波信息
function formatNrca(nrca: string, pre: string, type: number, index: number): string {
  if (!nrca) return '-';
  const carriers = nrca.split(';').filter(item => item.trim() !== '');
  const carrier = carriers[type];
  if (!carrier) return '-';
  const fields = carrier.split(',');
  // index 越界
  if (index < 0 || index >= fields.length) return '-';
  const value = fields[index];
  // 参数为空
  if (!value || value.trim() === '') return '-';
  const num = Number(value);
  if (Number.isNaN(num)) return '-';
  return pre + String(num);
}

// 获取网络运营商
const netWorkProvider = computed(() => {
  let provider = data.value?.network_provider;
  const fullname = data.value?.network_provider_fullname;
  const Operator = simInfo2?.value?.Operator;

  if (!provider && !fullname) return '-';
  // 中国联通 特殊处理
  if (provider === 'UNICOM') provider = 'CUCC';
  const providerMap: Record<string, string> = {
    CMCC: '中国移动',
    CUCC: '中国联通',
    UNICOM: '中国联通',
    CT: '中国电信',
    CBN: '中国广电',
  };
  // 优先走 code 映射，映射不到再走后端全名
  return providerMap[provider] ?? fullname ?? '-';
});

async function callUbusBatch(
    requests: any[]
): Promise<Record<number, any>> {
  const resp = await axios.post<ZteRpcResponse[]>(
      '/api/ubus', requests
  )
  const map: Record<number, any> = {}
  for (const item of resp.data) {
    const [code, data] = item.result
    if (code === 0) {
      map[item.id] = data
    } else {
      console.error(`ubus call failed, id=${item.id}`, data)
    }
  }
  return map
}

async function fetchAllData() {
  loading.value = true
  error.value = null
  try {
    const resultMap = await callUbusBatch(batchRequests)
    if (!resultMap[1]) {
      throw new Error('关键状态数据缺失')
    }
    // 按 id 取值（清晰又稳定）
    data.value        = resultMap[1]
    if (!networkForm.net_select && resultMap[1]?.net_select) {
      networkForm.net_select = resultMap[1].net_select
    }
    lanData.value     = resultMap[2]
    wanData.value     = resultMap[3]
    wan6Data.value    = resultMap[4]
    trafficData.value = resultMap[5]
    deviceInfo.value  = resultMap[6]
    cpuTemp.value     = resultMap[7]
    simInfo.value     = resultMap[8]
    simInfo2.value    = resultMap[9]
    wwanInfo.value    = resultMap[10]
    lanUserList.value = resultMap[11]
    wifiStatus.value = resultMap[14]
    sysVersion.value = resultMap[15]?.values ?? sysVersion.value
    usbStatus.value = resultMap[16] ?? usbStatus.value
    connectionOk.value = true
  } catch (e: any) {
    connectionOk.value = false
    error.value = e?.message || '请求失败'
    console.error('数据获取失败:', e)
  } finally {
    connectionLoaded.value = true
    loading.value = false
  }
}

function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value;

  if (autoRefresh.value) {
    startAutoRefresh();
    ElMessage.success('已恢复刷新');
  } else {
    stopAutoRefresh();
    ElMessage.warning('已停止刷新');
  }
}

function startAutoRefresh() {
  stopAutoRefresh();
  refreshTimer = window.setInterval(() => {
    fetchAllData();
  }, refreshInterval.value);
}

function stopAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
  if (refreshTimer2) {
    clearInterval(refreshTimer2);
    refreshTimer2 = null;
  }
}

function handleOpenAdbClick() {
  if (usbStatus?.value.connect != 1) {
    ElMessage.warning('请连接数据线后再试')
    return
  }
  oneClickDebug()
}

function oneClickDebug() {
  ElMessage.info('正在执行操作，请稍候...')

  axios.post('/api/openadb')
    .then(res => {
      // 可以根据后端返回判断是否成功
      const success = res?.data?.code === 0 || res?.data?.success
      setTimeout(() => {
        if (success) {
          ElMessage.success('ADB 调试模式已开启')
        } else {
          ElMessage.error('ADB 开启失败，请重试')
        }
      }, 1500)
    })
    .catch(err => {
      ElMessage.error('请求失败：' + (err?.message || '未知错误'))
    })
}

// ─────────────────────────── Mihomo ───────────────────────────

interface MmFileInfo { name: string; desc: string; exists: boolean; size: number; mod_time: string }
interface MmStatusData {
  running: boolean; pid: number; mihomo_dir: string; local_version: string; files: MmFileInfo[]
  binary_version: string; start_time: string; api_reachable: boolean; api_version: string; external_controller: string
  autostart_enabled: boolean
}
interface MmVersionData { remote_version: string; local_version: string; has_update: boolean; installed?: boolean }
interface MmUpdateStatusData {
  state: string; msg: string; file_name: string; file_index: number
  file_total: number; downloaded: number; total: number; percent: number
}
interface MmInstallStatusData {
  state: string; msg: string; downloaded: number; total: number; percent: number
}

const mmDialogVisible = ref(false)
const mmActiveTab = ref('overview')
const mmLoadingStatus = ref(false)
const mmControlling = ref('')
const mmControlOutput = ref('')
const mmCheckingVersion = ref(false)
const mmBinaryChecking = ref(false)
const mmConfigLoading = ref(false)
const mmConfigSaving = ref(false)
const mmConfigChecking = ref(false)
const mmConfigText = ref('')
const mmConfigError = ref('')
const mmConfigCheckOutput = ref('')
const mmUninstalling = ref('')
const mmAutostartChanging = ref(false)
const mmEntryUnlocked = ref(false)
let mmUpdatePollTimer: ReturnType<typeof setInterval> | null = null
let mmInstallPollTimer: ReturnType<typeof setInterval> | null = null
let mmGateClickCount = 0
let mmGateClickTimer: ReturnType<typeof setTimeout> | null = null

const mmStatus = reactive<MmStatusData>({
  running: false, pid: 0, mihomo_dir: '/data/kano_plugins/mihomo', local_version: '',
  files: [], binary_version: '', start_time: '', api_reachable: false, api_version: '', external_controller: '',
  autostart_enabled: false
})
const mmVersionInfo = reactive<MmVersionData>({
  remote_version: '', local_version: '', has_update: false, installed: true
})
const mmUpdateStatus = reactive<MmUpdateStatusData>({
  state: 'idle', msg: '', file_name: '', file_index: 0, file_total: 0, downloaded: 0, total: 0, percent: 0
})
const mmBinaryVersionInfo = reactive<MmVersionData>({
  remote_version: '', local_version: '', has_update: false, installed: true
})
const mmInstallStatus = reactive<MmInstallStatusData>({
  state: 'idle', msg: '', downloaded: 0, total: 0, percent: 0
})

const mmGate = (() => {
  const decode = (codes: number[], key: number) => String.fromCharCode(...codes.map(code => code ^ key))
  return {
    cookie: decode([58, 63, 54, 58, 56, 58, 10, 52, 55, 59, 58, 48, 62, 49], 87),
    pass: decode([64, 64, 3, 5], 53),
  }
})()

function getCookieValue(name: string): string {
  const prefix = `${encodeURIComponent(name)}=`
  return document.cookie
    .split(';')
    .map(item => item.trim())
    .find(item => item.startsWith(prefix))
    ?.slice(prefix.length) || ''
}

function setCookieValue(name: string, value: string, maxAgeSeconds: number) {
  document.cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}; max-age=${maxAgeSeconds}; path=/; SameSite=Lax`
}

function initMmEntryState() {
  mmEntryUnlocked.value = getCookieValue(mmGate.cookie) === '1'
  if (mmEntryUnlocked.value) loadMmStatus()
}

async function handleUptimeSecretClick() {
  if (mmEntryUnlocked.value) return
  mmGateClickCount += 1
  if (mmGateClickTimer) clearTimeout(mmGateClickTimer)
  mmGateClickTimer = setTimeout(() => {
    mmGateClickCount = 0
    mmGateClickTimer = null
  }, 1800)

  if (mmGateClickCount < 5) return
  mmGateClickCount = 0
  if (mmGateClickTimer) {
    clearTimeout(mmGateClickTimer)
    mmGateClickTimer = null
  }

  try {
    const { value } = await ElMessageBox.prompt('输入神秘代码', '哦豁', {
      inputType: 'password',
      confirmButtonText: '确认',
      cancelButtonText: '取消',
      inputPattern: /.+/,
      inputErrorMessage: '输入神秘代码',
      closeOnClickModal: false,
    })
    if (value !== mmGate.pass) {
      ElMessage.error('密码错误')
      return
    }
    mmEntryUnlocked.value = true
    setCookieValue(mmGate.cookie, '1', 180 * 24 * 60 * 60)
    loadMmStatus()
    ElMessage.success('你发现了秘密通道！')
  } catch {
    // canceled
  }
}

const mmUpdateLabel = computed(() => (({
  downloading: '下载中', done: '已完成', failed: '失败', canceled: '已取消'
} as Record<string, string>)[mmUpdateStatus.state] ?? mmUpdateStatus.state))

const mmUpdateTagType = computed(() => (({
  downloading: 'primary', done: 'success', failed: 'danger', canceled: 'info'
} as Record<string, string>)[mmUpdateStatus.state] ?? 'info') as any)

const mmInstallLabel = computed(() => (({
  downloading: '下载中', done: '已完成', failed: '失败', canceled: '已取消'
} as Record<string, string>)[mmInstallStatus.state] ?? mmInstallStatus.state))

const mmInstallTagType = computed(() => (({
  downloading: 'primary', done: 'success', failed: 'danger', canceled: 'info'
} as Record<string, string>)[mmInstallStatus.state] ?? 'info') as any)

async function loadMmStatus() {
  mmLoadingStatus.value = true
  try {
    const res = await axios.get('/api/mihomo/status')
    if (res.data.code === 0) Object.assign(mmStatus, res.data.data)
  } catch { /* ignore */ } finally {
    mmLoadingStatus.value = false
  }
}

async function setMmAutostart(enabled: boolean) {
  mmAutostartChanging.value = true
  try {
    const res = await axios.post('/api/mihomo/autostart', { enabled })
    if (res.data.code === 0) {
      mmStatus.autostart_enabled = enabled
      ElMessage.success(enabled ? '已开启开机自启' : '已关闭开机自启')
    } else {
      ElMessage.error(res.data.msg || '设置失败')
      mmStatus.autostart_enabled = !enabled
    }
  } catch {
    ElMessage.error('请求失败')
    mmStatus.autostart_enabled = !enabled
  } finally {
    mmAutostartChanging.value = false
  }
}

function openMmDialog() {
  mmDialogVisible.value = true
  mmActiveTab.value = 'overview'
  loadMmStatus()
}

// 切换到配置文件 tab 时自动加载
watch(mmActiveTab, (tab) => {
  if (tab === 'config' && mmConfigText.value === '') {
    loadMmConfig()
  }
})

async function mmControl(action: string) {
  mmControlling.value = action
  mmControlOutput.value = ''
  try {
    const res = await axios.post('/api/mihomo/control', { action })
    if (res.data.code === 0) {
      ElMessage.success(action + ' 成功')
      mmControlOutput.value = (res.data.output ?? '').trim()
    } else {
      ElMessage.error(res.data.msg)
      mmControlOutput.value = (res.data.output ?? res.data.msg ?? '').trim()
    }
    await loadMmStatus()
  } catch (e: any) {
    ElMessage.error('请求失败: ' + (e.message ?? e))
  } finally {
    mmControlling.value = ''
  }
}

async function checkMmVersion() {
  mmCheckingVersion.value = true
  try {
    const res = await axios.get('/api/mihomo/data/version')
    if (res.data.code === 0) {
      Object.assign(mmVersionInfo, res.data.data)
      mmVersionInfo.has_update
        ? ElMessage.warning('新版本：' + mmVersionInfo.remote_version)
        : ElMessage.success('已是最新版本')
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('检查失败: ' + (e.message ?? e))
  } finally {
    mmCheckingVersion.value = false
  }
}

async function startMmUpdate() {
  try {
    const res = await axios.post('/api/mihomo/data/update')
    if (res.data.code === 0) {
      ElMessage.success('开始下载更新...')
      Object.assign(mmUpdateStatus, res.data.data)
      startMmUpdatePoll()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('启动更新失败: ' + (e.message ?? e))
  }
}

async function cancelMmUpdate() {
  try {
    const res = await axios.post('/api/mihomo/data/update/cancel')
    if (res.data.code === 0) ElMessage.info('已取消')
    else ElMessage.error(res.data.msg)
  } catch (e: any) {
    ElMessage.error(e.message ?? e)
  }
}

function startMmUpdatePoll() {
  if (mmUpdatePollTimer) return
  mmUpdatePollTimer = setInterval(async () => {
    try {
      const res = await axios.get('/api/mihomo/data/update/status')
      if (res.data.code !== 0) return
      Object.assign(mmUpdateStatus, res.data.data)
      if (mmUpdateStatus.state === 'done') {
        ElMessage.success('数据文件更新完成！')
        stopMmUpdatePoll()
        loadMmStatus()
      } else if (mmUpdateStatus.state === 'failed') {
        ElMessage.error('更新失败：' + mmUpdateStatus.msg)
        stopMmUpdatePoll()
      } else if (mmUpdateStatus.state === 'canceled') {
        stopMmUpdatePoll()
      }
    } catch { /* ignore */ }
  }, 1000)
}

function stopMmUpdatePoll() {
  if (mmUpdatePollTimer) { clearInterval(mmUpdatePollTimer); mmUpdatePollTimer = null }
}

// ── 配置文件 ──

async function loadMmConfig() {
  mmConfigLoading.value = true
  mmConfigError.value = ''
  try {
    const res = await axios.get('/api/mihomo/config')
    if (res.data.code === 0) {
      mmConfigText.value = res.data.data?.content ?? ''
    } else {
      mmConfigError.value = res.data.msg
    }
  } catch (e: any) {
    mmConfigError.value = '加载失败: ' + (e.message ?? e)
  } finally {
    mmConfigLoading.value = false
  }
}

async function saveMmConfig() {
  mmConfigSaving.value = true
  try {
    const res = await axios.put('/api/mihomo/config', { content: mmConfigText.value })
    if (res.data.code === 0) {
      ElMessage.success('配置已保存')
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e.message ?? e))
  } finally {
    mmConfigSaving.value = false
  }
}

async function checkMmConfig() {
  mmConfigChecking.value = true
  mmConfigCheckOutput.value = ''
  try {
    const res = await axios.post('/api/mihomo/config/check')
    const output = (res.data.output ?? '').trim()
    if (res.data.code === 0) {
      ElMessage.success(res.data.msg || '配置有效')
      mmConfigCheckOutput.value = output || '配置校验通过。'
    } else {
      ElMessage.error(res.data.msg || '配置校验失败')
      mmConfigCheckOutput.value = output || (res.data.msg ?? '')
    }
  } catch (e: any) {
    ElMessage.error('请求失败: ' + (e.message ?? e))
  } finally {
    mmConfigChecking.value = false
  }
}

// ── 二进制安装 ──

async function checkMmBinaryVersion() {
  mmBinaryChecking.value = true
  try {
    const res = await axios.get('/api/mihomo/binary/version')
    if (res.data.code === 0) {
      Object.assign(mmBinaryVersionInfo, res.data.data)
      if (!mmBinaryVersionInfo.installed) {
        ElMessage.warning('Mihomo 内核未安装，可安装版本：' + (mmBinaryVersionInfo.remote_version || '未知'))
      } else if (mmBinaryVersionInfo.has_update) {
        ElMessage.warning('新版本可用：' + mmBinaryVersionInfo.remote_version)
      } else {
        ElMessage.success('已是最新版本')
      }
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('检查失败: ' + (e.message ?? e))
  } finally {
    mmBinaryChecking.value = false
  }
}

async function startMmInstall() {
  try {
    const res = await axios.post('/api/mihomo/install')
    if (res.data.code === 0) {
      ElMessage.success('开始安装...')
      Object.assign(mmInstallStatus, res.data.data)
      startMmInstallPoll()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('启动安装失败: ' + (e.message ?? e))
  }
}

async function cancelMmInstall() {
  try {
    const res = await axios.post('/api/mihomo/install/cancel')
    if (res.data.code === 0) ElMessage.info('已取消')
    else ElMessage.error(res.data.msg)
  } catch (e: any) {
    ElMessage.error(e.message ?? e)
  }
}

function startMmInstallPoll() {
  if (mmInstallPollTimer) return
  mmInstallPollTimer = setInterval(async () => {
    try {
      const res = await axios.get('/api/mihomo/install/status')
      if (res.data.code !== 0) return
      Object.assign(mmInstallStatus, res.data.data)
      if (mmInstallStatus.state === 'done') {
        ElMessage.success('Mihomo 安装/更新完成！')
        stopMmInstallPoll()
        loadMmStatus()
      } else if (mmInstallStatus.state === 'failed') {
        ElMessage.error('安装失败：' + mmInstallStatus.msg)
        stopMmInstallPoll()
      } else if (mmInstallStatus.state === 'canceled') {
        stopMmInstallPoll()
      }
    } catch { /* ignore */ }
  }, 1000)
}

function stopMmInstallPoll() {
  if (mmInstallPollTimer) { clearInterval(mmInstallPollTimer); mmInstallPollTimer = null }
}

async function uninstallMm(mode: string) {
  const label = mode === 'full' ? '完全卸载' : '仅删除内核'
  try {
    await ElMessageBox.confirm(`确认执行：${label}？此操作不可逆。`, '卸载确认', {
      confirmButtonText: '确认卸载',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch { return }
  mmUninstalling.value = mode
  try {
    const res = await axios.post('/api/mihomo/uninstall', { mode })
    if (res.data.code === 0) {
      ElMessage.success('卸载完成')
      await loadMmStatus()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('卸载失败: ' + (e.message ?? e))
  } finally {
    mmUninstalling.value = ''
  }
}

function stopMmAllPolls() {
  stopMmUpdatePoll()
  stopMmInstallPoll()
}

function formatMmSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

// 短信转发
function smsForwardHandler() {
  ElNotification({
    title: '功能未实现',
    message: '短信转发功能尚未实现，敬请期待！',
    type: 'warning',
    duration: 5000,
  });
}

function netAmbrGetHandler() {
  axios.post('/api/net/ambr/get', { })
    .then((res) => {
      if (res.data.code !== 0) return;
      const data = res.data.data;
      netAmbr.value = {
        ...netAmbr.value,
        ...data,
        dl: { ...netAmbr.value.dl, ...data.dl },
        ul: { ...netAmbr.value.ul, ...data.ul },
      };
    })
}
function psmGetHandler() {
  axios.post('/api/wifi/psm/get', { ifaces: ['wlan0', 'wlan1', 'wlan2', 'wlan3'], })
    .then((res) => {
      if (res.data.code !== 0) return;
      const data = res.data.data;
      wifiInfo.value.wlan0 = data.wlan0_psm;
      wifiInfo.value.wlan1 = data.wlan1_psm;
      wifiInfo.value.wlan2 = data.wlan2_psm;
      wifiInfo.value.wlan3 = data.wlan3_psm;

      // 2.4G: wlan0
      wifiInfo.value.wifiStatus24 = data.wlan0_status === 'up';
      // 5G: wlan2
      wifiInfo.value.wifiStatus5 = data.wlan2_status === 'up';
      wifiForm.wifi24_enabled = wifiInfo.value.wifiStatus24;
      wifiForm.wifi5_enabled = wifiInfo.value.wifiStatus5;

      // psm
      wifiInfo.value.highPerformance = data.wlan2_psm === 'off';
      wifiForm.high_performance = wifiInfo.value.highPerformance;
    })
}
async function psmSetHandler(val:boolean, showMessage: boolean = true){
  if (wifiPsmSaving.value) return;
  wifiPsmSaving.value = true;
  wifiSettingsSaving.value = 'psm';
  try {
    const res = await axios.post('/api/wifi/psm/set', {
      ifaces: ['wlan0', 'wlan1', 'wlan2', 'wlan3'],
      mode: val ? 'off' : 'on',
    });
    if (res.data.code !== 0) throw new Error(res.data.msg || 'WiFi 性能模式切换失败');
    psmGetHandler();
    if (showMessage) ElMessage.success('WiFi已切换为:' + (val ? '高性能模式' : '省电模式'));
  } catch (err: any) {
    ElMessage.error(err.message || 'WiFi 性能模式切换失败');
  } finally {
    wifiPsmSaving.value = false;
    wifiSettingsSaving.value = '';
  }
}
function wifiStateSetHandler(iface:string, val:boolean){
  axios.post('/api/wifi/state/set', {
    ifaces: [iface],
    up: val,
  }).then((res) => {
    psmGetHandler()
    if (iface === 'wlan0') wifiForm.wifi24_enabled = val;
    if (iface === 'wlan2') wifiForm.wifi5_enabled = val;
    ElMessage.success((iface == 'wlan0' ? '2.4G' : ((iface == 'wlan2' ? '5G' : '其他'))) + '-WiFi已' + (val ? '开启' : '关闭'));
  });
}
function clampPercent(value: number) {
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

const totalCpuLoad = computed(() => {
  const idle = Number(deviceInfo.value.cpuinfo?.[0]?.idle ?? 100);
  return clampPercent(100 - idle);
});

// 根据电量百分比给电池图标分配颜色等级
const batteryStateClass = computed(() => {
  const p = Number(deviceInfo.value?.bat_percent ?? NaN);
  if (!Number.isFinite(p)) return '';
  if (p <= 20) return 'low';
  if (p <= 50) return 'medium';
  return '';
});

const cpuCoreLoads = computed(() => {
  return [1, 2, 3, 4].map((index) => {
    const idle = Number(deviceInfo.value.cpuinfo?.[index]?.idle ?? 100);
    return {
      name: `核心${index}`,
      value: clampPercent(100 - idle),
    };
  });
});

const cpuPieStyle = computed(() => {
  const load = totalCpuLoad.value;
  return {
    background: `conic-gradient(
      #63e6be 0% ${load}%,
      rgba(255,255,255,0.12) ${load}% 100%
    )`,
  };
});

function getTempPercent(temp: unknown): number {
  const n = Number(temp);
  if (Number.isNaN(n)) return 0;

  // 假设 0~100℃ 映射成 0~100%
  return Math.max(0, Math.min(100, n));
}

function getTempClass(temp: unknown): string {
  const n = Number(temp);
  if (Number.isNaN(n)) return 'normal';

  if (n >= 70) return 'danger';
  if (n >= 55) return 'warning';
  return 'normal';
}

function getTempText(temp: unknown): string {
  const n = Number(temp);
  if (Number.isNaN(n)) return '-';

  if (n >= 70) return '过热';
  if (n >= 55) return '偏高';
  return '正常';
}

// ─────────────────────────── 已连接设备列表（无线 + 有线） ───────────────────────────

interface ConnectedDevice {
  ip_address: string;
  mac_address: string;
  hostname: string;
  interface_type: string;
  access_time: string;
  // 以下字段由 iwinfo assoclist 按 MAC 匹配补充（仅无线终端）
  signal?: number;   // 信号强度，dBm
  tx_rate?: number;  // 设备下行协商速率（AP 发送给终端），kbit/s
}

const deviceDialogVisible = ref(false);
const deviceListLoading = ref(false);
const wirelessDeviceList = ref<ConnectedDevice[]>([]);
const wiredDeviceList = ref<ConnectedDevice[]>([]);

function getDeviceTagClass(interfaceType: string): string {
  if (interfaceType === '5G') return 'tag-5g';
  if (interfaceType === 'Ethernet') return 'tag-ethernet';
  return 'tag-24g';
}

// 协商速率：iwinfo 的 rate 单位为 kbit/s，转成 Mbps 整数展示（单位在模板里统一加）
function formatRate(kbit?: number): string {
  if (!kbit || kbit <= 0) return '-';
  return String(Math.round(kbit / 1000));
}

// 信号强度分级配色：数值越接近 0 越强
function signalClass(signal?: number): string {
  if (signal == null) return '';
  if (signal >= -50) return 'signal-strong';
  if (signal >= -60) return 'signal-good';
  if (signal >= -70) return 'signal-fair';
  return 'signal-weak';
}

// 归一化 MAC：去分隔符 + 大写，兼容 ZTE 列表与 iwinfo 之间冒号/横线/大小写差异
const normMac = (m: any) => String(m || '').toUpperCase().replace(/[^0-9A-F]/g, '');

// 从 iwinfo assoclist 的批量结果（id 100=wlan0/2.4G、101=wlan2/5G）里，
// 按 MAC 建立 信号 + 下行协商速率(AP→终端) 索引
function buildRfMap(resultMap: Record<number, any>): Record<string, { signal: number; txRate: number }> {
  const rfMap: Record<string, { signal: number; txRate: number }> = {};
  for (const id of [100, 101]) {
    const stations = resultMap[id]?.results;
    if (!Array.isArray(stations)) continue;
    for (const st of stations) {
      const key = normMac(st?.mac);
      if (!key) continue;
      rfMap[key] = {
        signal: st.signal,
        txRate: st.tx?.rate ?? 0,  // AP 发给终端 = 终端下行
      };
    }
  }
  return rfMap;
}

// 只拉两个频段的 iwinfo assoclist（弹窗每秒刷新用，比整张设备列表轻）
async function fetchWirelessRfMap(): Promise<Record<string, { signal: number; txRate: number }>> {
  const resultMap = await callUbusBatch([
    { jsonrpc: '2.0', id: 100, method: 'call', params: [SESSION_ID, 'iwinfo', 'assoclist', { device: 'wlan0' }] },
    { jsonrpc: '2.0', id: 101, method: 'call', params: [SESSION_ID, 'iwinfo', 'assoclist', { device: 'wlan2' }] },
  ]);
  return buildRfMap(resultMap);
}

// 把信号/下行速率原地更新到现有无线设备（按 MAC）；匹配不到则清空那两行
function applyRfToWireless(rfMap: Record<string, { signal: number; txRate: number }>) {
  for (const d of wirelessDeviceList.value) {
    const rf = rfMap[normMac(d.mac_address)];
    d.signal = rf ? rf.signal : undefined;
    d.tx_rate = rf ? rf.txRate : undefined;
  }
}

// 弹窗期间每秒刷新 信号 + 协商速率。用自调度 setTimeout（等上次完成再排下次），
// 避免慢网络下请求叠加；关闭弹窗或组件卸载时停止。
let deviceRfTimer: number | null = null;
function startDeviceRfRefresh() {
  stopDeviceRfRefresh();
  const tick = async () => {
    if (!deviceDialogVisible.value) return;
    try {
      const rfMap = await fetchWirelessRfMap();
      if (!deviceDialogVisible.value) return; // 等待期间被关掉
      applyRfToWireless(rfMap);
    } catch { /* 刷新失败静默，不打断弹窗 */ }
    if (deviceDialogVisible.value) {
      deviceRfTimer = window.setTimeout(tick, 1000);
    }
  };
  deviceRfTimer = window.setTimeout(tick, 1000);
}
function stopDeviceRfRefresh() {
  if (deviceRfTimer != null) {
    clearTimeout(deviceRfTimer);
    deviceRfTimer = null;
  }
}

async function openDeviceDialog() {
  deviceDialogVisible.value = true;
  deviceListLoading.value = true;
  wirelessDeviceList.value = [];
  wiredDeviceList.value = [];
  try {
    const resultMap = await callUbusBatch([
      {
        jsonrpc: '2.0',
        id: 98,
        method: 'call',
        params: [
          SESSION_ID,
          'zwrt_router.api',
          'router_wireless_access_list',
          { start_id: 1, end_id: 64 },
        ],
      },
      {
        jsonrpc: '2.0',
        id: 99,
        method: 'call',
        params: [
          SESSION_ID,
          'zwrt_router.api',
          'router_lan_access_list',
          {},
        ],
      },
      // iwinfo assoclist：U60Pro 上 wlan0=2.4G、wlan2=5G，
      // 用来补充每个无线终端的信号值与协商速率（下面按 MAC 匹配）
      {
        jsonrpc: '2.0',
        id: 100,
        method: 'call',
        params: [SESSION_ID, 'iwinfo', 'assoclist', { device: 'wlan0' }],
      },
      {
        jsonrpc: '2.0',
        id: 101,
        method: 'call',
        params: [SESSION_ID, 'iwinfo', 'assoclist', { device: 'wlan2' }],
      },
    ]);

    // 按 MAC 把首批 信号 + 下行协商速率 合并进无线设备列表
    const rfMap = buildRfMap(resultMap);
    if (resultMap[98]?.wireless_access_list_info) {
      wirelessDeviceList.value = (resultMap[98].wireless_access_list_info as ConnectedDevice[]).map((d) => {
        const rf = rfMap[normMac(d.mac_address)];
        return rf ? { ...d, signal: rf.signal, tx_rate: rf.txRate } : d;
      });
    }
    if (resultMap[99]?.lan_access_list_info) {
      wiredDeviceList.value = resultMap[99].lan_access_list_info;
    }
  } catch (e: any) {
    ElMessage.error('获取设备列表失败: ' + (e?.message ?? e));
  } finally {
    deviceListLoading.value = false;
  }
  // 弹窗期间每秒刷新 信号 + 协商速率（有无线设备才轮询）
  if (deviceDialogVisible.value && wirelessDeviceList.value.length > 0) {
    startDeviceRfRefresh();
  }
}

// 弹窗打开时锁住底层页面滚动。index.html 把 html/body/#app 都设为 height:100%,
// 整页滚动发生在 html/window 上。若直接给 html 设 overflow:hidden,浏览器会把滚动位置
// 强制归零（整页跳到顶部），且关闭后无法恢复——这正是“点开弹窗页面跳回顶部”的根因。
// 改用「固定 body + 记录/还原 scrollY」：锁定时把 body 设为 position:fixed 并上移 scrollY,
// 既挡住背景滚动又保留视觉位置；关闭时还原样式并 scrollTo 回原位。
let lockedScrollY = 0;
watch([deviceDialogVisible, mmDialogVisible, networkSettingsDialogVisible, wifiSettingsDialogVisible, systemToolsDialogVisible], ([deviceOpen, mmOpen, networkOpen, wifiOpen, toolsOpen]) => {
  const anyOpen = deviceOpen || mmOpen || networkOpen || wifiOpen || toolsOpen;
  const body = document.body;
  if (anyOpen) {
    lockedScrollY = window.scrollY || document.documentElement.scrollTop || 0;
    body.style.position = 'fixed';
    body.style.top = `-${lockedScrollY}px`;
    body.style.left = '0';
    body.style.right = '0';
    body.style.width = '100%';
    body.style.height = 'auto'; // 覆盖 index.html 的 height:100%，否则 fixed 后超出一屏的内容会被裁掉
    body.style.overflow = 'hidden';
  } else {
    body.style.position = '';
    body.style.top = '';
    body.style.left = '';
    body.style.right = '';
    body.style.width = '';
    body.style.height = '';
    body.style.overflow = '';
    window.scrollTo(0, lockedScrollY);
  }
});

// 关闭「已连接设备」弹窗时停止每秒刷新（按钮 / 点遮罩 / ESC 都会触发）
watch(deviceDialogVisible, (open) => {
  if (!open) stopDeviceRfRefresh();
});

watch(systemToolsActiveTab, (tab) => {
  if (!systemToolsDialogVisible.value) return;
  if (tab === 'sms') {
    loadSmsForwardStatus();
    if (smsMessages.value.length === 0) loadSmsMessages();
  }
  if (tab === 'devui') {
    loadDevuiStatus();
    loadDevuiDownloadStatus().then(() => {
      if (devui.downloadState === 'downloading') startDevuiDownloadPoll();
    });
  }
  if (tab === 'rcLocal' && !rcLocal.loaded) loadRcLocal();
});

onMounted(() => {
  initMmEntryState();
  loadSmsForwardStatus();
  loadDevuiStatus();
  fetchAllData();
  loadDeviceSettings();
  if (autoRefresh.value) {
    startAutoRefresh();
  }
  // 获取WiFi状态
  psmGetHandler();
  // 获取签约速率
  netAmbrGetHandler();
});

onUnmounted(() => {
  stopAutoRefresh();
  stopDevuiDownloadPoll();
  if (devuiDownloadClearTimer) {
    clearTimeout(devuiDownloadClearTimer);
    devuiDownloadClearTimer = null;
  }
  if (devui.wallpaperPreview) {
    URL.revokeObjectURL(devui.wallpaperPreview);
  }
  stopMmAllPolls();
  stopLocalSpeedTest();
  stopDeviceRfRefresh();
  if (mmGateClickTimer) {
    clearTimeout(mmGateClickTimer);
    mmGateClickTimer = null;
  }
  // 兜底还原（防止组件卸载时仍残留锁定样式）
  const body = document.body;
  body.style.position = '';
  body.style.top = '';
  body.style.left = '';
  body.style.right = '';
  body.style.width = '';
  body.style.height = '';
  body.style.overflow = '';
});
</script>

<style scoped>
/* 基础样式 */
.page {
  color: white;
  min-height: 100vh;
  min-height: 100dvh;
  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 50%, #3b82f6 100%);
  padding:
    calc(20px + var(--app-safe-top, 0px))
    calc(20px + var(--app-safe-right, 0px))
    calc(20px + var(--app-safe-bottom, 0px))
    calc(20px + var(--app-safe-left, 0px));
  box-sizing: border-box;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

/* 页面头部 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  backdrop-filter: blur(20px);
  border-radius: 16px;
  padding: 12px 16px;
  margin-bottom: 24px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.3);
}


.title-section {
  display: flex;
  align-items: center;
  gap: 16px;
}

.title {
  font-size: 28px;
  font-weight: 700;
  color: #ffffff;
  margin: 0;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  animation: pulse 2s infinite;
}

.status-dot.online {
  background: #48bb78;
}

.status-dot.pending {
  background: #f6ad55;
}

.status-dot.offline {
  background: #e53e3e;
}

.status-text {
  font-size: 14px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.8);
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

/* 控制区域 */
.controls {
  display: flex;
  align-items: center;
  gap: 16px;
}

.top-status-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.quick-actions-grid {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(132px, max-content);
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.quick-action-button,
.wifi-mode-button {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  padding: 6px 10px 6px 10px;
  border: 1px solid rgba(125, 211, 252, 0.35);
  border-radius: 8px;
  color: rgba(255, 255, 255, 0.92);
  background: rgba(14, 165, 233, 0.18);
  box-shadow: 0 6px 18px rgba(14, 116, 144, 0.18);
  cursor: pointer;
  transition: transform 0.2s ease, border-color 0.2s ease, background 0.2s ease, box-shadow 0.2s ease;
}

.quick-action-button.active {
  border-color: rgba(74, 222, 128, 0.45);
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.26), rgba(14, 165, 233, 0.18));
  box-shadow: 0 6px 18px rgba(34, 197, 94, 0.18);
}

.adb-action-button.active {
  border-color: rgba(96, 165, 250, 0.5);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.28), rgba(14, 165, 233, 0.2));
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.2);
}

.mm-action-button.active {
  border-color: rgba(167, 139, 250, 0.5);
  background: linear-gradient(135deg, rgba(124, 58, 237, 0.28), rgba(14, 165, 233, 0.2));
  box-shadow: 0 6px 18px rgba(124, 58, 237, 0.2);
}

.speedtest-action-button.active {
  border-color: rgba(45, 212, 191, 0.52);
  background: linear-gradient(135deg, rgba(20, 184, 166, 0.28), rgba(14, 165, 233, 0.2));
  box-shadow: 0 6px 18px rgba(20, 184, 166, 0.2);
}

/* Mihomo 弹窗样式见文件底部非 scoped 块（因 el-dialog teleport 到 body） */

/* ============ 数据模式 Select 触发器 - 玻璃风格 ============ */
.net-select {
  width: 78px;
  margin-top: 3px; /* 与上方 title 留间距，对齐其他 quick-action 副标题的视觉节奏 */
}
.net-select :deep(.el-select__wrapper) {
  min-height: 20px;
  height: 20px;
  width: 78px;
  padding: 0 6px;
  font-size: 11px;
  background: rgba(255, 255, 255, 0.15);
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 6px;
  box-shadow: none;
  transition: background 0.2s ease, border-color 0.2s ease;
}
.net-select :deep(.el-select__wrapper.is-hovering),
.net-select :deep(.el-select__wrapper.is-focused) {
  background: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.55);
  box-shadow: none;
}
.net-select :deep(.el-select__selected-item) {
  color: #ffffff;
  font-weight: 600;
}
.net-select :deep(.el-select__placeholder) {
  color: rgba(255, 255, 255, 0.55);
}
.net-select :deep(.el-select__caret),
.net-select :deep(.el-icon) {
  color: rgba(255, 255, 255, 0.75);
}
/* iOS Safari/Chrome 在聚焦字号 < 16px 的表单控件时会自动放大页面，且选完不会自动缩回。
   el-select 真正聚焦的是内部隐藏 <input>，把它字号提到 16px 即可阻止放大；
   可见文字渲染在 .el-select__selected-item 上，仍保持 11px，外观不变。 */
.net-select :deep(.el-select__input) {
  font-size: 16px;
}


.wifi-mode-button.active {
  border-color: rgba(251, 191, 36, 0.48);
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.28), rgba(14, 165, 233, 0.2));
  box-shadow: 0 6px 18px rgba(245, 158, 11, 0.2);
}

.quick-action-button:hover:not(:disabled),
.wifi-mode-button:hover:not(:disabled) {
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.55);
  box-shadow: 0 10px 22px rgba(15, 23, 42, 0.24);
}

.quick-action-button:disabled,
.wifi-mode-button:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.quick-action-button:focus-visible,
.wifi-mode-button:focus-visible {
  outline: 2px solid rgba(255, 255, 255, 0.5);
  outline-offset: 2px;
}

.wifi-mode-button.saving .wifi-mode-icon {
  animation: pulse 1s ease-in-out infinite;
}

.quick-action-icon,
.wifi-mode-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 800;
  color: #0f172a;
  background: rgba(255, 255, 255, 0.9);
}

.quick-action-copy,
.wifi-mode-copy {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
  line-height: 1.15;
}

.quick-action-title,
.wifi-mode-title {
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.quick-action-subtitle,
.wifi-mode-action {
  margin-top: 3px;
  font-size: 11px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.72);
  white-space: nowrap;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.9);
}

.checkbox-label input[type='checkbox'] {
  display: none;
}

.checkmark {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.6);
  border-radius: 4px;
  position: relative;
  transition: all 0.2s ease;
}

.checkbox-label input[type='checkbox']:checked + .checkmark {
  background: #4299e1;
  border-color: #4299e1;
}

.checkbox-label input[type='checkbox']:checked + .checkmark::after {
  content: '✓';
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: white;
  font-size: 12px;
  font-weight: bold;
}

.auto-refresh-controls select {
  padding: 4px 6px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(10px);
  font-size: 14px;
  color: rgba(255, 255, 255, 0.9);
  cursor: pointer;
  transition: border-color 0.2s ease;
}

.auto-refresh-controls select:focus {
  outline: none;
  border-color: #4299e1;
}

.auto-refresh-controls select:disabled {
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.4);
  cursor: not-allowed;
}

/* 按钮样式 */
.btn {
  padding: 5px 10px;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: #409eff5d;
  color: white;
  box-shadow: 0 4px 12px rgba(66, 153, 225, 0.1);
}

.btn-primary:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(66, 153, 225, 0.4);
}

.btn-danger {
  background: linear-gradient(135deg, #e53e3e, #c53030);
  color: white;
  box-shadow: 0 4px 12px rgba(229, 62, 62, 0.3);
}

.btn-danger:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(229, 62, 62, 0.4);
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid transparent;
  border-top: 2px solid currentColor;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* 状态页面 */
.loading,
.error,
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  padding: 40px;
  text-align: center;
  border: 1px solid rgba(255, 255, 255, 0.3);
}

.loading-spinner {
  width: 48px;
  height: 48px;
  border: 4px solid #e2e8f0;
  border-top: 4px solid #4299e1;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 20px;
}

.error-icon {
  font-size: 48px;
  margin-bottom: 16px;
}

.empty-icon {
  font-size: 64px;
  margin-bottom: 20px;
}

.loading p,
.error h3,
.error p,
.empty h3,
.empty p {
  color: rgba(255, 255, 255, 0.9);
  margin: 0;
}

.error h3,
.empty h3 {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 8px;
}

/* 内容区域 - 等宽网格 */
.content {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
  justify-content: center;
  gap: 24px;
  align-items: stretch; /* 卡片等高 */
}

/* 卡片样式 */
.card {
  backdrop-filter: blur(20px);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.3);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  display: flex;
  flex-direction: column;
}

.card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
}

.hd {
  display: flex;
  gap: 10px;
}
.hd img {
  width: 24px;
}

.card-header {
  background: rgba(255, 255, 255, 0.1);
  padding: 12px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  font-size: 18px;
  font-weight: 600;
  color: #ffffff;
  margin: 0;
}

.card-tags {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.card-content {
  padding: 16px;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
}

/* 设备信息卡片 */
.device-info-card {
  grid-column: 1 / -1; /* 占据整行 */
}

/* 操作卡片 */
.device-actions-card {
  grid-column: 1 / -1; /* 占据整行 */
}

.device-stats {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 24px;
  align-items: stretch;
}

.device-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.health-card {
  position: relative;          /* 相对定位，避免被父元素限制 */
  z-index: 1;                  /* 确保浮动在上层显示 */
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.health-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 6px 12px rgba(0,0,0,0.15);
}

.device-label {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.device-values {
  display: flex;
  gap: 16px;
}

.load-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.load-label {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.6);
}

.load-value {
  font-size: 16px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.9);
}

.memory-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.memory-bar {
  position: relative;
  height: 20px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.memory-fill {
  height: 100%;
  background: linear-gradient(
    90deg,
    #62718abb 0%,
    #68d391bb 70%,
    #63b3edbb 100%
  );
  border-radius: 10px;
  transition: width 0.3s ease;
}

.memory-text {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 11px;
  font-weight: 600;
  color: #ffffff;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
}

.memory-details {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 16px;
  color: rgba(255, 255, 255, 0.8);
}

.memory-used {
  font-weight: 600;
}

.memory-separator {
  color: rgba(255, 255, 255, 0.5);
}

.memory-total {
  color: rgba(255, 255, 255, 0.7);
}

.temp-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.temp-value {
  font-size: 18px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.9);
}

.temp-bar {
  height: 16px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.temp-fill {
  height: 100%;
  background: linear-gradient(
    90deg,
    #62718abb 0%,
    #68d391bb 70%,
    #63b3edbb 100%
  );
  border-radius: 8px;
  transition: width 0.3s ease;
}

.uptime-value {
  font-size: 20px;
  font-weight: 600;
  line-height: 1.25;
  color: rgba(255, 255, 255, 0.9);
  white-space: nowrap;
}

/* 信息网格 */
/* 信息网格：默认双列 */
.info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.info-item .label {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.info-item .value {
  font-size: 14px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.9);
  word-break: break-all;
  overflow-wrap: anywhere;
  white-space: normal;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-item .label {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.info-item .value {
  font-size: 14px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.9);
  word-break: break-all;
  white-space: pre-wrap;
  word-wrap: break-word;
}

/* 标签样式 */
.tag {
  display: inline-block;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* 连接状态 tag 含 IPv4/IPv6 等大小写敏感的写法，关掉统一大写以免显示成 IPV4 */
.tag.conn-status {
  text-transform: none;
}

.tag.success {
  background: rgba(54, 237, 69, 0.2);
  color: #75f655;
  border: 1px solid rgba(54, 237, 69, 0.3);
}

.tag.warning {
  background: rgba(237, 137, 54, 0.2);
  color: #f6ad55;
  border: 1px solid rgba(237, 137, 54, 0.3);
}

.tag.danger {
  background: rgba(229, 62, 62, 0.2);
  color: #fc8181;
  border: 1px solid rgba(229, 62, 62, 0.3);
}

.tag.excellent {
  background: rgba(72, 187, 120, 0.22);
  color: #7ee787;
  border: 1px solid rgba(72, 187, 120, 0.35);
}

.tag.good {
  background: rgba(56, 189, 248, 0.18);
  color: #7dd3fc;
  border: 1px solid rgba(56, 189, 248, 0.34);
}

.tag.fair {
  background: rgba(237, 137, 54, 0.2);
  color: #f6ad55;
  border: 1px solid rgba(237, 137, 54, 0.3);
}

.tag.poor {
  background: rgba(229, 62, 62, 0.2);
  color: #fc8181;
  border: 1px solid rgba(229, 62, 62, 0.3);
}

.tag.unknown {
  background: rgba(148, 163, 184, 0.16);
  color: #cbd5e1;
  border: 1px solid rgba(148, 163, 184, 0.26);
}

/* 信号进度条 */
.signal-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 16px;
}

.signal-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.signal-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}

.signal-label-help {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.signal-help-trigger {
  width: 18px;
  height: 18px;
  padding: 0;
  border: 1px solid rgba(125, 211, 252, 0.45);
  border-radius: 50%;
  background: rgba(56, 189, 248, 0.12);
  color: #7dd3fc;
  cursor: pointer;
  font-size: 13px;
  font-weight: 800;
  line-height: 18px;
  text-align: center;
  transition: background 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.signal-help-trigger:hover,
.signal-help-trigger[aria-expanded='true'] {
  background: rgba(56, 189, 248, 0.24);
  border-color: rgba(125, 211, 252, 0.7);
  color: #dff6ff;
}

.signal-help-panel {
  padding: 10px 12px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.72);
  color: rgba(255, 255, 255, 0.82);
  font-size: 12px;
  line-height: 1.55;
}

.signal-help-title {
  color: #ffffff;
  font-size: 13px;
  font-weight: 700;
}

.signal-help-desc {
  margin-top: 4px;
}

.signal-help-ranges {
  display: grid;
  gap: 4px;
  margin-top: 8px;
}

.signal-help-ranges div {
  display: flex;
  align-items: center;
  gap: 6px;
}

.signal-help-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: 0 0 auto;
}

.signal-help-dot.excellent {
  background: #68d391;
}

.signal-help-dot.good {
  background: #38bdf8;
}

.signal-help-dot.fair {
  background: #f6ad55;
}

.signal-help-dot.poor {
  background: #fc8181;
}

.signal-status {
  flex: 0 0 auto;
  min-width: 38px;
  padding: 2px 7px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.35;
  text-align: center;
  white-space: nowrap;
}

.signal-status.excellent {
  background: rgba(72, 187, 120, 0.18);
  color: #7ee787;
}

.signal-status.good {
  background: rgba(56, 189, 248, 0.16);
  color: #7dd3fc;
}

.signal-status.fair {
  background: rgba(237, 137, 54, 0.18);
  color: #f6ad55;
}

.signal-status.poor {
  background: rgba(229, 62, 62, 0.18);
  color: #fc8181;
}

.signal-status.unknown {
  background: rgba(148, 163, 184, 0.14);
  color: #cbd5e1;
}

.signal-value {
  font-weight: 800;
  transition: color 0.2s ease, text-shadow 0.2s ease;
}

.signal-value.excellent {
  color: #7ee787;
  text-shadow: 0 0 10px rgba(126, 231, 135, 0.26);
}

.signal-value.good {
  color: #7dd3fc;
  text-shadow: 0 0 10px rgba(125, 211, 252, 0.22);
}

.signal-value.fair {
  color: #f6ad55;
  text-shadow: 0 0 10px rgba(246, 173, 85, 0.22);
}

.signal-value.poor {
  color: #fc8181;
  text-shadow: 0 0 10px rgba(252, 129, 129, 0.24);
}

.signal-value.unknown {
  color: #cbd5e1;
  text-shadow: none;
}

.progress-bar {
  position: relative;
  height: 24px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #64748bbb 0%, #38bdf8bb 100%);
  border-radius: 12px;
  transition: width 0.3s ease;
  position: relative;
}

.progress-fill.excellent {
  background: linear-gradient(90deg, #2f855abb 0%, #68d391dd 100%);
}

.progress-fill.good {
  background: linear-gradient(90deg, #0f766ebb 0%, #38bdf8dd 100%);
}

.progress-fill.fair {
  background: linear-gradient(90deg, #8a5a1fbb 0%, #f6ad55dd 100%);
}

.progress-fill.poor {
  background: linear-gradient(90deg, #7f1d1dbb 0%, #fc8181dd 100%);
}

.progress-fill.unknown {
  background: linear-gradient(90deg, #475569bb 0%, #94a3b8bb 100%);
}

.progress-text {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 12px;
  font-weight: 600;
  color: #ffffff;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
  z-index: 1;
}

/* 顶部网络信息卡片中的信号条图标 */
.signal-bars {
  display: inline-flex;
  align-items: flex-end;
  gap: 3px;
}

.bar {
  width: 5px;
  height: 6px;
  background: rgba(255, 255, 255, 0.16);
  border-radius: 2.5px 2.5px 1px 1px; /* 顶部更圆，更像信号图标 */
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.05);
  transition: height 0.2s ease, background 0.25s ease, box-shadow 0.25s ease;
}

.full-bar {
  height: 18px;
}

.bar:nth-child(1) {
  height: 6px;
}
.bar:nth-child(2) {
  height: 9px;
}
.bar:nth-child(3) {
  height: 12px;
}
.bar:nth-child(4) {
  height: 15px;
}
.bar:nth-child(5) {
  height: 18px;
}

/* 激活的信号条按整体强度着色，并带同色辉光，告别一刀切的纯绿 */
.signal-bars.level-weak .bar.active {
  background: linear-gradient(180deg, #fca5a5, #ef4444);
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.5);
}
.signal-bars.level-medium .bar.active {
  background: linear-gradient(180deg, #fcd34d, #f59e0b);
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.45);
}
.signal-bars.level-strong .bar.active {
  background: linear-gradient(180deg, #86efac, #22c55e);
  box-shadow: 0 0 7px rgba(34, 197, 94, 0.5);
}

/* ========= 电池图标 ========= */
.battery {
  position: relative;
  width: 34px;
  height: 16px;
  border: 1.5px solid rgba(255, 255, 255, 0.7);
  border-radius: 4px;
  box-sizing: border-box;
  display: inline-block;
  padding: 1.5px;
  background:
    linear-gradient(180deg, rgba(255,255,255,0.05) 0%, rgba(0,0,0,0.12) 100%);
  box-shadow:
    inset 0 1px 1px rgba(255, 255, 255, 0.18),
    inset 0 -1px 1px rgba(0, 0, 0, 0.15);
  transition: border-color 0.3s ease, box-shadow 0.3s ease;
}

.battery-head {
  position: absolute;
  right: -4px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 8px;
  background: rgba(255, 255, 255, 0.7);
  border-radius: 0 2px 2px 0;
  transition: background 0.3s ease;
}

.battery-level {
  height: 100%;
  border-radius: 2px;
  background: linear-gradient(90deg, #34d399 0%, #22c55e 100%);
  box-shadow: 0 0 6px rgba(52, 211, 153, 0.45), inset 0 1px 1px rgba(255, 255, 255, 0.35);
  transition: width 0.4s ease, background 0.3s ease, box-shadow 0.3s ease;
}

/* 中等电量（21%~50%）：琥珀色 */
.battery.medium .battery-level {
  background: linear-gradient(90deg, #fbbf24 0%, #f59e0b 100%);
  box-shadow: 0 0 6px rgba(251, 191, 36, 0.5), inset 0 1px 1px rgba(255, 255, 255, 0.3);
}

/* 低电量（<=20%）：红色 + 红边框 */
.battery.low {
  border-color: rgba(248, 113, 113, 0.75);
}
.battery.low .battery-head {
  background: rgba(248, 113, 113, 0.75);
}
.battery.low .battery-level {
  background: linear-gradient(90deg, #f87171 0%, #ef4444 100%);
  box-shadow: 0 0 8px rgba(248, 113, 113, 0.55), inset 0 1px 1px rgba(255, 255, 255, 0.3);
  animation: battery-low-pulse 1.6s ease-in-out infinite;
}

@keyframes battery-low-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.65; }
}

/* 充电中：金色边框光晕 + 脉冲 */
.battery.charging {
  border-color: rgba(252, 211, 77, 0.85);
  box-shadow:
    inset 0 1px 1px rgba(255, 255, 255, 0.18),
    inset 0 -1px 1px rgba(0, 0, 0, 0.15),
    0 0 8px rgba(252, 211, 77, 0.4);
  animation: battery-charge-glow 2s ease-in-out infinite;
}
.battery.charging .battery-head {
  background: rgba(252, 211, 77, 0.85);
}

@keyframes battery-charge-glow {
  0%, 100% { box-shadow:
    inset 0 1px 1px rgba(255, 255, 255, 0.18),
    inset 0 -1px 1px rgba(0, 0, 0, 0.15),
    0 0 6px rgba(252, 211, 77, 0.3); }
  50% { box-shadow:
    inset 0 1px 1px rgba(255, 255, 255, 0.18),
    inset 0 -1px 1px rgba(0, 0, 0, 0.15),
    0 0 12px rgba(252, 211, 77, 0.6); }
}

.battery-charging {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 14px;
  line-height: 1;
  color: #fff8b3;
  text-shadow: 0 0 6px rgba(253, 224, 71, 0.95), 0 0 2px rgba(255, 255, 255, 0.6);
  display: none;
  z-index: 2;
  pointer-events: none;
}
.battery.charging .battery-charging {
  display: block;
}

/* 百分比文字 */
.battery-percent {
  padding-left: 5px;
  padding-bottom: 2px;
  font-size: 12px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.92);
  letter-spacing: 0.3px;
  white-space: nowrap;
}

/* 接口区域 */
.interface-section {
  margin-bottom: 24px;
}

.interface-section:last-child {
  margin-bottom: 0;
}

.interface-section h4 {
  font-size: 16px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid rgba(255, 255, 255, 0.2);
}

/* 网络接口状态网格布局 */
.interface-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
}

.info-grid-compact {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.info-grid-compact .info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-grid-compact .info-item .label {
  font-size: 11px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.6);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.info-grid-compact .info-item .value {
  font-size: 13px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.9);
  word-break: break-all;
}

/* 流量统计 */
.traffic-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 16px;
}

.traffic-item {
  text-align: center;
  padding: 16px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.traffic-label {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}

.traffic-value {
  font-size: 18px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.9);
}

.traffic-value.upload {
  color: #fc8181;
}

.traffic-value.download {
  color: #68d391;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .page {
    padding:
      calc(12px + var(--app-safe-top, 0px))
      calc(12px + var(--app-safe-right, 0px))
      calc(12px + var(--app-safe-bottom, 0px))
      calc(12px + var(--app-safe-left, 0px));
  }

  .page-header {
    flex-direction: column;
    gap: 16px;
    padding: 16px;
  }

  .title {
    font-size: 24px;
  }

  .uptime-value {
    font-size: 15px;
    line-height: 1.35;
    white-space: normal;
  }

  .controls {
    flex-direction: column;
    gap: 12px;
    width: 100%;
  }

  .top-status-controls {
    justify-content: center;
  }

  .quick-actions-grid {
    grid-auto-flow: row;
    grid-auto-columns: auto;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    width: 80%;
  }

  .content {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .info-grid {
    grid-template-columns: 1fr;
  }

  .interface-grid {
    grid-template-columns: 1fr;
  }

  .info-grid-compact {
    grid-template-columns: 1fr;
  }

  .traffic-stats {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 1100px) {
  .page {
    padding:
      calc(8px + var(--app-safe-top, 0px))
      calc(8px + var(--app-safe-right, 0px))
      calc(8px + var(--app-safe-bottom, 0px))
      calc(8px + var(--app-safe-left, 0px));
  }

  .card-content {
    padding: 16px;
  }

  .device-stats {
    grid-template-columns: 1fr;
  }

  .traffic-item {
    padding: 12px;
  }

  .traffic-value {
    font-size: 16px;
  }
}

.mytable{caption-side: bottom;}
.mytable{border-collapse: collapse;}
.mytable,tr,td{border: 1px solid rgb(59, 104, 141);font-size: 14px;text-align: center;}
.dbmstyle{color: rgb(104, 211, 145)}

.hp {
  color: #d97706;
  font-weight: 600;
}
.psm {
  color: #059669;
  font-weight: 600;
}

.btn-disabled {
  background: #9ca3af;
  border-color: #9ca3af;
  color: #fff;
  cursor: not-allowed;
  opacity: 0.75;
}

/* 表格手机端横向滚动 */
.table-wrapper {
  width: 100%;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.table-wrapper .mytable {
  min-width: 720px;
  width: 100%;
}

/* 手机端仍然保持网络信息双列 */
@media (max-width: 768px) {
  .info-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .info-item {
    padding: 8px;
    min-width: 0;
  }

  .info-item .label {
    font-size: 11px;
  }

  .info-item .value {
    font-size: 12px;
    line-height: 1.35;
    word-break: break-all;
    overflow-wrap: anywhere;
  }

  .card-content {
    padding: 12px;
  }
}
.child {
  width: 100%;
  max-width: 1600px;
  margin: 0 auto;
  padding: 16px;
  box-sizing: border-box;
}

.top-cards {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  width: 100%;
}

.top-cards .card {
  width: 100%;
  min-width: 0;
}

@media (max-width: 900px) {
  .top-cards {
    grid-template-columns: 1fr;
  }
}

.cpu-health-card {
  min-width: 0;
}

.cpu-health-layout {
  display: grid;
  grid-template-columns: 150px 1fr;
  align-items: center;
}

.cpu-pie-box {
  display: flex;
  align-items: center;
  justify-content: center;
}

.cpu-pie {
  width: 115px;
  height: 115px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow:
    inset 0 0 0 1px rgba(255, 255, 255, 0.08),
    0 16px 34px rgba(0, 0, 0, 0.26);
}

.cpu-pie-inner {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: #315697;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.cpu-pie-value {
  font-size: 30px;
  font-weight: 900;
  line-height: 1;
  color: #fff;
}

.cpu-pie-text {
  margin-top: 8px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.62);
}

.cpu-core-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.cpu-core-card {
  min-width: 0;
  padding: 10px 12px;
  padding-bottom: 16px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.045);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.cpu-core-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.cpu-core-name {
  font-size: 12px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.62);
}

.cpu-core-value {
  font-size: 16px;
  font-weight: 900;
  color: #fff;
}

.cpu-core-bar {
  height: 8px;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.12);
}

.cpu-core-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #4facfe 0%, #63e6be 100%);
  transition: width 0.25s ease;
}

.health-card {
  min-width: 0;
  padding: 18px;
  border-radius: 18px;
  background:
    radial-gradient(circle at top left, rgba(99, 230, 190, 0.12), transparent 38%),
    rgba(255, 255, 255, 0.045);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.health-title {
  font-size: 13px;
  font-weight: 800;
  letter-spacing: 0.04em;
  color: rgba(255, 255, 255, 0.68);
  margin-bottom: 16px;
}

.temp-gauges {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.temp-gauge {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.temp-ring {
  --ring-color: #63e6be;

  width: 104px;
  height: 104px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background:
    conic-gradient(
      var(--ring-color) 0 var(--percent),
      rgba(255, 255, 255, 0.13) var(--percent) 100%
    );
  transition: background 0.25s ease;
}

.temp-ring.normal {
  --ring-color: #63e6be;
}

.temp-ring.warning {
  --ring-color: #ffd166;
}

.temp-ring.danger {
  --ring-color: #ff6b6b;
}

.temp-ring-inner {
  width: 74px;
  height: 74px;
  border-radius: 50%;
  background: #315697;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

/* 暗色模式支持 */
@media (prefers-color-scheme: dark) {
  .page {
    color: white;
    background: linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #334155 100%);
  }

  .page-header,
  .card,
  .loading,
  .error,
  .empty {
    background: rgba(15, 23, 42, 0.8);
    border-color: rgba(255, 255, 255, 0.1);
  }

  .title,
  .card-header h3,
  .info-item .value,
  .traffic-value {
    color: #f1f5f9;
  }

  .status-text,
  .info-item .label,
  .traffic-label {
    color: #cbd5e1;
  }

  .card-header {
    background: rgba(30, 41, 59, 0.5);
  }

  .cpu-pie-inner {
    background: #263144;
  }

  .temp-ring-inner {
    background: #263144;
  }

  .interface-section h4 {
    color: #f1f5f9;
    border-bottom-color: rgba(255, 255, 255, 0.1);
  }
}


.temp-ring-inner strong {
  font-size: 22px;
  line-height: 1;
  color: #fff;
}

.temp-ring-inner span {
  margin-top: 6px;
  font-size: 12px;
  color: rgba(255,255,255,0.6);
}

.temp-state {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 800;
  background: rgba(255,255,255,0.08);
}

.temp-state.normal {
  color: #63e6be;
  background: rgba(99, 230, 190, 0.14);
}

.temp-state.warning {
  color: #ffd166;
  background: rgba(255, 209, 102, 0.14);
}

.temp-state.danger {
  color: #ff6b6b;
  background: rgba(255, 107, 107, 0.14);
}

.memory-health-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.memory-big {
  font-size: 46px;
  font-weight: 900;
  line-height: 1;
  color: #ffffff;
}

.memory-detail {
  margin-top: 8px;
  font-size: 18px;
  font-weight: 800;
  color: rgba(255,255,255,0.9);
}

.memory-detail span {
  font-size: 14px;
  color: rgba(255,255,255,0.55);
}

.memory-stack {
  margin-top: 18px;
  height: 16px;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(255,255,255,0.12);
  box-shadow: inset 0 0 0 1px rgba(255,255,255,0.05);
}

.memory-stack-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #63e6be 0%, #4facfe 100%);
}

.memory-caption {
  margin-top: 10px;
  font-size: 12px;
  color: rgba(255,255,255,0.48);
}

@media (max-width: 100px) {
  .cpu-health-layout {
    grid-template-columns: 1fr;
  }

  .cpu-core-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

/* 连接设备数量可点击链接 */
.device-count-link {
  background: none;
  border: none;
  padding: 0;
  font-size: inherit;
  font-weight: inherit;
  color: rgba(147, 210, 255, 0.9);
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 2px;
  transition: color 0.2s ease;
}
.device-count-link:hover {
  color: #ffffff;
}

.local-speedtest-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.local-speedtest-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 14px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.08);
}

.local-speedtest-hint {
  margin-top: 6px;
  color: rgba(255, 255, 255, 0.62);
  font-size: 12px;
  line-height: 1.5;
}

.local-speedtest-url-field {
  flex: 1 1 260px;
  width: min(360px, 100%);
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.local-speedtest-url {
  width: 100%;
}

.local-speedtest-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.local-speedtest-option {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.08);
}

.local-speedtest-option > span {
  color: rgba(255, 255, 255, 0.78);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.local-speedtest-option .el-input-number {
  width: 120px;
}

.local-speedtest-meter {
  padding: 18px;
  border: 1px solid rgba(96, 165, 250, 0.22);
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.2), rgba(14, 165, 233, 0.12));
}

.local-speedtest-value {
  color: #ffffff;
  font-size: 34px;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: 0;
}

.local-speedtest-label {
  margin: 6px 0 14px;
  color: rgba(255, 255, 255, 0.62);
  font-size: 12px;
}

.local-speedtest-chart {
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid rgba(96, 165, 250, 0.22);
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.18);
}
.local-speedtest-chart-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 8px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}
.local-speedtest-chart-head .lst-peak {
  color: #7dd3fc;
  font-weight: 700;
}
.local-speedtest-plot {
  display: flex;
  align-items: flex-start;
}
.lst-ylabels {
  position: relative;
  flex: 0 0 34px;
  width: 34px;
  height: 90px;
  overflow: visible;
}
.lst-ylabels span {
  position: absolute;
  right: 6px;
  transform: translateY(-50%);
  font-size: 10px;
  line-height: 1;
  color: rgba(255, 255, 255, 0.45);
  white-space: nowrap;
}
.lst-main {
  flex: 1 1 auto;
  min-width: 0;
}
.local-speedtest-chart-svg {
  display: block;
  width: 100%;
  height: 90px;
}
.lst-xlabels {
  position: relative;
  height: 14px;
  margin-top: 3px;
  overflow: visible;
}
.lst-xlabels span {
  position: absolute;
  transform: translateX(-50%);
  font-size: 10px;
  line-height: 1;
  color: rgba(255, 255, 255, 0.45);
  white-space: nowrap;
}
.local-speedtest-chart-svg .lst-grid line {
  stroke: rgba(255, 255, 255, 0.1);
  stroke-width: 1;
  vector-effect: non-scaling-stroke;
}
.local-speedtest-chart-svg .lst-line {
  fill: none;
  stroke: #38bdf8;
  stroke-width: 2;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.local-speedtest-chart-svg .lst-area {
  fill: rgba(56, 189, 248, 0.14);
  stroke: none;
}

.local-speedtest-stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}

.local-speedtest-stats > div {
  min-width: 0;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.08);
}

.local-speedtest-stats span {
  display: block;
  margin-bottom: 6px;
  color: rgba(255, 255, 255, 0.58);
  font-size: 12px;
}

.local-speedtest-stats strong {
  display: block;
  color: #ffffff;
  font-size: 15px;
  font-weight: 650;
  word-break: break-word;
}

.local-speedtest-message {
  min-height: 20px;
  color: rgba(255, 255, 255, 0.72);
  font-size: 13px;
}

.system-tools-tabs :deep(.el-tabs__item) {
  color: rgba(255, 255, 255, 0.7);
}
.system-tools-tabs :deep(.el-tabs__item.is-active) {
  color: #ffffff;
}
.system-tools-tabs :deep(.el-tabs__active-bar) {
  background: #7dd3fc;
}
.system-tool-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.sms-forward-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.system-tool-section {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.08);
}
.system-tool-section-title {
  color: rgba(255, 255, 255, 0.86);
  font-size: 13px;
  font-weight: 700;
}
.system-tool-actions,
.system-tool-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}
/* 短信操作按钮：等宽填充，避免 loading 图标导致按钮忽大忽小 */
.system-tool-actions .el-button {
  flex: 1 1 0;
  min-width: 0;
  margin-left: 0 !important;
}
.sms-forward-switches {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.sms-forward-hint {
  color: rgba(255, 255, 255, 0.58);
  font-size: 12px;
  line-height: 1.5;
}
.system-tool-hint {
  margin-top: 6px;
  color: rgba(255, 255, 255, 0.58);
  font-size: 12px;
}
.devui-status-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}
.devui-status-grid > div {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
}
.devui-status-grid span {
  display: block;
  color: rgba(255, 255, 255, 0.56);
  font-size: 12px;
  line-height: 1.4;
}
.devui-status-grid strong {
  display: block;
  margin-top: 4px;
  color: rgba(255, 255, 255, 0.88);
  font-size: 13px;
  line-height: 1.4;
}
.devui-patch-status .el-button {
  margin-top: 8px;
  margin-left: 0;
}
.devui-download-progress {
  margin-top: 8px;
}
.devui-download-progress div {
  margin-top: 4px;
  color: rgba(255, 255, 255, 0.58);
  font-size: 11px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}
.devui-status-grid em {
  display: block;
  margin-top: 4px;
  color: rgba(248, 113, 113, 0.9);
  font-size: 11px;
  font-style: normal;
  line-height: 1.35;
  overflow-wrap: anywhere;
}
.devui-wallpaper-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 14px;
  align-items: center;
}
.devui-wallpaper-preview {
  width: 120px;
  aspect-ratio: 320 / 456;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.22);
  color: rgba(255, 255, 255, 0.46);
  font-size: 12px;
}
.devui-wallpaper-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.devui-wallpaper-actions {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.devui-file-input {
  display: none;
}
.devui-wallpaper-name {
  min-width: 0;
  max-width: 100%;
  color: rgba(255, 255, 255, 0.62);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sms-message-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 280px;
  overflow: auto;
}
.sms-message-item {
  padding: 12px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.18);
}
.sms-message-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
  color: rgba(255, 255, 255, 0.58);
  font-size: 12px;
}
.sms-message-meta strong {
  color: rgba(255, 255, 255, 0.9);
}
.sms-message-content {
  color: rgba(255, 255, 255, 0.86);
  font-size: 13px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}
.system-tool-empty {
  padding: 18px;
  color: rgba(255, 255, 255, 0.56);
  text-align: center;
}
.rc-local-editor :deep(.el-textarea__inner) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  line-height: 1.5;
}

@media (max-width: 560px) {
  .sms-forward-grid {
    grid-template-columns: 1fr;
  }
  .sms-forward-switches {
    grid-template-columns: 1fr;
  }
  .devui-status-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .devui-wallpaper-row {
    grid-template-columns: 1fr;
    justify-items: start;
  }
  .devui-wallpaper-actions {
    width: 100%;
  }
  .local-speedtest-header {
    flex-direction: column;
    gap: 10px;
    padding: 12px;
  }
  .local-speedtest-url-field {
    width: 100%;
    flex: 0 1 auto;
  }
  .local-speedtest-options {
    /* 移动端也保持两列同一行，不堆成两行 */
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .local-speedtest-option {
    padding: 8px 10px;
    gap: 8px;
  }
  .local-speedtest-option > span {
    font-size: 12px;
    flex: 0 0 auto;
  }
  .local-speedtest-option .el-input-number {
    width: auto;
    flex: 1 1 auto;
    min-width: 0;
  }
  .local-speedtest-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .local-speedtest-meter {
    padding: 14px;
  }
  .local-speedtest-value {
    font-size: 28px;
  }
  .local-speedtest-hint {
    font-size: 11px;
    line-height: 1.45;
  }
}

</style>

<!-- 无线设备弹窗的样式必须放在非 scoped 块中：
     Element Plus 的 el-dialog 会 teleport 到 body，
     scoped CSS 的 data-v 属性无法可靠传递到弹窗内部。 -->
<style>
.wireless-dialog.el-dialog {
  --el-text-color-primary: rgba(255, 255, 255, 0.92);
  --el-text-color-regular: rgba(255, 255, 255, 0.82);
  --el-text-color-placeholder: rgba(255, 255, 255, 0.45);
  --el-fill-color-blank: rgba(255, 255, 255, 0.08);
  --el-border-color: rgba(255, 255, 255, 0.18);
  --el-border-color-hover: rgba(255, 255, 255, 0.38);
  --el-input-bg-color: rgba(255, 255, 255, 0.08);
  --el-input-border-color: rgba(255, 255, 255, 0.18);
  --el-input-hover-border-color: rgba(255, 255, 255, 0.38);
  --el-input-focus-border-color: rgba(96, 165, 250, 0.9);
  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 60%, #3b82f6 100%) !important;
  border: 1px solid rgba(255, 255, 255, 0.3) !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4) !important;
  border-radius: 16px !important;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.wireless-dialog.el-dialog .el-dialog__header {
  background: transparent;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding: 18px 22px 14px;
  margin-right: 0;
}
.wireless-dialog.el-dialog .el-dialog__title {
  color: #ffffff !important;
  font-size: 18px;
  font-weight: 600;
}
.wireless-dialog.el-dialog .el-dialog__headerbtn {
  top: 14px;
  right: 14px;
}
.wireless-dialog.el-dialog .el-dialog__headerbtn .el-dialog__close {
  color: rgba(255, 255, 255, 0.65) !important;
  font-size: 20px;
}
.wireless-dialog.el-dialog .el-dialog__headerbtn:hover .el-dialog__close {
  color: #ffffff !important;
}
.wireless-dialog.el-dialog .el-dialog__body {
  padding: 18px 22px;
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  color: rgba(255, 255, 255, 0.9);
}
.wireless-dialog.el-dialog .el-dialog__footer {
  background: transparent;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding: 12px 22px;
}
.wireless-dialog.el-dialog .el-button {
  background: rgba(255, 255, 255, 0.15);
  border: 1px solid rgba(255, 255, 255, 0.3);
  color: rgba(255, 255, 255, 0.9);
}
.wireless-dialog.el-dialog .el-button:hover {
  background: rgba(255, 255, 255, 0.25);
  border-color: rgba(255, 255, 255, 0.5);
  color: #ffffff;
}

/* el-input-number 加减按钮：默认用浅色填充(--el-fill-color-light)，在深色弹窗里很突兀，这里统一成深色玻璃风格 */
.wireless-dialog .el-input-number__decrease,
.wireless-dialog .el-input-number__increase {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.75);
}
.wireless-dialog .el-input-number__decrease:hover,
.wireless-dialog .el-input-number__increase:hover {
  background: rgba(255, 255, 255, 0.18);
  color: #ffffff;
}
.wireless-dialog .el-input-number.is-disabled .el-input-number__decrease,
.wireless-dialog .el-input-number.is-disabled .el-input-number__increase,
.wireless-dialog .el-input-number__decrease.is-disabled,
.wireless-dialog .el-input-number__increase.is-disabled {
  background: rgba(255, 255, 255, 0.04);
  color: rgba(255, 255, 255, 0.3);
}

/* 设备卡片网格 */
.wireless-dialog .wireless-device-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}
.wireless-dialog .wireless-device-item {
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(20px);
  border-radius: 12px;
  padding: 14px 16px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.wireless-dialog .wireless-device-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.2);
}
.wireless-dialog .wireless-device-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}
.wireless-dialog .wireless-device-name {
  font-size: 15px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.95);
  word-break: break-all;
}
.wireless-dialog .wireless-device-type-tag {
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  flex: 0 0 auto;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.wireless-dialog .tag-5g {
  background: rgba(56, 189, 248, 0.18);
  color: #7dd3fc;
  border: 1px solid rgba(56, 189, 248, 0.34);
}
.wireless-dialog .tag-24g {
  background: rgba(72, 187, 120, 0.2);
  color: #7ee787;
  border: 1px solid rgba(72, 187, 120, 0.3);
}
.wireless-dialog .tag-ethernet {
  background: rgba(167, 139, 250, 0.2);
  color: #c4b5fd;
  border: 1px solid rgba(167, 139, 250, 0.36);
}

/* 分段标题（无线 / 有线） */
.wireless-dialog .device-section-spaced {
  margin-top: 20px;
}
.wireless-dialog .device-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 12px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.65);
  text-transform: uppercase;
  letter-spacing: 1.5px;
}
.wireless-dialog .device-section-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 18px;
  padding: 0 6px;
  border-radius: 9px;
  background: rgba(255, 255, 255, 0.14);
  color: #ffffff;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0;
}
.wireless-dialog .wireless-device-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.wireless-dialog .wireless-info-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
}
.wireless-dialog .wireless-info-label {
  color: rgba(255, 255, 255, 0.6);
  flex: 0 0 60px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.wireless-dialog .wireless-info-value {
  color: rgba(255, 255, 255, 0.9);
  word-break: break-all;
  font-weight: 500;
}
/* 信号强度分级配色 */
.wireless-dialog .wireless-info-value.signal-strong { color: #52db5d; font-weight: 700; }
.wireless-dialog .wireless-info-value.signal-good   { color: #2bc8ae; font-weight: 700; }
.wireless-dialog .wireless-info-value.signal-fair   { color: #dc8811; font-weight: 700; }
.wireless-dialog .wireless-info-value.signal-weak   { color: #e03737; font-weight: 700; }

/* 加载状态 spinner 颜色 */
.wireless-dialog .loading-spinner {
  border: 3px solid rgba(255, 255, 255, 0.2);
  border-top-color: #ffffff;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

/* 暗色系统模式 */
@media (prefers-color-scheme: dark) {
  .wireless-dialog.el-dialog {
    background: linear-gradient(135deg, #0f172a 0%, #1e293b 60%, #334155 100%) !important;
    border-color: rgba(255, 255, 255, 0.1) !important;
  }
  .wireless-dialog .wireless-device-item {
    background: rgba(15, 23, 42, 0.7);
    border-color: rgba(255, 255, 255, 0.1);
  }
}

/* ============== Mihomo 弹窗 - 深色玻璃风格 ============== */
.mm-dialog.el-dialog {
  /* 通过覆盖 Element Plus CSS 变量级联到所有子组件 */
  --el-text-color-primary: rgba(255, 255, 255, 0.92);
  --el-text-color-regular: rgba(255, 255, 255, 0.8);
  --el-text-color-secondary: rgba(255, 255, 255, 0.6);
  --el-text-color-placeholder: rgba(255, 255, 255, 0.45);
  --el-bg-color: transparent;
  --el-bg-color-overlay: rgba(255, 255, 255, 0.06);
  --el-bg-color-page: transparent;
  --el-fill-color: rgba(255, 255, 255, 0.08);
  --el-fill-color-light: rgba(255, 255, 255, 0.05);
  --el-fill-color-lighter: rgba(255, 255, 255, 0.03);
  --el-fill-color-blank: transparent;
  --el-border-color: rgba(255, 255, 255, 0.18);
  --el-border-color-light: rgba(255, 255, 255, 0.12);
  --el-border-color-lighter: rgba(255, 255, 255, 0.08);
  --el-border-color-extra-light: rgba(255, 255, 255, 0.05);
  --el-disabled-bg-color: rgba(255, 255, 255, 0.04);
  --el-disabled-text-color: rgba(255, 255, 255, 0.35);
  --el-disabled-border-color: rgba(255, 255, 255, 0.1);

  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 60%, #3b82f6 100%) !important;
  border: 1px solid rgba(255, 255, 255, 0.3) !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4) !important;
  border-radius: 16px !important;
  max-height: 92vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  color: rgba(255, 255, 255, 0.9);
}

/* Header / Footer / Body */
.mm-dialog.el-dialog .el-dialog__header {
  background: transparent;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding: 18px 22px 14px;
  margin-right: 0;
  flex-shrink: 0;
}
.mm-dialog.el-dialog .el-dialog__title {
  color: #ffffff !important;
  font-size: 18px;
  font-weight: 600;
}
.mm-dialog.el-dialog .el-dialog__headerbtn {
  top: 14px;
  right: 14px;
}
.mm-dialog.el-dialog .el-dialog__headerbtn .el-dialog__close {
  color: rgba(255, 255, 255, 0.65) !important;
  font-size: 20px;
}
.mm-dialog.el-dialog .el-dialog__headerbtn:hover .el-dialog__close {
  color: #ffffff !important;
}
.mm-dialog.el-dialog .el-dialog__body {
  padding: 0;
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  color: rgba(255, 255, 255, 0.9);
}
.mm-dialog.el-dialog .el-dialog__footer {
  background: transparent;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding: 12px 22px;
  flex-shrink: 0;
}

/* Tabs */
.mm-dialog .mm-tabs {
  border: none;
  box-shadow: none;
  background: transparent;
}
.mm-dialog .el-tabs--border-card {
  background: transparent;
}
.mm-dialog .el-tabs--border-card > .el-tabs__header {
  background: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  margin-bottom: 0;
  flex-shrink: 0;
}
.mm-dialog .el-tabs--border-card > .el-tabs__header .el-tabs__item {
  color: rgba(255, 255, 255, 0.65);
  border-right: 1px solid rgba(255, 255, 255, 0.06);
  background: transparent;
  transition: color 0.2s ease, background 0.2s ease;
}
.mm-dialog .el-tabs--border-card > .el-tabs__header .el-tabs__item:hover {
  color: rgba(255, 255, 255, 0.95);
  background: rgba(255, 255, 255, 0.05);
}
.mm-dialog .el-tabs--border-card > .el-tabs__header .el-tabs__item.is-active {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1);
  border-right-color: rgba(255, 255, 255, 0.1);
}
.mm-dialog .el-tabs__content {
  padding: 16px;
}

/* 内部 mh-* 组件 */
.mm-dialog .mh-status-card,
.mm-dialog .mh-install-version-card {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 14px;
  backdrop-filter: blur(10px);
}
.mm-dialog .mh-status-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.mm-dialog .mh-meta { font-size: 12px; color: rgba(255, 255, 255, 0.55); }
.mm-dialog .mh-dir { word-break: break-all; }
.mm-dialog .mh-info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.mm-dialog .mh-info-item { display: flex; flex-direction: column; align-items: flex-start; gap: 3px; }
.mm-dialog .mh-info-item > .el-tag { align-self: flex-start; }
.mm-dialog .mh-info-label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.mm-dialog .mh-info-value {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.92);
  font-weight: 500;
  word-break: break-all;
}
.mm-dialog .mh-info-value a {
  color: #7dd3fc;
}
.mm-dialog .mh-info-value a:hover {
  color: #bae6fd;
}
.mm-dialog .mh-control-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
.mm-dialog .mh-autostart-row {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
  margin-top: 12px; padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}
.mm-dialog .mh-output {
  background: rgba(0, 0, 0, 0.4);
  color: #c8d4e8;
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 1.6;
  max-height: 130px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin-top: 10px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
}
.mm-dialog .mh-data-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; }
.mm-dialog .mh-version-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.mm-dialog .mh-progress-area {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 12px;
}
.mm-dialog .mh-table-wrap { overflow-x: auto; -webkit-overflow-scrolling: touch; }
.mm-dialog .mh-config-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 10px; flex-wrap: wrap; gap: 6px;
}
.mm-dialog .mh-config-error {
  color: #fca5a5;
  font-size: 12px;
  margin-bottom: 6px;
  background: rgba(220, 38, 38, 0.15);
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid rgba(220, 38, 38, 0.3);
}
.mm-dialog .mh-config-editor {
  width: 100%;
  height: clamp(180px, 45vh, 340px);
  background: rgba(0, 0, 0, 0.4);
  color: #e4e8ee;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  padding: 10px 12px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.7;
  resize: vertical;
  box-sizing: border-box;
  outline: none;
}
.mm-dialog .mh-config-editor:focus {
  border-color: rgba(125, 211, 252, 0.6);
  box-shadow: 0 0 0 2px rgba(125, 211, 252, 0.15);
}

/* Element Plus 表格暗色覆盖 */
.mm-dialog .el-table {
  background: transparent !important;
  color: rgba(255, 255, 255, 0.9);
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-header-bg-color: rgba(255, 255, 255, 0.06);
  --el-table-row-hover-bg-color: rgba(255, 255, 255, 0.06);
  --el-table-border-color: rgba(255, 255, 255, 0.1);
  --el-table-text-color: rgba(255, 255, 255, 0.85);
  --el-table-header-text-color: rgba(255, 255, 255, 0.7);
}
.mm-dialog .el-table th.el-table__cell,
.mm-dialog .el-table td.el-table__cell {
  background: transparent !important;
  border-bottom-color: rgba(255, 255, 255, 0.08) !important;
}
.mm-dialog .el-table tr {
  background: transparent !important;
}
.mm-dialog .el-table--enable-row-hover .el-table__body tr:hover > td.el-table__cell {
  background: rgba(255, 255, 255, 0.06) !important;
}
.mm-dialog .el-table::before,
.mm-dialog .el-table::after,
.mm-dialog .el-table__inner-wrapper::before,
.mm-dialog .el-table__inner-wrapper::after {
  background: rgba(255, 255, 255, 0.1) !important;
}

/* 默认按钮（无 type）的玻璃风格 */
.mm-dialog .el-button {
  border-color: rgba(255, 255, 255, 0.25);
}
.mm-dialog .el-button:not(.el-button--primary):not(.el-button--success):not(.el-button--warning):not(.el-button--danger):not(.el-button--info) {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.25);
  color: rgba(255, 255, 255, 0.92);
}
.mm-dialog .el-button:not(.el-button--primary):not(.el-button--success):not(.el-button--warning):not(.el-button--danger):not(.el-button--info):hover {
  background: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.45);
  color: #ffffff;
}

/* divider - 弱化为细分隔线 + 小字标签 */
.mm-dialog .el-divider {
  background-color: rgba(255, 255, 255, 0.1);
  margin-top: 18px;
  margin-bottom: 10px;
}
.mm-dialog .el-divider__text {
  background-color: #2a5298;
  color: rgba(255, 255, 255, 0.55);
  padding: 0 10px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 1.5px;
}

/* switch 文字 */
.mm-dialog .el-switch__label {
  color: rgba(255, 255, 255, 0.5);
}
.mm-dialog .el-switch__label.is-active {
  color: #ffffff;
}

/* progress 文字 */
.mm-dialog .el-progress__text {
  color: rgba(255, 255, 255, 0.9) !important;
}
.mm-dialog .el-progress-bar__outer {
  background: rgba(255, 255, 255, 0.12);
}

/* tag 在深色背景下保持可读 */
.mm-dialog .el-tag {
  border-color: rgba(255, 255, 255, 0.18);
}

@media (max-width: 600px) {
  .mm-dialog .el-tabs__content { padding: 12px 10px; }
  .mm-dialog .mh-status-card,
  .mm-dialog .mh-install-version-card { padding: 12px; margin-bottom: 10px; }
  .mm-dialog .mh-info-grid { grid-template-columns: 1fr; gap: 6px; }
  .mm-dialog .mh-control-row { gap: 6px; }
}

/* 暗色系统模式 */
@media (prefers-color-scheme: dark) {
  .mm-dialog.el-dialog {
    background: linear-gradient(135deg, #0f172a 0%, #1e293b 60%, #334155 100%) !important;
    border-color: rgba(255, 255, 255, 0.1) !important;
  }
  .mm-dialog .el-divider__text {
    background-color: #1e293b;
  }
}

/* ============ 数据模式 Select 下拉面板 - 深色玻璃风格 ============ */
/* el-select 的 popper 会 teleport 到 body，必须在非 scoped 块里定位 */
.net-select-popper.el-popper {
  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 60%, #3b82f6 100%) !important;
  border: 1px solid rgba(255, 255, 255, 0.25) !important;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4) !important;
  border-radius: 10px !important;
  padding: 0 !important;
}
.net-select-popper.el-popper .el-select-dropdown {
  background: transparent;
  border: none;
}
.net-select-popper.el-popper .el-select-dropdown__list {
  padding: 4px 0;
}
.net-select-popper.el-popper .el-select-dropdown__item {
  color: rgba(255, 255, 255, 0.85);
  font-size: 13px;
  height: 32px;
  line-height: 32px;
  padding: 0 14px;
  background: transparent;
  transition: background 0.15s ease, color 0.15s ease;
}
.net-select-popper.el-popper .el-select-dropdown__item.is-hovering,
.net-select-popper.el-popper .el-select-dropdown__item:hover {
  background: rgba(255, 255, 255, 0.12) !important;
  color: #ffffff !important;
}
.net-select-popper.el-popper .el-select-dropdown__item.is-selected,
.net-select-popper.el-popper .el-select-dropdown__item.selected {
  color: #7dd3fc !important;
  font-weight: 600 !important;
  background: rgba(255, 255, 255, 0.06) !important;
}
.net-select-popper.el-popper .el-select-dropdown__empty {
  color: rgba(255, 255, 255, 0.55);
  padding: 12px 0;
}
/* popper 小箭头 */
.net-select-popper.el-popper .el-popper__arrow::before {
  background: #2a5298 !important;
  border: 1px solid rgba(255, 255, 255, 0.25) !important;
}

@media (prefers-color-scheme: dark) {
  .net-select-popper.el-popper {
    background: linear-gradient(135deg, #0f172a 0%, #1e293b 60%, #334155 100%) !important;
  }
  .net-select-popper.el-popper .el-popper__arrow::before {
    background: #1e293b !important;
  }
}

.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-section {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  padding: 14px;
  background: rgba(255, 255, 255, 0.05);
}

.settings-section-title {
  font-size: 14px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.92);
  display: flex;
  margin-bottom: 0px;
  margin-top: 5px;
}

.settings-small-title {
  font-size: 13px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.84);
  margin-bottom: 8px;
}

.settings-inline,
.settings-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.network-mode-row {
  gap: 12px;
}

.wireless-dialog .network-mode-row .net-select {
  width: 96px;
  margin-top: 0;
}

.wireless-dialog .network-mode-row .net-select .el-select__wrapper {
  width: 96px;
  height: 26px;
  min-height: 26px;
  padding: 0 8px;
  font-size: 12px;
  border-radius: 7px;
}

.wireless-dialog .network-mode-row .net-select .el-select__selected-item {
  font-size: 12px;
}

.wireless-dialog .network-mode-row .network-mode-apply {
  width: 96px;
  height: 26px;
  padding: 0 12px;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 800;
}

.wifi-tuning-grid {
  display: grid;
  grid-template-columns: minmax(190px, 0.8fr) minmax(260px, 1.4fr) minmax(140px, 0.7fr);
  gap: 12px;
  align-items: stretch;
}

.wifi-tuning-item {
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 10px;
  padding: 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.wifi-tuning-item .settings-section-title,
.wifi-radio-card .settings-section-title {
  margin-bottom: 0;
}

.wifi-tuning-item .el-input {
  width: 100%;
}

.settings-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.settings-help-icon {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #ffffff;
  border: 1px solid rgba(255, 255, 255, 0.45);
  background: rgba(255, 255, 255, 0.12);
  font-size: 12px;
  font-weight: 800;
  line-height: 1;
  cursor: help;
}

.wifi-value-pill {
  flex: 0 0 auto;
  min-width: 54px;
  padding: 3px 8px;
  border-radius: 999px;
  background: rgba(96, 165, 250, 0.18);
  color: #ffffff;
  font-size: 12px;
  font-weight: 800;
  text-align: center;
  border: 1px solid rgba(96, 165, 250, 0.35);
}

.settings-field-label {
  color: rgba(255, 255, 255, 0.9);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.wifi-power-control {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.wifi-distance-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.wifi-distance-option {
  min-width: 0;
  height: 46px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.07);
  color: rgba(255, 255, 255, 0.76);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  font-weight: 800;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
}

.wifi-distance-option small {
  color: rgba(255, 255, 255, 0.5);
  font-size: 11px;
  font-weight: 700;
}

.wifi-distance-option:hover {
  border-color: rgba(96, 165, 250, 0.7);
  background: rgba(96, 165, 250, 0.14);
  color: #ffffff;
}

.wifi-distance-option.active {
  border-color: rgba(96, 165, 250, 0.95);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.95), rgba(14, 165, 233, 0.9));
  color: #ffffff;
}

.wifi-distance-option.active small {
  color: rgba(255, 255, 255, 0.86);
}

.wireless-dialog .settings-panel .el-switch__label {
  color: rgba(255, 255, 255, 0.5);
}

.wireless-dialog .settings-panel .el-switch__label.is-active {
  color: #ffffff;
}

.wireless-dialog .settings-panel .el-slider__runway {
  background: rgba(255, 255, 255, 0.18);
}

.wireless-dialog .settings-panel .el-slider__bar {
  background: #60a5fa;
}

.wireless-dialog .settings-panel .el-slider__button {
  border-color: #ffffff;
}

.wireless-dialog .settings-panel .el-slider {
  --el-slider-main-bg-color: #60a5fa;
  --el-slider-runway-bg-color: rgba(255, 255, 255, 0.18);
}

.settings-select {
  width: min(260px, 100%);
}

.band-checkbox-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.wireless-dialog .band-checkbox-grid .el-checkbox-button {
  margin: 0;
}

.wireless-dialog .band-checkbox-grid .el-checkbox-button__inner {
  min-width: 24px;
  height: 22px;
  padding: 0 10px;
  border-radius: 999px !important;
  border: 1px solid rgba(255, 255, 255, 0.18) !important;
  background: rgba(255, 255, 255, 0.07);
  color: rgba(255, 255, 255, 0.76);
  font-size: 10px;
  font-weight: 600;
  line-height: 20px;
  box-shadow: none !important;
}

.wireless-dialog .band-checkbox-grid .el-checkbox-button__inner:hover {
  border-color: rgba(96, 165, 250, 0.7) !important;
  color: #ffffff;
  background: rgba(96, 165, 250, 0.14);
}

.wireless-dialog .band-checkbox-grid .el-checkbox-button.is-checked .el-checkbox-button__inner {
  border-color: rgba(96, 165, 250, 0.95) !important;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.95), rgba(14, 165, 233, 0.9));
  color: #ffffff;
}

.wireless-dialog .band-checkbox-grid .el-checkbox-button:first-child .el-checkbox-button__inner,
.wireless-dialog .band-checkbox-grid .el-checkbox-button:last-child .el-checkbox-button__inner {
  border-radius: 999px !important;
}

.cell-lock-grid,
.wifi-settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.wifi-radio-card {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.wifi-card-control {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex: 0 0 auto;
  flex-wrap: wrap;
}

.wifi-radio-subtitle {
  color: rgba(255, 255, 255, 0.56);
  font-size: 12px;
}

.wifi-setting-actions {
  margin-top: 2px;
  justify-content: flex-end;
}

.cell-lock-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.wireless-dialog .settings-panel .el-input__wrapper {
  background: rgba(10, 31, 68, 0.24);
  border: 1px solid rgba(255, 255, 255, 0.16);
  box-shadow: none;
}

.wireless-dialog .settings-panel .el-input__wrapper:hover {
  border-color: rgba(255, 255, 255, 0.34);
  box-shadow: none;
}

.wireless-dialog .settings-panel .el-input__wrapper.is-focus {
  border-color: rgba(96, 165, 250, 0.9);
  box-shadow: 0 0 0 1px rgba(96, 165, 250, 0.25);
}

.wireless-dialog .settings-panel .el-input__inner {
  color: rgba(255, 255, 255, 0.92);
}

.wireless-dialog .settings-panel .el-input__inner::placeholder {
  color: rgba(255, 255, 255, 0.45);
}

.wifi-settings-grid .settings-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

@media (max-width: 720px) {
  .wifi-tuning-grid,
  .cell-lock-grid,
  .wifi-settings-grid {
    grid-template-columns: 1fr;
  }

  .wifi-radio-card {
    align-items: flex-start;
    flex-direction: column;
  }

  .wifi-card-control,
  .wifi-setting-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
