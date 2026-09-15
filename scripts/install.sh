#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

install_dir="/opt/media"

[[ $EUID -ne 0 ]] && echo -e "${red}错误：${plain} 必须使用root用户运行此脚本！\n" && exit 1

if [[ -f /etc/alpine-release ]]; then
    release="alpine"
elif [[ -d /run/systemd/system ]]; then
    release="systemd"
else
    echo -e "${red}未检测到系统版本或系统不支持，请联系脚本作者！${plain}\n" && exit 1
fi

arch=$(arch)
if [[ $arch == "x86_64" || $arch == "x64" || $arch == "amd64" ]]; then
    arch="amd64"
elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
    arch="arm64"
else
    echo -e "${red}检测架构失败，当前架构不受支持: ${arch}${plain}"
    exit 1
fi

echo "架构: ${arch}"

if [[ $(getconf LONG_BIT) != '64' ]]; then
    echo "本软件不支持 32 位系统，请使用 64 位系统"
    exit 2
fi

is_cmd_exist() {
    local cmd="$1"
    [[ -n "$cmd" ]] && command -v "$cmd" >/dev/null 2>&1
}

install_base() {
    if [[ $release == "alpine" ]]; then
        apk add --no-cache wget curl tar
    elif is_cmd_exist apt-get; then
        apt-get update
        apt-get install -y wget curl tar
    elif is_cmd_exist dnf; then
        dnf install -y wget curl tar
    elif is_cmd_exist yum; then
        yum install -y wget curl tar
    fi
}

install_media() {
    local version="${1:-latest}"
    local temp_dir
    local archive
    local url
    local binary

    mkdir -p "$install_dir"
    temp_dir=$(mktemp -d)
    archive="${temp_dir}/media-agent.tar.gz"
    if [[ $version == "latest" ]]; then
        version=$(curl -fsSL "https://api.github.com/repos/HuTuTuOnO/MediaUnlock/releases/latest" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
        [[ -z $version ]] && echo -e "${red}获取 Media 最新版本失败${plain}" && rm -rf "$temp_dir" && exit 1
        version=${version#v}
        url="https://github.com/HuTuTuOnO/MediaUnlock/releases/download/v${version}/agent-v${version}-linux-${arch}.tar.gz"
    else
        version=${version#v}
        url="https://github.com/HuTuTuOnO/MediaUnlock/releases/download/v${version}/agent-v${version}-linux-${arch}.tar.gz"
    fi

    echo -e "开始安装 Media ${version}"
    curl -fL --retry 3 -o "$archive" "$url"
    if [[ $? -ne 0 ]]; then
        echo -e "${red}下载 Media 失败，请确保你的服务器能够下载 Github 的文件${plain}"
        rm -rf "$temp_dir"
        exit 1
    fi

    mkdir -p "${temp_dir}/unpack"
    tar zxvf "$archive" -C "${temp_dir}/unpack" >/dev/null
    binary=$(find "${temp_dir}/unpack" -type f \( -name agent -o -name media-agent \) -print -quit)
    [[ -z $binary ]] && echo -e "${red}安装包中未找到 Media 文件${plain}" && rm -rf "$temp_dir" && exit 1
    command install -m 0755 "$binary" "${install_dir}/media-agent"
    rm -rf "$temp_dir"

    if [[ ! -f "${install_dir}/config.yml" ]]; then
        if ! curl -fsSL -o "${install_dir}/config.yml" "https://raw.githubusercontent.com/HuTuTuOnO/MediaUnlock/main/scripts/extras/config.yml"; then
            echo -e "${red}下载配置文件失败，请确保你的服务器能够下载 Github 的文件${plain}"
            rm -f "${install_dir}/config.yml"
            exit 1
        fi
        chmod 0644 "${install_dir}/config.yml"
    fi

    if [[ $release == "alpine" ]]; then
        if ! curl -fsSL -o /etc/init.d/media "https://raw.githubusercontent.com/HuTuTuOnO/MediaUnlock/main/scripts/services/media"; then
            echo -e "${red}下载服务脚本失败，请确保你的服务器能够下载 Github 的文件${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/media
        rc-update add media default
    else
        if ! curl -fsSL -o /etc/systemd/system/media.service "https://raw.githubusercontent.com/HuTuTuOnO/MediaUnlock/main/scripts/services/media.service"; then
            echo -e "${red}下载服务脚本失败，请确保你的服务器能够下载 Github 的文件${plain}"
            exit 1
        fi
        chmod 0644 /etc/systemd/system/media.service
        systemctl daemon-reload
        systemctl enable media.service
    fi

    if ! curl -fsSL -o /usr/bin/media "https://raw.githubusercontent.com/HuTuTuOnO/MediaUnlock/main/scripts/media.sh"; then
        echo -e "${red}下载 Media 管理脚本失败，请确保你的服务器能够下载 Github 的文件${plain}"
        exit 1
    fi
    chmod +x /usr/bin/media

    echo -e "${green}Media 安装完成，已设置开机自启${plain}"
    echo -e "请配置 ${install_dir}/config.yml，然后运行 media start"
}

main() {
    echo -e "${green}开始安装${plain}"
    install_base
    install_media "${1:-latest}"
}

main "$@"
