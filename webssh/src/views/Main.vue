<template>
  <div class="page">
    <div class="child">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="title-section">
        <h1 class="title">
          <div class="uptime-value">
            已运行{{ formatUptime(deviceInfo.device_uptime as any) }}
          </div>
        </h1>
        <div class="status-indicator">
          <div :class="['status-dot', connectionStatusClass]"></div>
          <span class="status-text">{{ connectionStatusText }}</span>
        </div>
      </div>

      <div class="controls">
        <div style="display: flex; gap: 10px">

          <div style="display: flex; position: relative">
            <span class="uptime-label">{{ netWorkProvider }} {{ networkType }}{{ is5GA ? 'A' : '' }}</span>

          </div>

          <!-- <div style="display: flex; position: relative"> -->
            <!-- <div class="signal-bars">
              <div
                v-for="n in 5"
                :key="n"
                :class="['bar full-bar', { active: n <= signalBars }]"></div>
            </div> -->
            <!-- <span>{{ networkType }}{{ is5GA ? 'A' : '' }}</span> -->
          <!-- </div> -->

          <div style="display: flex; align-items: center">
            <div
              :class="[
                'battery',
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
            <span style="padding-left: 10px" v-if="deviceInfo.bat_percent"
              >{{ deviceInfo.bat_percent }} %</span
            >
          </div>
        </div>

        <div class="auto-refresh-controls" style="display: flex;align-items: center;gap: 10px;flex-wrap: wrap;">
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

          <div class="auto-refresh-controls">
            <button
              class="quick-action-button adb-action-button"
              :class="{ active: usbStatus?.connect == 1 }"
              @click="handleOpenAdbClick" >
              <span class="quick-action-icon">ADB</span>
              <span class="quick-action-copy">
                <span class="quick-action-title">开启 ADB</span>
                <span class="quick-action-subtitle">{{ usbStatus?.connect == 1 ? 'USB 已连接' : '等待 USB 连接' }}</span>
              </span>
            </button>
          </div>

        </div>

        <div class="auto-refresh-controls" style="display: flex;align-items: center;gap: 10px;flex-wrap: wrap;">
          <button
              class="quick-action-button mihomo-action-button"
              :class="{ active: mihomoStatus.running }"
              @click="openMihomoDialog">
              <span class="quick-action-icon">MH</span>
              <span class="quick-action-copy">
                <span class="quick-action-title">Mihomo</span>
                <span class="quick-action-subtitle">{{ mihomoStatus.running ? '代理运行中' : '代理已停止' }}</span>
              </span>
            </button>

          <button
            class="wifi-mode-button"
            :class="{ active: wifiInfo.highPerformance, saving: wifiPsmSaving }"
            :disabled="wifiPsmSaving"
            @click="psmSetHandler(!wifiInfo.highPerformance)"
          >
            <span class="wifi-mode-icon">{{ wifiInfo.highPerformance ? 'HP' : 'PS' }}</span>
            <span class="wifi-mode-copy">
              <span class="wifi-mode-title">WiFi {{ wifiModeText }}</span>
              <span class="wifi-mode-action">{{ wifiPsmSaving ? '切换中...' : wifiButtonText }}</span>
            </span>
          </button>
        </div>
        
        <div v-if="wifiStatus?.main2g_ssid !== wifiStatus?.main5g_ssid" class="auto-refresh-controls" style="display: flex;align-items: center;gap: 10px;flex-wrap: wrap;">
          2.4G-WIFI: {{wifiInfo.wifiStatus24?'开':'关'}}
          <button style="margin-left: 1px;" class="btn" :class="wifiInfo.wifiStatus24 ? 'btn-primary' : 'btn-primary'"
                  @click="wifiStateSetHandler('wlan0',!wifiInfo.wifiStatus24)">{{wifiInfo.wifiStatus24?'关闭':'开启'}}</button>
          5G-WIFI: {{wifiInfo.wifiStatus5?'开':'关'}}
          <button style="margin-left: 1px;" class="btn" :class="wifiInfo.wifiStatus5 ? 'btn-primary' : 'btn-primary'"
                  @click="wifiStateSetHandler('wlan2',!wifiInfo.wifiStatus5)">{{wifiInfo.wifiStatus5?'关闭':'开启'}}</button>
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
      <button class="btn btn-danger" @click="refresh">重试</button>
    </div>

    <!-- 数据展示 -->
    <div v-else-if="dataReady" class="content">
      <div class="top-cards">
        <!-- NR 5G 信号卡片 -->
        <div class="card" v-if="networkType === '5G'">
          <div class="card-header">
            <h3 class="hd">
              <img style="width: 24px" :src="NetworkIcon" alt="" />NR 5G 信号
            </h3>
            <div class="card-tags">
              <span class="tag success">已激活</span>
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
              <img style="width: 24px" :src="NetworkIcon" alt="" />LTE 信号
            </h3>
            <div class="card-tags">
              <span class="tag success">已激活</span>
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

            <!-- <div class="device-item">
              <div class="device-label">
                CPU 负载
                {{
                  (100 - (deviceInfo.cpuinfo?.[0]?.idle as any) || 0).toFixed(2)
                }}
                %
              </div>
              <div class="device-values">
                <div class="load-item">
                  <span class="load-label">核心1</span>
                  <span class="load-value"
                    >{{
                      (
                        100 - (deviceInfo.cpuinfo?.[1]?.idle as any) || 0
                      ).toFixed(0)
                    }}
                    %</span
                  >
                </div>
                <div class="load-item">
                  <span class="load-label">核心2</span>
                  <span class="load-value"
                    >{{
                      (
                        100 - (deviceInfo.cpuinfo?.[2]?.idle as any) || 0
                      ).toFixed(0)
                    }}
                    %</span
                  >
                </div>
                <div class="load-item">
                  <span class="load-label">核心3</span>
                  <span class="load-value"
                    >{{
                      (
                        100 - (deviceInfo.cpuinfo?.[3]?.idle as any) || 0
                      ).toFixed(0)
                    }}
                    %</span
                  >
                </div>
                <div class="load-item">
                  <span class="load-label">核心4</span>
                  <span class="load-value"
                    >{{
                      (
                        100 - (deviceInfo.cpuinfo?.[4]?.idle as any) || 0
                      ).toFixed(0)
                    }}
                    %</span
                  >
                </div>
              </div>
            </div> -->
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
            <div class="auto-refresh-controls info-item" style="align-items: normal;">
              QCI: {{ netAmbr.qci2 || netAmbr.qci1 }}
            </div>
            <div class="auto-refresh-controls info-item" style="align-items: normal;">
              ⬇️ {{ netAmbr.dl.value }} {{ formatSpeedUnit(netAmbr.dl.unit) }}
              ⬆️ {{ netAmbr.ul.value }} {{ formatSpeedUnit(netAmbr.dl.unit) }}
            </div>
            <div class="info-item">
              <span class="label">运营商</span>
              <span class="value">{{ netWorkProvider }}</span>
            </div>
            <div class="info-item">
              <span class="label">网络类型</span>
              <span class="value">{{ networkType }}{{ is5GA ? 'A' : '' }}</span>
            </div>
            <!-- <div class="info-item">
              <span class="label">驻网状态</span>
              <span class="value">{{ d.simcard_roam || '-' }}</span>
            </div> -->
            <!-- <div class="info-item">
              <span class="label">选择模式</span>
              <span class="value">{{ d.net_select_mode || '-' }}</span>
            </div> -->
            <!-- <div class="info-item">
              <span class="label">选择策略</span>
              <span class="value">{{ d.net_select || '-' }}</span>
            </div> -->
            <div class="info-item">
              <span class="label">信号强度</span>
              <div class="signal-bars">
                <div
                  v-for="n in 5"
                  :key="n"
                  :class="['bar', { active: n <= signalBars }]"></div>
              </div>
            </div>
            <div class="info-item">
              <span class="label">连接数量</span>
              <span class="value"
                >有线：{{ lanUserList?.lan_num || '0' }} / 无线：{{
                  lanUserList?.wireless_num || '0'
                }}</span
              >
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
            <!-- <div class="info-item">
              <span class="label">LTE 锁频</span>
              <span class="value">{{ d.lte_band_lock || '-' }}</span>
            </div> -->
            <!-- <div class="info-item">
              <span class="label">NR SA 锁频</span>
              <span class="value">{{ d.nr5g_sa_band_lock || '-' }}</span>
            </div> -->
            <!-- <div class="info-item">
              <span class="label">LTE 频段</span>
              <span
                class="value"
                style="
                  white-space: pre-wrap;
                  word-wrap: break-word;
                  overflow: hidden;
                "
                >{{ d.lte_band || '-' }}</span
              >
            </div> -->
            <div class="info-item">
              <span class="label">ICCID</span>
              <span class="value">{{ simInfo2.sim_iccid ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">IMSI</span>
              <span class="value">{{ simInfo2.sim_imsi ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">IMEI</span>
              <span class="value">{{ simInfo?.values?.imei ?? '-' }}</span>
            </div>
            <!-- <div class="info-item">
              <span class="label">Lock Status</span>
              <span class="value">{{
                simInfo?.values?.lock_status ?? '-'
              }}</span>
            </div> -->
            <div class="info-item">
              <span class="label">Modem MSN</span>
              <span class="value">{{ simInfo?.values?.modem_msn ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">WLAN MAC</span>
              <span class="value">{{
                simInfo?.values?.wlan_mac_address ?? '-'
              }}</span>
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

      
      <!-- 频段与锁定卡片 -->
      <!-- <div class="card">
        <div class="card-header">
          <h3 class="hd">
            <img style="width: 24px" :src="LockIcon" alt="" />频段与锁定
          </h3>
        </div>
        <div class="card-content">
          <div class="info-grid">
            <div class="info-item">
              <span class="label">主载波</span>
              <span class="value">{{
                d.wan_active_band?.toUpperCase() || '-'
              }}</span>
            </div>
            <div class="info-item">
              <span class="label">工作频段</span>
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
              <span class="label">LTE 锁频</span>
              <span class="value">{{ d.lte_band_lock || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">NR SA 锁频</span>
              <span class="value">{{ d.nr5g_sa_band_lock || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">LTE 频段</span>
              <span
                class="value"
                style="
                  white-space: pre-wrap;
                  word-wrap: break-word;
                  overflow: hidden;
                "
                >{{ d.lte_band || '-' }}</span
              >
            </div>
          </div>
        </div>
      </div> -->

      <!-- 标识信息卡片 -->
      <!-- <div class="card">
        <div class="card-header">
          <h3 class="hd">
            <img style="width: 24px" :src="TagIcon" alt="" />标识信息
          </h3>
        </div>
        <div class="card-content">
          <div class="info-grid">
            <div class="info-item">
              <span class="label">ICCID</span>
              <span class="value">{{ simInfo2.sim_iccid ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">IMSI</span>
              <span class="value">{{ simInfo2.sim_imsi ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">IMEI</span>
              <span class="value">{{ simInfo?.values?.imei ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">Lock Status</span>
              <span class="value">{{
                simInfo?.values?.lock_status ?? '-'
              }}</span>
            </div>
            <div class="info-item">
              <span class="label">Modem MSN</span>
              <span class="value">{{ simInfo?.values?.modem_msn ?? '-' }}</span>
            </div>
            <div class="info-item">
              <span class="label">WLAN MAC</span>
              <span class="value">{{
                simInfo?.values?.wlan_mac_address ?? '-'
              }}</span>
            </div>
          </div>
        </div>
      </div> -->

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
    v-model="mihomoDialogVisible"
    title="Mihomo 代理管理"
    width="min(760px, 96vw)"
    :close-on-click-modal="true"
    destroy-on-close
    class="mihomo-dialog">

    <el-tabs v-model="mihomoActiveTab" type="border-card" class="mihomo-tabs">

      <!-- ── Tab 1: 总览 ── -->
      <el-tab-pane label="总览" name="overview">
        <!-- 状态卡片 -->
        <div class="mh-status-card">
          <div class="mh-status-row">
            <el-tag :type="mihomoStatus.running ? 'success' : 'info'" size="large" effect="dark">
              {{ mihomoStatus.running ? '● 运行中' : '○ 已停止' }}
            </el-tag>
            <span v-if="mihomoStatus.running" class="mh-meta">PID {{ mihomoStatus.pid }}</span>
            <span v-if="mihomoStatus.running && mihomoStatus.start_time" class="mh-meta">启动于 {{ mihomoStatus.start_time }}</span>
            <span class="mh-meta mh-dir">{{ mihomoStatus.mihomo_dir }}</span>
          </div>
          <div class="mh-info-grid">
            <div class="mh-info-item">
              <span class="mh-info-label">二进制版本</span>
              <span class="mh-info-value">{{ mihomoStatus.binary_version || '未安装' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">API 状态</span>
              <el-tag v-if="mihomoStatus.running" :type="mihomoStatus.api_reachable ? 'success' : 'warning'" size="small">
                {{ mihomoStatus.api_reachable ? '可访问' : '不可达' }}
              </el-tag>
              <span v-else class="mh-info-value">—</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">API 版本</span>
              <span class="mh-info-value">{{ mihomoStatus.api_version || '—' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">外部控制器</span>
              <span class="mh-info-value">{{ mihomoStatus.external_controller || '—' }}</span>
            </div>
          </div>
        </div>

        <!-- 控制按钮 -->
        <div class="mh-control-row">
          <el-button-group>
            <el-button type="success" :loading="mihomoControlling==='start'" @click="mihomoControl('start')">启动</el-button>
            <el-button type="warning" :loading="mihomoControlling==='restart'" @click="mihomoControl('restart')">重启</el-button>
            <el-button type="danger" :loading="mihomoControlling==='stop'" @click="mihomoControl('stop')">停止</el-button>
            <el-button :loading="mihomoControlling==='reload-ipset'" @click="mihomoControl('reload-ipset')">重载 ipset</el-button>
          </el-button-group>
          <el-button :icon="RefreshIcon" @click="loadMihomoStatus" :loading="mihomoLoadingStatus">刷新</el-button>
        </div>
        <transition name="el-fade-in">
          <pre v-if="mihomoControlOutput" class="mh-output">{{ mihomoControlOutput }}</pre>
        </transition>
      </el-tab-pane>

      <!-- ── Tab 2: 数据更新 ── -->
      <el-tab-pane label="数据更新" name="data">
        <div class="mh-data-header">
          <div class="mh-version-row">
            <span v-if="mihomoVersionInfo.remote_version" class="mh-meta">
              远端：{{ mihomoVersionInfo.remote_version }}&nbsp;|&nbsp;本地：{{ mihomoStatus.local_version || '未知' }}
            </span>
            <el-tag v-if="mihomoVersionInfo.has_update" type="warning" size="small">有更新</el-tag>
            <el-tag v-else-if="mihomoVersionInfo.remote_version && !mihomoVersionInfo.has_update" type="success" size="small">已最新</el-tag>
          </div>
          <div style="display:flex;gap:6px;flex-wrap:wrap">
            <el-button size="small" @click="checkMihomoVersion" :loading="mihomoCheckingVersion">检查更新</el-button>
            <el-button size="small" type="primary"
              :loading="mihomoUpdateStatus.state==='downloading'"
              :disabled="mihomoUpdateStatus.state==='downloading'"
              @click="startMihomoUpdate">一键更新</el-button>
            <el-button v-if="mihomoUpdateStatus.state==='downloading'" size="small" type="danger" @click="cancelMihomoUpdate">取消</el-button>
          </div>
        </div>

        <div v-if="['downloading','done','failed','canceled'].includes(mihomoUpdateStatus.state)" class="mh-progress-area">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px">
            <el-tag :type="mihomoUpdateTagType" size="small">{{ mihomoUpdateLabel }}</el-tag>
            <span style="font-size:12px;color:#606266">{{ mihomoUpdateStatus.msg }}</span>
          </div>
          <el-progress
            v-if="mihomoUpdateStatus.state==='downloading'"
            :percentage="mihomoUpdateStatus.percent"
            :format="() => `${mihomoUpdateStatus.percent}%  ${mihomoUpdateStatus.file_name} [${mihomoUpdateStatus.file_index}/${mihomoUpdateStatus.file_total}]`"
            striped striped-flow :duration="10" />
        </div>

        <div class="mh-table-wrap">
          <el-table :data="mihomoStatus.files" size="small" style="width:100%;margin-top:10px">
            <el-table-column prop="name" label="文件" width="140" />
            <el-table-column prop="desc" label="说明" min-width="80" />
            <el-table-column label="状态" width="64">
              <template #default="scope">
                <el-tag :type="scope.row.exists ? 'success' : 'danger'" size="small">{{ scope.row.exists ? '存在' : '缺失' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="76">
              <template #default="scope">{{ scope.row.exists ? formatMihomoSize(scope.row.size) : '—' }}</template>
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
          <span class="mh-meta">{{ mihomoStatus.mihomo_dir }}/config.yaml</span>
          <div style="display:flex;gap:6px">
            <el-button size="small" :icon="RefreshIcon" @click="loadMihomoConfig" :loading="mihomoConfigLoading">重新加载</el-button>
            <el-button size="small" type="primary" @click="saveMihomoConfig" :loading="mihomoConfigSaving">保存</el-button>
          </div>
        </div>
        <div v-if="mihomoConfigError" class="mh-config-error">{{ mihomoConfigError }}</div>
        <textarea
          v-model="mihomoConfigText"
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
              <span class="mh-info-value">{{ mihomoStatus.binary_version || '未安装' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">远端最新版本</span>
              <span class="mh-info-value">{{ mihomoBinaryVersionInfo.remote_version || '—' }}</span>
            </div>
            <div class="mh-info-item">
              <span class="mh-info-label">安装目录</span>
              <span class="mh-info-value">/data/kano_plugins/mihomo</span>
            </div>
            <div class="mh-info-item">
              <el-tag v-if="mihomoBinaryVersionInfo.has_update" type="warning" size="small">有新版本</el-tag>
              <el-tag v-else-if="mihomoBinaryVersionInfo.remote_version && !mihomoBinaryVersionInfo.has_update" type="success" size="small">已是最新</el-tag>
            </div>
          </div>
          <div style="display:flex;gap:8px;margin-top:12px;flex-wrap:wrap">
            <el-button size="small" @click="checkMihomoBinaryVersion" :loading="mihomoBinaryChecking">检查版本</el-button>
            <el-button size="small" type="primary"
              :loading="mihomoInstallStatus.state==='downloading'"
              :disabled="mihomoInstallStatus.state==='downloading'"
              @click="startMihomoInstall">
              {{ mihomoStatus.binary_version ? '更新二进制' : '安装 Mihomo' }}
            </el-button>
            <el-button v-if="mihomoInstallStatus.state==='downloading'" size="small" type="danger" @click="cancelMihomoInstall">取消</el-button>
          </div>
        </div>

        <!-- 安装进度 -->
        <div v-if="['downloading','done','failed','canceled'].includes(mihomoInstallStatus.state)" class="mh-progress-area" style="margin-top:12px">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px">
            <el-tag :type="mihomoInstallTagType" size="small">{{ mihomoInstallLabel }}</el-tag>
            <span style="font-size:12px;color:#606266">{{ mihomoInstallStatus.msg }}</span>
          </div>
          <el-progress
            v-if="mihomoInstallStatus.state==='downloading'"
            :percentage="mihomoInstallStatus.percent"
            striped striped-flow :duration="10" />
        </div>

        <!-- 卸载区 -->
        <el-divider style="margin:16px 0 10px">卸载</el-divider>
        <div style="display:flex;gap:8px;flex-wrap:wrap">
          <el-button size="small" type="warning" @click="uninstallMihomo('soft')" :loading="mihomoUninstalling==='soft'">
            软卸载（仅删除二进制）
          </el-button>
          <el-button size="small" type="danger" @click="uninstallMihomo('full')" :loading="mihomoUninstalling==='full'">
            完全卸载（删除全部文件）
          </el-button>
        </div>
      </el-tab-pane>

    </el-tabs>

    <template #footer>
      <el-button @click="mihomoDialogVisible = false">关闭</el-button>
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
    wlan_mac_address: string;
  };
}

interface SimInfo2 {
  sim_iccid: string;
  sim_imsi: string;
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

// 请求体定义
// const netInfoRequest = {
//   id: 1,
//   service: 'zte_nwinfo_api',
//   method: 'nwinfo_get_netinfo',
//   params: {},
// };

// const lanRequest = {
//   id: 2,
//   service: 'network.interface.lan',
//   method: 'status',
//   params: {},
// };
//
// const wanRequest = {
//   id: 3,
//   service: 'network.interface.zte_wan',
//   method: 'status',
//   params: {},
// };
//
// const wan6Request = {
//   id: 4,
//   service: 'network.interface.zte_wan6',
//   method: 'status',
//   params: {},
// };
//
// const trafficRequest = {
//   id: 5,
//   service: 'zwrt_data',
//   method: 'get_wwandst',
//   params: { source_module: 'web', cid: 1, type: 4 },
// };
//
// const simInfoRequest = {
//   id: 6,
//   service: 'uci',
//   method: 'get',
//   params: {
//     config: 'zwrt_zte_mdm',
//     section: 'device_info',
//   },
// };
//
// const simInfo2Request = {
//   id: 7,
//   service: 'zwrt_zte_mdm.api',
//   method: 'get_sim_info',
//   params: {},
// };
//
// const deviceInfoRequest = {
//   id: 8,
//   service: 'zwrt_mc.device.manager',
//   method: 'get_device_info',
//   params: {},
// };
//
// const cpuTempRequest = {
//   id: 9,
//   service: 'zwrt_bsp.thermal',
//   method: 'get_cpu_temp',
//   params: {},
// };
//
// const wwanRequest = {
//   id: 10,
//   service: 'zwrt_data',
//   method: 'get_wwaniface',
//   params: {
//     source_module: 'web',
//     cid: 1,
//     connect_status: '',
//   },
// };
//
// const lanUserListRequest = {
//   id: 11,
//   service: 'zwrt_router.api',
//   method: 'router_get_user_list_num',
//   params: {},
// };
//
// const openAdbRequest = {
//   id: 12,
//   service: 'zwrt_bsp.usb',
//   method: 'set',
//   params: {
//     mode: 'debug',
//   },
// };
//
// const closeAdbRequest = {
//   id: 13,
//   service: 'zwrt_bsp.usb',
//   method: 'set',
//   params: {
//     mode: 'user',
//   },
// };

// session 固定值（未登录）
const SESSION_ID = '00000000000000000000000000000000'

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

// 打开 ADB
// const openAdbRequest = {
//   jsonrpc: '2.0',
//   id: 12,
//   method: 'call',
//   params: [
//     SESSION_ID,
//     'zwrt_bsp.usb',
//     'set',
//     {
//       mode: 'debug',
//     },
//   ],
// }

// 关闭 ADB
// const closeAdbRequest = {
//   jsonrpc: '2.0',
//   id: 13,
//   method: 'call',
//   params: [
//     SESSION_ID,
//     'zwrt_bsp.usb',
//     'set',
//     {
//       mode: 'user',
//     },
//   ],
// }

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
  // deviceInfoRequest,
  cpuTempRequest,
  simInfoRequest,
  simInfo2Request,
  wifiStatusRequest,
  sysVersionRequest,
  usbStatusRequest,
  // wwanRequest,
  // lanUserListRequest,
]
const batchRequests2 = [
  deviceInfoRequest,
  wwanRequest,
  lanUserListRequest
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
const d = computed(() => data.value || {});

const signalBars = computed(() => {
  const bars = Number(d.value.signalbar || 0);
  if (Number.isNaN(bars)) return 0;
  return Math.max(0, Math.min(5, bars));
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
  // return (providerMap[Operator] ?? Operator ) + (Operator === provider ? '' : '(' + (providerMap[provider] ?? fullname ?? '') + ')');
});

// API 调用函数
// async function callUbus<T>(request: any): Promise<T> {
//   const response = await axios.post<UbusResponse<T>>('/api/ubus', request);
//   if (response.data.code === 0) {
//     return response.data.result as T;
//   } else {
//     throw new Error(response.data.msg || '接口返回错误');
//   }
// }

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

// async function fetchAllData() {
//   loading.value = true;
//   error.value = null;
//
//   try {
//     // 并行请求所有数据
//     const [
//       netInfo,
//       lan,
//       wan,
//       wan6,
//       traffic,
//       device,
//       cpuTempData,
//       simInfoData,
//       simInfo2Data,
//       wwanInfoData,
//       lanUserData,
//     ] = await Promise.all([
//       callUbus<NetInfoResult>(netInfoRequest),
//       callUbus<NetworkInterface>(lanRequest),
//       callUbus<NetworkInterface>(wanRequest),
//       callUbus<NetworkInterface>(wan6Request),
//       callUbus<TrafficData>(trafficRequest),
//       callUbus<DeviceInfo>(deviceInfoRequest),
//       callUbus<CpuTemp>(cpuTempRequest),
//       callUbus<SimInfo>(simInfoRequest),
//       callUbus<SimInfo2>(simInfo2Request),
//       callUbus<WwanInfo>(wwanRequest),
//       callUbus<LanUserList>(lanUserListRequest),
//     ]);
//
//     data.value = netInfo;
//     lanData.value = lan;
//     wanData.value = wan;
//     wan6Data.value = wan6;
//     trafficData.value = traffic;
//     deviceInfo.value = device;
//     cpuTemp.value = cpuTempData;
//     simInfo.value = simInfoData;
//     simInfo2.value = simInfo2Data;
//     wwanInfo.value = wwanInfoData;
//     lanUserList.value = lanUserData;
//   } catch (e: any) {
//     error.value = e?.message || '请求失败';
//     console.error('数据获取失败:', e);
//   } finally {
//     loading.value = false;
//   }
// }

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
    lanData.value     = resultMap[2]
    wanData.value     = resultMap[3]
    wan6Data.value    = resultMap[4]
    trafficData.value = resultMap[5]
    // deviceInfo.value  = resultMap[6]
    cpuTemp.value     = resultMap[7]
    simInfo.value     = resultMap[8]
    simInfo2.value    = resultMap[9]
    // wwanInfo.value    = resultMap[10]
    // lanUserList.value = resultMap[11]
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
async function fetchAllData2() {
  loading.value = true
  error.value = null
  try {
    const resultMap = await callUbusBatch(batchRequests2)
    // 按 id 取值（清晰又稳定）
    // data.value        = resultMap[1]
    // lanData.value     = resultMap[2]
    // wanData.value     = resultMap[3]
    // wan6Data.value    = resultMap[4]
    // trafficData.value = resultMap[5]
    deviceInfo.value  = resultMap[6]
    // cpuTemp.value     = resultMap[7]
    // simInfo.value     = resultMap[8]
    // simInfo2.value    = resultMap[9]
    wwanInfo.value    = resultMap[10]
    lanUserList.value = resultMap[11]
  } catch (e: any) {
    error.value = e?.message || '请求失败'
    console.error('数据获取失败:', e)
  } finally {
    loading.value = false
  }

}


function refresh() {
  fetchAllData().then((res) => {
    ElMessage.success('数据已刷新');
  });
  fetchAllData2();
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

function updateRefreshInterval() {
  if (autoRefresh.value) {
    stopAutoRefresh();
    startAutoRefresh();
  }
}

function startAutoRefresh() {
  stopAutoRefresh();
  refreshTimer = window.setInterval(() => {
    fetchAllData();
  }, refreshInterval.value);
  refreshTimer2 = window.setInterval(() => {
    fetchAllData2();
  }, refreshInterval2.value);
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

const adbOpened = ref(false)
function handleOpenAdbClick() {
  if (usbStatus?.value.connect != 1) {
    ElMessage.warning('请先连接数据线再试')
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

// function oneClickDebugClose() { ... }

// ─────────────────────────── Mihomo ───────────────────────────

interface MihomoFileInfo { name: string; desc: string; exists: boolean; size: number; mod_time: string }
interface MihomoStatusData {
  running: boolean; pid: number; mihomo_dir: string; local_version: string; files: MihomoFileInfo[]
  binary_version: string; start_time: string; api_reachable: boolean; api_version: string; external_controller: string
}
interface MihomoVersionData { remote_version: string; local_version: string; has_update: boolean }
interface MihomoUpdateStatusData {
  state: string; msg: string; file_name: string; file_index: number
  file_total: number; downloaded: number; total: number; percent: number
}
interface MihomoInstallStatusData {
  state: string; msg: string; downloaded: number; total: number; percent: number
}

const mihomoDialogVisible = ref(false)
const mihomoActiveTab = ref('overview')
const mihomoLoadingStatus = ref(false)
const mihomoControlling = ref('')
const mihomoControlOutput = ref('')
const mihomoCheckingVersion = ref(false)
const mihomoBinaryChecking = ref(false)
const mihomoConfigLoading = ref(false)
const mihomoConfigSaving = ref(false)
const mihomoConfigText = ref('')
const mihomoConfigError = ref('')
const mihomoUninstalling = ref('')
let mihomoUpdatePollTimer: ReturnType<typeof setInterval> | null = null
let mihomoInstallPollTimer: ReturnType<typeof setInterval> | null = null

const mihomoStatus = reactive<MihomoStatusData>({
  running: false, pid: 0, mihomo_dir: '/data/kano_plugins/mihomo', local_version: '',
  files: [], binary_version: '', start_time: '', api_reachable: false, api_version: '', external_controller: ''
})
const mihomoVersionInfo = reactive<MihomoVersionData>({
  remote_version: '', local_version: '', has_update: false
})
const mihomoUpdateStatus = reactive<MihomoUpdateStatusData>({
  state: 'idle', msg: '', file_name: '', file_index: 0, file_total: 0, downloaded: 0, total: 0, percent: 0
})
const mihomoBinaryVersionInfo = reactive<MihomoVersionData>({
  remote_version: '', local_version: '', has_update: false
})
const mihomoInstallStatus = reactive<MihomoInstallStatusData>({
  state: 'idle', msg: '', downloaded: 0, total: 0, percent: 0
})

const mihomoUpdateLabel = computed(() => (({
  downloading: '下载中', done: '已完成', failed: '失败', canceled: '已取消'
} as Record<string, string>)[mihomoUpdateStatus.state] ?? mihomoUpdateStatus.state))

const mihomoUpdateTagType = computed(() => (({
  downloading: 'primary', done: 'success', failed: 'danger', canceled: 'info'
} as Record<string, string>)[mihomoUpdateStatus.state] ?? 'info') as any)

const mihomoInstallLabel = computed(() => (({
  downloading: '下载中', done: '已完成', failed: '失败', canceled: '已取消'
} as Record<string, string>)[mihomoInstallStatus.state] ?? mihomoInstallStatus.state))

const mihomoInstallTagType = computed(() => (({
  downloading: 'primary', done: 'success', failed: 'danger', canceled: 'info'
} as Record<string, string>)[mihomoInstallStatus.state] ?? 'info') as any)

async function loadMihomoStatus() {
  mihomoLoadingStatus.value = true
  try {
    const res = await axios.get('/api/mihomo/status')
    if (res.data.code === 0) Object.assign(mihomoStatus, res.data.data)
  } catch { /* ignore */ } finally {
    mihomoLoadingStatus.value = false
  }
}

function openMihomoDialog() {
  mihomoDialogVisible.value = true
  mihomoActiveTab.value = 'overview'
  loadMihomoStatus()
  // 若后台有数据更新任务在跑，恢复轮询
  axios.get('/api/mihomo/data/update/status').then(res => {
    if (res.data.code === 0) {
      Object.assign(mihomoUpdateStatus, res.data.data)
      if (mihomoUpdateStatus.state === 'downloading') startMihomoUpdatePoll()
    }
  }).catch(() => {})
  // 若后台有安装任务在跑，恢复轮询
  axios.get('/api/mihomo/install/status').then(res => {
    if (res.data.code === 0) {
      Object.assign(mihomoInstallStatus, res.data.data)
      if (mihomoInstallStatus.state === 'downloading') startMihomoInstallPoll()
    }
  }).catch(() => {})
}

// 切换到配置文件 tab 时自动加载
watch(mihomoActiveTab, (tab) => {
  if (tab === 'config' && mihomoConfigText.value === '') {
    loadMihomoConfig()
  }
})

async function mihomoControl(action: string) {
  mihomoControlling.value = action
  mihomoControlOutput.value = ''
  try {
    const res = await axios.post('/api/mihomo/control', { action })
    if (res.data.code === 0) {
      ElMessage.success(action + ' 成功')
      mihomoControlOutput.value = (res.data.output ?? '').trim()
    } else {
      ElMessage.error(res.data.msg)
      mihomoControlOutput.value = (res.data.output ?? res.data.msg ?? '').trim()
    }
    await loadMihomoStatus()
  } catch (e: any) {
    ElMessage.error('请求失败: ' + (e.message ?? e))
  } finally {
    mihomoControlling.value = ''
  }
}

async function checkMihomoVersion() {
  mihomoCheckingVersion.value = true
  try {
    const res = await axios.get('/api/mihomo/data/version')
    if (res.data.code === 0) {
      Object.assign(mihomoVersionInfo, res.data.data)
      mihomoVersionInfo.has_update
        ? ElMessage.warning('新版本：' + mihomoVersionInfo.remote_version)
        : ElMessage.success('已是最新版本')
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('检查失败: ' + (e.message ?? e))
  } finally {
    mihomoCheckingVersion.value = false
  }
}

async function startMihomoUpdate() {
  try {
    const res = await axios.post('/api/mihomo/data/update')
    if (res.data.code === 0) {
      ElMessage.success('开始下载更新...')
      Object.assign(mihomoUpdateStatus, res.data.data)
      startMihomoUpdatePoll()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('启动更新失败: ' + (e.message ?? e))
  }
}

async function cancelMihomoUpdate() {
  try {
    const res = await axios.post('/api/mihomo/data/update/cancel')
    if (res.data.code === 0) ElMessage.info('已取消')
    else ElMessage.error(res.data.msg)
  } catch (e: any) {
    ElMessage.error(e.message ?? e)
  }
}

function startMihomoUpdatePoll() {
  if (mihomoUpdatePollTimer) return
  mihomoUpdatePollTimer = setInterval(async () => {
    try {
      const res = await axios.get('/api/mihomo/data/update/status')
      if (res.data.code !== 0) return
      Object.assign(mihomoUpdateStatus, res.data.data)
      if (mihomoUpdateStatus.state === 'done') {
        ElMessage.success('数据文件更新完成！')
        stopMihomoUpdatePoll()
        loadMihomoStatus()
      } else if (mihomoUpdateStatus.state === 'failed') {
        ElMessage.error('更新失败：' + mihomoUpdateStatus.msg)
        stopMihomoUpdatePoll()
      } else if (mihomoUpdateStatus.state === 'canceled') {
        stopMihomoUpdatePoll()
      }
    } catch { /* ignore */ }
  }, 1000)
}

function stopMihomoUpdatePoll() {
  if (mihomoUpdatePollTimer) { clearInterval(mihomoUpdatePollTimer); mihomoUpdatePollTimer = null }
}

// ── 配置文件 ──

async function loadMihomoConfig() {
  mihomoConfigLoading.value = true
  mihomoConfigError.value = ''
  try {
    const res = await axios.get('/api/mihomo/config')
    if (res.data.code === 0) {
      mihomoConfigText.value = res.data.data?.content ?? ''
    } else {
      mihomoConfigError.value = res.data.msg
    }
  } catch (e: any) {
    mihomoConfigError.value = '加载失败: ' + (e.message ?? e)
  } finally {
    mihomoConfigLoading.value = false
  }
}

async function saveMihomoConfig() {
  mihomoConfigSaving.value = true
  try {
    const res = await axios.put('/api/mihomo/config', { content: mihomoConfigText.value })
    if (res.data.code === 0) {
      ElMessage.success('配置已保存')
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e.message ?? e))
  } finally {
    mihomoConfigSaving.value = false
  }
}

// ── 二进制安装 ──

async function checkMihomoBinaryVersion() {
  mihomoBinaryChecking.value = true
  try {
    const res = await axios.get('/api/mihomo/binary/version')
    if (res.data.code === 0) {
      Object.assign(mihomoBinaryVersionInfo, res.data.data)
      mihomoBinaryVersionInfo.has_update
        ? ElMessage.warning('新版本可用：' + mihomoBinaryVersionInfo.remote_version)
        : ElMessage.success('已是最新版本')
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('检查失败: ' + (e.message ?? e))
  } finally {
    mihomoBinaryChecking.value = false
  }
}

async function startMihomoInstall() {
  try {
    const res = await axios.post('/api/mihomo/install')
    if (res.data.code === 0) {
      ElMessage.success('开始安装...')
      Object.assign(mihomoInstallStatus, res.data.data)
      startMihomoInstallPoll()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('启动安装失败: ' + (e.message ?? e))
  }
}

async function cancelMihomoInstall() {
  try {
    const res = await axios.post('/api/mihomo/install/cancel')
    if (res.data.code === 0) ElMessage.info('已取消')
    else ElMessage.error(res.data.msg)
  } catch (e: any) {
    ElMessage.error(e.message ?? e)
  }
}

function startMihomoInstallPoll() {
  if (mihomoInstallPollTimer) return
  mihomoInstallPollTimer = setInterval(async () => {
    try {
      const res = await axios.get('/api/mihomo/install/status')
      if (res.data.code !== 0) return
      Object.assign(mihomoInstallStatus, res.data.data)
      if (mihomoInstallStatus.state === 'done') {
        ElMessage.success('Mihomo 安装/更新完成！')
        stopMihomoInstallPoll()
        loadMihomoStatus()
      } else if (mihomoInstallStatus.state === 'failed') {
        ElMessage.error('安装失败：' + mihomoInstallStatus.msg)
        stopMihomoInstallPoll()
      } else if (mihomoInstallStatus.state === 'canceled') {
        stopMihomoInstallPoll()
      }
    } catch { /* ignore */ }
  }, 1000)
}

function stopMihomoInstallPoll() {
  if (mihomoInstallPollTimer) { clearInterval(mihomoInstallPollTimer); mihomoInstallPollTimer = null }
}

async function uninstallMihomo(mode: string) {
  const label = mode === 'full' ? '完全卸载（删除全部文件）' : '软卸载（仅删除二进制）'
  try {
    await ElMessageBox.confirm(`确认执行：${label}？此操作不可逆。`, '卸载确认', {
      confirmButtonText: '确认卸载',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch { return }
  mihomoUninstalling.value = mode
  try {
    const res = await axios.post('/api/mihomo/uninstall', { mode })
    if (res.data.code === 0) {
      ElMessage.success('卸载完成')
      await loadMihomoStatus()
    } else {
      ElMessage.error(res.data.msg)
    }
  } catch (e: any) {
    ElMessage.error('卸载失败: ' + (e.message ?? e))
  } finally {
    mihomoUninstalling.value = ''
  }
}

function stopMihomoAllPolls() {
  stopMihomoUpdatePoll()
  stopMihomoInstallPoll()
}

function formatMihomoSize(bytes: number): string {
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

      // psm
      wifiInfo.value.highPerformance = data.wlan2_psm === 'off';
    })
}
function psmSetHandler(val:boolean){
  if (wifiPsmSaving.value) return;
  wifiPsmSaving.value = true;
  axios.post('/api/wifi/psm/set', {
    ifaces: ['wlan0', 'wlan1', 'wlan2', 'wlan3'],
    mode: val ? 'off' : 'on',
  }).then((res) => {
    psmGetHandler()
    ElMessage.success('WiFi已切换为:' + (val ? '高性能模式(据说会降低WiFi延迟)' : '省电模式'));
  }).finally(() => {
    wifiPsmSaving.value = false;
  });
}
function wifiStateSetHandler(iface:string, val:boolean){
  axios.post('/api/wifi/state/set', {
    ifaces: [iface],
    up: val,
  }).then((res) => {
    psmGetHandler()
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

onMounted(() => {
  fetchAllData();
  fetchAllData2();
  if (autoRefresh.value) {
    startAutoRefresh();
  }
  // 获取WiFi状态
  psmGetHandler();
  // 获取签约速率
  netAmbrGetHandler();
  // 静默预取 mihomo 状态（让按钮副标题保持准确）
  loadMihomoStatus();
});

onUnmounted(() => {
  stopAutoRefresh();
  stopMihomoAllPolls();
});
</script>

<style scoped>
/* 基础样式 */
.page {
  color: white;
  min-height: 100vh;
  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 50%, #3b82f6 100%);
  padding: 20px;
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

.auto-refresh-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.quick-action-button,
.wifi-mode-button {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  max-width: 100%;
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

.mihomo-action-button.active {
  border-color: rgba(167, 139, 250, 0.5);
  background: linear-gradient(135deg, rgba(124, 58, 237, 0.28), rgba(14, 165, 233, 0.2));
  box-shadow: 0 6px 18px rgba(124, 58, 237, 0.2);
}

/* Mihomo dialog 内样式 */
.mihomo-dialog :deep(.el-dialog) {
  max-height: 92vh;
  display: flex;
  flex-direction: column;
}
.mihomo-dialog :deep(.el-dialog__header) { flex-shrink: 0; }
.mihomo-dialog :deep(.el-dialog__footer) { flex-shrink: 0; }
.mihomo-dialog :deep(.el-dialog__body) {
  padding: 0;
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}
.mihomo-tabs { border: none; box-shadow: none; }
.mihomo-tabs :deep(.el-tabs__header) { margin-bottom: 0; flex-shrink: 0; }
.mihomo-tabs :deep(.el-tabs__content) { padding: 16px; }

.mh-status-card { background: #f8f9fb; border-radius: 8px; padding: 14px 16px; margin-bottom: 14px; }
.mh-status-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.mh-meta { font-size: 12px; color: #909399; }
.mh-dir { word-break: break-all; }
.mh-info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.mh-info-item { display: flex; flex-direction: column; gap: 2px; }
.mh-info-label { font-size: 11px; color: #c0c4cc; }
.mh-info-value { font-size: 13px; color: #303133; font-weight: 500; word-break: break-all; }
.mh-control-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
.mh-output {
  background: #1a1a2e; color: #a8b4c8; padding: 8px 12px; border-radius: 6px;
  font-size: 11px; line-height: 1.6; max-height: 130px; overflow-y: auto;
  white-space: pre-wrap; word-break: break-all; margin-top: 10px;
}
.mh-data-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; }
.mh-version-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.mh-progress-area { background: #f8f9fb; border-radius: 6px; padding: 10px 12px; }
.mh-table-wrap { overflow-x: auto; -webkit-overflow-scrolling: touch; }
.mh-install-version-card { background: #f8f9fb; border-radius: 8px; padding: 14px 16px; }
.mh-config-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; flex-wrap: wrap; gap: 6px; }
.mh-config-error { color: #f56c6c; font-size: 12px; margin-bottom: 6px; }
.mh-config-editor {
  width: 100%; height: clamp(180px, 45vh, 340px); background: #1e1e1e; color: #d4d4d4;
  border: 1px solid #3c3c3c; border-radius: 6px; padding: 10px 12px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 12px; line-height: 1.7; resize: vertical;
  box-sizing: border-box; outline: none;
}
.mh-config-editor:focus { border-color: #409eff; }

@media (max-width: 600px) {
  .mihomo-tabs :deep(.el-tabs__content) { padding: 10px 8px; }
  .mh-status-card { padding: 10px 12px; margin-bottom: 10px; }
  .mh-info-grid { grid-template-columns: 1fr; gap: 6px; }
  .mh-control-row { gap: 6px; }
  .mh-install-version-card { padding: 10px 12px; }
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
  font-size: 14px;
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
  background: rgba(255, 255, 255, 0.25);
  border-radius: 2px;
  transition: height 0.2s ease, background 0.2s ease;
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

.bar.active:nth-child(1) {
  background: #68d391;
}
.bar.active:nth-child(2) {
  background: #68d391;
}
.bar.active:nth-child(3) {
  background: #68d391;
}
.bar.active:nth-child(4) {
  background: #68d391;
}
.bar.active:nth-child(5) {
  background: #68d391;
}

.battery {
  position: relative;
  width: 30px;
  height: 14px;
  border: 2px solid #ffffffc4;
  border-radius: 3px;
  box-sizing: border-box;
  display: inline-block;
}

.battery-head {
  position: absolute;
  right: -4px;
  top: 1px;
  width: 3px;
  height: 8px;
  background: #ffffffc4;
  border-radius: 0 2px 2px 0;
}

.battery-level {
  height: 100%;
  background: #40d67a; /* 默认绿色 */
  transition: width 0.3s ease, background 0.3s ease;
  border-radius: 1px;
}

.battery-charging {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -55%) scale(1.8) rotate(20deg);
  font-size: 10px;
  color: #fff200;
  display: none; /* 默认不显示 */
}

/* 电量低时变红 */
.battery.low .battery-level {
  background: #f56565;
}

/* 充电状态显示闪电 */
.battery.charging .battery-charging {
  display: block;
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
    padding: 12px;
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

  .auto-refresh-controls {
    justify-content: center;
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
    padding: 8px;
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
</style>
