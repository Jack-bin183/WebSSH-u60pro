#!/bin/sh
# Version identification
Module_dir="/data/kano_plugins/kano_web_ssh"
BOOT_CMD="sh $Module_dir/service.sh start"
STOP_CMD="sh $Module_dir/service.sh stop"
FILE="/etc/rc.local"

SCRIPT_URL="https://raw.githubusercontent.com/cdwangtao/WebSSH-u60pro/refs/heads/master/webssh.sh"
SCRIPT_TMP="/tmp/webssh_install.sh"

VERSION_URL="https://github.com/cdwangtao/WebSSH-u60pro/releases/latest/download/version.txt"
WEBSH_URL_PREFIX="https://github.com/cdwangtao/WebSSH-u60pro/releases/latest/download/webssh_"

PROXIES="
https://v6.gh-proxy.org/
https://gh-proxy.org/
https://hk.gh-proxy.org/
https://cdn.gh-proxy.org/
https://edgeone.gh-proxy.org/
https://fastgit.cc/
https://git.yylx.win/
https://gh.llkk.cc/
https://ghfast.top/
"

fetch_url() {
    _original_url="$1"
    for _proxy in $PROXIES; do
        _url="${_proxy}${_original_url}"
        _result=$(curl -fsSL --connect-timeout 10 "$_url" 2>/dev/null)
        if [ $? -eq 0 ] && [ -n "$_result" ]; then
            echo "$_result"
            return 0
        fi
    done
    _result=$(curl -fsSL --connect-timeout 10 "$_original_url" 2>/dev/null)
    if [ $? -eq 0 ] && [ -n "$_result" ]; then
        echo "$_result"
        return 0
    fi
    return 1
}

download_file() {
    _original_url="$1"
    _output="$2"
    _show_progress="${3:-0}"
    for _proxy in $PROXIES; do
        _url="${_proxy}${_original_url}"
        if [ "$_show_progress" = "1" ]; then
            curl -fSL --connect-timeout 10 -# "$_url" --output "$_output" && return 0
        else
            curl -fSL --connect-timeout 10 "$_url" --output "$_output" 2>/dev/null && return 0
        fi
    done
    if [ "$_show_progress" = "1" ]; then
        curl -fSL --connect-timeout 10 -# "$_original_url" --output "$_output" && return 0
    else
        curl -fSL --connect-timeout 10 "$_original_url" --output "$_output" 2>/dev/null && return 0
    fi
    return 1
}

add_alias() {
    ALIAS_CMD="alias webssh=\"ash $Module_dir/webssh.sh\""

    for f in /etc/shinit /etc/profile; do
        if [ ! -f "$f" ]; then
            touch "$f"
        fi

        grep -F "$ALIAS_CMD" "$f" >/dev/null 2>&1
        if [ $? -ne 0 ]; then
            echo "$ALIAS_CMD" >> "$f"
        fi
    done
}

setup_webssh() {
    echo "检查版本信息..."
    if ! REMOTE_VERSION=$(fetch_url "$VERSION_URL" 2>/dev/null); then
        echo "  获取远程版本失败，请检查网络连接"
        return 1
    fi
    REMOTE_VERSION=$(echo "$REMOTE_VERSION" | tr -d '\r\n')
    echo "  最新版本: $REMOTE_VERSION"
    
    if [ -f "$Module_dir/webssh" ]; then
        if [ -f "$Module_dir/VERSION.txt" ]; then
            LOCAL_VERSION=$(cat "$Module_dir/VERSION.txt" 2>/dev/null | tr -d '\r\n')
        else
            LOCAL_VERSION="未知"
        fi
        
        echo "  当前版本: $LOCAL_VERSION"
        
        if [ "$LOCAL_VERSION" = "$REMOTE_VERSION" ]; then
            echo "已是最新版本，无需更新"
            return 0
        fi
        
        # 确认更新
        read -rp "发现新版本 $REMOTE_VERSION，是否更新？[y/N]: " update_choice </dev/tty
        if [ "$(echo "$update_choice" | tr '[:upper:]' '[:lower:]')" != "y" ]; then
            echo "已取消更新"
            return 0
        fi
        
        echo "停止 WebSSH 服务..."
        $STOP_CMD 2>/dev/null
        
        echo "下载最新版本..."
        if ! download_file "${WEBSH_URL_PREFIX}${REMOTE_VERSION}" "$Module_dir/webssh.new" 1; then
            echo "  下载失败，请检查网络连接"
            echo "  尝试启动旧版本..."
            $BOOT_CMD 2>/dev/null
            return 1
        fi
        echo "  下载完成"
        
        # 设置执行权限并替换
        chmod 755 "$Module_dir/webssh.new"
        mv -f "$Module_dir/webssh.new" "$Module_dir/webssh"
        
        # 更新版本文件
        echo "$REMOTE_VERSION" > "$Module_dir/VERSION.txt"
        
        # 启动服务
        echo "重启 WebSSH 服务..."
        if ! $BOOT_CMD 2>/dev/null; then
            echo "启动失败，请检查 $Module_dir/service.sh"
            return 1
        fi
        
        clear
        echo ""
        echo "======================================"
        echo "       WebSSH 更新完成"
        echo "--------------------------------------"
        echo "  版本   : $LOCAL_VERSION -> $REMOTE_VERSION"
        echo "  访问   : http://192.168.0.1:8899"
        echo "======================================"
        echo ""
    else
        # 安装场景
        # 确保目标目录存在
        mkdir -p "$Module_dir"

        echo "下载 WebSSH 主程序..."
        if ! download_file "${WEBSH_URL_PREFIX}${REMOTE_VERSION}" "$Module_dir/webssh" 1; then
            echo "  下载失败，请检查网络或链接"
            return 1
        fi
        echo "  下载完成"

        chmod 755 "$Module_dir/webssh"

        cat > "$Module_dir/service.sh" << SEOF
#!/system/bin/sh
Module_dir="$Module_dir"
chmod 777 \$Module_dir/webssh
start() {
    if ! (ps -ef | grep -- 'webssh' | grep -vE 'grep') > /dev/null ; then
        cd \$Module_dir
        nohup \$Module_dir/webssh > /dev/null 2>&1 &

        echo "WebSSH已启动"
    else
        echo "WebSSH正在运行中，不执行启动命令"
    fi
}

stop() {
    if (ps -ef | grep -- 'webssh' | grep -vE 'grep') > /dev/null ; then
        pid=\$(pgrep -f 'webssh')
        kill -15 \$pid

        echo "WebSSH已关闭"
    else
        echo "WebSSH未在运行，不执行停止命令"
    fi
}

case "\$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    *)
        echo "只能使用start|stop两个参数控制WebSSH启动或停止"
        exit 1
        ;;
esac
SEOF
        chmod 755 "$Module_dir/service.sh"
        
        # 创建版本文件
        echo "$REMOTE_VERSION" > "$Module_dir/VERSION.txt"

        # 确保 rc.local 存在
        if [ ! -f "$FILE" ]; then
            echo "没有找到自启动脚本，插件不会开机自启"
        else
            # 开机自启（写入 exit 0 前）
            echo "设置开机自启..."
            if grep -F "$BOOT_CMD" "$FILE" >/dev/null 2>&1; then
                echo "开机脚本已存在，无需重复添加"
            else
                sed -i "/^exit 0/i $BOOT_CMD" "$FILE"
                echo "已添加: $BOOT_CMD"
            fi
        fi

        # 启动
        echo "启动 WebSSH 服务..."
        $STOP_CMD 2>/dev/null
        if ! $BOOT_CMD 2>/dev/null; then
            echo "启动失败，请检查 $Module_dir/service.sh"
            return 1
        fi
        
        sleep 3
        clear

        download_file "$SCRIPT_URL" "$SCRIPT_TMP" && cp "$SCRIPT_TMP" "$Module_dir/webssh.sh"
        add_alias

        echo ""
        echo "======================================"
        echo "       WebSSH 安装完成"
        echo "--------------------------------------"
        echo "  版本   : $REMOTE_VERSION"
        echo "  目录   : $Module_dir"
        echo "  访问   : http://192.168.0.1:8899"
        echo "  快捷键 : webssh (重开终端生效)"
        echo "======================================"
        echo ""
        echo "  首次使用请访问上方地址进行初始化配置"
    fi
}

force_install() {
    echo "强制安装模式 - 跳过版本检查"
    
    echo "获取版本信息..."
    if REMOTE_VERSION=$(fetch_url "$VERSION_URL" 2>/dev/null); then
        REMOTE_VERSION=$(echo "$REMOTE_VERSION" | tr -d '\r\n')
        echo "  最新版本: $REMOTE_VERSION"
    else
        REMOTE_VERSION="未知"
        echo "  获取版本失败，使用未知版本"
    fi
    
    echo "停止 WebSSH 服务..."
    $STOP_CMD 2>/dev/null || true
    
    mkdir -p "$Module_dir"
    
    echo "下载 WebSSH 主程序..."
    if ! download_file "${WEBSH_URL_PREFIX}${REMOTE_VERSION}" "$Module_dir/webssh" 1; then
        echo "  下载失败，请检查网络或链接"
        return 1
    fi
    echo "  下载完成"

    chmod 755 "$Module_dir/webssh"

    cat > "$Module_dir/service.sh" << SEOF
#!/system/bin/sh
Module_dir="$Module_dir"
chmod 777 \$Module_dir/webssh
start() {
    if ! (ps -ef | grep -- 'webssh' | grep -vE 'grep') > /dev/null ; then
        cd \$Module_dir
        nohup \$Module_dir/webssh > /dev/null 2>&1 &

        echo "WebSSH已启动"
    else
        echo "WebSSH正在运行中，不执行启动命令"
    fi
}

stop() {
    if (ps -ef | grep -- 'webssh' | grep -vE 'grep') > /dev/null ; then
        pid=\$(pgrep -f 'webssh')
        kill -15 \$pid

        echo "WebSSH已关闭"
    else
        echo "WebSSH未在运行，不执行停止命令"
    fi
}

case "\$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    *)
        echo "只能使用start|stop两个参数控制WebSSH启动或停止"
        exit 1
        ;;
esac
SEOF
    chmod 755 "$Module_dir/service.sh"
    
    # 创建版本文件
    echo "$REMOTE_VERSION" > "$Module_dir/VERSION.txt"

    # 确保 rc.local 存在
    if [ ! -f "$FILE" ]; then
        echo "没有找到自启动脚本，插件不会开机自启"
    else
        # 开机自启（写入 exit 0 前）
        echo "设置开机自启..."
        if grep -F "$BOOT_CMD" "$FILE" >/dev/null 2>&1; then
            echo "开机脚本已存在，无需重复添加"
        else
            sed -i "/^exit 0/i $BOOT_CMD" "$FILE"
            echo "已添加: $BOOT_CMD"
        fi
    fi

    # 启动
    echo "启动 WebSSH 服务..."
    if ! $BOOT_CMD 2>/dev/null; then
        echo "启动失败，请检查 $Module_dir/service.sh"
        return 1
    fi
    
    sleep 3
    clear

    download_file "$SCRIPT_URL" "$SCRIPT_TMP" && cp "$SCRIPT_TMP" "$Module_dir/webssh.sh"
    add_alias

    echo ""
    echo "======================================"
    echo "       WebSSH 强制安装完成"
    echo "--------------------------------------"
    echo "  版本   : $REMOTE_VERSION"
    echo "  目录   : $Module_dir"
    echo "  访问   : http://192.168.0.1:8899"
    echo "  快捷键 : webssh (重开终端生效)"
    echo "======================================"
    echo ""
    echo "  首次使用请访问上方地址进行初始化配置"
}

remove() {
    clear
    # 停止
    if ! $STOP_CMD 2>/dev/null; then
        echo "停止失败或服务未运行"
    fi

    # 删除 rc.local 中的相关行
    if [ -f "$FILE" ]; then
        sed -i "/kano_web_ssh/d" "$FILE"
    fi

    # 清除别名
    ALIAS_CMD="alias webssh=\"ash $Module_dir/webssh.sh\""
    for f in /etc/shinit /etc/profile; do
        if [ -f "$f" ]; then
            sed -i "\|$ALIAS_CMD|d" "$f"
        fi
    done

    # 删除目录
    rm -rf "$Module_dir"

    echo "卸载完成"
}

check_is_installed() {
    # 确保已安装
    if [ ! -f "$Module_dir/service.sh" ]; then
        echo "未检测到 WEBSSH，请先安装"
        exit 1
    fi
}





start() {
    # 确保已安装
    check_is_installed
    $BOOT_CMD
}

stop() {
    # 确保已安装
    check_is_installed
    $STOP_CMD
}

restart() {
    check_is_installed
    echo "重启 WebSSH 服务..."
    $STOP_CMD
    $BOOT_CMD
}

while true; do
    clear
    _menu_ver=$(fetch_url "$VERSION_URL" 2>/dev/null | tr -d '\r\n')
    _menu_ver=${_menu_ver:-"未知"}
    _menu_date=$(echo "$_menu_ver" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)_.*/\1-\2-\3/')
    if [ -f "$Module_dir/VERSION.txt" ]; then
        _local_ver=$(cat "$Module_dir/VERSION.txt" 2>/dev/null | tr -d '\r\n')
    else
        _local_ver=""
    fi

    if [ -z "$_local_ver" ]; then
        _installed=0
    else
        _installed=1
    fi

    if [ "$_installed" = "1" ] && [ "$_local_ver" != "$_menu_ver" ]; then
        _has_update=1
    else
        _has_update=0
    fi

    _idx=1
    _install_idx=""
    _start_idx=""
    _stop_idx=""
    _restart_idx=""
    echo "======================================"
    echo "       WebSSH(高级后台) 管理脚本"
    echo "--------------------------------------"
    echo "  程序作者 : MiniKano GITHUB@cdwangtao(二改)"
    if [ "$_installed" = "1" ]; then
        if [ "$_has_update" = "1" ]; then
            echo "  当前版本: $_local_ver (有更新)"
        else
            echo "  当前版本: $_local_ver"
        fi
    fi
    if [ "$_has_update" = "1" ]; then
        echo "  最新版本: $_menu_ver"
        echo "  发布日期: $_menu_date"
    fi
    echo "--------------------------------------"
    if [ "$_installed" = "0" ]; then
        _install_idx=$_idx
        echo "  $_idx) 安装 (install)"
        _idx=$((_idx + 1))
    elif [ "$_has_update" = "1" ]; then
        _install_idx=$_idx
        echo "  $_idx) 更新 (update)"
        _idx=$((_idx + 1))
    fi
    _force_idx=$_idx; echo "  $_idx) 强制安装 (force install)"; _idx=$((_idx + 1))
    _remove_idx=$_idx; echo "  $_idx) 卸载 (remove)"; _idx=$((_idx + 1))
    if [ "$_installed" = "1" ]; then
        _start_idx=$_idx; echo "  $_idx) 启动 (start)"; _idx=$((_idx + 1))
        _stop_idx=$_idx; echo "  $_idx) 停止 (stop)"; _idx=$((_idx + 1))
        _restart_idx=$_idx; echo "  $_idx) 重启 (restart)"; _idx=$((_idx + 1))
    fi
    _exit_idx=0
    echo "  0) 退出 (exit)"
    echo "======================================"
    echo
    read -rp "请输入选择: " choice </dev/tty

    case "$choice" in
        $_install_idx)
            setup_webssh
            read -rp "按回车键继续..." dummy </dev/tty
            ;;
        $_force_idx)
            force_install
            read -rp "按回车键继续..." dummy </dev/tty
            ;;
        $_remove_idx)
            remove
            read -rp "按回车键继续..." dummy </dev/tty
            ;;
        $_start_idx)
            start
            read -rp "按回车键继续..." dummy </dev/tty
            ;;
        $_stop_idx)
            stop
            read -rp "按回车键继续..." dummy </dev/tty
            ;;
        $_restart_idx)
            restart
            read -rp "按回车键继续..." dummy </dev/tty
            ;;
        0)
            echo "已退出"
            exit 0
            ;;
        *)
            echo "无效的选择，请输入 1-5 或 0"
            sleep 1
            ;;
    esac
done