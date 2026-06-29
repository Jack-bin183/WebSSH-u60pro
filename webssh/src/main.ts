import { createPinia } from "pinia";
import { createApp } from "vue";

import axios from "axios";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import piniaPluginPersistedstate from "pinia-plugin-persistedstate";
import App from "./App.vue";
import router from "./router";

import { useGlobalStore } from "./stores/store";

function isIOSStandalonePwa() {
    const standalone = Boolean((navigator as Navigator & { standalone?: boolean }).standalone);
    const iosDevice = /iPad|iPhone|iPod/.test(navigator.userAgent);
    const iPadDesktopMode = navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
    return standalone && (iosDevice || iPadDesktopMode);
}

if (isIOSStandalonePwa()) {
    document.documentElement.classList.add("ios-pwa");
}

    const app = createApp(App);

    const pinia = createPinia();
    pinia.use(piniaPluginPersistedstate);

    app.use(pinia);
    app.use(ElementPlus, { zIndex: 2000 });
    // app.use(ElementPlus, { size: "small", zIndex: 2000 });
    app.use(router);

    let globalStore = useGlobalStore();

    // 导航守卫 配置
    // 使用 router.beforeEach 注册一个全局前置守卫，判断用户是否登陆
    router.beforeEach((to, from) => {
        if ((!globalStore.isInit) && to.name === "SysInit") {
            return true;
        }

        if (to.name === "Login") {
            return true;
        }

        var local_auth = localStorage.getItem("auth");
        if (local_auth === "yes" && globalStore.isLogin) {
            return true
        }

        router.push({ "name": "Login" });
        return false;
    });


    //////////////
    //   拦截器
    //////////////

    axios.interceptors.request.use(
        (req) => {
            let basePath = window.location.pathname.replace("/app/", "");

            if (import.meta.env.VITE_ROUTE_MODE === "WebHistory") {
                if (import.meta.env.VITE_WEB_BASE_DIR) {
                    basePath = `${import.meta.env.VITE_WEB_BASE_DIR}`;
                } else {
                    basePath = "";
                }
            }
            // req.url = `${basePath}${req.url}`;
            // req.url = `http://127.0.0.1:3000${req.url}`;
            req.url = `${req.url}`;
            // 在发送请求之前加token
            req.headers.Time = String(new Date().getTime());
            req.headers.Authorization = localStorage.getItem("token");
            return req;
        },
        (err) => {
            return Promise.reject(err);
        }
    );

    // 添加响应拦截器
    axios.interceptors.response.use(
        (res) => {
            let newToken = res.headers["newtoken"];
            if (newToken) {
                // 若有newtoken则刷新token
                localStorage.setItem("token", newToken);
            }
            return res;
        },
        (err) => {
            if (err.response && (err.response.status === 401)) {
                router.replace({ "name": "Login" });
            }
            return Promise.reject(err);
        }
    )

    app.mount("#app");

    if ("serviceWorker" in navigator && import.meta.env.PROD) {
        window.addEventListener("load", () => {
            navigator.serviceWorker.register(new URL("sw.js", window.location.href), { scope: "./" }).catch((err) => {
                console.warn("Service worker registration failed:", err);
            });
        });
    }
