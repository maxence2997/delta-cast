#!/usr/bin/env bash
# =============================================================================
# GCP Live Stream Cleanup — DeltaCast
#
# 停止並刪除所有非 STOPPED/STOPPING 狀態的 Live Stream channel 與對應 input。
# 在 make res-close 時自動執行，防止閒置 channel 持續計費。
#
# 執行方式：
#   chmod +x script/gcp-livestream-cleanup.sh
#   ./script/gcp-livestream-cleanup.sh
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
  echo "Error: GCP_PROJECT_ID is not set" >&2
  echo "Copy script/.env.example to script/.env and fill in values." >&2
  exit 1
fi

PROJECT_ID="${GCP_PROJECT_ID}"
REGION="${GCP_REGION:-asia-east1}"

# ── 顏色輸出 ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
skip()    { echo -e "${YELLOW}[SKIP]${NC}  $*"; }

# ── 取得 access token ─────────────────────────────────────────────────────────
TOKEN=$(gcloud auth print-access-token 2>/dev/null)
if [ -z "${TOKEN}" ]; then
  echo "Error: failed to get GCP access token (run 'gcloud auth login')" >&2
  exit 1
fi

BASE_URL="https://livestream.googleapis.com/v1/projects/${PROJECT_ID}/locations/${REGION}"

# ── 列出所有 channels ──────────────────────────────────────────────────────────
info "掃描 Live Stream channels（project=${PROJECT_ID}, region=${REGION}）..."

CHANNELS_JSON=$(curl -sf \
  -H "Authorization: Bearer ${TOKEN}" \
  "${BASE_URL}/channels" || echo '{}')

CHANNEL_IDS=$(echo "${CHANNELS_JSON}" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for ch in data.get('channels', []):
    name = ch['name'].split('/')[-1]
    state = ch.get('streamingState', 'UNKNOWN')
    print(f'{name} {state}')
" 2>/dev/null || true)

if [ -z "${CHANNEL_IDS}" ]; then
  skip "無 Live Stream channel，跳過清理"
  exit 0
fi

CLEANED=0

while IFS=' ' read -r channel_id state; do
  [ -z "${channel_id}" ] && continue

  if [ "${state}" = "STOPPED" ] || [ "${state}" = "STOPPING" ]; then
    info "channel ${channel_id} 已是 ${state}，跳過"
    continue
  fi

  warn "發現計費中 channel: ${channel_id}（state=${state}）→ 開始清理"

  # Stop channel
  info "  StopChannel: ${channel_id}"
  curl -sf -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    "${BASE_URL}/channels/${channel_id}:stop" \
    -d '{}' > /dev/null || warn "  stop channel 失敗（已忽略）"

  # Wait for STOPPED (最多 30 秒)
  for _ in $(seq 1 6); do
    sleep 5
    STATE=$(curl -sf \
      -H "Authorization: Bearer ${TOKEN}" \
      "${BASE_URL}/channels/${channel_id}" \
      | python3 -c "import json,sys; print(json.load(sys.stdin).get('streamingState',''))" 2>/dev/null || echo "")
    if [ "${STATE}" = "STOPPED" ]; then
      break
    fi
    info "  等待 STOPPED（目前：${STATE}）..."
  done

  # Delete channel
  info "  DeleteChannel: ${channel_id}"
  curl -sf -X DELETE \
    -H "Authorization: Bearer ${TOKEN}" \
    "${BASE_URL}/channels/${channel_id}" > /dev/null || warn "  delete channel 失敗（已忽略）"

  # Delete corresponding input (naming convention: channel-{id} → input-{id})
  INPUT_ID="input-${channel_id#channel-}"
  info "  DeleteInput: ${INPUT_ID}"
  curl -sf -X DELETE \
    -H "Authorization: Bearer ${TOKEN}" \
    "${BASE_URL}/inputs/${INPUT_ID}" > /dev/null || warn "  delete input 失敗（已忽略）"

  success "channel ${channel_id} 清理完成"
  CLEANED=$((CLEANED + 1))

done <<< "${CHANNEL_IDS}"

echo ""
if [ "${CLEANED}" -gt 0 ]; then
  success "共清理 ${CLEANED} 個計費中 channel"
else
  skip "無需清理（所有 channel 皆已 STOPPED）"
fi
