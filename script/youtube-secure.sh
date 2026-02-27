#!/usr/bin/env bash
# =============================================================================
# YouTube 安全控制腳本 — DeltaCast
#
# 測試前後快速開關 YouTube 直播資源的對外可見性：
#
#   --mode lock         所有 non-complete Broadcasts 改為 private
#                       （等同 gcp-storage-secure.sh --mode lock）
#   --mode unlock       還原為 unlisted
#                       （等同 gcp-storage-secure.sh --mode unlock）
#   --mode status       顯示 API 啟用狀態 + 所有 Broadcasts 的 privacy 狀態
#   --mode disable-api  停用 YouTube Data API v3
#                       （等同 gcp-cdn-armor.sh --mode deny-all）
#   --mode enable-api   重新啟用 YouTube Data API v3
#                       （等同 gcp-cdn-armor.sh --mode allow-all）
#
# 幂等性：
#   - lock：所有 Broadcasts 已是 private → skip
#   - unlock：所有 Broadcasts 已是 unlisted → skip
#   - disable-api：API 已停用 → skip
#   - enable-api：API 已啟用 → skip
#
# 前置需求（lock/unlock/status 模式）：
#   YOUTUBE_CLIENT_ID, YOUTUBE_CLIENT_SECRET, YOUTUBE_REFRESH_TOKEN
#
# 使用方式：
#   chmod +x script/youtube-secure.sh
#   ./script/youtube-secure.sh --mode status
#   ./script/youtube-secure.sh --mode lock
#   ./script/youtube-secure.sh --mode unlock
#   ./script/youtube-secure.sh --mode disable-api
#   ./script/youtube-secure.sh --mode enable-api
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

PROJECT_ID="${GCP_PROJECT_ID}"
YOUTUBE_API="https://www.googleapis.com/youtube/v3"

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

MODE="${1:---mode}"
MODEARG="${2:-status}"
if [[ "$MODE" == "--mode" ]]; then MODE_VALUE="$MODEARG"; else MODE_VALUE="status"; fi

gcloud config set project "$PROJECT_ID" --quiet

# ── disable-api / enable-api：只需 gcloud，不用 Access Token ─────────────────
if [[ "$MODE_VALUE" == "disable-api" ]]; then
  echo ""
  echo -e "${CYAN}=====================================================${NC}"
  echo -e "${CYAN}  YouTube Secure — disable-api${NC}"
  echo -e "${CYAN}=====================================================${NC}"
  echo ""
  if gcloud services list --enabled --filter="name:youtube.googleapis.com" \
       --format="value(name)" 2>/dev/null | grep -q "youtube.googleapis.com"; then
    info "停用 youtube.googleapis.com..."
    gcloud services disable youtube.googleapis.com --force --quiet
    success "YouTube Data API v3 已停用"
  else
    skip "youtube.googleapis.com 已停用，無需變更"
  fi
  echo ""
  warn "DeltaCast server 的 /prepare 端點將無法建立 YouTube 資源。"
  warn "重新啟用：./script/youtube-secure.sh --mode enable-api"
  exit 0
fi

if [[ "$MODE_VALUE" == "enable-api" ]]; then
  echo ""
  echo -e "${CYAN}=====================================================${NC}"
  echo -e "${CYAN}  YouTube Secure — enable-api${NC}"
  echo -e "${CYAN}=====================================================${NC}"
  echo ""
  if gcloud services list --enabled --filter="name:youtube.googleapis.com" \
       --format="value(name)" 2>/dev/null | grep -q "youtube.googleapis.com"; then
    skip "youtube.googleapis.com 已啟用，無需變更"
  else
    info "啟用 youtube.googleapis.com..."
    gcloud services enable youtube.googleapis.com --quiet
    success "YouTube Data API v3 已重新啟用"
  fi
  exit 0
fi

# ── lock / unlock / status：需要 Access Token ─────────────────────────────────
CLIENT_ID="${YOUTUBE_CLIENT_ID:-}"
CLIENT_SECRET="${YOUTUBE_CLIENT_SECRET:-}"
REFRESH_TOKEN="${YOUTUBE_REFRESH_TOKEN:-}"

if [[ -z "$CLIENT_ID" || -z "$CLIENT_SECRET" || -z "$REFRESH_TOKEN" ]]; then
  err "lock/unlock/status 模式需要 YOUTUBE_CLIENT_ID、YOUTUBE_CLIENT_SECRET、YOUTUBE_REFRESH_TOKEN"
  err "請設定環境變數後重新執行，或使用 server/.env.local 中的值。"
  exit 1
fi

TOKEN_RESPONSE=$(curl -sf -X POST https://oauth2.googleapis.com/token \
  -d "client_id=${CLIENT_ID}" \
  -d "client_secret=${CLIENT_SECRET}" \
  -d "refresh_token=${REFRESH_TOKEN}" \
  -d "grant_type=refresh_token" || true)

ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | python3 -c \
  "import sys,json; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null || true)

if [[ -z "$ACCESS_TOKEN" ]]; then
  err "無法取得 Access Token，請確認 YouTube 環境變數是否正確。"
  exit 1
fi

# ── 取得所有 non-complete Broadcasts ─────────────────────────────────────────
fetch_broadcasts() {
  curl -sf \
    "${YOUTUBE_API}/liveBroadcasts?part=id,snippet,status&broadcastStatus=all&mine=true&maxResults=50" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" 2>/dev/null || echo '{"items":[]}'
}

# ── status 模式 ───────────────────────────────────────────────────────────────
if [[ "$MODE_VALUE" == "status" ]]; then
  echo ""
  echo -e "${CYAN}=====================================================${NC}"
  echo -e "${CYAN}  YouTube Secure — status${NC}"
  echo -e "${CYAN}=====================================================${NC}"
  echo ""

  # API 啟用狀態
  info "YouTube Data API v3 狀態："
  if gcloud services list --enabled --filter="name:youtube.googleapis.com" \
       --format="value(name)" 2>/dev/null | grep -q "youtube.googleapis.com"; then
    success "  youtube.googleapis.com — 已啟用 ✅"
  else
    warn "  youtube.googleapis.com — 已停用 ❌"
  fi

  echo ""
  info "LiveBroadcasts（所有）："
  BROADCAST_DATA=$(fetch_broadcasts)
  BROADCAST_COUNT=$(echo "$BROADCAST_DATA" | python3 -c \
    "import sys,json; print(len(json.load(sys.stdin).get('items',[])))" 2>/dev/null || echo 0)

  if [[ "$BROADCAST_COUNT" == "0" ]]; then
    skip "  無 LiveBroadcast 資源"
  else
    echo "$BROADCAST_DATA" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for item in data.get('items', []):
    bid   = item['id']
    title = item['snippet']['title']
    priv  = item['status']['privacyStatus']
    life  = item['status']['lifeCycleStatus']
    icon  = '🔒' if priv == 'private' else ('🔓' if priv == 'unlisted' else '🌐')
    print(f'  {icon}  {bid}  [{priv}] [{life}]  {title}')
" 2>/dev/null || warn "  無法解析 Broadcast 資訊"
  fi
  exit 0
fi

# ── lock 模式 ─────────────────────────────────────────────────────────────────
if [[ "$MODE_VALUE" == "lock" ]]; then
  echo ""
  echo -e "${CYAN}=====================================================${NC}"
  echo -e "${CYAN}  🔒  鎖定 YouTube Broadcasts（private）${NC}"
  echo -e "${CYAN}=====================================================${NC}"
  echo ""

  BROADCAST_DATA=$(fetch_broadcasts)

  # 幂等性：若所有 Broadcast 已是 private 則 skip
  NON_PRIVATE=$(echo "$BROADCAST_DATA" | python3 -c "
import sys, json
data = json.load(sys.stdin)
items = [i for i in data.get('items', [])
         if i['status']['privacyStatus'] != 'private'
         and i['status']['lifeCycleStatus'] != 'complete']
print(len(items))
" 2>/dev/null || echo 0)

  if [[ "$NON_PRIVATE" == "0" ]]; then
    skip "所有 Broadcast 已是 private（或無資源），無需變更"
    exit 0
  fi

  echo "$BROADCAST_DATA" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for item in data.get('items', []):
    if item['status']['privacyStatus'] != 'private' and \
       item['status']['lifeCycleStatus'] != 'complete':
        print(item['id'])
" 2>/dev/null | while IFS= read -r broadcast_id; do
    [[ -z "$broadcast_id" ]] && continue
    info "設為 private：$broadcast_id"
    RESULT=$(curl -sf -X PUT \
      "${YOUTUBE_API}/liveBroadcasts?part=status" \
      -H "Authorization: Bearer ${ACCESS_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{\"id\":\"${broadcast_id}\",\"status\":{\"privacyStatus\":\"private\"}}" 2>/dev/null || true)
    if echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('id') else 1)" 2>/dev/null; then
      success "已設為 private：$broadcast_id"
    else
      warn "更新失敗，略過：$broadcast_id"
    fi
  done

  echo ""
  echo -e "${GREEN}=====================================================${NC}"
  echo -e "${GREEN}  ✅  YouTube Broadcasts 已鎖定（private）${NC}"
  echo -e "${GREEN}=====================================================${NC}"
  echo ""
  echo "  ❌  YouTube Watch URL → 觀眾無法觀看（需登入且已分享才可見）"
  echo ""
  warn "解鎖：./script/youtube-secure.sh --mode unlock"
  exit 0
fi

# ── unlock 模式 ───────────────────────────────────────────────────────────────
if [[ "$MODE_VALUE" == "unlock" ]]; then
  echo ""
  warn "⚠️  還原 YouTube Broadcasts 為 unlisted，持有連結者可觀看。"
  echo ""

  BROADCAST_DATA=$(fetch_broadcasts)

  # 幂等性：若所有 non-complete Broadcast 已是 unlisted 則 skip
  NON_UNLISTED=$(echo "$BROADCAST_DATA" | python3 -c "
import sys, json
data = json.load(sys.stdin)
items = [i for i in data.get('items', [])
         if i['status']['privacyStatus'] != 'unlisted'
         and i['status']['lifeCycleStatus'] != 'complete']
print(len(items))
" 2>/dev/null || echo 0)

  if [[ "$NON_UNLISTED" == "0" ]]; then
    skip "所有 Broadcast 已是 unlisted（或無資源），無需變更"
    exit 0
  fi

  echo "$BROADCAST_DATA" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for item in data.get('items', []):
    if item['status']['privacyStatus'] != 'unlisted' and \
       item['status']['lifeCycleStatus'] != 'complete':
        print(item['id'])
" 2>/dev/null | while IFS= read -r broadcast_id; do
    [[ -z "$broadcast_id" ]] && continue
    info "設為 unlisted：$broadcast_id"
    RESULT=$(curl -sf -X PUT \
      "${YOUTUBE_API}/liveBroadcasts?part=status" \
      -H "Authorization: Bearer ${ACCESS_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{\"id\":\"${broadcast_id}\",\"status\":{\"privacyStatus\":\"unlisted\"}}" 2>/dev/null || true)
    if echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('id') else 1)" 2>/dev/null; then
      success "已設為 unlisted：$broadcast_id"
    else
      warn "更新失敗，略過：$broadcast_id"
    fi
  done

  echo ""
  echo -e "${YELLOW}=====================================================${NC}"
  echo -e "${YELLOW}  🔓  YouTube Broadcasts 已解鎖（unlisted）${NC}"
  echo -e "${YELLOW}=====================================================${NC}"
  echo ""
  warn "測試完成後記得重新鎖定：./script/youtube-secure.sh --mode lock"
  exit 0
fi

err "未知模式：$MODE_VALUE"
echo "用法: ./youtube-secure.sh --mode [status|lock|unlock|disable-api|enable-api]"
exit 1
