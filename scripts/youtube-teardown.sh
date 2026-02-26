#!/usr/bin/env bash
# =============================================================================
# YouTube Teardown Script — DeltaCast
#
# 清除所有為 DeltaCast 建立的 YouTube 資源：
#   1. 刪除所有 LiveBroadcasts
#   2. 刪除所有 LiveStreams
#   3. 撤銷 Refresh Token
#   4. 停用 YouTube Data API v3
#
# 幂等性：
#   - 無 Broadcast/Stream → skip
#   - API 已停用 → skip
#   - Token 已失效 → revoke 仍回 200，無副作用
#
# ⚠️  OAuth 2.0 Client（Client ID / Client Secret）無 CLI 刪除方式，
#     請手動至 Console 刪除：
#     https://console.cloud.google.com/apis/credentials
#
# 執行方式：
#   chmod +x scripts/youtube-teardown.sh
#   YOUTUBE_CLIENT_ID=xxx YOUTUBE_CLIENT_SECRET=xxx YOUTUBE_REFRESH_TOKEN=xxx \
#     ./scripts/youtube-teardown.sh
#
# 若要跳過確認提示（CI 環境）：
#   SKIP_CONFIRM=1 ./scripts/youtube-teardown.sh
# =============================================================================

set -euo pipefail

# ── 設定 ──────────────────────────────────────────────────────────────────────
PROJECT_ID="${GCP_PROJECT_ID:-omega-pivot-488513-k6}"
YOUTUBE_API="https://www.googleapis.com/youtube/v3"

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

# ── 輔助函式：執行指令，失敗不中斷腳本 ──────────────────────────────────────
run() {
  local desc="$1"; shift
  if eval "$@" 2>/dev/null; then
    success "$desc"
  else
    warn "失敗或已不存在，略過：$desc"
  fi
}

# ── 確認提示 ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${RED}=====================================================${NC}"
echo -e "${RED}  ⚠️  YouTube TEARDOWN — 此操作不可復原  ⚠️${NC}"
echo -e "${RED}=====================================================${NC}"
echo ""
echo "  Project : $PROJECT_ID"
echo ""
echo "  將執行："
echo "  - 刪除所有 LiveBroadcasts 與 LiveStreams"
echo "  - 撤銷 Refresh Token"
echo "  - 停用 YouTube Data API v3"
echo ""
echo -e "  ${YELLOW}⚠️  OAuth client (Client ID/Secret) 需手動刪除：${NC}"
echo "     https://console.cloud.google.com/apis/credentials?project=${PROJECT_ID}"
echo ""

if [[ "${SKIP_CONFIRM:-0}" != "1" ]]; then
  read -r -p "確定要清除所有 DeltaCast YouTube 資源嗎？輸入 'yes' 繼續：" confirm
  if [[ "$confirm" != "yes" ]]; then
    echo "已取消。"
    exit 0
  fi
fi

gcloud config set project "$PROJECT_ID" --quiet

# ── 前置：取得 Access Token ───────────────────────────────────────────────────
echo ""
info "════ 前置：取得 Access Token ════"

CLIENT_ID="${YOUTUBE_CLIENT_ID:-}"
CLIENT_SECRET="${YOUTUBE_CLIENT_SECRET:-}"
REFRESH_TOKEN="${YOUTUBE_REFRESH_TOKEN:-}"

if [[ -z "$CLIENT_ID" ]]; then
  read -rp "  請輸入 YOUTUBE_CLIENT_ID：" CLIENT_ID
fi
if [[ -z "$CLIENT_SECRET" ]]; then
  read -rsp "  請輸入 YOUTUBE_CLIENT_SECRET：" CLIENT_SECRET
  echo ""
fi
if [[ -z "$REFRESH_TOKEN" ]]; then
  read -rsp "  請輸入 YOUTUBE_REFRESH_TOKEN：" REFRESH_TOKEN
  echo ""
fi

if [[ -z "$CLIENT_ID" || -z "$CLIENT_SECRET" || -z "$REFRESH_TOKEN" ]]; then
  err "缺少必要認證資訊，無法繼續。"
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
  err "無法取得 Access Token，請確認認證資訊是否正確。"
  echo "$TOKEN_RESPONSE"
  exit 1
fi
success "Access Token 取得成功"

# ── Step 1：刪除所有 LiveBroadcasts ──────────────────────────────────────────
echo ""
info "════ Step 1：刪除所有 LiveBroadcasts ════"

BROADCASTS=$(curl -sf \
  "${YOUTUBE_API}/liveBroadcasts?part=id,snippet&broadcastStatus=all&mine=true&maxResults=50" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | \
  python3 -c "
import sys, json
data = json.load(sys.stdin)
ids = [item['id'] for item in data.get('items', [])]
print('\n'.join(ids))
" 2>/dev/null || true)

if [[ -z "$BROADCASTS" ]]; then
  skip "無 LiveBroadcast 資源"
else
  while IFS= read -r broadcast_id; do
    [[ -z "$broadcast_id" ]] && continue
    run "刪除 Broadcast: $broadcast_id" \
      "curl -sf -X DELETE '${YOUTUBE_API}/liveBroadcasts?id=${broadcast_id}' \
        -H 'Authorization: Bearer ${ACCESS_TOKEN}'"
  done <<< "$BROADCASTS"
fi

# ── Step 2：刪除所有 LiveStreams ───────────────────────────────────────────────
echo ""
info "════ Step 2：刪除所有 LiveStreams ════"

STREAMS=$(curl -sf \
  "${YOUTUBE_API}/liveStreams?part=id,snippet&mine=true&maxResults=50" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | \
  python3 -c "
import sys, json
data = json.load(sys.stdin)
ids = [item['id'] for item in data.get('items', [])]
print('\n'.join(ids))
" 2>/dev/null || true)

if [[ -z "$STREAMS" ]]; then
  skip "無 LiveStream 資源"
else
  while IFS= read -r stream_id; do
    [[ -z "$stream_id" ]] && continue
    run "刪除 Stream: $stream_id" \
      "curl -sf -X DELETE '${YOUTUBE_API}/liveStreams?id=${stream_id}' \
        -H 'Authorization: Bearer ${ACCESS_TOKEN}'"
  done <<< "$STREAMS"
fi

# ── Step 3：撤銷 Refresh Token ────────────────────────────────────────────────
echo ""
info "════ Step 3：撤銷 Refresh Token ════"

REVOKE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  "https://oauth2.googleapis.com/revoke?token=${REFRESH_TOKEN}" \
  -H "Content-Type: application/x-www-form-urlencoded" || true)

if [[ "$REVOKE_STATUS" == "200" ]]; then
  success "Refresh Token 已撤銷"
else
  warn "撤銷回傳 HTTP ${REVOKE_STATUS}（Token 可能已失效，視為正常）"
fi

# ── Step 4：停用 YouTube Data API v3 ─────────────────────────────────────────
echo ""
info "════ Step 4：停用 YouTube Data API v3 ════"

if gcloud services list --enabled --filter="name:youtube.googleapis.com" \
     --format="value(name)" 2>/dev/null | grep -q "youtube.googleapis.com"; then
  run "停用 youtube.googleapis.com" \
    "gcloud services disable youtube.googleapis.com --force --quiet"
else
  skip "youtube.googleapis.com 已停用"
fi

# ── 完成摘要 ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}  ✅  YouTube Teardown 完成${NC}"
echo -e "${GREEN}=====================================================${NC}"
echo ""
info "已保留（需手動刪除）："
echo -e "  ${YELLOW}OAuth 2.0 Client（Client ID / Client Secret）${NC}"
echo "  → https://console.cloud.google.com/apis/credentials?project=${PROJECT_ID}"
echo ""
warn "⚠️  重新部署時，執行 ./scripts/youtube-setup.sh 重新取得 Refresh Token。"
