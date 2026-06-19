<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <h1>欢迎登录</h1>
      </div>

      <div class="login-form">
        <div class="form-item">
          <el-input
            v-model="data.name"
            placeholder="请输入用户名"
            :prefix-icon="User"
            clearable
          />
        </div>

        <div class="form-item">
          <el-input
            v-model="data.pwd"
            placeholder="请输入密码"
            :prefix-icon="Lock"
            type="password"
            show-password
            clearable
          />
        </div>

        <div class="form-item remember-row">
          <el-checkbox
            v-model="data.remember"
            @change="onRememberChange"
          >
            记住密码
          </el-checkbox>

          <el-checkbox
            v-model="data.autoLogin"
            :disabled="!data.remember"
          >
            自动登录
          </el-checkbox>
        </div>

        <div class="form-item">
          <el-button
            type="primary"
            class="login-button"
            :loading="loading"
            @click="login"
          >
            登录
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useGlobalStore } from "@/stores/store";
import { Lock, User } from "@element-plus/icons-vue";
import axios from "axios";
import { ElMessage } from "element-plus";
import { onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";

/* ================= 基础 ================= */

const router = useRouter();
const globalStore = useGlobalStore();
const loading = ref(false);

/* ================= 接口类型 ================= */

interface ResponseData {
  code: number;
  msg: string;
  token: string;
  is_admin: "Y" | "N";
  is_root: "Y" | "N";
  user_name: string;
  user_desc: string;
  user_expiry_at: string;
}

/* ================= 表单数据 ================= */

const data = reactive({
  name: "",
  pwd: "",
  remember: false,
  autoLogin: false,
});

/* ================= Base64 工具 ================= */

function encodePwd(pwd: string) {
  return btoa(encodeURIComponent(pwd));
}

function decodePwd(pwd: string) {
  return decodeURIComponent(atob(pwd));
}

/* ================= localStorage ================= */

const STORAGE_KEY = "login_account";

interface LoginAccount {
  remember: boolean;
  autoLogin: boolean;
  name: string;
  pwd: string;
}

function saveAccount() {
  const account: LoginAccount = {
    remember: data.remember,
    autoLogin: data.autoLogin,
    name: data.name,
    pwd: encodePwd(data.pwd),
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(account));
}

function loadAccount(): LoginAccount | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  return raw ? JSON.parse(raw) : null;
}

function clearAccount() {
  localStorage.removeItem(STORAGE_KEY);
}

/* ================= remember 联动 autoLogin ================= */

function onRememberChange(val: boolean) {
  if (!val) {
    data.autoLogin = false;
  }
}

/* ================= 页面加载：回填 + 自动登录 ================= */

onMounted(() => {
  const account = loadAccount();
  // const hasToken = !!localStorage.getItem("token");

  if (!account) return;

  if (account.remember) {
    data.name = account.name;
    data.pwd = decodePwd(account.pwd);
    data.remember = true;
    data.autoLogin = account.autoLogin;
  } else {
    data.name = account.name;
  }

  // ⚠️ 只有勾选了自动登录才会自动登录
  if (account.remember && account.autoLogin) {
    // autoLogin();
    loading.value = true;
    setTimeout(() => {
      autoLogin();
    }, 1000);
  }
});

/* ================= 自动登录 ================= */

function autoLogin() {
  loading.value = true;

  axios
    .post<ResponseData>("/api/login", {
      name: data.name,
      pwd: data.pwd,
    })
    .then((ret) => {
      if (ret.data.code === 0) {
        handleLoginSuccess(ret.data);
      } else {
        clearAccount();
      }
    })
    .catch(() => {
      clearAccount();
    })
    .finally(() => {
      loading.value = false;
    });
}

/* ================= 手动登录 ================= */

function login() {
  if (data.name.trim().length < 2) {
    ElMessage.error("用户名至少两个字符");
    return;
  }

  if (data.pwd.trim().length < 2) {
    ElMessage.error("密码至少两个字符");
    return;
  }

  loading.value = true;

  axios
    .post<ResponseData>("/api/login", {
      name: data.name,
      pwd: data.pwd,
    })
    .then((ret) => {
      if (ret.data.code === 0) {
        handleLoginSuccess(ret.data);
      } else {
        ElMessage.error("登录失败");
      }
    })
    .catch(() => {
      ElMessage.error("登录失败");
    })
    .finally(() => {
      loading.value = false;
    });
}

/* ================= 登录成功统一处理 ================= */

function handleLoginSuccess(res: ResponseData) {
  ElMessage.success("登录成功");

  localStorage.setItem("token", res.token);
  localStorage.setItem("auth", "yes");

  if (data.remember) {
    saveAccount();
  } else {
    clearAccount();
  }

  globalStore.login(
    res.is_admin,
    res.is_root,
    res.user_name,
    res.user_desc,
    res.user_expiry_at
  );

  router.replace({ name: "Home" });
}
</script>


<style scoped>
.login-container {
  min-height: 100vh;
  min-height: 100dvh;
  height: 100vh;
  height: 100dvh;
  background: linear-gradient(135deg, #1a365d 0%, #2d3748 100%);
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding:
    calc(20px + var(--app-safe-top, 0px))
    calc(20px + var(--app-safe-right, 0px))
    calc(20px + var(--app-safe-bottom, 0px))
    calc(20px + var(--app-safe-left, 0px));
  overflow: hidden;
  box-sizing: border-box;
}

.login-box {
  width: 100%;
  max-width: 420px;
  background: rgba(255, 255, 255, 0.98);
  border-radius: 20px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  padding: 40px 30px;
  backdrop-filter: blur(10px);
  margin-top: 10vh;
}

.login-header {
  text-align: center;
  margin-bottom: 25px;
}

.login-header h1 {
  color: #2d3748;
  font-size: clamp(1.8em, 4vw, 2.2em);
  font-weight: 600;
  margin: 0 0 10px 0;
}

.login-header p {
  color: #718096;
  font-size: clamp(0.9em, 2vw, 1.1em);
  margin: 0;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-item {
  width: 100%;
}

:deep(.el-input__wrapper) {
  background-color: #f8fafc;
  border: 2px solid transparent;
  border-radius: 12px;
  transition: all 0.3s ease;
  box-shadow: none !important;
  padding: 0 15px;
  height: 45px;
}

:deep(.el-input__wrapper:hover),
:deep(.el-input__wrapper.is-focus) {
  border-color: #4c51bf;
  background-color: #ffffff;
}

:deep(.el-input__inner) {
  height: 45px;
  font-size: 16px;
  color: #2d3748;
}

:deep(.el-input__prefix) {
  font-size: 1.2em;
  margin-right: 10px;
  color: #718096;
}

:deep(.el-input__inner::placeholder) {
  color: #a0aec0;
  opacity: 1;
  /* 修复Firefox中placeholder颜色过浅的问题 */
}

:deep(.el-input__inner::-webkit-input-placeholder) {
  color: #a0aec0;
}

:deep(.el-input__inner::-moz-placeholder) {
  color: #a0aec0;
}

:deep(.el-input__inner:-ms-input-placeholder) {
  color: #a0aec0;
}

.login-button {
  width: 100%;
  height: 45px;
  font-size: 16px;
  font-weight: 500;
  letter-spacing: 2px;
  background: -webkit-gradient(linear, left top, right top, from(#4c51bf), to(#667eea));
  background: -o-linear-gradient(left, #4c51bf, #667eea);
  background: linear-gradient(to right, #4c51bf, #667eea);
  border: none;
  border-radius: 12px;
  -webkit-transition: all 0.3s ease;
  -o-transition: all 0.3s ease;
  transition: all 0.3s ease;
  cursor: pointer;
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
}

.login-button:hover {
  -webkit-transform: translateY(-2px);
  -ms-transform: translateY(-2px);
  transform: translateY(-2px);
  -webkit-box-shadow: 0 6px 20px rgba(76, 81, 191, 0.3);
  box-shadow: 0 6px 20px rgba(76, 81, 191, 0.3);
}

.login-button:active {
  -webkit-transform: translateY(0);
  -ms-transform: translateY(0);
  transform: translateY(0);
}

/* 响应式布局 */
@media screen and (max-width: 768px) {
  .login-container {
    padding:
      calc(15px + var(--app-safe-top, 0px))
      calc(15px + var(--app-safe-right, 0px))
      calc(15px + var(--app-safe-bottom, 0px))
      calc(15px + var(--app-safe-left, 0px));
  }

  .login-box {
    padding: 25px 20px;
    width: 90%;
    margin-top: 8vh;
  }

  :deep(.el-input__wrapper) {
    height: 40px;
  }

  .login-button {
    height: 40px;
    font-size: 15px;
  }
}

/* 平板设备适配 */
@media screen and (min-width: 768px) and (max-width: 1024px) {
  .login-box {
    max-width: 480px;
    padding: 35px 25px;
  }
}

/* 横屏手机适配 */
@media screen and (max-height: 480px) {
  .login-container {
    align-items: flex-start;
    padding:
      calc(10px + var(--app-safe-top, 0px))
      calc(10px + var(--app-safe-right, 0px))
      calc(10px + var(--app-safe-bottom, 0px))
      calc(10px + var(--app-safe-left, 0px));
  }

  .login-box {
    padding: 20px 15px;
    margin-top: 5vh;
  }

  .login-header {
    margin-bottom: 20px;
  }

  .login-form {
    gap: 15px;
  }
}

/* 确保在小屏幕设备上内容不会溢出 */
@media screen and (max-height: 600px) {
  .login-box {
    margin-top: 5vh;
    padding: 20px 15px;
  }
}

/* 暗色模式支持 */
@media (prefers-color-scheme: dark) {
  .login-box {
    background: rgba(255, 255, 255, 0.95);
  }

  :deep(.el-input__wrapper) {
    background-color: rgba(248, 250, 252, 0.95);
  }

  .login-header h1 {
    color: #1a202c;
  }

  .login-header p {
    color: #4a5568;
  }
}

/* 兼容性补充 */
@supports not (gap: 25px) {
  .login-form {
    margin: -12.5px 0;
  }

  .login-form>* {
    margin: 12.5px 0;
  }
}

@supports not (backdrop-filter: blur(10px)) {
  .login-box {
    background: rgba(255, 255, 255, 0.98);
  }
}
</style>
