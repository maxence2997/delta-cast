#!/usr/bin/env bash
# =============================================================================
# YouTube Setup Script — DeltaCast
#
# 半自動完成 YouTube Data API v3 初次設定：
#   1. 啟用 YouTube Data API v3
#   2. 等待使用者在 Console 建立 OAuth 2.0 client（無 CLI 支援）
#   3. 打開瀏覽器授權，捕捉 callback code，交換 Refresh Token
#   4. 驗證 Refresh Token 有效
#
# 幂等性：
#   - API 已啟用 → skip
#   - YOUTUBE_REFRESH_TOKEN 已設定 → skip OAuth 流程，直接驗證
#
# 前置需求：
#   - gcloud CLI 已登入（gcloud auth login）
#   - python3（捕捉 OAuth callback）
#
# 執行方式：
#   chmod +x script/youtube-setup.sh
#   ./script/youtube-setup.sh
#
#   若已有 Refresh Token 只想驗證：
#   YOUTUBE_REFRESH_TOKEN=xxx ./script/youtube-setup.sh
# =============================================================================

set -euo pipefail

# ── 載入環境變數 ──────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
if [ -f "${SCRIPT_DIR}/.env.local" ]; then
  source "${SCRIPT_DIR}/.env.local"
elif [ -f "${SCRIPT_DIR}/.env" ]; then
  source "${SCRIPT_DIR}/.env"
fi

# ── 必填變數檢查 ──────────────────────────────────────────────────────────────
if [ -z "${GCP_PROJECT_ID:-}" ]; then
  echo "Error: GCP_PROJECT_ID is not set." >&2
  echo "Copy script/.env.example to script/.env and fill in values." >&2
  exit 1
fi

# ── 設定 ──────────────────────────────────────────────────────────────────────
PROJECT_ID="${GCP_PROJECT_ID}"
YOUTUBE_SCOPE="https://www.googleapis.com/auth/youtube"

# 自動找到一個可用的 port（從 8080 開始往下找）
find_free_port() {
  local port
  for port in 8080 8081 8082 8083 8084 8085; do
    if ! lsof -iTCP:${port} -sTCP:LISTEN -t &>/dev/null; then
      echo "$port"
      return
    fi
  done
  echo "" # 全部占用
}

REDIRECT_PORT=$(find_free_port)
if [[ -z "$REDIRECT_PORT" ]]; then
  err "無法找到可用 port（8080-8085 皆被占用），請手動釋放後重試。"
  exit 1
fi
REDIRECT_URI="http://localhost:${REDIRECT_PORT}/oauth/callback"

# ── 顏色輸出 ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
skip()    { echo -e "${YELLOW}[SKIP]${NC}  $*"; }
err()     { echo -e "${RED}[ERR]${NC}   $*"; }

echo ""
echo -e "${CYAN}=====================================================${NC}"
echo -e "${CYAN}  YouTube Setup — DeltaCast${NC}"
echo -e "${CYAN}=====================================================${NC}"
echo ""
echo "  Project : $PROJECT_ID"
echo ""

gcloud config set project "$PROJECT_ID" --quiet

# ── Step 1：啟用 YouTube Data API v3 ─────────────────────────────────────────
info "════ Step 1：啟用 YouTube Data API v3 ════"

if gcloud services list --enabled --filter="name:youtube.googleapis.com" \
     --format="value(name)" 2>/dev/null | grep -q "youtube.googleapis.com"; then
  skip "youtube.googleapis.com 已啟用"
else
  gcloud services enable youtube.googleapis.com --quiet
  success "已啟用 youtube.googleapis.com"
fi

# ── Step 2：建立 OAuth 2.0 Client（手動步驟，腳本暫停）────────────────────────
echo ""
info "════ Step 2：建立 OAuth 2.0 Client ════"
echo ""
warn "此步驟需要在 Google Cloud Console 手動完成（gcloud 不支援 CLI 建立 Web OAuth client）"
echo ""
echo "  請在瀏覽器開啟以下連結，完成 OAuth client 建立："
echo ""
echo "  https://console.cloud.google.com/apis/credentials?project=${PROJECT_ID}"
echo ""
echo "  操作步驟："
echo "    1. 點擊「+ CREATE CREDENTIALS」→「OAuth client ID」"
echo "    2. Application type：選擇 Web application"
echo "    3. Authorized redirect URIs：新增 ${REDIRECT_URI}"
echo "    4. 建立後，記錄 Client ID 與 Client Secret"
echo ""
echo "  ⚠️  若授權時出現 access_denied 錯誤："
echo "    - 前往 https://console.cloud.google.com/apis/credentials/consent?project=${PROJECT_ID}"
echo "    - 在「Test users」區塊新增你的 Google 帳號"
echo "    - 儲存後再重新授權"
echo ""
echo "  ℹ️  本次使用的 callback port：${REDIRECT_PORT}"
echo "     若與 OAuth client 設定的 redirect URI 不符，請在 Console 新增 ${REDIRECT_URI}"
echo ""

# 幂等性：若環境變數已設定則跳過輸入
if [[ -n "${YOUTUBE_CLIENT_ID:-}" && -n "${YOUTUBE_CLIENT_SECRET:-}" ]]; then
  skip "YOUTUBE_CLIENT_ID 與 YOUTUBE_CLIENT_SECRET 已設定，略過輸入"
else
  read -rp "  請輸入 Client ID：" YOUTUBE_CLIENT_ID
  echo ""
  read -rsp "  請輸入 Client Secret：" YOUTUBE_CLIENT_SECRET
  echo ""
fi

if [[ -z "${YOUTUBE_CLIENT_ID:-}" || -z "${YOUTUBE_CLIENT_SECRET:-}" ]]; then
  err "Client ID 或 Client Secret 為空，無法繼續。"
  exit 1
fi
success "OAuth client 資訊已取得"

# ── Step 3：取得 Refresh Token ────────────────────────────────────────────────
echo ""
info "════ Step 3：取得 Refresh Token ════"

# 幂等性：YOUTUBE_REFRESH_TOKEN 已有值則直接跳過整個 OAuth 流程
if [[ -n "${YOUTUBE_REFRESH_TOKEN:-}" ]]; then
  skip "YOUTUBE_REFRESH_TOKEN 已設定，略過 OAuth 授權流程，直接驗證"
else
  AUTH_URL="https://accounts.google.com/o/oauth2/auth"
  AUTH_URL+="?client_id=${YOUTUBE_CLIENT_ID}"
  AUTH_URL+="&redirect_uri=${REDIRECT_URI}"
  AUTH_URL+="&response_type=code"
  AUTH_URL+="&scope=${YOUTUBE_SCOPE}"
  AUTH_URL+="&access_type=offline"
  AUTH_URL+="&prompt=consent"

  info "開啟瀏覽器進行 OAuth 授權..."
  echo ""
  echo "  若瀏覽器未自動開啟，請手動複製以下 URL："
  echo ""
  echo "  ${AUTH_URL}"
  echo ""
  open "$AUTH_URL" 2>/dev/null || warn "無法自動開啟瀏覽器，請手動複製上方 URL"

  # ── 嘗試用 python3 監聽 callback ──
  AUTH_CODE=""
  if command -v python3 &>/dev/null; then
    info "啟動本地伺服器監聽 http://localhost:${REDIRECT_PORT}/oauth/callback ..."
    TMPDIR=$(mktemp -d)
    CALLBACK_LOG="${TMPDIR}/callback.log"

    python3 - "${REDIRECT_PORT}" "${CALLBACK_LOG}" << 'PYEOF' &
import sys, http.server, urllib.parse, os

port = int(sys.argv[1])
log_path = sys.argv[2]

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(parsed.query)
        code = params.get("code", [""])[0]
        if code:
            with open(log_path, "w") as f:
                f.write(code)
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"<h2>DeltaCast: Authorization complete. You may close this tab.</h2>")
            # Shutdown after first request
            import threading
            threading.Thread(target=self.server.shutdown).start()
        else:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b"No code received.")

httpd = http.server.HTTPServer(("", port), Handler)
httpd.serve_forever()
PYEOF

    PY_PID=$!
    info "等待授權完成（最多 120 秒）..."
    WAITED=0
    while [[ ! -f "$CALLBACK_LOG" && $WAITED -lt 120 ]]; do
      sleep 2
      WAITED=$((WAITED + 2))
    done

    kill "$PY_PID" 2>/dev/null || true
    wait "$PY_PID" 2>/dev/null || true

    if [[ -f "$CALLBACK_LOG" ]]; then
      AUTH_CODE=$(cat "$CALLBACK_LOG")
      rm -rf "$TMPDIR"
      success "已捕捉 authorization code"
    else
      rm -rf "$TMPDIR"
      warn "python3 監聽逾時，切換為手動輸入模式"
    fi
  else
    warn "python3 不可用，切換為手動輸入模式"
  fi

  # Fallback：手動貼上 callback URL
  if [[ -z "$AUTH_CODE" ]]; then
    echo ""
    echo "  授權完成後，瀏覽器會跳轉至類似以下的 URL（可能顯示連線失敗，這是正常的）："
    echo "  http://localhost:${REDIRECT_PORT}/oauth/callback?code=<AUTH_CODE>&..."
    echo ""
    read -rp "  請貼上完整 callback URL 或只貼 code 部分：" CALLBACK_INPUT
    # 支援貼整個 URL 或只貼 code
    if [[ "$CALLBACK_INPUT" == http* ]]; then
      AUTH_CODE=$(echo "$CALLBACK_INPUT" | grep -oP '(?<=code=)[^&]+' || true)
    else
      AUTH_CODE="$CALLBACK_INPUT"
    fi
  fi

  if [[ -z "$AUTH_CODE" ]]; then
    err "無法取得 authorization code，請重新執行。"
    exit 1
  fi

  info "交換 Refresh Token..."
  TOKEN_RESPONSE=$(curl -sf -X POST https://oauth2.googleapis.com/token \
    -d "code=${AUTH_CODE}" \
    -d "client_id=${YOUTUBE_CLIENT_ID}" \
    -d "client_secret=${YOUTUBE_CLIENT_SECRET}" \
    -d "redirect_uri=${REDIRECT_URI}" \
    -d "grant_type=authorization_code")

  YOUTUBE_REFRESH_TOKEN=$(echo "$TOKEN_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refresh_token',''))" 2>/dev/null || true)

  if [[ -z "$YOUTUBE_REFRESH_TOKEN" ]]; then
    err "無法取得 Refresh Token，API 回應："
    echo "$TOKEN_RESPONSE"
    exit 1
  fi
  success "Refresh Token 取得成功"
fi

# ── Step 4：驗證 Refresh Token ────────────────────────────────────────────────
echo ""
info "════ Step 4：驗證 Refresh Token ════"

VERIFY_RESPONSE=$(curl -sf -X POST https://oauth2.googleapis.com/token \
  -d "client_id=${YOUTUBE_CLIENT_ID}" \
  -d "client_secret=${YOUTUBE_CLIENT_SECRET}" \
  -d "refresh_token=${YOUTUBE_REFRESH_TOKEN}" \
  -d "grant_type=refresh_token" || true)

ACCESS_TOKEN=$(echo "$VERIFY_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null || true)

if [[ -z "$ACCESS_TOKEN" ]]; then
  err "Refresh Token 驗證失敗，API 回應："
  echo "$VERIFY_RESPONSE"
  exit 1
fi
success "Refresh Token 有效（已成功換取 Access Token）"

# ── 完成摘要 ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}  ✅  YouTube Setup 完成${NC}"
echo -e "${GREEN}=====================================================${NC}"
echo ""
echo "請將以下值填入 server/.env.local："
echo ""
echo "  YOUTUBE_CLIENT_ID=${YOUTUBE_CLIENT_ID}"
echo "  YOUTUBE_CLIENT_SECRET=${YOUTUBE_CLIENT_SECRET}"
echo "  YOUTUBE_REFRESH_TOKEN=${YOUTUBE_REFRESH_TOKEN}"
echo ""
warn "⚠️  以上資訊含有敏感 Secret，請勿加入版本控制。"
warn "⚠️  YouTube 直播功能需頻道通過 24h 審核才可使用。"
warn "⚠️  若需刪除所有資源：./script/youtube-teardown.sh"
