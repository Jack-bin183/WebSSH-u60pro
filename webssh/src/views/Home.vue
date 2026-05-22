<template>
  <el-container>
    
          <el-header 
            v-show="!data.navCollapsed"
            class="top-nav-header">
            <div class="nav">
              <div style="flex:1">
                <el-button-group>
                  <!-- 打开已存在主机配置 -->
                  <el-button type="primary" :icon="Menu" @click="data.modify_devices_dialog_visible = true"></el-button>

                  <el-button type="primary" @click="newHost" :icon="CirclePlus"></el-button>
                  <!-- 执行命令及收藏 -->
                  <el-popover placement="bottom" trigger="click" width="fit-content">
                    <template #reference>
                      <el-button type="primary" :icon="CopyDocument"></el-button>
                    </template>
                    <el-form :model="cmd">
                      <el-form-item label="执行命令">
                        <el-input v-model="cmd.data" type="textarea" autocomplete="off" placeholder="命令或脚本" />
                      </el-form-item>
                      <el-row>
                        <el-form-item label="会话选择">
                          <el-radio-group v-model="cmd.node">
                            <el-radio value="current">当前会话</el-radio>
                            <el-radio value="all">所有会话</el-radio>
                          </el-radio-group>
                        </el-form-item>
                      </el-row>
                      <el-row>
                        <el-form-item>
                          <el-input v-model="cmd.name" maxlength="32" show-word-limit placeholder="收藏该命令,在此输名称">
                            <template #append>
                              <el-button-group style="color:blue;display:flex">
                                <el-button @click="addCmdNote">收藏</el-button>
                                <el-button @click="execCmd">执行</el-button>
                              </el-button-group>
                            </template>
                          </el-input>
                        </el-form-item>
                      </el-row>
                    </el-form>
                  </el-popover>

                  <!-- 命令收藏列表 -->
                  <el-popover placement="bottom" trigger="click" :width="'95%'" :style="{ maxWidth: '800px' }">
                    <template #reference>
                      <el-button type="primary" :icon="Star"></el-button>
                    </template>

                    <div style="overflow-x: auto;">
                      <el-table :data="filterCmdNoteTable" :height="260" style="min-width: 600px;">
                        <el-table-column sortable width="180" :show-overflow-tooltip="true" property="cmd_name"
                          label="名称"></el-table-column>

                        <el-table-column sortable property="cmd_data" label="命令">
                          <template #default="scope">
                            <el-popover effect="light" trigger="hover" placement="right" :width="'auto'">
                              <template #default>
                                <div style="word-break: break-all;">命令详情</div>
                                <el-input v-model="scope.row.cmd_data" type="textarea" :autosize="{ minRows: 4, maxRows: 20 }"
                                  :disabled="true" style="width: 100%; min-width: 300px;" />
                                <div style="margin-top: 10px;">
                                  <el-button-group style="display: flex; flex-wrap: wrap; gap: 5px;">
                                    <el-tooltip effect="dark" content="执行命令,发送到所有会话" placement="top-start">
                                      <el-button type="warning" @click="execCmdAllSession(scope.row)"
                                        style="flex: 1 1 auto;">发送所有会话</el-button>
                                    </el-tooltip>
                                    <el-tooltip effect="dark" content="执行命令,发送到当前会话" placement="top-start">
                                      <el-button type="primary" @click="execCmdCurrentSession(scope.row)"
                                        style="flex: 1 1 auto;">发送当前会话</el-button>
                                    </el-tooltip>
                                  </el-button-group>
                                </div>
                              </template>
                              <template #reference>
                                {{ scope.row.cmd_data.substring(0, 15) + "..." }}
                              </template>
                            </el-popover>
                          </template>
                        </el-table-column>

                        <el-table-column label="操作" fixed="right" width="320">
                          <template #header>
                            <el-input v-model="searchCmdNote" placeholder="名称搜索" style="width: 100%;" />
                          </template>
                          <template #default="scope">
                            <el-button-group style="display: flex; flex-wrap: wrap; gap: 5px;">
                              <el-popconfirm confirmButtonText="删除" cancelButtonText="取消" icon="el-icon-info" iconColor="red"
                                title="确定删除吗" @confirm="delCmdNote(scope.row.id)">
                                <template #reference>
                                  <el-button type="danger" style="flex: 1 1 auto;">删除</el-button>
                                </template>
                              </el-popconfirm>

                              <el-tooltip effect="dark" content="执行命令,发送到所有会话" placement="top-start">
                                <el-button type="warning" @click="execCmdAllSession(scope.row)"
                                  style="flex: 1 1 auto;">发送所有会话</el-button>
                              </el-tooltip>
                              <el-tooltip effect="dark" content="执行命令,发送到当前会话" placement="top-start">
                                <el-button type="primary" @click="execCmdCurrentSession(scope.row)"
                                  style="flex: 1 1 auto;">发送当前会话</el-button>
                              </el-tooltip>
                            </el-button-group>
                          </template>
                        </el-table-column>
                      </el-table>
                    </div>
                  </el-popover>
                </el-button-group>
              </div>
              <div class="right" style="text-align: right">
                <el-button-group>
                  <el-button type="primary" :icon="Upload" :loading="chkingUpdate" @click="checkUpdate"></el-button>

                  <el-button type="primary" :icon="Avatar" @click="data.modify_pwd_dialog_visible = true"></el-button>

                  <!-- admin 角色才能管理 -->
                  <el-button v-if="globalStore.isAdmin === 'Y'" type="danger" :icon="Tools" @click="toManage"></el-button>
                  <el-popconfirm confirmButtonText="退出" cancelButtonText="取消" icon="el-icon-info" iconColor="red"
                    title="确定退出吗" @confirm="logout">
                    <template #reference>
                      <el-button :icon="SwitchButton" type="danger"></el-button>
                    </template>
                  </el-popconfirm>
                </el-button-group>
              </div>
            </div>

            <!-- 展开状态：向上箭头，点击收起 -->
            <button
              class="nav-anchor-toggle nav-anchor-toggle-open"
              @click="data.navCollapsed = true"
              title="收起导航栏"
            >
              <span class="nav-anchor-icon">
                <el-icon>
                  <ArrowUp />
                </el-icon>
              </span>
            </button>
          </el-header>

          <!-- 折叠状态：只显示一个向下箭头 -->
          <button
            v-show="data.navCollapsed"
            class="nav-anchor-toggle nav-anchor-toggle-closed"
            @click="data.navCollapsed = false"
            title="展开导航栏"
          >
            <span class="nav-anchor-icon">
              <el-icon>
                <ArrowDown />
              </el-icon>
            </span>
          </button>

          <div>
            <el-dialog
              v-model="updateProgressDialogVisible"
              title="正在更新 WebSSH"
              width="92%"
              style="max-width: 460px;"
              custom-class="modern-dialog update-progress-dialog"
              :close-on-click-modal="false"
              :close-on-press-escape="false"
              :show-close="updateProgress.state === 'failed'"
            >
              <div class="update-progress-panel">
                <div class="update-status-line">
                  <span class="update-status-title">{{ updateProgress.msg || updateStateText }}</span>
                  <el-tag :type="updateProgress.state === 'failed' ? 'danger' : 'primary'" effect="dark">
                    {{ updateStateText }}
                  </el-tag>
                </div>

                <el-progress
                  :percentage="updatePercent"
                  :status="updateProgress.state === 'failed' ? 'exception' : updateProgress.state === 'restarting' ? 'success' : undefined"
                  :stroke-width="12"
                  striped
                  striped-flow
                />

                <div class="update-meta-grid">
                  <div class="update-meta-item">
                    <span>连接方式</span>
                    <strong>{{ updateProgress.mode || "等待连接" }}</strong>
                  </div>
                  <div class="update-meta-item">
                    <span>请求域名</span>
                    <strong>{{ updateProgress.domain || "-" }}</strong>
                  </div>
                  <div class="update-meta-item">
                    <span>更新文件</span>
                    <strong>{{ updateProgress.asset_name || "-" }}</strong>
                  </div>
                  <div class="update-meta-item">
                    <span>文件大小</span>
                    <strong>{{ formatUpdateSize(updateProgress.total) }}</strong>
                  </div>
                </div>

                <div class="update-transfer">
                  <span>{{ formatUpdateSize(updateProgress.downloaded) }} / {{ formatUpdateSize(updateProgress.total) }}</span>
                  <span>{{ updatePercent }}%</span>
                </div>
              </div>

              <template #footer>
                <div class="dialog-footer">
                  <el-button
                    v-if="updateProgress.state === 'failed'"
                    type="primary"
                    @click="updateProgressDialogVisible = false"
                  >
                    关闭
                  </el-button>
                  <el-button v-else disabled>
                    更新过程中请不要关闭页面
                  </el-button>
                </div>
              </template>
            </el-dialog>

            <el-dialog
              :title="'主机管理'"
              v-model="data.modify_devices_dialog_visible"
              :width="'95%'"
              :style="{ maxWidth: '780px', top: '20px' }"
              custom-class="modern-dialog host-manage-dialog"
              :modal-append-to-body="true"
              :destroy-on-close="true"
              :center="false"
              :fullscreen="isMobile"
            >
              <!-- 搜索输入 -->
              <el-input v-model="searchHost" placeholder="名称及主机搜索" clearable style="margin-bottom: 10px; width: 100%;" />

              <el-table :data="filterHostTable" :height="tableHeight" :show-overflow-tooltip="true" style="width: 100%;">
                <el-table-column sortable fixed="left" width="150" property="name" label="名称" />
                <el-table-column sortable width="150" property="address" label="主机" />
                <el-table-column sortable width="100" property="user" label="用户" />
                <el-table-column sortable width="90" property="port" label="端口" />
                <el-table-column label="操作" fixed="right" width="250">
                  <template #default="scope">
                    <el-button size="small" @click="editHost(scope.row)">编辑</el-button>
                    <el-popconfirm confirmButtonText="删除" cancelButtonText="取消" icon="el-icon-info" iconColor="red"
                      title="确定删除吗" @confirm="deleteHost(scope.row)">
                      <template #reference>
                        <el-button size="small" type="danger">删除</el-button>
                      </template>
                    </el-popconfirm>
                    <el-button size="small" type="primary" @click="connectHost(scope.row)">连接</el-button>
                  </template>
                </el-table-column>
              </el-table>

              <span slot="footer" class="dialog-footer">
                <el-button style="margin-top: 10px;" @click="data.modify_devices_dialog_visible = false">关闭</el-button>
              </span>
            </el-dialog>

            <!-- 修改密码 -->
            <el-dialog
              v-model="data.modify_pwd_dialog_visible"
              title="修改密码"
              custom-class="modern-dialog password-dialog"
              style="max-width: 360px;"
              width="100%"
              center
            >
              <el-form>
                <el-form-item>
                  <el-input v-model="data.new_pwd_one" trim type="password" minlength="3" maxlength="64" show-word-limit
                    show-password clearable placeholder="输入新密码">
                    <template #prepend>输入新密码</template>
                  </el-input>
                </el-form-item>
                <el-form-item>
                  <el-input v-model="data.new_pwd_two" trim type="password" minlength="3" maxlength="64" show-word-limit
                    show-password clearable placeholder="确认新密码">
                    <template #prepend>确认新密码</template>
                  </el-input>
                </el-form-item>
              </el-form>
              <template #footer>
                <div class="dialog-footer">
                  <el-button @click="data.modify_pwd_dialog_visible = false">取消</el-button>
                  <el-button type="primary" @click="modifyPassword">
                    提交
                  </el-button>
                </div>
              </template>
            </el-dialog>

            <!-- SSH主机配置弹窗 -->
            <el-dialog
              :title="data.mode == 0 ? '新增主机' : '更新主机'"
              v-model="data.host_dialog_visible"
              :width="'95%'"
              :style="{ maxWidth: '1040px', top: '20px' }"
              custom-class="modern-dialog host-config-dialog"
            >
              <el-form label-width="80px" ref="host_from">
                <el-collapse v-model="data.host_config_collapse">
                  <el-collapse-item title="基础配置" name="1">
                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="16">
                        <el-form-item label="名称" prop="name">
                          <el-input v-model.trim="data.name" minlength="1" maxlength="30" show-word-limit
                            placeholder="请输入名称"></el-input>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="16">
                        <el-form-item label="主机" prop="address">
                          <el-input v-model.trim="data.address" minlength="1" maxlength="60" show-word-limit
                            placeholder="请输入主机地址"></el-input>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="8">
                        <el-form-item label="网络" prop="net_type">
                          <el-radio-group v-model="data.net_type">
                            <el-radio value="tcp4">IPv4</el-radio>
                            <el-radio value="tcp6">IPv6</el-radio>
                          </el-radio-group>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="16">
                        <el-form-item label="用户" prop="user">
                          <el-input minlength="1" maxlength="60" v-model.trim="data.user" show-word-limit
                            placeholder="请输入用户名"></el-input>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="8">
                        <el-form-item label="端口" prop="port">
                          <el-input-number v-model="data.port" :min="1" :max="65535" style="width: 100%;"></el-input-number>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row>
                      <el-form-item label="认证方式">
                        <el-radio-group v-model="data.auth_type">
                          <el-radio value="pwd">密码</el-radio>
                          <el-radio value="cert">密钥</el-radio>
                        </el-radio-group>
                      </el-form-item>
                    </el-row>

                    <el-row :gutter="10" v-if="data.auth_type === 'cert'">
                      <el-col :xs="24" :sm="16">
                        <el-form-item label="密钥">
                          <el-input v-model="data.cert_data" type="textarea" placeholder="请输入密钥内容或上传"></el-input>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="8">
                        <el-form-item label="上传">
                          <el-button type="primary" @click="addCertFile">上传密钥文件</el-button>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="16">
                        <el-form-item v-if="data.auth_type === 'cert'" label="密钥口令" prop="cert_pwd">
                          <el-input v-model.trim="data.cert_pwd" type="password" show-password show-word-limit
                            placeholder="密钥口令"></el-input>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="16">
                        <el-form-item v-if="data.auth_type === 'pwd'" label="SSH密码" prop="pwd">
                          <el-input v-model.trim="data.pwd" type="password" show-password show-word-limit
                            placeholder="SSH密码"></el-input>
                        </el-form-item>
                      </el-col>
                    </el-row>
                  </el-collapse-item>

                  <el-collapse-item title="高级配置" name="2">
                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="9">
                        <el-form-item label="终端类型" prop="pty_type">
                          <el-select v-model="data.pty_type" placeholder="请选择终端类型" style="width: 100%;">
                            <el-option label="xterm-256color" value="xterm-256color" />
                            <el-option label="linux" value="linux" />
                            <el-option label="xtrem" value="xtrem" />
                          </el-select>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="5">
                        <el-form-item label="字体颜色" prop="foreground">
                          <el-color-picker v-model="data.foreground"></el-color-picker>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="5">
                        <el-form-item label="背景颜色" prop="background">
                          <el-color-picker v-model="data.background"></el-color-picker>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="5">
                        <el-form-item label="光标颜色" prop="cursor_color">
                          <el-color-picker v-model="data.cursor_color"></el-color-picker>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row :gutter="10">
                      <el-col :xs="24" :sm="9">
                        <el-form-item label="字体">
                          <el-select v-model="data.font_family" placeholder="请选择字体" style="width: 100%;">
                            <el-option label="Courier" value="Courier" />
                            <el-option label="Courier New" value="Courier New" />
                            <el-option label="Menlo" value="Menlo" />
                            <el-option label="Monaco" value="Monaco" />
                            <el-option label="monospace" value="monospace" />
                          </el-select>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="5">
                        <el-form-item label="字体大小">
                          <el-select v-model.number="data.font_size" placeholder="请选择字体大小" style="width: 100%;">
                            <el-option v-for="n in [8, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34]" :key="n" :label="n"
                              :value="n"></el-option>
                          </el-select>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="4">
                        <el-form-item label="光标样式">
                          <el-select v-model="data.cursor_style" placeholder="请选择光标样式" style="width: 100%;">
                            <el-option label="块状" value="block" />
                            <el-option label="下划线" value="underline" />
                            <el-option label="竖线" value="bar" />
                          </el-select>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row>
                      <el-col :xs="24">
                        <el-form-item label="连接命令">
                          <el-input v-model="data.init_cmd" type="textarea" :rows="2" placeholder="请输入连接后执行命令"
                            style="width: 100%;" />
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-row>
                      <el-col :xs="24">
                        <el-form-item label="连接横幅">
                          <el-input v-model="data.init_banner" type="textarea" :rows="2" placeholder="请输入连接后提示横幅"
                            style="width: 100%;" />
                        </el-form-item>
                      </el-col>
                    </el-row>
                  </el-collapse-item>
                </el-collapse>
              </el-form>

              <template #footer>
                <span class="dialog-footer">
                  <el-button style="margin-top: 10px;" @click="data.host_dialog_visible = false">取消</el-button>
                  <el-button style="margin-top: 10px;" type="success" @click="connect">连接</el-button>
                </span>
                <div style="width: 10px;display:inline-block"></div>
                <span v-if="data.mode == 0" class="dialog-footer">
                  <el-button style="margin-top: 10px;" type="primary" @click="createHost(false)">保存</el-button>
                  <el-button style="margin-top: 10px;" type="primary" @click="createHost(true)">连接并保存</el-button>
                </span>
                <span v-if="data.mode == 1" class="dialog-footer">
                  <el-button style="margin-top: 10px;" type="primary" @click="updateHost(false)">更新</el-button>
                  <el-button style="margin-top: 10px;" type="primary" @click="updateHost(true)">连接并更新</el-button>
                </span>
              </template>
            </el-dialog>

            <!-- SSH文件上传下载弹窗 -->
            <el-dialog
              v-model="data.file_dialog_visible"
              width="80%"
              custom-class="modern-dialog file-dialog"
              top="60px"
            >
              <template #header>
                <span v-html="title"></span>
              </template>

              <el-button-group style="width:auto;display: flex; flex-wrap: nowrap;overflow-x: auto;">
                <el-button v-for="(item, index) in data.dir_info.paths" :key="index"
                  @click="listDir(item.dir, data.current_host)">{{ item.name }}</el-button>
              </el-button-group>
              </br>

              <el-form-item style="margin-top: 10px;">
                <el-input v-model="data.sftp_current_dir" style="width: 100%;" placeholder="请输入路径" class="input-with-select">
                  <template #append>
                    <el-button-group style="color:blue">
                      <el-button @click="listDir(data.sftp_current_dir, data.current_host)">进入</el-button>
                      <el-button @click="uploadFile(data.sftp_current_dir)">上传</el-button>
                      <el-button @click="createFile(data.sftp_current_dir)">新建文件</el-button>
                      <el-button @click="createDir(data.sftp_current_dir, data.current_host)">创建目录</el-button>
                      <el-button @click="listDir(data.sftp_current_dir, data.current_host)">刷新</el-button>
                    </el-button-group>
                  </template>
                </el-input>
              </el-form-item>
              </br>

              <el-row>
                <el-col :span="24">
                  <el-progress v-if="data.sftp_upload_visible" :percentage="data.sftp_upload_percentage" />
                </el-col>
              </el-row>

              <el-table :data="data.dir_info.files" height="400" :show-overflow-tooltip="true">
                <el-table-column prop="name" label="文件名" fixed="left" sortable>
                  <template #default="scope">
                    <el-button v-if="scope.row.type === 'f'" @click="downloadFile(scope.row)" type="primary" link
                      :icon="Files" style="color: green">{{ scope.row.name }}</el-button>
                    <el-button v-if="scope.row.type === 'd'" @click="listDir(scope.row.path, data.current_host)"
                      type="primary" link :icon="FolderOpened">{{ scope.row.name }}</el-button>
                  </template>
                </el-table-column>
                <el-table-column prop="size" label="大小" width="100" sortable></el-table-column>
                <el-table-column prop="mode" label="权限" width="100" sortable></el-table-column>
                <el-table-column prop="mod_time" label="修改日期" width="180" sortable></el-table-column>
                <el-table-column label="操作" width="360" fixed="right">
                  <template #default="scope">
                    <el-button-group style="display: flex; flex-wrap: nowrap;">
                      <el-button v-if="scope.row.type == 'f'" @click="downloadFile(scope.row)" type="success"
                        :icon="Bottom">下载</el-button>
                      <el-button v-else type="primary" :icon="Upload" @click="uploadFile(scope.row.path)">上传</el-button>
                      <el-button v-if="scope.row.type == 'f' && scope.row.size <= SFTP_EDIT_MAX_BYTES" type="primary" @click="openEditor(scope.row)">编辑</el-button>
                      <el-button v-if="scope.row.type == 'd'" type="success" @click="compressDir(scope.row)">压缩</el-button>
                      <el-button v-if="isArchiveFile(scope.row)" type="warning" @click="openExtractDialog(scope.row)">解压</el-button>
                      <el-button type="warning" @click="changePermission(scope.row)">权限</el-button>
                      <el-button type="primary" @click="renameFile(scope.row)">重命名</el-button>
                      <el-popconfirm confirmButtonText="删除" cancelButtonText="取消" icon="el-icon-info" iconColor="red"
                        title="确定删除吗" @confirm="deleteFile(scope.row)">
                        <template #reference>
                          <el-button type="danger">删除</el-button>
                        </template>
                      </el-popconfirm>
                    </el-button-group>
                  </template>
                </el-table-column>
              </el-table>
            </el-dialog>

            <el-dialog
              v-model="data.compress_dialog_visible"
              :title="'正在压缩: ' + data.compress_name"
              width="92%"
              style="max-width: 520px;"
              custom-class="modern-dialog file-compress-dialog"
              :close-on-click-modal="data.compress_status === 'exception'"
              :close-on-press-escape="data.compress_status === 'exception'"
              :show-close="data.compress_status === 'exception'"
            >
              <el-progress
                :percentage="data.compress_percentage"
                :status="data.compress_status === 'exception' ? 'exception' : data.compress_percentage >= 100 ? 'success' : undefined"
                :stroke-width="12"
                striped
                striped-flow
              />
              <div class="compress-progress-text">
                {{ data.compress_status_text }}
              </div>
              <template #footer>
                <span class="dialog-footer">
                  <el-button
                    v-if="data.compress_status === 'exception'"
                    type="primary"
                    @click="data.compress_dialog_visible = false"
                  >
                    关闭
                  </el-button>
                </span>
              </template>
            </el-dialog>

            <el-dialog
              v-model="data.extract_dialog_visible"
              :title="'解压: ' + data.extract_file_name"
              width="92%"
              style="max-width: 560px;"
              custom-class="modern-dialog file-extract-dialog"
              :close-on-click-modal="false"
            >
              <el-form-item label="解压路径">
                <el-input v-model="data.extract_dst_path" placeholder="请输入解压路径" />
              </el-form-item>
              <template #footer>
                <span class="dialog-footer">
                  <el-button @click="data.extract_dialog_visible = false">取消</el-button>
                  <el-button type="primary" :loading="data.extracting" @click="extractArchive">解压</el-button>
                </span>
              </template>
            </el-dialog>

            <el-dialog
              v-model="data.permission_dialog_visible"
              :title="'设置权限: ' + data.permission_path"
              width="92%"
              style="max-width: 520px;"
              custom-class="modern-dialog file-permission-dialog"
            >
              <div class="permission-grid">
                <div></div>
                <div>读取</div>
                <div>写入</div>
                <div>执行</div>
                <div>所有者</div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.owner.read" /></div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.owner.write" /></div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.owner.execute" /></div>
                <div>用户组</div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.group.read" /></div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.group.write" /></div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.group.execute" /></div>
                <div>所有人</div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.other.read" /></div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.other.write" /></div>
                <div class="permission-check"><el-checkbox v-model="data.permission_bits.other.execute" /></div>
              </div>
              <template #footer>
                <span class="dialog-footer">
                  <el-button @click="data.permission_dialog_visible = false">取消</el-button>
                  <el-button type="primary" :loading="data.permission_saving" @click="savePermission">保存</el-button>
                </span>
              </template>
            </el-dialog>

            <el-dialog
              v-model="data.editor_dialog_visible"
              :title="'编辑文件: ' + data.editor_path"
              width="80%"
              custom-class="modern-dialog file-editor-dialog"
              top="50px"
              :close-on-click-modal="false"
            >
              <el-input
                v-model="data.editor_content"
                type="textarea"
                :autosize="{ minRows: 18, maxRows: 28 }"
                resize="vertical"
                spellcheck="false"
                style="font-family: Consolas, Monaco, monospace;"
              />
              <template #footer>
                <span class="dialog-footer">
                  <el-button @click="data.editor_dialog_visible = false">取消</el-button>
                  <el-button type="primary" :loading="data.editor_saving" @click="saveEditor">保存</el-button>
                </span>
              </template>
            </el-dialog>

            <!-- 管理 -->
            <el-dialog title="系统管理" v-model="data.manage_dialog_visible" v-bind:fullscreen="true">
              <Manage></Manage>
            </el-dialog>
          </div>
    
          

    <!-- 主内容页 -->
    <div v-if="data.host_tabs.length === 0"
      style="background: linear-gradient(135deg, #1e3c72 0%, #2a5298 50%, #3b82f6 100%);">
      <Main></Main>
    </div>


    <div v-else>
      <el-tabs v-model="data.current_host.session_id" type="card" closable @tab-remove="removeTab"
        @tab-click="selectTab">
        <el-tab-pane v-for="item in data.host_tabs" :key="item.session_id" :label="item.name" :name="item.session_id">
          <template #label>
            <el-button-group style="width:auto;display: flex; flex-wrap: nowrap;overflow-x: auto;">
              <el-popover placement="bottom" :width="400" trigger="hover">
                <template #reference>
                  <el-button :type="item.session_id === data.current_host.session_id
                    ? 'primary' : 'info'">
                    <span v-if="item.is_close" style="color:red">{{ item.name }}</span>
                    <span v-else="item.is_close" style="color:white">{{ item.name }}</span>
                  </el-button>
                </template>
                <div>
                  <div style="padding-top: 5px;">
                    <el-button-group>
                      <el-button type="primary" @click="connectHost(item, true)">重连</el-button>
                      <el-button type="primary" @click="item.term.clear()">清空缓冲区</el-button>
                    </el-button-group>
                  </div>
                  <div style="padding-top: 5px;">
                    <div>
                      <el-input disabled v-model="item.session_id">
                        <template #prepend>会话</template>
                      </el-input>
                    </div>
                    <div>
                      <el-input disabled v-model="item.address">
                        <template #prepend>主机</template>
                      </el-input>
                    </div>
                    <div>
                      <el-input disabled v-model="item.user">
                        <template #prepend>用户</template>
                      </el-input>
                    </div>
                    <div>
                      <el-input disabled v-model="item.port">
                        <template #prepend>端口</template>
                      </el-input>
                    </div>
                  </div>
                </div>
              </el-popover>

              <el-tooltip class="item" effect="dark" content="文件传输" placement="top">
                <el-button :type="item.session_id === data.current_host.session_id
                  ? 'primary'
                  : 'info'
                  " @click="listDir('/', item)" :icon="Sort"></el-button>
              </el-tooltip>
            </el-button-group>
          </template>
          <template #default>
            <div id="term-data" style="margin: 1px">
              <div :id="item.session_id" style="width: 100vw;height:100vh"></div>
            </div>
          </template>
        </el-tab-pane>
      </el-tabs>
    </div>
  </el-container>
</template>

<script setup lang="ts">
import { useGlobalStore } from "@/stores/store";
import {
  ArrowDown,
  ArrowUp,
  Avatar,
  Bottom,
  CirclePlus,
  CopyDocument,
  Eleme,
  Files,
  FolderOpened,
  Menu,
  Sort,
  Star,
  SwitchButton,
  Tools,
  Upload
} from "@element-plus/icons-vue";
import { AttachAddon } from "@xterm/addon-attach";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import axios, { type AxiosProgressEvent } from "axios";
import { ElMessage, ElMessageBox, ElNotification, ElPopover } from "element-plus";
import { computed, defineAsyncComponent, nextTick, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import Main from "./Main.vue";
const Manage = defineAsyncComponent(() => import('./Manage.vue'))


let router = useRouter();
let globalStore = useGlobalStore();
const SFTP_EDIT_MAX_BYTES = 2 * 1024 * 1024;
const archiveExtensions = [
  ".tar.gz",
  ".tgz",
  ".tar.bz2",
  ".tbz2",
  ".tar.xz",
  ".txz",
  ".tar",
  ".zip",
];

enum Mode {
  "create" = 0,
  "update" = 1,
}

interface ResponseData {
  code: number;
  msg: string;
  data?: any;
}

/**
 * 连接Host对象
 */
interface Host {
  id: number;
  name: string;
  address: string;
  user: string;
  auth_type: "pwd" | "cert";
  net_type: "tcp4" | "tcp6";
  cert_data: string;
  cert_pwd: string;
  pwd: string;
  port: number;
  font_size: number;
  background: string;
  foreground: string;
  cursor_color: string;
  font_family: string;
  cursor_style: "block" | "underline" | "bar";
  shell: string;
  pty_type: "xterm-256color" | "xterm" | "linux";
  init_cmd: string;
  init_banner: string;
  session_id: string;
  term: Terminal;
  fit: FitAddon;
  ws: WebSocket;
  is_close: boolean;
}

/**
 * 表单验证
 */
interface VerifyFromData {
  host: Host;
  is_success: boolean;
}

/**
 * sftp Path
 */
interface Path {
  dir: string;
  name: string;
}

/**
 * sftp FileInfo
 */
interface FileInfo {
  name: string;
  mod_time: string;
  mode: string;
  path: string;
  type: "d" | "f";
  size: number;
}

/**
 * sftp DirInfo
 */
interface DirInfo {
  current_dir: string;
  dir_count: number;
  file_count: number;
  files: Array<FileInfo>;
  paths: Array<Path>;
}

let data = reactive({
  navCollapsed: true,
  mode: Mode.create,
  id: 0,
  name: "",
  address: "",
  user: "",
  auth_type: "pwd",
  net_type: "tcp4",
  cert_data: "",
  cert_pwd: "",
  pwd: "",
  port: 22,
  h: 20,
  w: 80,
  session_id: "",
  background: "#000000",
  foreground: "#FFFFFF",
  cursor_color: "#FFFFFF",
  font_family: "Courier",
  font_size: 16,
  cursor_style: "block",
  shell: "bash",
  pty_type: "xterm-256color",
  init_cmd: "",
  init_banner: "",

  upload_path: "",
  download_path: "",
  host_list: [] as Array<Host>,
  host_tabs: [] as Array<Host>,

  current_host: { session_id: "" } as Host,
  host_config_collapse: ['1'],
  host_dialog_visible: false,
  file_dialog_visible: false,
  modify_devices_dialog_visible: false,
  modify_pwd_dialog_visible: false,
  manage_dialog_visible: false,
  compress_dialog_visible: false,
  extract_dialog_visible: false,
  permission_dialog_visible: false,
  editor_dialog_visible: false,
  dir_info: {} as DirInfo,
  sftp_current_dir: "",
  sftp_upload_percentage: 0,
  sftp_upload_visible: false,
  compress_name: "",
  compress_percentage: 0,
  compress_status: "" as "" | "exception",
  compress_status_text: "",
  extract_path: "",
  extract_file_name: "",
  extract_dst_path: "",
  extracting: false,
  permission_path: "",
  permission_saving: false,
  permission_bits: {
    owner: { read: false, write: false, execute: false },
    group: { read: false, write: false, execute: false },
    other: { read: false, write: false, execute: false },
  },
  editor_path: "",
  editor_content: "",
  editor_saving: false,
  new_pwd_one: "",
  new_pwd_two: "",
});

/**
 * 调试
 */
function debug() {
  console.log(data);
  console.log(data.current_host);
  console.log(data.host_list);
  console.log(data.host_tabs);
}

/**
 * 批量执行命令
 */
let cmd = reactive({ name: "", data: "", node: "current" });

interface CmdNode {
  id: number;
  cmd_name: string;
  cmd_data: string;
}

let cmdNotes = ref<Array<CmdNode>>([]);

/**
 * 搜索主机列表
 */
const searchHost = ref("");
const filterHostTable = computed(() =>
  data.host_list.filter(
    (i) =>
      !searchHost.value ||
      i.name.toLowerCase().includes(searchHost.value.toLowerCase()) ||
      i.address.toLowerCase().includes(searchHost.value.toLowerCase())
  )
)

// 检查更新
const chkingUpdate = ref(false);
const updateProgressDialogVisible = ref(false);
let updateStatusTimer = 0;

interface UpdateProgressInfo {
  state: "idle" | "starting" | "downloading" | "installing" | "restarting" | "failed" | string;
  msg: string;
  mode: string;
  domain: string;
  url: string;
  asset_name: string;
  downloaded: number;
  total: number;
  percent: number;
  current_version: string;
  latest_version: string;
  release_url: string;
}

const updateProgress = reactive<UpdateProgressInfo>({
  state: "idle",
  msg: "",
  mode: "",
  domain: "",
  url: "",
  asset_name: "",
  downloaded: 0,
  total: 0,
  percent: 0,
  current_version: "",
  latest_version: "",
  release_url: "",
});

const updatePercent = computed(() => {
  const percent = Number(updateProgress.percent || 0);
  if (Number.isFinite(percent)) {
    return Math.max(0, Math.min(100, Math.round(percent)));
  }

  if (updateProgress.total > 0) {
    return Math.max(0, Math.min(100, Math.round(updateProgress.downloaded * 100 / updateProgress.total)));
  }

  return 0;
});

const updateStateText = computed(() => {
  switch (updateProgress.state) {
    case "starting":
      return "准备中";
    case "downloading":
      return "下载中";
    case "installing":
      return "准备安装";
    case "restarting":
      return "即将重启";
    case "failed":
      return "更新失败";
    default:
      return "等待中";
  }
});

function formatUpdateSize(size?: number) {
  const bytes = Number(size || 0);
  if (!bytes || bytes < 0) return "-";

  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value = value / 1024;
    unitIndex++;
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function escapeHtml(value: any) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function updateProgressFromStatus(status: any) {
  if (!status) return;
  Object.assign(updateProgress, {
    state: status.state || "idle",
    msg: status.msg || "",
    mode: status.mode || "",
    domain: status.domain || "",
    url: status.url || "",
    asset_name: status.asset_name || "",
    downloaded: Number(status.downloaded || 0),
    total: Number(status.total || 0),
    percent: Number(status.percent || 0),
    current_version: status.current_version || "",
    latest_version: status.latest_version || "",
    release_url: status.release_url || "",
  });
}

function stopUpdateStatusPolling() {
  if (updateStatusTimer) {
    clearInterval(updateStatusTimer);
    updateStatusTimer = 0;
  }
}

async function refreshUpdateStatus() {
  try {
    const ret = await axios.get<ResponseData>("/api/update/status");
    if (ret.data.code === 0) {
      updateProgressFromStatus(ret.data.data);
      if (["failed", "restarting"].includes(updateProgress.state)) {
        stopUpdateStatusPolling();
      }
    }
  } catch (err) {
    console.log(err);
    if (updateProgress.state !== "restarting") {
      updateProgress.msg = "连接已中断，程序可能正在重启";
    }
    stopUpdateStatusPolling();
  }
}

function startUpdateStatusPolling() {
  stopUpdateStatusPolling();
  refreshUpdateStatus();
  updateStatusTimer = window.setInterval(refreshUpdateStatus, 700);
}

async function checkUpdate() {
  if (chkingUpdate.value) return;

  chkingUpdate.value = true;

  try {
    const ret = await axios.get<ResponseData>("/api/update/version");

    if (ret.data.code !== 0) {
      ElMessage.error(ret.data.msg || "检测更新失败");
      return;
    }

    const info = ret.data.data;

    if (!info.has_update) {
      ElMessage.success(`已是最新版本：${info.current_version}`);
      return;
    }

    await ElMessageBox.confirm(
      `<div class="update-confirm-card">
        <div class="update-confirm-head">
          <div>
            <div class="update-confirm-kicker">发现可用更新</div>
            <div class="update-confirm-version">${escapeHtml(info.latest_version)}</div>
          </div>
          <span>NEW</span>
        </div>
        <div class="update-confirm-grid">
          <div><label>当前版本</label><strong>${escapeHtml(info.current_version)}</strong></div>
          <div><label>最新版本</label><strong>${escapeHtml(info.latest_version)}</strong></div>
          <div><label>更新文件</label><strong>${escapeHtml(info.asset_name || "-")}</strong></div>
          <div><label>文件大小</label><strong>${formatUpdateSize(info.asset_size)}</strong></div>
        </div>
        <p>确认后将开始下载更新文件，下载期间会显示连接方式、请求域名和实时进度。</p>
      </div>`,
      "发现新版本",
      {
        confirmButtonText: "确认更新",
        cancelButtonText: "取消",
        type: "warning",
        customClass: "update-confirm-message",
        dangerouslyUseHTMLString: true,
      }
    );

    await runUpdate();
  } catch (err: any) {
    if (err === "cancel" || err === "close") {
      ElMessage.info("已取消更新");
      return;
    }

    console.log(err);
    ElMessage.error("检测更新异常");
  } finally {
    chkingUpdate.value = false;
  }
}
// 执行更新
async function runUpdate() {
  try {
    updateProgressDialogVisible.value = true;
    Object.assign(updateProgress, {
      state: "starting",
      msg: "正在提交更新任务",
      mode: "",
      domain: "",
      url: "",
      downloaded: 0,
      percent: 0,
    });

    const ret = await axios.post<ResponseData>("/api/update/run");

    if (ret.data.code === 0) {
      updateProgressFromStatus(ret.data.data);
      startUpdateStatusPolling();
      ElNotification({
        title: "更新已开始",
        type: "success",
        duration: 5000,
        message: ret.data.msg || "正在下载更新文件",
      });
    } else {
      updateProgressFromStatus(ret.data.data);
      updateProgress.state = "failed";
      updateProgress.msg = ret.data.msg || "启动更新失败";
      ElMessage.error(ret.data.msg || "启动更新失败");
    }
  } catch (err) {
    console.log(err);
    updateProgress.state = "failed";
    updateProgress.msg = "执行更新异常，可能程序正在重启";
    ElMessage.error("执行更新异常，可能程序正在重启");
  }
}

/**
 * 主机管理弹窗移动端适配
 */
const windowWidth = ref(window.innerWidth)

const isMobile = computed(() => windowWidth.value <= 768)

const tableHeight = computed(() => {
  return isMobile.value
    ? Math.max(window.innerHeight - 190, 300)
    : 500
})

function updateWindowWidth() {
  windowWidth.value = window.innerWidth
}

/**
 * 搜索命令收藏列表
 */
const searchCmdNote = ref("");
const filterCmdNoteTable = computed(() =>
  cmdNotes.value.filter(
    (i) =>
      !searchCmdNote.value ||
      i.cmd_name.toLowerCase().includes(searchCmdNote.value.toLowerCase())
  )
)

/**
 * 状态报告定时器
 */
let statusSetInterval: number;

/**
 * sftp 文件传输弹窗title
 */
const title = computed(() => {
  let titleHtml = `<span style="color:red;">当前名称:${data.current_host.name} &nbsp;&nbsp;&nbsp;当前主机:${data.current_host.address}</span>`;
  return titleHtml;
});

/**
 * 修改密码
 */
function modifyPassword() {
  if (data.new_pwd_one.length < 2) {
    ElMessage.error("密码至少两个字符");
    return
  }
  if (data.new_pwd_two.length < 2) {
    ElMessage.error("密码至少两个字符");
    return
  }
  if (data.new_pwd_one !== data.new_pwd_two) {
    ElMessage.error("两次密码输入不一致");
    return
  }

  axios.patch<ResponseData>("/api/user/pwd", { "pwd": data.new_pwd_one }).then((ret) => {
    if (ret.data.code === 0) {
      ElMessage.success("密码修改成功");
    } else {
      ElMessage.error("密码修改失败");
    }
  }).catch(() => {
    ElMessage.error("密码修改错误");
  })
  data.modify_pwd_dialog_visible = false;
}

/**
 * 执行命令
 */
function execCmd() {
  if (cmd.node == "current") {
    execCmdCurrentSession({ "id": 0, "cmd_name": "", "cmd_data": cmd.data });
  }
  if (cmd.node == "all") {
    execCmdAllSession({ "id": 0, "cmd_name": "", "cmd_data": cmd.data });
  }
}

/**
 * 添加命令收藏
 */
function addCmdNote() {
  if (cmd.data.trim().length === 0) {
    ElMessage.error("收藏的命令不能为空");
    return;
  }

  if (cmd.name.trim().length === 0) {
    ElMessage.error("如果收藏命令,必须输入收藏名称");
    return;
  }

  axios.post<ResponseData>(`/api/cmd_note/`, { "cmd_name": cmd.name, "cmd_data": cmd.data })
    .then((ret) => {
      if (ret.data.code === 0) {
        ElMessage.success("收藏成功");
        getAllCmdNote();
      } else {
        ElMessage.error("收藏命令出错了");
      }
    });

}

/**
 * 删除命令收藏
 */
function delCmdNote(id: number) {
  axios.delete<ResponseData>(`/api/cmd_note/${id}`)
    .then((ret) => {
      if (ret.data.code === 0) {
        cmdNotes.value = ret.data.data;
        ElMessage.success("删除成功");
      } else {
        ElMessage.error("删除命令收藏出错了");
      }
    });
}

/**
 * 更新命令收藏
 */
function putCmdNote(id: number) {

}

/**
 * 查询所有命令收藏
 */
function getAllCmdNote() {
  axios.get<ResponseData>("/api/cmd_note").then((ret) => {
    if (ret.data.code === 0) {
      cmdNotes.value = ret.data.data;
    } else {
      ElMessage.error("获取主机列表错误");
    }
  });
}

/**
 * 在当前会话执行收藏命令
 */
function execCmdCurrentSession(row: CmdNode) {
  try {
    data.current_host.ws.send(row.cmd_data + "\n");
  } catch (e) {
    ElMessage.error("当前会话执行命令失败");
  }
}

/**
 * 在所有会话执行收藏命令
 */
function execCmdAllSession(row: CmdNode) {
  try {
    if (data.host_tabs.length === 0) {
      ElMessage.error("没有连接会话");
      return;
    }
    data.host_tabs.forEach((h) => {
      h.ws.send(row.cmd_data + "\n");
    });
  } catch (e) {
    ElMessage.error("执行命令失败");
  }
}

/**
 * 添加密钥文件
 */
function addCertFile() {
  const input = document.createElement("input");
  input.type = "file";
  input.addEventListener("change", (event) => {
    const files = (event.target as HTMLInputElement).files;
    if (files && files.length > 0) {
      let certFile = files[0];
      const isLt1M = certFile.size / 1024 / 1024 < 1;
      if (!isLt1M) {
        ElMessage.error("上传文件大小不能超过 1MB!");
        return;
      }
      const reader = new FileReader();
      reader.onload = (e) => {
        data.cert_data = (e.target as FileReader).result as string;
      };
      reader.readAsText(certFile);
    }
  });
  input.click();
}

/**
 * 验证输入的主机信息
 */
function verifyFrom(): VerifyFromData {
  let verifyFromData: VerifyFromData = {
    host: {} as Host,
    is_success: false,
  };

  if (data.name.length === 0) {
    ElMessage.error("名称不能为空");
    return verifyFromData;
  }

  if (data.name.length > 30) {
    ElMessage.error("名称不能大于30个字符");
    return verifyFromData;
  }

  if (data.address.length === 0) {
    ElMessage.error("主机不能为空");
    return verifyFromData;
  }

  if (data.address.length > 60) {
    ElMessage.error("主机不能大于60个字符");
    return verifyFromData;
  }

  if (data.user.length === 0) {
    ElMessage.error("用户名不能为空");
    return verifyFromData;
  }

  if (data.user.length > 60) {
    ElMessage.error("用户名不能大于60个字符");
    return verifyFromData;
  }

  if (data.user.length === 0) {
    ElMessage.error("用户名不能为空");
    return verifyFromData;
  }

  if (data.user.length > 60) {
    ElMessage.error("用户名不能大于60个字符");
    return verifyFromData;
  }

  if (data.auth_type === "pwd" && data.pwd.length === 0) {
    ElMessage.error("密码不能为空");
    return verifyFromData;
  }

  if (data.user.length > 60) {
    ElMessage.error("密码不能大于60个字符");
    return verifyFromData;
  }

  if (!data.port) {
    ElMessage.error("端口输入错误,必须是1-65535");
    return verifyFromData;
  }

  if (data.port < 1 || data.port > 65535) {
    ElMessage.error("端口范围错误,必须是1-65535");
    return verifyFromData;
  }

  if (data.auth_type === "cert" && data.cert_data === "") {
    ElMessage.error("使用密钥登陆,密钥内容不能为空");
    return verifyFromData;
  }

  let h = {
    id: data.id,
    name: data.name,
    address: data.address,
    user: data.user,
    auth_type: data.auth_type,
    net_type: data.net_type,
    cert_data: data.cert_data,
    cert_pwd: data.cert_pwd,
    pwd: data.pwd,
    port: data.port,
    session_id: data.session_id,
    background: data.background,
    foreground: data.foreground,
    cursor_color: data.cursor_color,
    font_family: data.font_family,
    font_size: data.font_size,
    cursor_style: data.cursor_style,
    shell: data.shell,
    pty_type: data.pty_type,
    init_cmd: data.init_cmd,
    init_banner: data.init_banner,
  };
  let result: VerifyFromData = {
    host: h as Host,
    is_success: true,
  };
  return result;
}

/**
 * 清空表单数据
 */
function cleanFrom() {
  data.id = 0;
  data.name = "";
  data.address = "";
  data.user = "";
  data.pwd = "";
  data.auth_type = "pwd";
  data.net_type = "tcp4";
  data.cert_data = "";
  data.cert_pwd = "";
  data.port = 22;
  data.session_id = "";
  data.background = "#000000";
  data.foreground = "#FFFFFF";
  data.cursor_color = "#FFFFFF";
  data.font_family = "Courier";
  data.font_size = 16;
  data.cursor_style = "block";
  data.shell = "bash";
  data.pty_type = "xterm-256color";
  data.init_cmd = "";
  data.init_banner = "";
  data.host_config_collapse = ['1'];
}

/**
 * 连接
 */
function connect() {
  let result = verifyFrom();
  if (!result.is_success) {
    return;
  }
  connectHost(result.host);
}

/**
 * 打开文件列表
 */
function listDir(dir: string, h: Host) {
  data.file_dialog_visible = true;
  if (h) {
    setCurrentAcitveHost(h.session_id);
  }
  let host = { ...data.current_host };

  if (!host.hasOwnProperty("session_id")) {
    // 没有连接主机
    return;
  }

  let formData = new FormData();
  formData.append("session_id", host.session_id);
  formData.append("path", dir);
  axios.post<ResponseData>("/api/sftp/list", formData).then((ret) => {
    if (ret.data.code === 0) {
      data.dir_info = ret.data.data;
      data.sftp_current_dir = dir;
    } else {
      ElMessage.error("获取文件列表错误");
    }
  });
}

/**
 * 上传文件
 */
function uploadFile(path: string) {
  data.sftp_upload_percentage = 0;
  function upload(fileList: FileList) {
    if (fileList.length === 0) {
      return;
    }
    data.sftp_upload_visible = true;
    let formData = new FormData();
    formData.append("session_id", data.current_host.session_id);
    formData.append("path", path);
    for (let i = 0; i < fileList.length; i++) {
      formData.append("files", fileList[i]);
    }

    axios({
      url: '/api/sftp/upload',
      method: 'put',
      data: formData,
      //上传进度
      onUploadProgress: (progressEvent: AxiosProgressEvent) => {
        const { loaded, total } = progressEvent;
        if (!total) {
          // 没有获取到总大小，可能是流式上传或者chunked传输
          data.sftp_upload_percentage = loaded;
        } else {
          // 计算进度，可以用 loaded / total 得到一个0到1的数字
          data.sftp_upload_percentage = loaded / total * 100 | 0;
        }
      }
    }).then((ret) => {
      if (ret.data.code === 0) {
        data.sftp_upload_percentage = 100;
        // ElMessage.success(ret.data.msg);
        listDir(data.sftp_current_dir, data.current_host);
        let list = ret.data.data as Array<string>;
        if (list) {
          let msg = "";
          list.forEach((i) => {
            msg += `<p>${i}</p>`;
          });
          ElNotification({
            type: 'success',
            duration: 7000,
            title: ret.data.msg,
            dangerouslyUseHTMLString: true,
            message: msg,
          });
        }
      } else {
        ElMessage.error("上传失败");
      }
    }).catch(() => {
      ElMessage.error("上传异常");
    }).finally(() => {
      setTimeout(() => {
        data.sftp_upload_visible = false;
        data.sftp_upload_percentage = 0;
      }, 800);
    });
  }

  let fileInput = document.createElement("input");
  fileInput.type = "file";
  fileInput.multiple = true;

  fileInput.onchange = function (f: any) {
    let fileList = fileInput.files as FileList;
    upload(fileList);
  };
  fileInput.click();
}

/**
 * 下载文件(只能是文件,不能是目录)
 */
function downloadFile(file: FileInfo) {
  /*
  // POST 方式
  let formData = new FormData();
  formData.append("session_id", data.current_host.session_id);
  formData.append("path", file.path);
  axios.post<Blob>("/api/sftp/download", formData).then((ret) => {
    let blob = new Blob([ret.data], { type: 'application/x-download' });
    let a = document.createElement("a");
    a.style.display = 'none';
    let url = window.URL.createObjectURL(blob);
    a.href = url;
    a.download = file.name;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a); 
    window.URL.revokeObjectURL(url);
  });
  */
  let reqUrl = `/api/sftp/download?Authorization=${localStorage.getItem("token")}&session_id=${data.current_host.session_id}&path=${encodeURIComponent(file.path).replace(/%/g, "%25")}`;
  let a = document.createElement("a");
  a.style.display = 'none';
  a.href = reqUrl;
  a.download = file.name;
  a.click();
}

/**
 * SFTP文件删除
 */
function deleteFile(file: FileInfo) {
  let body = {
    "session_id": data.current_host.session_id,
    "path": file.path
  }
  axios.delete<ResponseData>("/api/sftp/delete", { data: body }).then((ret) => {
    if (ret.data.code === 0) {
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("删除文件成功");
    } else {
      ElMessage.error("删除文件出错了");
    }
  });
}

function getFileDir(filePath: string) {
  let index = filePath.lastIndexOf("/");
  if (index <= 0) {
    return "/";
  }
  return filePath.substring(0, index);
}

function joinSftpPath(dir: string, name: string) {
  if (dir === "/") {
    return `/${name}`;
  }
  return `${dir}/${name}`;
}

function isArchiveFile(file: FileInfo) {
  if (file.type !== "f") {
    return false;
  }
  let lowerName = file.name.toLowerCase();
  return archiveExtensions.some((ext) => lowerName.endsWith(ext));
}

let compressProgressTimer: ReturnType<typeof setInterval> | null = null;

function stopCompressProgress() {
  if (compressProgressTimer) {
    clearInterval(compressProgressTimer);
    compressProgressTimer = null;
  }
}

function startCompressProgress(file: FileInfo) {
  stopCompressProgress();
  data.compress_name = file.name;
  data.compress_percentage = 3;
  data.compress_status = "";
  data.compress_status_text = "正在准备压缩任务";
  data.compress_dialog_visible = true;
  compressProgressTimer = setInterval(() => {
    if (data.compress_percentage < 90) {
      data.compress_percentage += data.compress_percentage < 50 ? 4 : 2;
      data.compress_status_text = "正在压缩目录, 请稍候";
    }
  }, 600);
}

function finishCompressProgress(success: boolean, message: string) {
  stopCompressProgress();
  if (success) {
    data.compress_percentage = 100;
    data.compress_status = "";
    data.compress_status_text = message;
    setTimeout(() => {
      data.compress_dialog_visible = false;
    }, 800);
  } else {
    data.compress_status = "exception";
    data.compress_status_text = message;
  }
}

function compressDir(file: FileInfo) {
  startCompressProgress(file);
  let body = {
    "session_id": data.current_host.session_id,
    "path": file.path,
  };
  axios.post<ResponseData>("/api/sftp/compress", body).then((ret) => {
    if (ret.data.code === 0) {
      listDir(data.sftp_current_dir, data.current_host);
      finishCompressProgress(true, "压缩完成");
      ElMessage.success("压缩成功");
    } else {
      finishCompressProgress(false, ret.data.msg || "压缩失败");
      ElMessage.error(ret.data.msg || "压缩失败");
    }
  }).catch(() => {
    finishCompressProgress(false, "压缩异常");
    ElMessage.error("压缩异常");
  });
}

function openExtractDialog(file: FileInfo) {
  data.extract_path = file.path;
  data.extract_file_name = file.name;
  data.extract_dst_path = getFileDir(file.path);
  data.extract_dialog_visible = true;
}

function extractArchive() {
  let dstPath = data.extract_dst_path.trim();
  if (!dstPath) {
    ElMessage.error("请输入解压路径");
    return;
  }

  data.extracting = true;
  let body = {
    "session_id": data.current_host.session_id,
    "path": data.extract_path,
    "dst_path": dstPath,
  };
  axios.post<ResponseData>("/api/sftp/extract", body).then((ret) => {
    if (ret.data.code === 0) {
      data.extract_dialog_visible = false;
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("解压成功");
    } else {
      ElMessage.error(ret.data.msg || "解压失败");
    }
  }).catch(() => {
    ElMessage.error("解压异常");
  }).finally(() => {
    data.extracting = false;
  });
}

async function createFile(dir: string) {
  try {
    const ret = await ElMessageBox.prompt("请输入文件名", "新建文件", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      inputPattern: /^(?!\s*$)[^/]+$/,
      inputErrorMessage: "文件名不能为空, 且不能包含 /",
    });
    let fileName = String(ret.value || "").trim();
    if (!fileName) {
      return;
    }
    let body = {
      "session_id": data.current_host.session_id,
      "path": joinSftpPath(dir, fileName),
    };
    let response = await axios.post<ResponseData>("/api/sftp/create_file", body);
    if (response.data.code === 0) {
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("创建文件成功");
    } else {
      ElMessage.error(response.data.msg || "创建文件失败");
    }
  } catch {
  }
}

function modeToOctal(mode: string) {
  let text = mode.trim();
  if (/^[0-7]{3,4}$/.test(text)) {
    return text;
  }
  if (text.length < 10) {
    return "";
  }
  let value = 0;
  let chars = text.slice(-9);
  let weights = [256, 128, 64, 32, 16, 8, 4, 2, 1];
  for (let i = 0; i < chars.length; i++) {
    if (chars[i] !== "-") {
      value += weights[i];
    }
  }
  return value.toString(8).padStart(3, "0");
}

type PermissionSubject = "owner" | "group" | "other";

function setPermissionBitsFromMode(mode: string) {
  let octal = modeToOctal(mode).slice(-3).padStart(3, "0");
  let subjects: Array<PermissionSubject> = ["owner", "group", "other"];
  subjects.forEach((subject, index) => {
    let value = Number(octal[index] || "0");
    data.permission_bits[subject].read = (value & 4) > 0;
    data.permission_bits[subject].write = (value & 2) > 0;
    data.permission_bits[subject].execute = (value & 1) > 0;
  });
}

function getPermissionMode() {
  let subjects: Array<PermissionSubject> = ["owner", "group", "other"];
  return subjects.map((subject) => {
    let value = 0;
    if (data.permission_bits[subject].read) {
      value += 4;
    }
    if (data.permission_bits[subject].write) {
      value += 2;
    }
    if (data.permission_bits[subject].execute) {
      value += 1;
    }
    return value.toString();
  }).join("");
}

async function renameFile(file: FileInfo) {
  try {
    const ret = await ElMessageBox.prompt("请输入新名称", "重命名", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      inputValue: file.name,
      inputPattern: /^(?!\s*$)[^/]+$/,
      inputErrorMessage: "名称不能为空, 且不能包含 /",
    });
    let newName = String(ret.value || "").trim();
    if (!newName || newName === file.name) {
      return;
    }
    let body = {
      "session_id": data.current_host.session_id,
      "old_path": file.path,
      "new_path": joinSftpPath(getFileDir(file.path), newName),
    };
    let response = await axios.patch<ResponseData>("/api/sftp/rename", body);
    if (response.data.code === 0) {
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("重命名成功");
    } else {
      ElMessage.error(response.data.msg || "重命名失败");
    }
  } catch {
  }
}

function changePermission(file: FileInfo) {
  data.permission_path = file.path;
  setPermissionBitsFromMode(file.mode);
  data.permission_dialog_visible = true;
}

function savePermission() {
  data.permission_saving = true;
  let body = {
    "session_id": data.current_host.session_id,
    "path": data.permission_path,
    "mode": getPermissionMode(),
  };
  axios.patch<ResponseData>("/api/sftp/chmod", body).then((ret) => {
    if (ret.data.code === 0) {
      data.permission_dialog_visible = false;
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("权限设置成功");
    } else {
      ElMessage.error(ret.data.msg || "权限设置失败");
    }
  }).catch(() => {
    ElMessage.error("权限设置异常");
  }).finally(() => {
    data.permission_saving = false;
  });
}

function openEditor(file: FileInfo) {
  if (file.size > SFTP_EDIT_MAX_BYTES) {
    ElMessage.warning("文件超过 2MB, 请下载后编辑");
    return;
  }
  let body = {
    "session_id": data.current_host.session_id,
    "path": file.path,
  };
  axios.post<ResponseData>("/api/sftp/read", body).then((ret) => {
    if (ret.data.code === 0) {
      data.editor_path = file.path;
      data.editor_content = ret.data.data.content || "";
      data.editor_dialog_visible = true;
    } else {
      ElMessage.error(ret.data.msg || "读取文件失败");
    }
  }).catch(() => {
    ElMessage.error("读取文件异常");
  });
}

function saveEditor() {
  data.editor_saving = true;
  let body = {
    "session_id": data.current_host.session_id,
    "path": data.editor_path,
    "content": data.editor_content,
  };
  axios.put<ResponseData>("/api/sftp/save", body).then((ret) => {
    if (ret.data.code === 0) {
      data.editor_dialog_visible = false;
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("保存成功");
    } else {
      ElMessage.error(ret.data.msg || "保存失败");
    }
  }).catch(() => {
    ElMessage.error("保存异常");
  }).finally(() => {
    data.editor_saving = false;
  });
}

/**
 * SFTP创建目录
 */
function createDir(dir: string, h: Host) {
  let body = {
    "session_id": h.session_id,
    "path": dir
  }
  axios.post<ResponseData>("/api/sftp/create_dir", body).then((ret) => {
    if (ret.data.code === 0) {
      listDir(data.sftp_current_dir, data.current_host);
      ElMessage.success("创建目录成功");
    } else {
      ElMessage.error("创建目录出错了");
    }
  });
}

/**
 * 获取所有主机列表
 */
function getAllHost() {
  axios.get<ResponseData>("/api/conn_conf").then((ret) => {
    if (ret.data.code === 0) {
      data.host_list = ret.data.data;
    } else {
      ElMessage.error("获取主机列表错误");
    }
  })
}

/**
 * 创建或更新主机
 */
function createOrUpdateHost(host: Host, m: Mode) {
  // 关闭模态框,from 表单验证后续在搞 :rules="host_from_rules"
  if (m == 0) {
    for (let i = 0; i < data.host_list.length; i++) {
      // 数据库中name是unique约束
      let item = data.host_list[i];
      if (item.name == host.name) {
        ElMessage.error("名称已经存在,请修改");
        return;
      }
    }
  }

  // 关闭模态框
  data.host_dialog_visible = false;
  if (m == 0) {
    // 新增
    axios.post<ResponseData>("/api/conn_conf", host)
      .then((ret) => {
        if (ret.data.code === 0) {
          data.host_list = ret.data.data;
          cleanFrom();
        } else {
          ElMessage.error("新增出错了");
        }
      })
  } else {
    // 更新
    axios.put<ResponseData>(`/api/conn_conf`, host)
      .then((ret) => {
        if (ret.data.code === 0) {
          data.host_list = ret.data.data;
          cleanFrom();
        }
        else {
          ElMessage.error("更新出错了");
        }
      })
  }
}

/**
 * 进入主机创建模式
 */
function newHost() {
  cleanFrom();
  data.host_dialog_visible = true;
  data.mode = 0;
}

/**
 * 创建主机并保存(也可以创建主机并保存且保存)
 */
function createHost(isConnect: boolean = false) {
  // 创建模式
  data.mode = Mode.create;
  let result = verifyFrom();
  if (!result.is_success) {
    return;
  }
  createOrUpdateHost(result.host, Mode.create);
  if (isConnect) {
    connectHost(result.host);
  }
}

/**
 * 编辑主机
 */
function editHost(row: Host) {
  // 打开模态框
  data.host_dialog_visible = true;

  // 编辑模式
  data.mode = Mode.update;
  data.id = row.id;
  data.address = row.address;
  data.name = row.name;
  data.user = row.user;
  data.auth_type = row.auth_type;
  data.net_type = row.net_type;
  data.cert_data = row.cert_data;
  data.cert_pwd = row.cert_pwd;
  data.pwd = row.pwd;
  data.port = row.port;
  data.background = row.background;
  data.foreground = row.foreground;
  data.cursor_color = row.cursor_color;
  data.font_family = row.font_family;
  data.font_size = row.font_size;
  data.cursor_style = row.cursor_style;
  data.shell = row.shell;
  data.pty_type = row.pty_type;
  data.init_cmd = row.init_cmd;
  data.init_banner = row.init_banner;
  data.host_config_collapse = ['1'];
}

/**
 * 更新主机信息
 */
function updateHost(isConnect: boolean = false) {
  let result = verifyFrom();
  if (!result.is_success) {
    return;
  }
  createOrUpdateHost(result.host, Mode.update);
  if (isConnect) {
    connectHost(result.host);
  }
}

/**
 * 删除已经保存的主机
 */
function deleteHost(row: any) {
  axios.delete<ResponseData>(`/api/conn_conf/${row.id}`)
    .then((ret) => {
      if (ret.data.code === 0) {
        data.host_list = ret.data.data;
        cleanFrom();
      } else {
        ElMessage.error("删除主机出错了");
      }
    });
}

/**
 * 去掉几个引用对象属性
 * @param data 
 */
function getHost(data: Host): Omit<Host, 'fit' | 'term' | 'ws' | 'is_close'> {
  if (data.term) {
    try {
      data.fit.dispose();
      data.term.dispose();
      data.ws.close();
    } catch (err) {
      console.log("清理资源错误:" + err);
    }
  }

  let connectTabElement = document.getElementById(data.session_id);
  if (connectTabElement) {
    connectTabElement.innerHTML = "";
  }

  return {
    id: data.id,
    name: data.name,
    address: data.address,
    user: data.user,
    auth_type: data.auth_type,
    net_type: data.net_type,
    cert_data: data.cert_data,
    cert_pwd: data.cert_pwd,
    pwd: data.pwd,
    port: data.port,
    session_id: data.session_id,
    background: data.background,
    foreground: data.foreground,
    cursor_color: data.cursor_color,
    font_family: data.font_family,
    font_size: data.font_size,
    cursor_style: data.cursor_style,
    shell: data.shell,
    pty_type: data.pty_type,
    init_cmd: data.init_cmd,
    init_banner: data.init_banner,
  };
}

/**
 * 连接已经保存过的主机
 */
function connectHost(host: Host, isReconnect: boolean = false) {
  // 关闭新增/编辑主机弹窗
  data.host_dialog_visible = false;

  // 关闭主机管理弹窗
  data.modify_devices_dialog_visible = false;

  let requestUrl = "/api/ssh/create_session";
  // 如果重连,在url加上会话ID
  if (isReconnect) {
    requestUrl += `?session_id=${host.session_id}`;
  }

  // 上一个版本的解包
  let connHost = getHost(host) as Host;
  axios.post<ResponseData>(requestUrl, connHost)
    .then((ret) => {
      if (ret.data.code === 0) {
        let session_id = ret.data.data;
        connHost.session_id = session_id;

        // 窗口大小适应插件
        connHost.fit = new FitAddon();

        connHost.term = new Terminal({
          cursorBlink: true,
          theme: {
            background: connHost.background,
            foreground: connHost.foreground,
            cursor: connHost.cursor_color,
          },
          fontSize: connHost.font_size,
          fontFamily: connHost.font_family,
          cursorStyle: connHost.cursor_style,
        });

        // 加载窗口大小自适应插件
        connHost.term.loadAddon(connHost.fit);

        // 如果是重连就不需要再建立tab页面,直接替换
        if (isReconnect) {
          for (let [index, h] of data.host_tabs.entries()) {
            if (h.session_id === session_id) {
              connHost.is_close = false;
              data.host_tabs[index] = connHost;
              break;
            }
          }
        } else {
          // 新连接添加tab 页面
          data.host_tabs.push(connHost);
        }

        nextTick(() => {
          let connectTabElement = document.getElementById(connHost.session_id);

          if (connectTabElement === null) {
            ElMessage.error("创建连接获取dom为空!");
            return;
          }

          const headerHeight = data.navCollapsed ? 0 : 70;
          connectTabElement.style.height = Math.floor(window.innerHeight - headerHeight) + "px";
          connHost.term.open(connectTabElement);
          connHost.fit.fit();

          const c = connHost.term.cols > 40 ? connHost.term.cols : 40
          const r = connHost.term.rows > 40 ? connHost.term.rows : 40

          let param = `h=${r}&w=${c}&session_id=${connHost.session_id}&Authorization=${localStorage.getItem("token")}`;
          let headPart = `${location.protocol == "http:" ? "ws://" : "wss://"}${location.host}`;
          let tailPart = `/api/ssh/conn?${param}`;

          let basePath = window.location.pathname.replace("/app/", "");
          if (import.meta.env.VITE_ROUTE_MODE === "WebHistory") {
            if (import.meta.env.VITE_WEB_BASE_DIR) {
              basePath = `${import.meta.env.VITE_WEB_BASE_DIR}`;
            } else {
              basePath = "";
            }
          }

          let webSockerUrl = `${headPart}${basePath}${tailPart}`;

          let ws = new WebSocket(webSockerUrl);
          ws.onopen = function () {
            try {
              // 初始化benner
              let bannerStr = connHost.init_banner.trim();
              if (bannerStr !== "") {
                connHost.term.writeln(bannerStr);
              }

              // 调整窗口大小
              windowResize();

              // 初始化命令
              let cmdStr = connHost.init_cmd.trim()
              if (cmdStr !== "") {
                ws.send(`${cmdStr}\n`)
              }
            } catch (err) {
              console.log(err);
            }
          }

          ws.onerror = function (err) {
            console.log("WebSocket error");
            connHost.term.writeln("##  连接出错,请重连!  ##");
          }

          ws.onclose = function () {
            console.log("WebSocket close:" + connHost.session_id);
            connHost.term.writeln("##  连接关闭,请重连!  ##");
            connHost.is_close = true;
            if (data.current_host.session_id === session_id) {
              data.current_host.is_close = true;
            }
          }

          connHost.term.attachCustomKeyEventHandler((event: KeyboardEvent) => {
            if (event.type === "keydown" && event.ctrlKey && event.key.toLowerCase() === "c") {
              if (connHost.term.hasSelection()) {
                return true;
              }
              if (ws.readyState === WebSocket.OPEN) {
                // 无选区时 Ctrl+C 应发送 ETX，让远端 PTY 触发 SIGINT。
                ws.send("\x03");
              }
              return false;
            }
            return true;
          });

          connHost.term.loadAddon(new AttachAddon(ws));
          connHost.ws = ws;
          connHost.is_close = false;
          connHost.term.focus();
          // 清空 from 表单数据
          cleanFrom();

          // 设置当前激活的host
          data.current_host = { ...connHost };
        });
      } else {
        ElMessage.error("创建连接出错了");
      }
    }).catch((err) => {
      ElMessage.error("创建会话出错了");
      console.log(err)
    });
}

/**
 * 删除tab
 */
function removeTab(tabId: string | number) {
  try {
    axios.post(`/api/ssh/disconnect?session_id=${tabId}`);
  } catch (error) {
    console.log(error);
  }

  let removeIndex = 0;
  for (let [index, h] of data.host_tabs.entries()) {
    if (h.session_id === String(tabId)) {
      removeIndex = index;
      break;
    }
  }

  // 销毁term 对象
  data.host_tabs[removeIndex].fit.dispose();
  data.host_tabs[removeIndex].term.dispose();
  data.host_tabs[removeIndex].ws.close();

  // 从tab页签中删除
  data.host_tabs.splice(removeIndex, 1);

  // 如果没有打开的tab页签,就直接返回
  if (data.host_tabs.length === 0) {
    return;
  }

  // 如果打开的tab页签只有一个,就把这个tab页签设置成激活状态
  if (data.host_tabs.length === 1) {
    let activeHost = { ...data.host_tabs[0] };
    setCurrentAcitveHost(activeHost.session_id);
    return;
  }

  // 如果打开的tab页签只有一个以上,删除以后把下一个tab页签设置成激活
  if (data.host_tabs.length > 1) {
    let activeHost = { ...data.host_tabs[removeIndex - 1] };
    setCurrentAcitveHost(activeHost.session_id);
  }
}

/***
 * 点击切换tab
 */
function selectTab(tab: any) {
  let sessionId = tab.props.name;
  if (data.current_host.session_id === sessionId) {
    // 激活的已经是当前窗口直接返回
    return;
  }
  setCurrentAcitveHost(sessionId);
}

/**
 * 设置当前正在使用的主机
 */
function setCurrentAcitveHost(sessionId: string) {
  for (const host of data.host_tabs) {
    if (host.session_id === sessionId) {
      data.current_host = { ...host };
      break;
    }
  }
  windowResize();
}

/**
 * 更改窗口大小
 */
function windowResize() {
  let currentHost = data.current_host;
  if (currentHost.session_id === "") {
    return;
  }
  // 没有在主机连接路由页面
  if (router.currentRoute.value.name !== "Home") {
    return;
  }
  nextTick(() => {
    let connectTabElement = document.getElementById(currentHost.session_id);
    if (connectTabElement === null) {
      console.log("调整窗口大小,没有获取到dom");
      return;
    }
    const headerHeight = data.navCollapsed ? 0 : 70;
    (connectTabElement as HTMLElement).style.height = Math.floor(window.innerHeight - headerHeight) + "px";

    currentHost.fit.fit();
    //if (data.h !== currentHost.term.rows || data.w !== currentHost.term.cols) {
    let url = `/api/ssh/conn?w=${currentHost.term.cols}&h=${currentHost.term.rows}&session_id=${currentHost.session_id}`;
    axios.patch<ResponseData>(url)
    //}

    data.h = Math.floor(currentHost.term.rows);
    data.w = Math.floor(currentHost.term.cols);
  });
}

/**
 * 报告连接状态
 */
function reportConnectStatus() {
  statusSetInterval = setInterval(() => {
    let fm = new FormData();
    data.host_tabs.forEach((hont) => {
      fm.append("ids", hont.session_id);
    });
    axios.put<ResponseData>("/api/conn_manage/refresh_conn_time", fm)
      .then((res) => {
        if (res.data.code !== 0) {
          console.log("刷新失败");
        }
      });
  }, 10000);
}

/**
 * 跳转到管理页面
 */
function toManage() {
  //router.push({ name: "Manage" });
  data.manage_dialog_visible = true;
}

/**
 * 防抖
 * @param fn 
 * @param delay 
 */
function debounce(fn: Function, delay: number) {
  let timer = 0;
  return function (event: Event) {
    clearTimeout(timer);
    timer = setTimeout(() => {
      fn();
    }, delay)
  }
}

/**
 * 节流
 * @param fn 
 * @param delay 
 */
function throttle(fn: Function, delay: number) {
  let record = Date.now();
  return function (event: Event) {
    let now = Date.now();
    if (now - record > delay) {
      fn();
      record = now;
    }
  }
}

/**
 * 断开所有会话
 */
function disconnectAllSession() {
  // 清理连接资源
  data.host_tabs.forEach((host, index) => {
    try {
      axios.post(`/api/ssh/disconnect?session_id=${host.session_id}`);
    } catch (error) {
      console.log(error);
    }
  });
}

/**
 * 退出登陆
 */
// function logout() {
//   disconnectAllSession();
//   globalStore.logout();
//   localStorage.setItem("auth", "no");
//   router.push({ "name": "Login" });
// }
/**
 * 退出登录（保留记住密码）
 */
function logout() {
  // 1. 断开所有连接
  disconnectAllSession();

  // 2. 清空内存态
  // globalStore.logout();

  // 3. 清除登录凭证
  localStorage.removeItem("token");
  localStorage.setItem("auth", "no");

  // 4. 关闭自动登录（但不清记住密码）
  disableAutoLogin();

  // 5. 跳转登录页
  router.replace({ name: "Login" });
}

function disableAutoLogin() {
  const raw = localStorage.getItem("login_account");
  if (!raw) return;
  try {
    const account = JSON.parse(raw);
    account.autoLogin = false;
    localStorage.setItem("login_account", JSON.stringify(account));
  } catch {
    // 如果解析失败，直接清掉
    localStorage.removeItem("login_account");
  }
}

/**
 * 挂载后执行
 */
onMounted(() => {
  router = useRouter();
  reportConnectStatus();
  getAllHost();
  getAllCmdNote();
  window.addEventListener("resize", debounce(windowResize, 200));
  window.addEventListener("resize", updateWindowWidth);
  windowResize();
  window.onbeforeunload = function () {
    return "关闭吗";
  };
});


/**
 * 销毁前执行
 */
onBeforeUnmount(() => {
  clearInterval(statusSetInterval);
  stopCompressProgress();
  stopUpdateStatusPolling();
  disconnectAllSession();
  window.removeEventListener("resize", updateWindowWidth);
  window.onbeforeunload = null;
})

// 添加计算属性获取终端背景色
const terminalBackground = computed(() => {
  if (data.current_host?.term?.options?.theme?.background) {
    return data.current_host.term.options.theme.background;
  }
  return data.background || '#000000'; // 使用配置的背景色或默认黑色
});

</script>


<style scoped>
.top-nav-header {
  position: relative;
  height: fit-content;
  padding: 8px 12px 10px;
  background:#315697;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.22);
  backdrop-filter: blur(14px);
  overflow: visible;
  z-index: 2000;
}

.nav {
  display: flex;
  align-items: center;
}

.nav :deep(.el-button) {
  border: none;
  border-radius: 12px !important;
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
  box-shadow: none;
  transition:
    transform 0.18s ease,
    background 0.18s ease,
    box-shadow 0.18s ease;
}

.nav :deep(.el-button:hover) {
  background: rgba(255, 255, 255, 0.24);
  transform: translateY(-1px);
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.18);
}

.el-button-group {
  display: flex;
  max-width: 100%; /* 防止超出屏幕 */
}

.nav :deep(.el-button-group) {
  display: flex;
  gap: 10px;
}

.nav :deep(.el-button-group > .el-button:not(:first-child)) {
  margin-left: 0;
}

.nav-anchor-toggle {
  position: absolute;
  left: 50%;
  width: 45px;
  height: 15px;
  border: 1px solid rgba(255, 255, 255, 0.24);
  border-top: none;
  border-radius: 0 0 20px 20px;
  transform: translateX(-50%);
  background: #315697;
  color: #e2e0e0;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(16px);
  transition:
    transform 0.2s ease,
    background 0.2s ease,
    box-shadow 0.2s ease,
    opacity 0.2s ease;
}

/* 暗色模式支持 */
@media (prefers-color-scheme: dark) {
  .top-nav-header {
    background: #263144;
  }
  .nav-anchor-toggle {
    background: #263144;
  }
  
}
.nav-anchor-toggle-open {
  top: 50px;
}

.nav-anchor-icon {
  width: 25px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nav-anchor-toggle .el-icon {
  font-size: 15px;
}

:deep(.modern-dialog) {
  border-radius: 20px;
  overflow: hidden;
  background: rgba(248, 250, 252, 0.96);
  box-shadow:
    0 24px 70px rgba(15, 23, 42, 0.28),
    0 0 0 1px rgba(148, 163, 184, 0.16);
  backdrop-filter: blur(18px);
}

:deep(.modern-dialog .el-dialog__header) {
  margin-right: 0;
  padding: 18px 22px 14px;
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(59, 130, 246, 0.04)),
    rgba(255, 255, 255, 0.72);
  border-bottom: 1px solid rgba(148, 163, 184, 0.18);
}

:deep(.modern-dialog .el-dialog__title) {
  font-size: 17px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: 0.2px;
}

:deep(.modern-dialog .el-dialog__headerbtn) {
  top: 14px;
  right: 14px;
  width: 34px;
  height: 34px;
  border-radius: 999px;
  transition:
    background 0.18s ease,
    transform 0.18s ease;
}

:deep(.modern-dialog .el-dialog__headerbtn:hover) {
  background: rgba(15, 23, 42, 0.08);
  transform: rotate(90deg);
}

:deep(.modern-dialog .el-dialog__body) {
  padding: 18px 22px;
  color: #334155;
}

:deep(.modern-dialog .el-dialog__footer) {
  padding: 14px 22px 20px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
  background: rgba(248, 250, 252, 0.72);
}

:deep(.modern-dialog .el-input__wrapper),
:deep(.modern-dialog .el-textarea__inner),
:deep(.modern-dialog .el-select__wrapper),
:deep(.modern-dialog .el-input-number .el-input__wrapper) {
  border-radius: 12px;
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.22) inset;
  background: rgba(255, 255, 255, 0.88);
  transition:
    box-shadow 0.18s ease,
    background 0.18s ease;
}

:deep(.modern-dialog .el-input__wrapper:hover),
:deep(.modern-dialog .el-textarea__inner:hover),
:deep(.modern-dialog .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.42) inset;
}

:deep(.modern-dialog .el-input__wrapper.is-focus),
:deep(.modern-dialog .el-select__wrapper.is-focused) {
  box-shadow:
    0 0 0 1px rgba(59, 130, 246, 0.75) inset,
    0 0 0 4px rgba(59, 130, 246, 0.12);
}

:deep(.modern-dialog .el-form-item__label) {
  color: #475569;
  font-weight: 600;
}
:deep(.modern-dialog .el-table) {
  border-radius: 16px;
  overflow: hidden;
  background: transparent;
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.14);
}

:deep(.modern-dialog .el-table th.el-table__cell) {
  background: #f8fafc;
  color: #475569;
  font-weight: 700;
}

:deep(.modern-dialog .el-table td.el-table__cell) {
  border-bottom-color: rgba(226, 232, 240, 0.9);
}

:deep(.modern-dialog .el-table__row:hover > td.el-table__cell) {
  background: rgba(59, 130, 246, 0.06);
}

:deep(.modern-dialog .el-table__inner-wrapper::before) {
  display: none;
}
:deep(.modern-dialog .el-collapse) {
  border: none;
}

:deep(.modern-dialog .el-collapse-item) {
  margin-bottom: 14px;
  border-radius: 16px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.74);
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.16);
}

:deep(.modern-dialog .el-collapse-item__header) {
  height: 48px;
  padding: 0 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  background:
    linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(255, 255, 255, 0.7));
  color: #0f172a;
  font-weight: 700;
}

:deep(.modern-dialog .el-collapse-item__wrap) {
  border-bottom: none;
  background: transparent;
}

:deep(.modern-dialog .el-collapse-item__content) {
  padding: 18px 16px 4px;
}
:deep(.modern-dialog .el-button) {
  border-radius: 12px;
  font-weight: 600;
}

:deep(.modern-dialog .el-button--primary) {
  background: linear-gradient(135deg, #2563eb, #3b82f6);
  border: none;
}

:deep(.modern-dialog .el-button--success) {
  background: linear-gradient(135deg, #059669, #10b981);
  border: none;
}

:deep(.modern-dialog .el-button--danger) {
  background: linear-gradient(135deg, #dc2626, #ef4444);
  border: none;
}

:deep(.modern-dialog .el-button:hover) {
  transform: translateY(-1px);
}

.update-progress-panel {
  display: grid;
  gap: 18px;
}

.update-status-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.update-status-title {
  min-width: 0;
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.update-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.update-meta-item {
  min-width: 0;
  padding: 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.76);
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.16);
}

.update-meta-item span {
  display: block;
  margin-bottom: 6px;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.update-meta-item strong {
  display: block;
  color: #0f172a;
  font-size: 14px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.update-transfer {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: #475569;
  font-size: 13px;
  font-weight: 700;
}

:global(.update-confirm-message) {
  width: min(92vw, 460px);
  border-radius: 20px;
  overflow: hidden;
}

:global(.update-confirm-message .el-message-box__header) {
  padding: 18px 20px 8px;
}

:global(.update-confirm-message .el-message-box__content) {
  padding: 10px 20px 6px;
}

:global(.update-confirm-message .el-message-box__btns) {
  padding: 12px 20px 18px;
}

:global(.update-confirm-card) {
  display: grid;
  gap: 14px;
}

:global(.update-confirm-head) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px;
  border-radius: 16px;
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(16, 185, 129, 0.1));
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.16) inset;
}

:global(.update-confirm-head span) {
  flex: 0 0 auto;
  padding: 5px 8px;
  border-radius: 999px;
  background: #2563eb;
  color: #fff;
  font-size: 12px;
  font-weight: 800;
}

:global(.update-confirm-kicker) {
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

:global(.update-confirm-version) {
  margin-top: 4px;
  color: #0f172a;
  font-size: 24px;
  font-weight: 800;
  line-height: 1.2;
}

:global(.update-confirm-grid) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

:global(.update-confirm-grid div) {
  min-width: 0;
  padding: 10px;
  border-radius: 12px;
  background: #f8fafc;
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.16);
}

:global(.update-confirm-grid label) {
  display: block;
  margin-bottom: 5px;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

:global(.update-confirm-grid strong) {
  display: block;
  color: #0f172a;
  font-size: 14px;
  overflow-wrap: anywhere;
}

:global(.update-confirm-card p) {
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.55;
}

.permission-grid {
  display: grid;
  grid-template-columns: 110px repeat(3, minmax(64px, 1fr));
  align-items: center;
  overflow: hidden;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
}

.permission-grid > div {
  min-height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-right: 1px solid #ebeef5;
  border-bottom: 1px solid #ebeef5;
}

.permission-grid > div:nth-child(4n) {
  border-right: 0;
}

.permission-grid > div:nth-last-child(-n + 4) {
  border-bottom: 0;
}

.permission-grid > div:nth-child(-n + 4) {
  background: #f5f7fa;
  color: #606266;
  font-weight: 600;
}

.permission-check :deep(.el-checkbox) {
  height: auto;
  margin: 0;
}

.permission-check :deep(.el-checkbox__label) {
  display: none;
}

.compress-progress-text {
  margin-top: 14px;
  color: #606266;
  font-size: 14px;
  text-align: center;
}

@media (max-width: 768px) {
  .nav .right {
    text-align: left !important;
  }

  :deep(.modern-dialog) {
    border-radius: 0;
  }

  :deep(.modern-dialog .el-dialog__body) {
    padding: 14px;
  }

  :deep(.modern-dialog .el-dialog__footer) {
    padding: 12px 14px 16px;
  }

  .update-meta-grid,
  :global(.update-confirm-grid) {
    grid-template-columns: 1fr;
  }

  .nav :deep(.el-button-group) {
    display: flex;
    gap: 6px;
  }
  .el-button {
    font-size: 10px;
    padding: 0px 11px;
  }

}
</style>
