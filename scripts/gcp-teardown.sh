#!/usr/bin/env bash
# =============================================================================
# GCP Teardown Script — DeltaCast
#
# 清除所有為 DeltaCast 建立的靜態 GCP 資源。
# 執行前確認 PROJECT_ID / REGION / BUCKET_NAME 與實際設定一致。
#
# 執行方式：
#   chmod +x scripts/gcp-teardown.sh
#   ./scripts/gcp-teardown.sh
#
# 若要跳過確認提示（CI 環境）：
#   SKIP_CONFIRM=1 ./scripts/gcp-teardown.sh
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
MISSING=()
[ -z "${GCP_PROJECT_ID:-}" ] && MISSING+=("GCP_PROJECT_ID")
[ -z "${GCP_BUCKET_NAME:-}" ] && MISSING+=("GCP_BUCKET_NAME")
if [ ${#MISSING[@]} -gt 0 ]; then
  echo "Error: missing required env vars: ${MISSING[*]}" >&2
  echo "Copy scripts/.env.example to scripts/.env and fill in values." >&2
  exit 1
fi

# ── 設定 ──────────────────────────────────────────────────────────────────────
PROJECT_ID="${GCP_PROJECT_ID}"
REGION="${GCP_REGION:-asia-east1}"
BUCKET_NAME="${GCP_BUCKET_NAME}"
SA_NAME="deltacast-server"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

# Cloud CDN 相關資源名稱（與 gcp-setup.sh 一致）
FORWARDING_RULE="deltacast-http-rule"
HTTP_PROXY="deltacast-http-proxy"
URL_MAP="deltacast-url-map"
BACKEND_BUCKET="deltacast-backend"
ARMOR_POLICY="deltacast-armor"

# Cloud DNS
DNS_ZONE_NAME="${DNS_ZONE_NAME:-asia-east1}"
DNS_ZONE_DNS_NAME="${DNS_ZONE_DNS_NAME:-cdn.deltacast.example.com.}"

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
err()     { echo -e "${RED}[ERR]${NC}   $*"; }

# ── 確認提示 ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${RED}=====================================================${NC}"
echo -e "${RED}  ⚠️  GCP TEARDOWN — 此操作不可復原  ⚠️${NC}"
echo -e "${RED}=====================================================${NC}"
echo ""
echo "  Project : $PROJECT_ID"
echo "  Region  : $REGION"
echo "  Bucket  : $BUCKET_NAME"
echo ""

if [[ "${SKIP_CONFIRM:-0}" != "1" ]]; then
  read -r -p "確定要清除所有 DeltaCast GCP 資源嗎？輸入 'yes' 繼續：" confirm
  if [[ "$confirm" != "yes" ]]; then
    echo "已取消。"
    exit 0
  fi
fi

gcloud config set project "$PROJECT_ID" --quiet

# ── 輔助函式：執行指令，失敗不中斷腳本 ──────────────────────────────────────
run() {
  if eval "$@" 2>/dev/null; then
    success "$*"
  else
    skip "已不存在或跳過：$*"
  fi
}

echo ""
info "════ Step 1：清除 Live Stream API 執行時資源 ════"
# Live Stream 的 Channel/Input/Event 是執行時動態建立的，需先清除

# 停止所有 Channel 並刪除 Event
info "列出所有 Channel..."
CHANNELS=$(gcloud beta livestream channels list \
  --location="$REGION" \
  --format="value(name)" 2>/dev/null || true)

if [[ -n "$CHANNELS" ]]; then
  while IFS= read -r channel; do
    CHANNEL_ID=$(basename "$channel")
    warn "停止 Channel: $CHANNEL_ID"
    gcloud beta livestream channels stop "$CHANNEL_ID" \
      --location="$REGION" --quiet 2>/dev/null || true
    sleep 5
    info "刪除 Channel Events: $CHANNEL_ID"
    EVENTS=$(gcloud beta livestream channels events list \
      --channel="$CHANNEL_ID" \
      --location="$REGION" \
      --format="value(name)" 2>/dev/null || true)
    while IFS= read -r event; do
      [[ -z "$event" ]] && continue
      EVENT_ID=$(basename "$event")
      run gcloud beta livestream channels events delete "$EVENT_ID" \
        --channel="$CHANNEL_ID" --location="$REGION" --quiet
    done <<< "$EVENTS"
    run gcloud beta livestream channels delete "$CHANNEL_ID" \
      --location="$REGION" --quiet
  done <<< "$CHANNELS"
else
  skip "無 Channel 資源"
fi

info "列出所有 Input..."
INPUTS=$(gcloud beta livestream inputs list \
  --location="$REGION" \
  --format="value(name)" 2>/dev/null || true)

if [[ -n "$INPUTS" ]]; then
  while IFS= read -r input; do
    [[ -z "$input" ]] && continue
    INPUT_ID=$(basename "$input")
    run gcloud beta livestream inputs delete "$INPUT_ID" \
      --location="$REGION" --quiet
  done <<< "$INPUTS"
else
  skip "無 Input 資源"
fi

echo ""
info "════ Step 2：清除 Cloud Armor 防護規則 ════"
if gcloud compute security-policies describe "$ARMOR_POLICY" --quiet 2>/dev/null; then
  # 先從 Backend Bucket 解除關聯
  run gcloud compute backend-buckets update "$BACKEND_BUCKET" \
    --no-security-policy --quiet
  run gcloud compute security-policies delete "$ARMOR_POLICY" --quiet
else
  skip "Cloud Armor Policy $ARMOR_POLICY 不存在"
fi

echo ""
info "════ Step 3：清除 Cloud CDN / Load Balancer 資源 ════"
# 必須依序刪除：Forwarding Rule → HTTP Proxy → URL Map → Backend Bucket
run gcloud compute forwarding-rules delete "$FORWARDING_RULE" \
  --global --quiet
run gcloud compute target-http-proxies delete "$HTTP_PROXY" --quiet
run gcloud compute url-maps delete "$URL_MAP" --quiet
run gcloud compute backend-buckets delete "$BACKEND_BUCKET" --quiet

echo ""
info "════ Step 4：清除 GCS Bucket ════"
if gsutil ls "gs://$BUCKET_NAME" 2>/dev/null; then
  warn "清空 Bucket 內容：gs://$BUCKET_NAME"
  gsutil -m rm -rf "gs://$BUCKET_NAME/**" 2>/dev/null || true
  run gsutil rb "gs://$BUCKET_NAME"
else
  skip "Bucket gs://$BUCKET_NAME 不存在"
fi

echo ""
info "════ Step 5：清除 Service Account ════"
# 列出所有 SA 金鑰並刪除
info "刪除 Service Account 金鑰..."
SA_KEYS=$(gcloud iam service-accounts keys list \
  --iam-account="$SA_EMAIL" \
  --managed-by=user \
  --format="value(name)" 2>/dev/null || true)

if [[ -n "$SA_KEYS" ]]; then
  while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    KEY_ID=$(basename "$key")
    run gcloud iam service-accounts keys delete "$KEY_ID" \
      --iam-account="$SA_EMAIL" --quiet
  done <<< "$SA_KEYS"
fi

# 移除 IAM Bindings
info "移除 IAM policy bindings..."
for role in "roles/livestream.editor" "roles/storage.objectAdmin"; do
  run gcloud projects remove-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SA_EMAIL" \
    --role="$role" \
    --quiet
done

run gcloud iam service-accounts delete "$SA_EMAIL" --quiet

# ── Step 6：清除 Cloud DNS ────────────────────────────────────────────────────
echo ""
info "════ Step 6：清除 Cloud DNS ════"

if gcloud dns managed-zones describe "$DNS_ZONE_NAME" --quiet 2>/dev/null; then
  DNS_RECORD_NAME="${DNS_ZONE_DNS_NAME%%.}"

  info "刪除 A Record..."
  run "刪除 A Record ${DNS_RECORD_NAME}." \
    "gcloud dns record-sets delete '${DNS_RECORD_NAME}.' \
      --zone='$DNS_ZONE_NAME' --type=A --quiet"

  info "刪除 Managed Zone: $DNS_ZONE_NAME"
  run "刪除 DNS Zone $DNS_ZONE_NAME" \
    "gcloud dns managed-zones delete '$DNS_ZONE_NAME' --quiet"
else
  skip "DNS Managed Zone $DNS_ZONE_NAME 不存在"
fi

# ── Step 7：停用 APIs ────────────────────────────────────────────────────────
echo ""
info "════ Step 7：停用 GCP APIs ════"
for api in livestream.googleapis.com storage.googleapis.com compute.googleapis.com dns.googleapis.com; do
  if gcloud services list --enabled --filter="name:$api" \
       --format="value(name)" 2>/dev/null | grep -q "$api"; then
    run "停用 $api" \
      "gcloud services disable $api --force --quiet"
  else
    skip "$api 已停用"
  fi
done

echo ""
echo -e "${GREEN}=================================================${NC}"
echo -e "${GREEN}  ✅  GCP Teardown 完成${NC}"
echo -e "${GREEN}=================================================${NC}"
echo ""
info "已保留（手動管理）："
echo "  - 本機金鑰檔：~/deltacast-sa-key.json（請手動刪除）"
echo ""
warn "若要重新部署，執行 ./scripts/gcp-setup.sh 重建所有資源。"
