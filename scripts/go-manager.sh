#!/bin/bash

# ============================================
# Go SDK 下载管理器 (403 修复版)  --By Iori
# 修复内容：
# 1. 解决 403 Forbidden 错误
# 2. 使用随机 User-Agent 头
# 3. 增加 Referer 和 Accept 头
# 4. 降低请求频率，避免触发防护
# ============================================

# 使用更安全的选项
set -euo pipefail
shopt -s nullglob

# 配置参数
readonly GO_INSTALL_DIR="${HOME}/.go/versions"
readonly GO_CURRENT_LINK="${HOME}/.go/current"
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_VERSION="4.0.3"
# 自定义主镜像（可选），例如: export GO_MIRROR="https://mirrors.aliyun.com/golang"
readonly GO_MIRROR="${GO_MIRROR:-}"
# 下载重试与网络参数（可通过环境变量覆盖）
readonly GO_DL_RETRIES="${GO_DL_RETRIES:-3}"
readonly GO_DL_CONNECT_TIMEOUT="${GO_DL_CONNECT_TIMEOUT:-12}"
readonly GO_DL_MAX_TIME="${GO_DL_MAX_TIME:-240}"
readonly GO_DL_BACKOFF_BASE="${GO_DL_BACKOFF_BASE:-3}"
readonly GO_DL_FORCE_IPV4="${GO_DL_FORCE_IPV4:-true}"
readonly GO_DL_PROBE_ENABLED="${GO_DL_PROBE_ENABLED:-auto}"
readonly GO_DL_PROBE_TIMEOUT="${GO_DL_PROBE_TIMEOUT:-4}"
readonly GO_DL_FAST_FAIL_HTTP="${GO_DL_FAST_FAIL_HTTP:-true}"
# 是否在校验失败时保留下载的压缩包，便于手动检查
# 可通过环境变量覆盖: export GO_KEEP_BAD_ARCHIVE=false
readonly KEEP_BAD_ARCHIVE="${GO_KEEP_BAD_ARCHIVE:-true}"
 # 调试开关，默认开启；如需关闭可设置: export DEBUG=false
readonly DEBUG="${DEBUG:-true}"

# 颜色输出函数
if [[ -t 1 ]]; then
    # 使用 $'' 语法生成真正的 ESC 控制符，避免在 here-doc 中打印出字面量 \033
    RED=$'\033[0;31m'
    GREEN=$'\033[0;32m'
    YELLOW=$'\033[1;33m'
    BLUE=$'\033[0;34m'
    MAGENTA=$'\033[0;35m'
    CYAN=$'\033[0;36m'
    NC=$'\033[0m'
    BOLD=$'\033[1m'
else
    RED='' GREEN='' YELLOW='' BLUE='' MAGENTA='' CYAN='' NC='' BOLD=''
fi

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { 
    echo -e "${RED}[ERROR]${NC} $1" >&2
    return 1
}
log_debug() { 
    [[ "${DEBUG}" == "true" ]] && echo -e "${MAGENTA}[DEBUG]${NC} $1"
}

# 全局临时资源跟踪，配合 EXIT trap 做统一清理
GLOBAL_TEMP_FILE=""
GLOBAL_TEMP_DIR=""

cleanup_on_exit() {
    # 清理临时文件
    if [[ -n "${GLOBAL_TEMP_FILE:-}" && -e "$GLOBAL_TEMP_FILE" ]]; then
        rm -f "$GLOBAL_TEMP_FILE" 2>/dev/null || true
    fi

    # 清理临时目录
    if [[ -n "${GLOBAL_TEMP_DIR:-}" && -d "$GLOBAL_TEMP_DIR" ]]; then
        rm -rf "$GLOBAL_TEMP_DIR" 2>/dev/null || true
    fi
}

# 只在 EXIT 时做统一清理，Ctrl+C 交给默认/主入口 trap 直接退出
trap cleanup_on_exit EXIT

# 检测 Shell 配置文件
detect_shell_profile() {
    local shell_name
    if [[ -n "${SHELL:-}" ]]; then
        shell_name="$(basename "$SHELL")"
    else
        shell_name="bash"
    fi
    
    case "$shell_name" in
        bash) echo "${HOME}/.bashrc" ;;
        zsh) echo "${HOME}/.zshrc" ;;
        fish) echo "${HOME}/.config/fish/config.fish" ;;
        *) echo "${HOME}/.bashrc" ;;
    esac
}

SHELL_PROFILE="${GO_SHELL_PROFILE:-$(detect_shell_profile)}"

# ============================================
# HTTP 头和 User-Agent 管理
# ============================================

# 生成随机 User-Agent
generate_user_agent() {
    local uas=(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0"
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/121.0"
        "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/121.0"
    )
    
    local idx=$(( RANDOM % ${#uas[@]} ))
    echo "${uas[$idx]}"
}

# 获取通用 HTTP 头
get_http_headers() {
    local user_agent="$(generate_user_agent)"
    
    cat <<EOF
User-Agent: $user_agent
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-US,en;q=0.5
Accept-Encoding: gzip, deflate, br
DNT: 1
Connection: keep-alive
Upgrade-Insecure-Requests: 1
Sec-Fetch-Dest: document
Sec-Fetch-Mode: navigate
Sec-Fetch-Site: none
Sec-Fetch-User: ?1
Cache-Control: max-age=0
EOF
}

# 获取下载文件的 HTTP 头
get_download_headers() {
    local user_agent="$(generate_user_agent)"
    
    cat <<EOF
User-Agent: $user_agent
Accept: */*
Accept-Language: en-US,en;q=0.9
Accept-Encoding: identity
Range: bytes=0-
Connection: keep-alive
Referer: https://golang.org/dl/
Sec-Fetch-Dest: document
Sec-Fetch-Mode: navigate
Sec-Fetch-Site: cross-site
Pragma: no-cache
Cache-Control: no-cache
EOF
}

build_download_referer() {
    local url="$1"

    # 常见镜像使用更贴近站点结构的 Referer，减少异常风控概率。
    case "$url" in
        *"mirrors.aliyun.com/golang/"*) echo "https://mirrors.aliyun.com/golang/"; return 0 ;;
        *"mirrors.ustc.edu.cn/golang/"*) echo "https://mirrors.ustc.edu.cn/golang/"; return 0 ;;
        *"mirrors.cloud.tencent.com/golang/"*) echo "https://mirrors.cloud.tencent.com/golang/"; return 0 ;;
        *"mirrors.tuna.tsinghua.edu.cn/golang/"*) echo "https://mirrors.tuna.tsinghua.edu.cn/golang/"; return 0 ;;
        *"mirrors.bfsu.edu.cn/golang/"*) echo "https://mirrors.bfsu.edu.cn/golang/"; return 0 ;;
        *"go.dev/dl/"*|*"golang.org/dl/"*|*"golang.google.cn/dl/"*) echo "https://go.dev/dl/"; return 0 ;;
        *"dl.google.com/go/"*|*"storage.googleapis.com/golang/"*) echo "https://go.dev/dl/"; return 0 ;;
    esac

    local host
    host="$(echo "$url" | sed -E 's#^https?://([^/]+)/?.*#\1#' 2>/dev/null || true)"
    if [[ -n "$host" ]]; then
        echo "https://${host}/dl/"
    else
        echo "https://go.dev/dl/"
    fi
}

# ============================================
# 验证版本号格式
# ============================================

validate_version() {
    local version="$1"
    if [[ ! "$version" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
        log_error "无效的版本号格式: $version (请使用如 1.25.0 格式)"
        return 1
    fi
    return 0
}

# ============================================
# 系统检测函数
# ============================================

detect_arch() {
    local arch
    arch="$(uname -m 2>/dev/null || echo "unknown")"
    
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        i386|i686) echo "386" ;;
        armv7l|armv8l) echo "armv6l" ;;
        *) echo "amd64" ;;
    esac
}

detect_os() {
    local os
    os="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo "linux")"
    
    case "$os" in
        linux) echo "linux" ;;
        darwin) echo "darwin" ;;
        freebsd) echo "freebsd" ;;
        openbsd) echo "openbsd" ;;
        netbsd) echo "netbsd" ;;
        windows) echo "windows" ;;
        *) echo "linux" ;;
    esac
}

# ============================================
# 依赖检查
# ============================================

check_dependencies() {
    log_info "检查系统依赖..."
    
    # 检查 tar 命令
    if ! command -v tar &>/dev/null; then
        log_error "缺少必需的命令: tar"
        return 1
    fi
    
    # 检查下载工具
    local has_download_tool=false
    
    for tool in curl wget python3 python; do
        if command -v "$tool" &>/dev/null; then
            has_download_tool=true
            break
        fi
    done
    
    if [[ "$has_download_tool" == "false" ]]; then
        log_error "没有检测到下载工具 (需要 curl, wget, python3 或 python)"
        return 1
    fi
    
    log_success "依赖检查通过"
    return 0
}

# ============================================
# 目录管理
# ============================================

create_directories() {
    log_info "正在创建必要目录..."
    
    # 创建 .go 目录
    local go_dir="${HOME}/.go"
    mkdir -p "$go_dir" 2>/dev/null || {
        log_error "无法创建目录: $go_dir"
        return 1
    }
    
    # 创建版本目录
    mkdir -p "$GO_INSTALL_DIR" 2>/dev/null || {
        log_error "无法创建目录: $GO_INSTALL_DIR"
        return 1
    }
    
    return 0
}

# ============================================
# 下载函数 (修复 403 错误)
# ============================================

# 清理临时文件
cleanup_temp_files() {
    local temp_file="${1:-}"
    if [[ -n "$temp_file" && -e "$temp_file" ]]; then
        rm -f "$temp_file" 2>/dev/null || true
    fi
}

# 使用 curl 下载 (带随机 UA 和延迟)
download_with_curl_fixed() {
    local url="$1"
    local output_file="$2"
    local attempt="${3:-1}"

    log_info "[尝试 $attempt] 使用 curl 下载"

    # 生成随机 User-Agent
    local user_agent
    user_agent="$(generate_user_agent)"
    local referer
    referer="$(build_download_referer "$url")"
    local cookie_file="${output_file}.cookies"
    local part_file="${output_file}.part"

    # 适度随机延迟，避免触发防护，但不要过长
    local delay=$(( RANDOM % 3 + 1 ))  # 1-3 秒
    log_debug "等待 ${delay} 秒以避免请求过快..."
    sleep "$delay"

    # 先请求下载页面，模拟浏览器访问流程并获取 cookie（失败不终止主下载）
    local preflight_opts=(
        "--location"
        "--silent"
        "--show-error"
        "--connect-timeout" "$GO_DL_CONNECT_TIMEOUT"
        "--max-time" "20"
        "--cookie-jar" "$cookie_file"
        "-H" "User-Agent: $user_agent"
        "-H" "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
    )
    if [[ "$GO_DL_FORCE_IPV4" == "true" ]]; then
        preflight_opts+=("--ipv4")
    fi
    curl "${preflight_opts[@]}" "$referer" -o /dev/null 2>/dev/null || true

    # 构建 curl 命令（增强版）
    local curl_opts=(
        "--location"                # 自动跟随重定向
        "--fail"                    # 4xx/5xx 视为失败
        "--silent"                  # 静默输出
        "--show-error"              # 仍输出错误信息
        "--http1.1"                 # 避免部分网络环境中的 HTTP/2 兼容问题
        "--continue-at" "-"        # 断点续传，提高长链路成功率
        "--retry" "$GO_DL_RETRIES"
        "--retry-delay" "2"
        "--retry-all-errors"
        "--connect-timeout" "$GO_DL_CONNECT_TIMEOUT"
        "--max-time" "$GO_DL_MAX_TIME"
        "--cookie" "$cookie_file"
        "-H" "User-Agent: $user_agent"
        "-H" "Accept: */*"
        "-H" "Accept-Language: en-US,en;q=0.9"
        "-H" "Accept-Encoding: identity"
        "-H" "Referer: $referer"
        "-H" "DNT: 1"
        "-H" "Connection: keep-alive"
        "-H" "Sec-Fetch-Dest: document"
        "-H" "Sec-Fetch-Mode: navigate"
        "-H" "Sec-Fetch-Site: cross-site"
        "-H" "Pragma: no-cache"
        "-H" "Cache-Control: no-cache"
    )
    if [[ "$GO_DL_FORCE_IPV4" == "true" ]]; then
        curl_opts+=("--ipv4")
    fi

    # 进度显示
    if [[ -t 1 ]]; then
        curl_opts+=("--progress-bar")
    else
        curl_opts+=("--no-progress-meter")
    fi

    log_debug "执行命令: curl ${curl_opts[*]} \"$url\" -o \"$part_file\""

    # set -e 下显式捕获退出码，避免命令失败导致脚本提前退出
    local http_code="000"
    set +e
    http_code="$(curl "${curl_opts[@]}" --write-out "%{http_code}" "$url" -o "$part_file")"
    local ec=$?
    set -e
    http_code="$(echo "$http_code" | tr -d '[:space:]')"
    if [[ $ec -ne 0 ]]; then
        log_warn "curl 下载失败，退出码: $ec, HTTP状态: ${http_code:-unknown}"
        if [[ "$GO_DL_FAST_FAIL_HTTP" == "true" && $ec -eq 22 ]]; then
            case "$http_code" in
                403|404|410)
                    log_warn "检测到确定性 HTTP 错误(${http_code})，该镜像本轮不再重试"
                    rm -f "$part_file" "$cookie_file" 2>/dev/null || true
                    return 2
                    ;;
            esac
        fi
        rm -f "$part_file" "$cookie_file" 2>/dev/null || true
        return 1
    fi

    if [[ ! -s "$part_file" ]]; then
        log_warn "curl 下载完成，但文件为空"
        rm -f "$part_file" "$cookie_file" 2>/dev/null || true
        return 1
    fi

    mv -f "$part_file" "$output_file" 2>/dev/null || {
        log_warn "curl 下载成功，但写入目标文件失败"
        rm -f "$part_file" "$cookie_file" 2>/dev/null || true
        return 1
    }

    local file_size
    file_size="$(wc -c < "$output_file" 2>/dev/null || echo 0)"
    rm -f "$cookie_file" 2>/dev/null || true
    log_success "curl 下载成功: $((file_size/1024/1024)) MB"
    return 0
}

# 使用 wget 下载 (带随机 UA)
download_with_wget_fixed() {
    local url="$1"
    local output_file="$2"
    local attempt="${3:-1}"
    
    log_info "[尝试 $attempt] 使用 wget 下载"
    
    # 生成随机 User-Agent
    local user_agent
    user_agent="$(generate_user_agent)"
    local referer
    referer="$(build_download_referer "$url")"
    local part_file="${output_file}.part"

    # 适度随机延迟
    local delay=$(( RANDOM % 3 + 1 ))
    log_debug "等待 ${delay} 秒以避免请求过快..."
    sleep "$delay"
    
    # 构建 wget 命令（增强版）
    local wget_opts=(
        "--tries=${GO_DL_RETRIES}"
        "--connect-timeout=${GO_DL_CONNECT_TIMEOUT}"
        "--read-timeout=90"
        "--dns-timeout=${GO_DL_CONNECT_TIMEOUT}"
        "--waitretry=3"
        "--retry-connrefused"
        "--continue"
        "--max-redirect=10"
        "--no-iri"
        "--user-agent=$user_agent"
        "--header=Accept: */*"
        "--header=Accept-Language: en-US,en;q=0.9"
        "--header=Accept-Encoding: identity"
        "--header=Referer: $referer"
        "--header=DNT: 1"
        "--header=Connection: keep-alive"
        "--header=Sec-Fetch-Dest: document"
        "--header=Sec-Fetch-Mode: navigate"
        "--header=Sec-Fetch-Site: cross-site"
        "--header=Pragma: no-cache"
        "--header=Cache-Control: no-cache"
    )
    if [[ "$GO_DL_FORCE_IPV4" == "true" ]]; then
        wget_opts+=("--inet4-only")
    fi

    # 进度显示
    if [[ -t 1 ]]; then
        wget_opts+=("--show-progress")
    else
        wget_opts+=("--quiet")
    fi
    
    log_debug "执行命令: wget ${wget_opts[*]} -O \"$part_file\" \"$url\""

    # set -e 下显式捕获退出码，避免命令失败导致脚本提前退出
    set +e
    wget "${wget_opts[@]}" -O "$part_file" "$url"
    local ec=$?
    set -e
    if [[ $ec -ne 0 ]]; then
        log_warn "wget 下载失败，退出码: $ec"
        rm -f "$part_file" 2>/dev/null || true
        return 1
    fi
    
    if [[ ! -s "$part_file" ]]; then
        log_warn "wget 下载完成，但文件为空"
        rm -f "$part_file" 2>/dev/null || true
        return 1
    fi

    mv -f "$part_file" "$output_file" 2>/dev/null || {
        log_warn "wget 下载成功，但写入目标文件失败"
        rm -f "$part_file" 2>/dev/null || true
        return 1
    }

    local file_size
    file_size="$(wc -c < "$output_file" 2>/dev/null || echo 0)"
    log_success "wget 下载成功: $((file_size/1024/1024)) MB"
    return 0
}

# 使用 Python 下载 (带随机 UA)
download_with_python_fixed() {
    local url="$1"
    local output_file="$2"
    local attempt="${3:-1}"
    
    log_info "[尝试 $attempt] 使用 Python 下载"
    
    # 添加随机延迟
    local delay=$(( RANDOM % 5 + 3 ))
    log_debug "等待 ${delay} 秒以避免请求过快..."
    sleep "$delay"
    
    # 尝试不同的 Python 解释器和方法
    for py_cmd in "python3" "python"; do
        if command -v "$py_cmd" &>/dev/null; then
            log_debug "使用 $py_cmd"
            
            # 方法1: 使用 urllib
            local python_code='
import sys
import urllib.request
import ssl
import time
import random

url = sys.argv[1]
output = sys.argv[2]
max_retries = 2

# 随机 User-Agent
user_agents = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
]
user_agent = random.choice(user_agents)

# 自定义 opener
class CustomOpener(urllib.request.FancyURLopener):
    version = user_agent

# 创建不验证 SSL 的上下文
ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

for i in range(max_retries):
    try:
        # 添加延迟避免请求过快
        if i > 0:
            time.sleep(random.uniform(3, 8))
        
        # 设置请求头
        headers = {
            "User-Agent": user_agent,
            "Accept": "*/*",
            "Accept-Language": "en-US,en;q=0.9",
            "Referer": "https://golang.org/dl/",
            "DNT": "1",
            "Connection": "keep-alive",
        }
        
        req = urllib.request.Request(url, headers=headers)
        
        with urllib.request.urlopen(req, context=ctx, timeout=60) as response:
            total_size = int(response.getheader("Content-Length", 0))
            downloaded = 0
            chunk_size = 65536  # 64KB chunks
            
            with open(output, "wb") as f:
                while True:
                    chunk = response.read(chunk_size)
                    if not chunk:
                        break
                    f.write(chunk)
                    downloaded += len(chunk)
                    
                    # 显示进度
                    if total_size > 0:
                        percent = (downloaded / total_size) * 100
                        sys.stderr.write(f"\\r下载进度: {percent:.1f}% ({downloaded}/{total_size} bytes)")
                        
                sys.stderr.write("\\n")
        
        print("下载完成")
        sys.exit(0)
        
    except Exception as e:
        print(f"尝试 {i+1} 失败: {e}")
        if i < max_retries - 1:
            print(f"等待 5 秒后重试...")
            time.sleep(5)
        else:
            print(f"所有尝试失败")
            sys.exit(1)
'
            
            log_debug "使用 urllib 方法"
            if echo "$python_code" | "$py_cmd" - "$url" "$output_file" 2>/dev/null; then
                if [[ -s "$output_file" ]]; then
                    log_success "Python urllib 下载成功"
                    return 0
                fi
            fi
            
            # 清理失败的文件
            rm -f "$output_file" 2>/dev/null || true
            
            # 方法2: 如果安装了 requests 库
            python_code='
try:
    import requests
    import sys
    import time
    import random

    url = sys.argv[1]
    output = sys.argv[2]
    
    # 随机 User-Agent
    user_agents = [
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
    ]
    user_agent = random.choice(user_agents)
    
    headers = {
        "User-Agent": user_agent,
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.9",
        "Referer": "https://golang.org/dl/",
        "DNT": "1",
        "Connection": "keep-alive",
    }
    
    # 添加延迟
    time.sleep(random.uniform(2, 5))
    
    response = requests.get(url, headers=headers, stream=True, timeout=60, verify=False)
    response.raise_for_status()
    
    total_size = int(response.headers.get("content-length", 0))
    downloaded = 0
    
    with open(output, "wb") as f:
        for chunk in response.iter_content(chunk_size=65536):
            if chunk:
                f.write(chunk)
                downloaded += len(chunk)
                
                if total_size > 0:
                    percent = (downloaded / total_size) * 100
                    sys.stderr.write(f"\\r下载进度: {percent:.1f}%")
    
    sys.stderr.write("\\n")
    print("下载完成")
    sys.exit(0)
    
except ImportError:
    print("requests 库未安装")
    sys.exit(1)
except Exception as e:
    print(f"下载失败: {e}")
    sys.exit(1)
'
            
            log_debug "尝试 requests 方法"
            if echo "$python_code" | "$py_cmd" - "$url" "$output_file" 2>/dev/null; then
                if [[ -s "$output_file" ]]; then
                    log_success "Python requests 下载成功"
                    return 0
                fi
            fi
            
            # 再次清理
            rm -f "$output_file" 2>/dev/null || true
        fi
    done
    
    log_warn "Python 下载失败"
    return 1
}

# 统一下载接口
download_file_fixed() {
    local url="$1"
    local output_file="$2"
    local max_attempts="${3:-3}"
    
    log_info "开始下载: $(basename "$url")"
    log_debug "URL: $url"
    
    # 确保输出目录存在
    local output_dir
    output_dir="$(dirname "$output_file")"
    mkdir -p "$output_dir" 2>/dev/null || true
    
    # 检测可用工具
    local tools=()
    
    if command -v curl &>/dev/null; then
        tools+=("curl")
    fi
    
    if command -v wget &>/dev/null; then
        tools+=("wget")
    fi
    # Python 下载器默认关闭，如需启用可设置环境变量 GO_USE_PYTHON_DOWNLOADER=true
    if [[ "${GO_USE_PYTHON_DOWNLOADER:-false}" == "true" ]]; then
        if command -v python3 &>/dev/null || command -v python &>/dev/null; then
            tools+=("python")
        fi
    fi
    
    if [[ ${#tools[@]} -eq 0 ]]; then
        log_error "没有可用的下载工具"
        return 1
    fi
    
    log_info "可用下载工具: ${tools[*]}"
    
    # 尝试每种工具
    local attempt=1
    while [[ $attempt -le $max_attempts ]]; do
        log_info "=== 下载尝试 $attempt/$max_attempts ==="
        local deterministic_http_error=false

        for tool in "${tools[@]}"; do
            local tool_rc=1
            case "$tool" in
                curl)
                    if download_with_curl_fixed "$url" "$output_file" "$attempt"; then
                        return 0
                    else
                        tool_rc=$?
                    fi
                    ;;
                wget)
                    if download_with_wget_fixed "$url" "$output_file" "$attempt"; then
                        return 0
                    else
                        tool_rc=$?
                    fi
                    ;;
                python)
                    if download_with_python_fixed "$url" "$output_file" "$attempt"; then
                        return 0
                    else
                        tool_rc=$?
                    fi
                    ;;
            esac

            if [[ $tool_rc -eq 2 ]]; then
                deterministic_http_error=true
            fi

            # 清理失败的文件
            rm -f "$output_file" 2>/dev/null || true
            
            # 在不同工具之间添加延迟
            sleep 2
        done

        if [[ "$deterministic_http_error" == "true" ]]; then
            log_warn "检测到确定性 HTTP 错误，跳过该镜像的后续重试"
            return 2
        fi

        attempt=$((attempt + 1))
        
        if [[ $attempt -le $max_attempts ]]; then
            local wait_time=$((attempt * GO_DL_BACKOFF_BASE + RANDOM % 5))
            log_warn "所有工具尝试失败，等待 ${wait_time} 秒后重试..."
            sleep "$wait_time"
        fi
    done
    
    log_error "所有下载方法均失败 (网络超时/连接失败/403)"
    log_info "可能的原因："
    log_info "1. 服务器临时限制或维护"
    log_info "2. 您的IP地址被暂时限制"
    log_info "3. 需要 VPN 或代理访问"
    
    # 建议的手动下载方案
    log_info ""
    log_info "💡 建议手动下载："
    log_info "1. 在其他网络环境下载文件"
    log_info "2. 复制到本机 /tmp/ 目录"
    log_info "3. 重新运行此脚本"
    
    return 1
}

# ============================================
# 镜像源管理
# ============================================

# 获取可用的镜像列表 (带备用)
get_mirror_urls_fixed() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local filename="go${version}.${os}-${arch}.tar.gz"

    local mirrors=()

    # 1. 用户自定义主镜像（如果配置了，则优先）
    if [[ -n "$GO_MIRROR" ]]; then
        # 去掉可能的末尾斜杠
        local trimmed_mirror="${GO_MIRROR%/}"
        mirrors+=("${trimmed_mirror}/${filename}")
    fi

    # 2. 官方/官方中国镜像（稳定性优先）
    mirrors+=(
        "https://golang.google.cn/dl/${filename}"   # 官方中国
        "https://dl.google.com/go/${filename}"      # 官方 Google
        "https://go.dev/dl/${filename}"             # Go 官网
        "https://storage.googleapis.com/golang/${filename}"
        "https://golang.org/dl/${filename}"
    )

    # 3. 国内公共镜像作为兜底
    mirrors+=(
        "https://mirrors.aliyun.com/golang/${filename}"
        "https://mirrors.ustc.edu.cn/golang/${filename}"
        "https://mirrors.cloud.tencent.com/golang/${filename}"
        "https://mirrors.tuna.tsinghua.edu.cn/golang/${filename}"
        "https://mirrors.bfsu.edu.cn/golang/${filename}"
    )

    # 稳定顺序输出（不再随机打乱，方便排查和保障稳定性）
    printf '%s\n' "${mirrors[@]}"
}

probe_mirror_url() {
    local url="$1"
    local user_agent
    user_agent="$(generate_user_agent)"

    # 先尝试 HEAD；若被目标站点策略拦截，再尝试 0-1 字节范围 GET
    if command -v curl &>/dev/null; then
        local probe_opts=(
            "--location"
            "--silent"
            "--show-error"
            "--connect-timeout" "$GO_DL_PROBE_TIMEOUT"
            "--max-time" "$GO_DL_PROBE_TIMEOUT"
            "-H" "User-Agent: $user_agent"
            "-H" "Accept: */*"
        )
        if [[ "$GO_DL_FORCE_IPV4" == "true" ]]; then
            probe_opts+=("--ipv4")
        fi

        if curl "${probe_opts[@]}" --fail --head "$url" -o /dev/null 2>/dev/null; then
            return 0
        fi
        if curl "${probe_opts[@]}" --fail -H "Range: bytes=0-1" "$url" -o /dev/null 2>/dev/null; then
            return 0
        fi
    fi

    if command -v wget &>/dev/null; then
        local wget_probe_opts=(
            "--spider"
            "--tries=1"
            "--connect-timeout=${GO_DL_PROBE_TIMEOUT}"
            "--read-timeout=${GO_DL_PROBE_TIMEOUT}"
            "--timeout=${GO_DL_PROBE_TIMEOUT}"
            "--user-agent=$user_agent"
            "--header=Accept: */*"
            "--quiet"
        )
        if [[ "$GO_DL_FORCE_IPV4" == "true" ]]; then
            wget_probe_opts+=("--inet4-only")
        fi

        if wget "${wget_probe_opts[@]}" "$url"; then
            return 0
        fi
    fi

    return 1
}

select_mirror_candidates() {
    local version="$1"
    local os="$2"
    local arch="$3"

    local all_mirrors=()
    mapfile -t all_mirrors < <(get_mirror_urls_fixed "$version" "$os" "$arch")

    if [[ "${GO_DL_PROBE_ENABLED}" == "false" || ${#all_mirrors[@]} -le 1 ]]; then
        printf '%s\n' "${all_mirrors[@]}"
        return 0
    fi

    if [[ "${GO_DL_PROBE_ENABLED}" != "true" && "${GO_DL_PROBE_ENABLED}" != "auto" ]]; then
        log_warn "未知 GO_DL_PROBE_ENABLED=${GO_DL_PROBE_ENABLED}，回退到 auto" >&2
    fi

    # 注意：本函数会被 mapfile 捕获 stdout，日志必须写到 stderr，避免污染 URL 列表。
    log_info "快速探测镜像可达性（超时 ${GO_DL_PROBE_TIMEOUT}s）..." >&2

    local reachable=()
    local degraded=()
    local url
    for url in "${all_mirrors[@]}"; do
        if [[ -z "$url" ]]; then
            continue
        fi
        if probe_mirror_url "$url"; then
            reachable+=("$url")
            log_debug "探测通过: $url" >&2
        else
            degraded+=("$url")
            log_debug "探测失败（仍保留兜底）: $url" >&2
        fi
    done

    if [[ ${#reachable[@]} -gt 0 ]]; then
        printf '%s\n' "${reachable[@]}"
    fi
    if [[ ${#degraded[@]} -gt 0 ]]; then
        printf '%s\n' "${degraded[@]}"
    fi
}

# 验证下载的 Go 压缩包
validate_go_archive() {
    local archive_file="$1"
    local expected_version="${2:-}"   # 期望版本号，如: 1.25.4
    
    if [[ ! -f "$archive_file" ]]; then
        log_error "文件不存在: $archive_file"
        return 1
    fi
    
    # 检查文件大小
    local file_size
    file_size="$(wc -c < "$archive_file" 2>/dev/null || echo 0)"
    
    log_debug "文件大小: $((file_size/1024/1024)) MB"
    
    if [[ $file_size -lt 20000000 ]]; then
        log_error "文件太小: $((file_size/1024/1024)) MB (预期至少50MB)"
        return 1
    fi
    
    # 检查是否是有效的 tar.gz 文件
    log_info "验证压缩包格式..."
    if ! tar -tzf "$archive_file" &>/dev/null; then
        log_error "不是有效的 tar.gz 文件"
        
        # 检查是否是HTML页面（403错误页面）
        if head -c 200 "$archive_file" | grep -qi "<html\|403\|forbidden"; then
            log_error "下载到了错误页面 (可能是403 Forbidden页面)"
            
            # 显示文件开头以帮助调试
            log_debug "文件开头内容:"
            while read -r line; do
                log_debug "  $line"
            done < <(head -3 "$archive_file" || true)
        fi
        
        return 1
    fi
    
    # 获取文件列表，用于定位 VERSION 文件
    log_info "检查压缩包内容..."
    local file_list
    file_list="$(tar -tzf "$archive_file" 2>/dev/null)"

    # 调试时展示前若干行，便于排查
    log_debug "压缩包内容示例（前 20 行）："
    while read -r line; do
        log_debug "  $line"
    done < <(printf '%s\n' "$file_list" | head -20 || true)

    # 检查 VERSION 文件是否存在且内容合理（进一步确认是 Go SDK）
    local version_file
    # 优先使用 go/VERSION，其次任意 VERSION
    version_file="$(echo "$file_list" | grep -E '^go/VERSION$' | head -1 || true)"
    if [[ -z "$version_file" ]]; then
        version_file="$(echo "$file_list" | grep -E '(^|/)VERSION$' | head -1 || true)"
    fi
    if [[ -z "$version_file" ]]; then
        log_error "压缩包中没有找到 VERSION 文件"
        return 1
    fi

    local version_line
    version_line="$(tar -xOf "$archive_file" "$version_file" 2>/dev/null | head -1 || true)"
    local version_trimmed
    version_trimmed="$(echo "$version_line" | tr -d '[:space:]')"

    if [[ -z "$version_trimmed" || ! "$version_trimmed" =~ ^go[0-9]+\.[0-9]+ ]]; then
        log_error "VERSION 文件内容异常: ${version_line:-<空>}"
        return 1
    fi

    log_debug "检测到 Go 版本标记: $version_trimmed"

    # 如果传入了期望版本号，则要求 VERSION 第一行与之完全匹配
    if [[ -n "$expected_version" ]]; then
        local expected_tag="go${expected_version}"
        if [[ "$version_trimmed" != "$expected_tag" ]]; then
            log_error "VERSION 文件版本不匹配: 期望 ${expected_tag}, 实际 ${version_trimmed}"
            return 1
        fi
    fi
    
    log_success "压缩包验证通过"
    return 0
}

# ============================================
# 主要安装函数
# ============================================

check_version_installed() {
    local version="$1"
    [[ -d "${GO_INSTALL_DIR}/go${version}" && -x "${GO_INSTALL_DIR}/go${version}/bin/go" ]]
}

install_go_version_fixed() {
    local version="$1"
    local os arch filename install_path
    local temp_file=""
    local download_success=false
    local final_download_url=""
    
    # 验证版本号
    if ! validate_version "$version"; then
        return 1
    fi
    
    # 检查是否已安装
    if check_version_installed "$version"; then
        log_warn "Go $version 已安装"
        read -r -p "是否重新安装? (y/N): " reinstall
        if [[ ! "$reinstall" =~ ^[Yy]$ ]]; then
            log_info "跳过安装"
            return 0
        fi
    fi
    
    # 检测系统和架构
    os="$(detect_os)" || return 1
    arch="$(detect_arch)" || return 1
    filename="go${version}.${os}-${arch}.tar.gz"
    install_path="${GO_INSTALL_DIR}/go${version}"
    
    log_info "准备安装 Go ${version} (${os}/${arch})..."
    log_info "安装路径: $install_path"
    
    # 创建安装目录
    mkdir -p "$install_path" || {
        log_error "无法创建安装目录: $install_path"
        return 1
    }
    
    # 先检查是否有手动下载的文件
    local possible_locations=(
        "/tmp/${filename}"
        "/tmp/go-${version}-*.tar.gz"
        "${HOME}/Downloads/${filename}"
        "./${filename}"
    )
    
    for location in "${possible_locations[@]}"; do
        for file in $location; do
            if [[ -f "$file" ]]; then
                log_info "发现已下载的文件: $file"
                read -r -p "是否使用此文件? (Y/n): " use_existing
                if [[ ! "$use_existing" =~ ^[Nn]$ ]]; then
                    temp_file="$(mktemp /tmp/go-use-existing-XXXXXX.tar.gz)"
                    cp "$file" "$temp_file"
                    
                    if validate_go_archive "$temp_file" "$version"; then
                        download_success=true
                        final_download_url="(本地文件: $file)"
                        log_success "使用本地文件: $file"
                        break 2
                    else
                        log_warn "本地文件验证失败"
                        rm -f "$temp_file"
                    fi
                fi
            fi
        done
    done
    
    # 如果没有本地文件，尝试下载
    if [[ "$download_success" != "true" ]]; then
        # 创建临时文件
        temp_file="$(mktemp /tmp/go-${version}-XXXXXX.tar.gz 2>/dev/null || echo "/tmp/go-${version}-$$.tar.gz")"
        GLOBAL_TEMP_FILE="$temp_file"
        
        # 获取镜像列表（探测可达性后优先尝试可用镜像）
        local candidate_mirrors=()
        mapfile -t candidate_mirrors < <(select_mirror_candidates "$version" "$os" "$arch")

        local mirror_index=0
        for mirror_url in "${candidate_mirrors[@]}"; do
            [[ -z "$mirror_url" ]] && continue

            # 防御性校验：仅处理 http/https URL，防止日志串或异常内容被当作镜像地址。
            if [[ ! "$mirror_url" =~ ^https?:// ]]; then
                log_warn "跳过无效镜像地址: $mirror_url"
                continue
            fi

            mirror_index=$((mirror_index + 1))
            
            # 提取镜像名称用于显示
            local mirror_name
            case "$mirror_url" in
                *aliyun*) mirror_name="阿里云镜像" ;;
                *ustc.edu*) mirror_name="中科大镜像" ;;
                *cloud.tencent*) mirror_name="腾讯云镜像" ;;
                *tuna.tsinghua*) mirror_name="清华大学镜像" ;;
                *bfsu.edu*) mirror_name="北外镜像" ;;
                *golang.google.cn*) mirror_name="官方中国镜像" ;;
                *dl.google.com*) mirror_name="Google镜像" ;;
                *golang.org*) mirror_name="官方镜像" ;;
                *storage.googleapis.com*) mirror_name="Google存储" ;;
                *go.dev*) mirror_name="Go官网" ;;
                *) mirror_name="镜像 #$mirror_index" ;;
            esac
            
            log_info "尝试 [$mirror_index] $mirror_name"
            
            # 使用修复的下载函数
            if download_file_fixed "$mirror_url" "$temp_file" 2; then
                if validate_go_archive "$temp_file" "$version"; then
                    download_success=true
                    final_download_url="$mirror_url"
                    log_success "下载成功: $mirror_name"
                    break
                else
                    log_warn "压缩包验证失败，尝试下一个镜像..."
                    # 默认保留无效压缩包，便于手动检查；可通过 GO_KEEP_BAD_ARCHIVE=false 关闭
                    if [[ "${KEEP_BAD_ARCHIVE}" == "true" ]]; then
                        log_info "已按配置保留无效压缩包: $temp_file"
                    else
                        rm -f "$temp_file" 2>/dev/null || true
                    fi
                fi
            else
                log_warn "下载失败: $mirror_name"
            fi
            
            # 在镜像之间添加延迟
            sleep 3
        done
    fi
    
    if [[ "$download_success" != "true" ]]; then
        log_error "无法下载 Go ${version}"
        
        # 提供详细的手动下载指南
        log_info ""
        log_info "📋 手动下载指南:"
        log_info "1. 请访问以下任意链接下载:"
        while read -r url; do
            log_info "   - $url"
        done < <(get_mirror_urls_fixed "$version" "$os" "$arch" | head -5 || true)

        log_info ""
        log_info "2. 将下载的文件保存为: $filename"
        log_info "3. 放置到以下任意位置:"
        log_info "   - /tmp/$filename"
        log_info "   - 当前目录 ($(pwd))"
        log_info "   - ~/Downloads/$filename"
        log_info ""
        log_info "4. 然后重新运行此脚本"
        
        cleanup_temp_files "$temp_file"
        return 1
    fi
    
    # 解压文件
    log_info "解压文件..."
    local temp_dir
    temp_dir="$(mktemp -d /tmp/go-install-XXXXXX 2>/dev/null || echo "/tmp/go-install-$$")"
    GLOBAL_TEMP_DIR="$temp_dir"
    
    if ! tar -xzf "$temp_file" -C "$temp_dir"; then
        log_error "解压失败"
        rm -rf "$temp_dir" 2>/dev/null || true
        cleanup_temp_files "$temp_file"
        return 1
    fi
    
    # 检查解压结果
    local go_dir="$temp_dir"
    if [[ -d "$temp_dir/go" ]]; then
        go_dir="$temp_dir/go"
    fi
    
    if [[ ! -f "$go_dir/bin/go" ]]; then
        log_error "解压后没有找到 go 可执行文件"
        rm -rf "$temp_dir" 2>/dev/null || true
        cleanup_temp_files "$temp_file"
        return 1
    fi
    
    # 清理并准备安装目录
    rm -rf "$install_path" 2>/dev/null || true
    if ! mkdir -p "$install_path"; then
        log_error "无法创建安装目录: $install_path"
        rm -rf "$temp_dir" 2>/dev/null || true
        cleanup_temp_files "$temp_file"
        return 1
    fi
    
    # 移动文件到安装目录
    log_info "安装到: $install_path"
    if mv "$go_dir"/* "$install_path"/ 2>/dev/null; then
        # 设置目录权限
        chmod -R 755 "$install_path/bin" 2>/dev/null || true
        
        # 验证安装
        if [[ -x "${install_path}/bin/go" ]]; then
            local installed_version
            installed_version="$("${install_path}/bin/go" version 2>/dev/null | awk '{print $3}' | sed 's/go//')"
            if [[ -z "$installed_version" ]]; then
                installed_version="$version"
                log_warn "无法读取 go version 输出，回退显示为请求版本: $version"
            fi

            log_success "Go 安装成功: $installed_version"
            [[ -n "$final_download_url" ]] && log_info "下载来源: $final_download_url"
            
            # 清理临时文件
            rm -rf "$temp_dir" 2>/dev/null || true
            cleanup_temp_files "$temp_file"
            
            # 创建当前版本链接
            link_go_version "$version"
            
            return 0
        else
            log_error "安装后验证失败"
            rm -rf "$install_path" 2>/dev/null || true
            return 1
        fi
    else
        log_error "移动文件失败"
        rm -rf "$install_path" 2>/dev/null || true
        return 1
    fi
}

# ============================================
# 环境配置函数
# ============================================

link_go_version() {
    local version="$1"
    local target_path="${GO_INSTALL_DIR}/go${version}"
    
    if [[ ! -d "$target_path" ]]; then
        log_error "Go $version 未安装"
        return 1
    fi
    
    # 创建软链接
    rm -f "$GO_CURRENT_LINK" 2>/dev/null || true
    if ln -sf "$target_path" "$GO_CURRENT_LINK"; then
        log_success "已链接 Go $version 为当前版本"
        
        # 配置环境变量
        configure_go_env
        return 0
    else
        log_error "创建软链接失败"
        return 1
    fi
}

configure_go_env() {
    log_info "配置 Go 环境变量..."
    
    # 检查 Shell 配置文件是否存在
    if [[ ! -f "$SHELL_PROFILE" ]]; then
        mkdir -p "$(dirname "$SHELL_PROFILE")" 2>/dev/null || true
        touch "$SHELL_PROFILE" 2>/dev/null || {
            log_error "无法创建配置文件: $SHELL_PROFILE"
            return 1
        }
    fi
    
    # 创建临时文件
    local temp_file
    temp_file="$(mktemp /tmp/go-env-XXXXXX 2>/dev/null || echo "/tmp/go-env-$$")"
    
    # 读取现有配置，移除旧的 Go 配置
    if [[ -f "$SHELL_PROFILE" ]]; then
        grep -v -E "(GOROOT=|GOPATH=|# Go环境|# Go SDK)" "$SHELL_PROFILE" > "$temp_file" 2>/dev/null || cat "$SHELL_PROFILE" > "$temp_file" 2>/dev/null
    fi
    
    # 添加新的 Go 配置
    cat <<EOF >> "$temp_file"

# Go 环境配置 (由 $SCRIPT_NAME v$SCRIPT_VERSION 配置)
export GOROOT="\${HOME}/.go/current"
export PATH="\${GOROOT}/bin:\${PATH}"
export GOPATH="\${HOME}/go"
export PATH="\${GOPATH}/bin:\${PATH}"
EOF
    
    # 替换原文件
    if mv "$temp_file" "$SHELL_PROFILE"; then
        # 确保 Go 工作目录存在
        mkdir -p "${HOME}/go/"{bin,src,pkg} 2>/dev/null || true
        
        # 为当前 shell 立即设置环境变量
        export GOROOT="${GO_CURRENT_LINK}"
        export PATH="${GOROOT}/bin:${PATH}"
        export GOPATH="${HOME}/go"
        export PATH="${GOPATH}/bin:${PATH}"
        
        log_success "环境变量已配置到: $SHELL_PROFILE"

        # 尝试在当前进程中加载配置文件（只能影响本脚本及其子进程，无法修改父 Shell）
        if [[ -f "$SHELL_PROFILE" ]]; then
            # shellcheck disable=SC1090
            . "$SHELL_PROFILE" 2>/dev/null || true
            log_info "已在当前会话中加载: $SHELL_PROFILE"
        fi

        # 提醒用户在交互式终端中执行一次刷新，使切换在当前 shell 生效
        log_info "切换成功后，请在终端执行: source $SHELL_PROFILE"
        return 0
    else
        log_error "更新配置文件失败"
        rm -f "$temp_file" 2>/dev/null || true
        return 1
    fi
}

# ============================================
# 辅助函数
# ============================================

list_installed_versions() {
    log_info "已安装的 Go 版本:"
    
    if [[ ! -d "$GO_INSTALL_DIR" ]] || [[ -z "$(ls -A "$GO_INSTALL_DIR" 2>/dev/null)" ]]; then
        log_warn "  未安装任何版本"
        return
    fi
    
    # 解析当前版本链接
    local current_target=""
    if [[ -L "$GO_CURRENT_LINK" ]]; then
        current_target="$(readlink "$GO_CURRENT_LINK" 2>/dev/null || echo "")"
    fi
    
    local count=0
    for dir in "${GO_INSTALL_DIR}"/go*; do
        if [[ -d "$dir" && -x "${dir}/bin/go" ]]; then
            local ver
            ver="$(basename "$dir" 2>/dev/null | sed 's/^go//')"
            if [[ -n "$current_target" && "$dir" == "$current_target" ]]; then
                echo "  - $ver (current)"
            else
                echo "  - $ver"
            fi
            count=$((count + 1))
        fi
    done
    
    if [[ $count -eq 0 ]]; then
        log_warn "  未找到有效的 Go 安装"
    fi
}

show_current_version() {
    if [[ ! -L "$GO_CURRENT_LINK" ]]; then
        log_warn "当前未选择任何 Go 版本"
        return 0
    fi

    local target
    target="$(readlink "$GO_CURRENT_LINK" 2>/dev/null || echo "")"
    if [[ -z "$target" || ! -x "$target/bin/go" ]]; then
        log_warn "当前链接无效: $GO_CURRENT_LINK -> ${target:-<空>}"
        return 0
    fi

    local ver
    ver="$(basename "$target" 2>/dev/null | sed 's/^go//')"
    log_info "当前 Go 版本: $ver"
    "$target/bin/go" version 2>/dev/null || true
}

uninstall_go_version() {
    local version="$1"
    if [[ -z "$version" ]]; then
        log_error "请指定要卸载的版本号"
        return 1
    fi

    local target="${GO_INSTALL_DIR}/go${version}"
    if [[ ! -d "$target" ]]; then
        log_error "Go $version 未安装"
        return 1
    fi

    read -r -p "确认卸载 Go $version ? (y/N): " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        log_info "已取消卸载"
        return 0
    fi

    # 如果当前链接指向该版本，先移除链接
    if [[ -L "$GO_CURRENT_LINK" ]]; then
        local current_target
        current_target="$(readlink "$GO_CURRENT_LINK" 2>/dev/null || echo "")"
        if [[ "$current_target" == "$target" ]]; then
            rm -f "$GO_CURRENT_LINK" 2>/dev/null || true
            log_info "已移除当前版本链接"
        fi
    fi

    rm -rf "$target" 2>/dev/null || {
        log_error "卸载失败，无法删除目录: $target"
        return 1
    }

    log_success "已卸载 Go $version"
    return 0
}

# ============================================
# 主函数
# ============================================

show_help() {
    cat <<EOF
${BOLD}Go SDK 下载管理器 (403 修复版) v${SCRIPT_VERSION}${NC}

用法: $0 <command> [version]
命令:
  install <version>   安装指定版本的 Go SDK (如: 1.25.0)
  list                列出所有已安装的版本，并标记当前版本
  current             显示当前使用的 Go 版本
  use <version>       切换当前使用的 Go 版本（已安装）
  uninstall <version> 卸载指定版本的 Go SDK
  check               检查系统环境
  help                显示此帮助信息

示例:
  $0 install 1.25.0      # 安装 Go 1.25.0
  $0 list                # 列出已安装版本
  $0 current             # 查看当前版本
  $0 use 1.25.0          # 切换到 Go 1.25.0
  $0 uninstall 1.24.0    # 卸载 Go 1.24.0

${BOLD}如果仍然失败:${NC}
  1. 在其他设备上手动下载 Go SDK
  2. 将文件复制到本机的 /tmp/ 目录
  3. 文件名为: go<VERSION>.<OS>-<ARCH>.tar.gz
  4. 重新运行安装命令

${BOLD}手动下载示例:${NC}
  对于 Go 1.25.0 on Linux AMD64:
  wget https://mirrors.aliyun.com/golang/go1.25.0.linux-amd64.tar.gz
  mv go1.25.0.linux-amd64.tar.gz /tmp/
  $0 install 1.25.0

${BOLD}下载参数（可选环境变量）:${NC}
  GO_DL_RETRIES=3             # 每次下载命令重试次数
  GO_DL_CONNECT_TIMEOUT=12    # TCP连接超时秒数
  GO_DL_MAX_TIME=240          # 单次 curl 最大耗时秒数
  GO_DL_BACKOFF_BASE=3        # 多轮重试回退基数秒
  GO_DL_FORCE_IPV4=true       # 优先使用 IPv4
  GO_DL_PROBE_ENABLED=auto    # 镜像探测: true/false/auto
  GO_DL_PROBE_TIMEOUT=4       # 镜像探测超时秒数
  GO_DL_FAST_FAIL_HTTP=true   # 对 403/404/410 快速失败并切换镜像
EOF
}

check_system() {
    log_info "系统检查:"
    echo "  操作系统: $(uname -s) $(uname -r)"
    echo "  系统架构: $(detect_arch)"
    echo "  安装目录: $GO_INSTALL_DIR"
    echo "  当前用户: $(whoami 2>/dev/null || echo "未知")"
    echo "  家目录: $HOME"
    echo "  Shell配置文件: $SHELL_PROFILE"
    echo "  下载重试: $GO_DL_RETRIES"
    echo "  连接超时(秒): $GO_DL_CONNECT_TIMEOUT"
    echo "  单次最大耗时(秒): $GO_DL_MAX_TIME"
    echo "  强制IPv4: $GO_DL_FORCE_IPV4"
    echo "  镜像探测: $GO_DL_PROBE_ENABLED"
    echo "  探测超时(秒): $GO_DL_PROBE_TIMEOUT"
    echo "  HTTP快速失败: $GO_DL_FAST_FAIL_HTTP"

    log_info "可用下载工具:"
    for tool in curl wget python3 python; do
        if command -v "$tool" &>/dev/null; then
            echo "  ✓ $tool"
        else
            echo "  ✗ $tool"
        fi
    done
    
    list_installed_versions
}

main() {
    local command="${1:-}"
    local version="${2:-}"
    
    # 检查依赖
    if ! check_dependencies; then
        return 1
    fi
    
    # 创建必要目录
    if ! create_directories; then
        log_error "初始化失败"
        return 1
    fi
    
    case "$command" in
        install)
            if [[ -z "$version" ]]; then
                log_error "请指定要安装的版本号"
                show_help
                return 1
            fi
            if ! install_go_version_fixed "$version"; then
                log_error "安装失败"
                return 1
            fi
            ;;
            
        list)
            list_installed_versions
            ;;

        current)
            show_current_version
            ;;

        use)
            if [[ -z "$version" ]]; then
                log_error "请指定要切换的版本号"
                show_help
                return 1
            fi
            if ! check_version_installed "$version"; then
                log_error "Go $version 未安装，请先运行: $SCRIPT_NAME install $version"
                return 1
            fi
            if ! link_go_version "$version"; then
                return 1
            fi
            ;;

        uninstall)
            if [[ -z "$version" ]]; then
                log_error "请指定要卸载的版本号"
                show_help
                return 1
            fi
            if ! uninstall_go_version "$version"; then
                return 1
            fi
            ;;
            
        check)
            check_system
            ;;
            
        help|--help|-h)
            show_help
            ;;
            
        "")
            show_help
            ;;
            
        *)
            log_error "未知命令: $command"
            show_help
            return 1
            ;;
    esac
    
    return 0
}

# ============================================
# 脚本入口
# ============================================

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    # 处理命令行参数
    if [[ $# -eq 0 ]]; then
        show_help
        exit 0
    fi
    
    # 捕获中断信号，立刻退出（EXIT trap 会负责清理）
    trap 'echo -e "\n${YELLOW}[INFO]${NC} 操作被中断"; exit 130' INT TERM
    
    # 运行主函数
    if main "$@"; then
        exit 0
    else
        exit 1
    fi
fi