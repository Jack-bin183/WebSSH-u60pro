#!/bin/sh

# =========================
# mihomo mixed transparent proxy control
# TCP  -> redir-port 7892
# DNS  -> mihomo dns 1053
# UDP  -> TUN utun via policy route
#
# 性能优先版：
# - chnroute 命中：IPv4 CN 直接 RETURN，不进 redir / TUN / mihomo
# - chnroute6 命中：IPv6 CN 直接走系统原生 IPv6 出口
# - 非 CN IPv6：快速 REJECT，避免 Google/GPT 等国外 IPv6 串出去，并促使客户端回落 IPv4
# - TCP DNS 先 RETURN，再由 DNS 劫持规则转到 1053
# - TUN 必须 auto-route: false，由本脚本手动给 UDP 打 mark
# =========================

MIHOMO_DIR="/data/kano_plugins/mihomo"
MIHOMO_BIN="$MIHOMO_DIR/mihomo"
MIHOMO_CONFIG="$MIHOMO_DIR/config.yaml"

LAN_IF="br-lan"
TUN_IF="utun"

TCP_REDIR_PORT="7892"
DNS_PORT="1053"

TABLE_ID="2022"
MARK_ID="0x162"
RULE_PREF="8999"

PID_FILE="$MIHOMO_DIR/mihomo.pid"
LOG_FILE="$MIHOMO_DIR/logs/mihomo.log"

TCP_CHAIN="MIHOMO_TCP"
UDP_CHAIN="MIHOMO_UDP"
V6_CHAIN="MIHOMO_V6"

# CN IP 绕过集合：一行一个 CIDR
IPSET_NAME="chnroute"
CHNROUTE_FILE="$MIHOMO_DIR/chnroute.txt"

# CN IPv6 绕过集合：一行一个 IPv6 CIDR，例如 240e::/20
IPSET6_NAME="chnroute6"
CHNROUTE6_FILE="$MIHOMO_DIR/chnroute6.txt"
ENABLE_IPV6_GUARD="1"

mkdir -p "$MIHOMO_DIR/logs"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

ipt_nat() {
    iptables -t nat "$@"
}

ipt_mangle() {
    iptables -t mangle "$@"
}

ip6t_filter() {
    ip6tables -t filter "$@"
}

# -------------------------
# load cn ipset
# -------------------------
load_chnroute() {
    if ! command -v ipset >/dev/null 2>&1; then
        log "未找到 ipset，跳过 CN IP 绕过"
        return 1
    fi

    ipset create "$IPSET_NAME" hash:net family inet hashsize 4096 maxelem 200000 2>/dev/null

    if [ ! -s "$CHNROUTE_FILE" ]; then
        log "未找到 $CHNROUTE_FILE，跳过 CN IP 绕过"
        return 1
    fi

    log "加载 CN IP 集合：$IPSET_NAME <- $CHNROUTE_FILE"
    ipset flush "$IPSET_NAME" 2>/dev/null

    while IFS= read -r ip; do
        case "$ip" in
            ""|\#*) continue ;;
        esac
        ipset add "$IPSET_NAME" "$ip" 2>/dev/null
    done < "$CHNROUTE_FILE"

    COUNT="$(ipset list "$IPSET_NAME" 2>/dev/null | awk -F': ' '/Number of entries/ {print $2}')"
    log "CN IP 集合加载完成，entries=${COUNT:-unknown}"
    return 0
}

ipset_ready() {
    ipset list "$IPSET_NAME" >/dev/null 2>&1
}

load_chnroute6() {
    if [ "$ENABLE_IPV6_GUARD" != "1" ]; then
        log "IPv6 防泄漏未启用，跳过 CN IPv6 集合"
        return 0
    fi

    if ! command -v ipset >/dev/null 2>&1; then
        log "未找到 ipset，跳过 CN IPv6 绕过"
        return 1
    fi

    ipset create "$IPSET6_NAME" hash:net family inet6 hashsize 4096 maxelem 200000 2>/dev/null

    if [ ! -s "$CHNROUTE6_FILE" ]; then
        log "未找到 $CHNROUTE6_FILE，将只做 IPv4 绕过；IPv6 防泄漏链不会启用"
        return 1
    fi

    log "加载 CN IPv6 集合：$IPSET6_NAME <- $CHNROUTE6_FILE"
    ipset flush "$IPSET6_NAME" 2>/dev/null

    while IFS= read -r ip; do
        case "$ip" in
            ""|\#*) continue ;;
        esac
        ipset add "$IPSET6_NAME" "$ip" 2>/dev/null
    done < "$CHNROUTE6_FILE"

    COUNT="$(ipset list "$IPSET6_NAME" 2>/dev/null | awk -F': ' '/Number of entries/ {print $2}')"
    log "CN IPv6 集合加载完成，entries=${COUNT:-unknown}"
    return 0
}

ipset6_ready() {
    [ "$ENABLE_IPV6_GUARD" = "1" ] && ipset list "$IPSET6_NAME" >/dev/null 2>&1
}

# -------------------------
# clean rules
# -------------------------
clean_rules() {
    log "清理 iptables / ip rule / route 规则..."

    # TCP redir
    while ipt_nat -D PREROUTING -i "$LAN_IF" -p tcp -j "$TCP_CHAIN" 2>/dev/null; do :; done
    ipt_nat -F "$TCP_CHAIN" 2>/dev/null
    ipt_nat -X "$TCP_CHAIN" 2>/dev/null

    # DNS hijack
    while ipt_nat -D PREROUTING -i "$LAN_IF" -p udp --dport 53 -j REDIRECT --to-ports "$DNS_PORT" 2>/dev/null; do :; done
    while ipt_nat -D PREROUTING -i "$LAN_IF" -p tcp --dport 53 -j REDIRECT --to-ports "$DNS_PORT" 2>/dev/null; do :; done

    # UDP mark
    while ipt_mangle -D PREROUTING -i "$LAN_IF" -p udp -j "$UDP_CHAIN" 2>/dev/null; do :; done
    ipt_mangle -F "$UDP_CHAIN" 2>/dev/null
    ipt_mangle -X "$UDP_CHAIN" 2>/dev/null

    # FORWARD allow
    while iptables -D FORWARD -i "$LAN_IF" -o "$TUN_IF" -j ACCEPT 2>/dev/null; do :; done
    while iptables -D FORWARD -i "$TUN_IF" -o "$LAN_IF" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null; do :; done

    # IPv6 guard
    while ip6t_filter -D FORWARD -i "$LAN_IF" -j "$V6_CHAIN" 2>/dev/null; do :; done
    ip6t_filter -F "$V6_CHAIN" 2>/dev/null
    ip6t_filter -X "$V6_CHAIN" 2>/dev/null

    # policy route
    ip rule del fwmark "$MARK_ID" table "$TABLE_ID" 2>/dev/null
    ip rule del pref "$RULE_PREF" 2>/dev/null
    ip route flush table "$TABLE_ID" 2>/dev/null
    ip route flush cache 2>/dev/null
}

# -------------------------
# add rules
# -------------------------
add_rules() {
    log "添加 TCP redir 规则..."

    ipt_nat -N "$TCP_CHAIN" 2>/dev/null
    ipt_nat -F "$TCP_CHAIN"

    # bypass reserved / private / multicast
    ipt_nat -A "$TCP_CHAIN" -d 0.0.0.0/8 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 10.0.0.0/8 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 127.0.0.0/8 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 169.254.0.0/16 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 172.16.0.0/12 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 192.168.0.0/16 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 224.0.0.0/4 -j RETURN
    ipt_nat -A "$TCP_CHAIN" -d 240.0.0.0/4 -j RETURN

    # TCP DNS 不走 redir，返回 PREROUTING 后由 DNS 劫持规则转到 1053
    ipt_nat -A "$TCP_CHAIN" -p tcp --dport 53 -j RETURN

    # CN IP 直连，不进 mihomo
    if ipset_ready; then
        ipt_nat -A "$TCP_CHAIN" -m set --match-set "$IPSET_NAME" dst -j RETURN
    fi

    # TCP -> mihomo redir-port
    ipt_nat -A "$TCP_CHAIN" -p tcp -j REDIRECT --to-ports "$TCP_REDIR_PORT"

    while ipt_nat -D PREROUTING -i "$LAN_IF" -p tcp -j "$TCP_CHAIN" 2>/dev/null; do :; done
    ipt_nat -A PREROUTING -i "$LAN_IF" -p tcp -j "$TCP_CHAIN"

    log "添加 DNS 劫持规则..."

    while ipt_nat -D PREROUTING -i "$LAN_IF" -p udp --dport 53 -j REDIRECT --to-ports "$DNS_PORT" 2>/dev/null; do :; done
    while ipt_nat -D PREROUTING -i "$LAN_IF" -p tcp --dport 53 -j REDIRECT --to-ports "$DNS_PORT" 2>/dev/null; do :; done

    ipt_nat -A PREROUTING -i "$LAN_IF" -p udp --dport 53 -j REDIRECT --to-ports "$DNS_PORT"
    ipt_nat -A PREROUTING -i "$LAN_IF" -p tcp --dport 53 -j REDIRECT --to-ports "$DNS_PORT"

    log "添加 UDP -> TUN 规则..."

    ipt_mangle -N "$UDP_CHAIN" 2>/dev/null
    ipt_mangle -F "$UDP_CHAIN"

    # bypass reserved / private / multicast
    ipt_mangle -A "$UDP_CHAIN" -d 0.0.0.0/8 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 10.0.0.0/8 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 127.0.0.0/8 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 169.254.0.0/16 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 172.16.0.0/12 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 192.168.0.0/16 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 224.0.0.0/4 -j RETURN
    ipt_mangle -A "$UDP_CHAIN" -d 240.0.0.0/4 -j RETURN

    # DNS 已经单独劫持到 1053，不再送 TUN
    ipt_mangle -A "$UDP_CHAIN" -p udp --dport 53 -j RETURN

    # CN IP 直连，不进 TUN
    if ipset_ready; then
        ipt_mangle -A "$UDP_CHAIN" -m set --match-set "$IPSET_NAME" dst -j RETURN
    fi

    # UDP -> mark
    ipt_mangle -A "$UDP_CHAIN" -p udp -j MARK --set-mark "$MARK_ID"

    while ipt_mangle -D PREROUTING -i "$LAN_IF" -p udp -j "$UDP_CHAIN" 2>/dev/null; do :; done
    ipt_mangle -A PREROUTING -i "$LAN_IF" -p udp -j "$UDP_CHAIN"

    log "添加 UDP 策略路由 table $TABLE_ID..."

    ip route flush table "$TABLE_ID" 2>/dev/null
    ip route add default dev "$TUN_IF" table "$TABLE_ID"

    ip rule del fwmark "$MARK_ID" table "$TABLE_ID" 2>/dev/null
    ip rule del pref "$RULE_PREF" 2>/dev/null
    ip rule add fwmark "$MARK_ID" table "$TABLE_ID" pref "$RULE_PREF"

    ip route flush cache 2>/dev/null

    log "添加 FORWARD 放行规则..."

    while iptables -D FORWARD -i "$LAN_IF" -o "$TUN_IF" -j ACCEPT 2>/dev/null; do :; done
    while iptables -D FORWARD -i "$TUN_IF" -o "$LAN_IF" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null; do :; done

    iptables -I FORWARD -i "$LAN_IF" -o "$TUN_IF" -j ACCEPT
    iptables -I FORWARD -i "$TUN_IF" -o "$LAN_IF" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

    add_ipv6_guard
}


# -------------------------
# add IPv6 guard
# -------------------------
add_ipv6_guard() {
    if [ "$ENABLE_IPV6_GUARD" != "1" ]; then
        log "IPv6 防泄漏未启用"
        return 0
    fi

    if ! command -v ip6tables >/dev/null 2>&1; then
        log "未找到 ip6tables，跳过 IPv6 防泄漏规则"
        return 1
    fi

    if ! ipset6_ready; then
        log "CN IPv6 集合未就绪，跳过 IPv6 防泄漏规则，避免误杀全部 IPv6"
        return 1
    fi

    log "添加 IPv6 规则：CN IPv6 直连，非 CN IPv6 快速 REJECT"

    ip6t_filter -N "$V6_CHAIN" 2>/dev/null
    ip6t_filter -F "$V6_CHAIN"

    # 本地/内网/组播 IPv6 不处理
    ip6t_filter -A "$V6_CHAIN" -d ::1/128 -j RETURN
    ip6t_filter -A "$V6_CHAIN" -d fe80::/10 -j RETURN
    ip6t_filter -A "$V6_CHAIN" -d fc00::/7 -j RETURN
    ip6t_filter -A "$V6_CHAIN" -d ff00::/8 -j RETURN

    # 大陆 IPv6 直连：返回原有 OpenWrt 转发/NAT/防火墙流程
    ip6t_filter -A "$V6_CHAIN" -m set --match-set "$IPSET6_NAME" dst -j RETURN

    # 其他 IPv6 直接拒绝，让客户端快速回落 IPv4，避免国外 IPv6 串出去
    ip6t_filter -A "$V6_CHAIN" -j REJECT

    while ip6t_filter -D FORWARD -i "$LAN_IF" -j "$V6_CHAIN" 2>/dev/null; do :; done
    ip6t_filter -I FORWARD -i "$LAN_IF" -j "$V6_CHAIN"
}

# -------------------------
# wait tun
# -------------------------
wait_tun() {
    log "等待 $TUN_IF 出现..."

    i=0
    while [ $i -lt 15 ]; do
        if ip link show "$TUN_IF" >/dev/null 2>&1; then
            log "$TUN_IF 已就绪"
            return 0
        fi
        sleep 1
        i=$((i + 1))
    done

    log "错误：$TUN_IF 未出现，请检查 mihomo tun 配置"
    return 1
}

# -------------------------
# start mihomo
# -------------------------
start_mihomo() {
    if [ -f "$PID_FILE" ]; then
        OLD_PID="$(cat "$PID_FILE" 2>/dev/null)"
        if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
            log "mihomo 已在运行，PID=$OLD_PID"
            return 0
        fi
    fi

    if pidof mihomo >/dev/null 2>&1; then
        log "检测到已有 mihomo 进程，先停止"
        killall mihomo 2>/dev/null
        sleep 1
    fi

    log "启动 mihomo..."
    # 日志超过 2MB 就清空
    [ -f "$LOG_FILE" ] && [ "$(wc -c < "$LOG_FILE")" -gt 2097152 ] && : > "$LOG_FILE"
    cd "$MIHOMO_DIR" || exit 1

    nohup "$MIHOMO_BIN" -d "$MIHOMO_DIR" -f "$MIHOMO_CONFIG" >> "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"

    sleep 2

    if ! kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        log "mihomo 启动失败，请查看日志：$LOG_FILE"
        return 1
    fi

    log "mihomo 已启动，PID=$(cat "$PID_FILE")"
}

# -------------------------
# stop mihomo
# -------------------------
stop_mihomo() {
    log "停止 mihomo..."

    if [ -f "$PID_FILE" ]; then
        PID="$(cat "$PID_FILE" 2>/dev/null)"
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            kill "$PID" 2>/dev/null
            sleep 2
            kill -9 "$PID" 2>/dev/null
        fi
        rm -f "$PID_FILE"
    fi

    killall mihomo 2>/dev/null

    ip link delete "$TUN_IF" 2>/dev/null
}

# -------------------------
# status
# -------------------------
status_mihomo() {
    echo "=== mihomo process ==="
    ps | grep -i mihomo | grep -v grep

    echo
    echo "=== listen ports ==="
    netstat -lntup 2>/dev/null | grep -E '7890|7891|7892|7893|1053|9090'

    echo
    echo "=== tun ==="
    ip addr show "$TUN_IF" 2>/dev/null

    echo
    echo "=== ip rule ==="
    ip rule

    echo
    echo "=== table $TABLE_ID ==="
    ip route show table "$TABLE_ID"

    echo
    echo "=== ipset $IPSET_NAME ==="
    if command -v ipset >/dev/null 2>&1 && ipset list "$IPSET_NAME" >/dev/null 2>&1; then
        ipset list "$IPSET_NAME" | grep -E 'Name:|Type:|Revision:|Header:|Number of entries'
    else
        echo "$IPSET_NAME not loaded"
    fi

    echo
    echo "=== ipset $IPSET6_NAME ==="
    if command -v ipset >/dev/null 2>&1 && ipset list "$IPSET6_NAME" >/dev/null 2>&1; then
        ipset list "$IPSET6_NAME" | grep -E 'Name:|Type:|Revision:|Header:|Number of entries'
    else
        echo "$IPSET6_NAME not loaded"
    fi

    echo
    echo "=== nat chain ==="
    iptables -t nat -L "$TCP_CHAIN" -n -v 2>/dev/null

    echo
    echo "=== udp chain ==="
    iptables -t mangle -L "$UDP_CHAIN" -n -v 2>/dev/null

    echo
    echo "=== forward ==="
    iptables -L FORWARD -n -v | grep -E "$LAN_IF|$TUN_IF|DROP|REJECT"

    echo
    echo "=== ipv6 guard chain ==="
    ip6tables -L "$V6_CHAIN" -n -v 2>/dev/null
}

case "$1" in
    start)
        clean_rules
        load_chnroute
        load_chnroute6
        start_mihomo || exit 1
        wait_tun || exit 1
        add_rules
        log "启动完成：CN -> direct，TCP -> redir:$TCP_REDIR_PORT，DNS -> $DNS_PORT，UDP -> $TUN_IF"
        ;;

    stop)
        clean_rules
        stop_mihomo
        log "已关闭 mihomo 并清理规则"
        ;;

    restart)
        clean_rules
        stop_mihomo
        sleep 1
        load_chnroute
        load_chnroute6
        start_mihomo || exit 1
        wait_tun || exit 1
        add_rules
        log "重启完成"
        ;;

    status)
        status_mihomo
        ;;

    clean)
        clean_rules
        ip link delete "$TUN_IF" 2>/dev/null
        log "规则已清理"
        ;;

    reload-ipset)
        load_chnroute
        load_chnroute6
        log "CN IPv4/IPv6 集合已重新加载"
        ;;

    *)
        echo "用法：$0 {start|stop|restart|status|clean|reload-ipset}"
        echo
        echo "start        启动 mihomo 并添加透明代理规则"
        echo "stop         停止 mihomo 并移除规则"
        echo "restart      重启 mihomo 并重建规则"
        echo "status       查看状态"
        echo "clean        只清理规则，不杀 mihomo"
        echo "reload-ipset 只重新加载 chnroute / chnroute6 ipset"
        exit 1
        ;;
esac

exit 0
