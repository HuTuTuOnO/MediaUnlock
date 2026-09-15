#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

[[ $EUID -ne 0 ]] && echo -e "${red}错误: ${plain} 必须使用root用户运行此脚本！\n" && exit 1

if [[ -f /etc/alpine-release ]]; then
    release="alpine"
elif [[ -d /run/systemd/system ]]; then
    release="systemd"
else
    echo -e "${red}未检测到系统版本或系统不支持，请联系脚本作者！${plain}\n" && exit 1
fi

confirm() {
    if [[ $# -gt 1 ]]; then
        echo && read -p "$1 [默认$2]: " temp
        [[ -z "$temp" ]] && temp=$2
    else
        read -p "$1 [y/n]: " temp
    fi
    [[ "$temp" == "y" || "$temp" == "Y" ]] && return 0 || return 1
}

before_show_menu() {
    echo && echo -n -e "${yellow}按回车返回主菜单: ${plain}" && read temp
    show_menu
}

install() {
    bash <(curl -Ls https://raw.githubusercontent.com/HuTuTuOnO/MediaUnlock/main/scripts/install.sh)
    [[ $? == 0 ]] && start 0
}

update() {
    if [[ $# == 0 ]]; then
        echo && echo -n -e "输入指定版本(默认最新版): " && read media_version
        [[ -z "$media_version" ]] && media_version="latest"
    else
        media_version="${2:-latest}"
    fi
    bash <(curl -Ls https://raw.githubusercontent.com/HuTuTuOnO/MediaUnlock/main/scripts/install.sh) "$media_version"
    if [[ $? == 0 ]]; then
        restart 0
        echo -e "${green}更新完成，已自动重启 Media，请使用 media log 查看运行日志${plain}"
        exit
    fi

    [[ $# == 0 ]] && before_show_menu
}

uninstall() {
    confirm "确定要卸载 Media 吗?" "n"
    if [[ $? != 0 ]]; then
        [[ $# == 0 ]] && show_menu
        return 0
    fi

    if [[ $release == "alpine" ]]; then
        rc-service media stop >/dev/null 2>&1
        rc-update del media >/dev/null 2>&1
        rm /etc/init.d/media -f
    else
        systemctl stop media >/dev/null 2>&1
        systemctl disable media >/dev/null 2>&1
        rm /etc/systemd/system/media.service -f
        systemctl daemon-reload
        systemctl reset-failed
    fi
    rm /opt/media/ -rf

    echo ""
    echo -e "卸载成功，如果你想删除此脚本，则退出脚本后运行 ${green}rm /usr/bin/media -f${plain} 进行删除"
    echo ""

    [[ $# == 0 ]] && before_show_menu
}

start() {
    check_status
    if [[ $? == 0 ]]; then
        echo ""
        echo -e "${green}Media已运行，无需再次启动，如需重启请选择重启${plain}"
    else
        if [[ $release == "alpine" ]]; then
            rc-service media start >/dev/null 2>&1
        else
            systemctl reset-failed media >/dev/null 2>&1
            systemctl start media >/dev/null 2>&1
        fi
        sleep 2
        check_status
        if [[ $? == 0 ]]; then
            echo -e "${green}Media 启动成功，请使用 media log 查看运行日志${plain}"
        else
            echo -e "${red}Media可能启动失败，请稍后使用 media log 查看日志信息${plain}"
        fi
    fi

    [[ $# == 0 ]] && before_show_menu
}

stop() {
    if [[ $release == "alpine" ]]; then
        rc-service media stop >/dev/null 2>&1
    else
        systemctl stop media >/dev/null 2>&1
    fi
    sleep 2
    check_status
    if [[ $? == 1 ]]; then
        echo -e "${green}Media 停止成功${plain}"
    else
        echo -e "${red}Media停止失败，可能是因为停止时间超过了两秒，请稍后查看日志信息${plain}"
    fi

    [[ $# == 0 ]] && before_show_menu
}

restart() {
    if [[ $release == "alpine" ]]; then
        rc-service media restart >/dev/null 2>&1
    else
        systemctl reset-failed media >/dev/null 2>&1
        systemctl restart media >/dev/null 2>&1
    fi
    sleep 2
    check_status
    if [[ $? == 0 ]]; then
        echo -e "${green}Media 重启成功，请使用 media log 查看运行日志${plain}"
    else
        echo -e "${red}Media可能启动失败，请稍后使用 media log 查看日志信息${plain}"
    fi
    [[ $# == 0 ]] && before_show_menu
}

enable() {
    if [[ $release == "alpine" ]]; then
        rc-update add media default
    else
        systemctl enable media
    fi
    if [[ $? == 0 ]]; then
        echo -e "${green}Media 设置开机自启成功${plain}"
    else
        echo -e "${red}Media 设置开机自启失败${plain}"
    fi

    [[ $# == 0 ]] && before_show_menu
}

disable() {
    if [[ $release == "alpine" ]]; then
        rc-update del media default
    else
        systemctl disable media
    fi
    if [[ $? == 0 ]]; then
        echo -e "${green}Media 取消开机自启成功${plain}"
    else
        echo -e "${red}Media 取消开机自启失败${plain}"
    fi

    [[ $# == 0 ]] && before_show_menu
}

show_log() {
    if [[ $release == "alpine" ]]; then
        tail -f /var/log/media.log
    else
        journalctl -u media.service -e --no-pager -f
    fi
    [[ $# == 0 ]] && before_show_menu
}

check_status() {
    if [[ $release == "alpine" ]]; then
        [[ ! -f /etc/init.d/media ]] && return 2
        rc-service media status 2>&1 | grep -q "started" && return 0
        return 1
    fi
    [[ ! -f /etc/systemd/system/media.service ]] && return 2
    systemctl is-active --quiet media.service && return 0
    return 1
}

check_enabled() {
    if [[ $release == "alpine" ]]; then
        rc-status | grep -q 'media'
        [[ $? == 0 ]] && return 0 || return 1
    fi
    temp=$(systemctl is-enabled media 2>/dev/null)
    [[ "$temp" == "enabled" ]] && return 0 || return 1
}

check_uninstall() {
    check_status
    if [[ $? != 2 ]]; then
        echo ""
        echo -e "${red}Media已安装，请不要重复安装${plain}"
        [[ $# == 0 ]] && before_show_menu
        return 1
    fi
    return 0
}

check_install() {
    check_status
    if [[ $? == 2 ]]; then
        echo ""
        echo -e "${red}请先安装 Media${plain}"
        [[ $# == 0 ]] && before_show_menu
        return 1
    fi
    return 0
}

show_status() {
    check_status
    case $? in
        0)
            echo -e "Media状态: ${green}已运行${plain}"
            show_enable_status
            ;;
        1)
            echo -e "Media状态: ${yellow}未运行${plain}"
            show_enable_status
            ;;
        2)
            echo -e "Media状态: ${red}未安装${plain}"
            ;;
    esac
}

show_enable_status() {
    check_enabled
    if [[ $? == 0 ]]; then
        echo -e "是否开机自启: ${green}是${plain}"
    else
        echo -e "是否开机自启: ${red}否${plain}"
    fi
}

show_media_version() {
    echo -n "Media 版本："
    /opt/media/media-agent -version
    echo ""
    [[ $# == 0 ]] && before_show_menu
}

show_usage() {
    echo "Media 管理脚本使用方法: "
    echo "------------------------------------------"
    echo "media                    - 显示管理菜单"
    echo "media start              - 启动 Media"
    echo "media stop               - 停止 Media"
    echo "media restart            - 重启 Media"
    echo "media status             - 查看 Media 状态"
    echo "media enable             - 设置 Media 开机自启"
    echo "media disable            - 取消 Media 开机自启"
    echo "media log                - 查看 Media 日志"
    echo "media update             - 更新 Media 最新版"
    echo "media update x.x.x       - 更新 Media 指定版本"
    echo "media install            - 安装 Media"
    echo "media uninstall          - 卸载 Media"
    echo "media version            - 查看 Media 版本"
    echo "------------------------------------------"
}

show_menu() {
    echo -e "
  ${green}Media 后端管理脚本${plain}

  ${green}0.${plain} 退出脚本
————————————————
  ${green}1.${plain} 安装 Media
  ${green}2.${plain} 更新 Media
  ${green}3.${plain} 卸载 Media
————————————————
  ${green}4.${plain} 启动 Media
  ${green}5.${plain} 停止 Media
  ${green}6.${plain} 重启 Media
  ${green}7.${plain} 查看 Media 日志
————————————————
  ${green}8.${plain} 设置 Media 自启
  ${green}9.${plain} 取消 Media 自启
————————————————
 ${green}10.${plain} 查看 Media 状态
 ${green}11.${plain} 查看 Media 版本
 "
    show_status
    echo && read -p "请输入选择 [0-11]: " num

    case "$num" in
        0) exit 0 ;;
        1) check_uninstall && install ;;
        2) check_install && update ;;
        3) check_install && uninstall ;;
        4) check_install && start ;;
        5) check_install && stop ;;
        6) check_install && restart ;;
        7) check_install && show_log ;;
        8) check_install && enable ;;
        9) check_install && disable ;;
        10) show_status ;;
        11) check_install && show_media_version ;;
        *) echo -e "${red}请输入正确的数字 [0-11]${plain}" ;;
    esac
}

if [[ $# -gt 0 ]]; then
    case "$1" in
        start) check_install 0 && start 0 ;;
        stop) check_install 0 && stop 0 ;;
        restart) check_install 0 && restart 0 ;;
        status) show_status ;;
        enable) check_install 0 && enable 0 ;;
        disable) check_install 0 && disable 0 ;;
        log) check_install 0 && show_log 0 ;;
        update)
            check_install 0 && update 0 "${2:-latest}"
            ;;
        install) check_uninstall 0 && install 0 ;;
        uninstall) check_install 0 && uninstall 0 ;;
        version) check_install 0 && show_media_version 0 ;;
        *) show_usage ;;
    esac
else
    show_menu
fi
