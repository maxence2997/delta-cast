#!/usr/bin/env bash
# =============================================================================
# GCS Bucket 存取控制腳本 — DeltaCast
#
# 解決問題：GCS bucket 有直接公開 URL（storage.googleapis.com/BUCKET/...），
# Cloud Armor 只保護 CDN 流量，繞過 CDN 直接打 GCS 無法被擋。
#
# 作法：
#   Step 1. 建立 CDN Signed URL Key → 觸發 GCP 自動建立 CDN fill SA
#   Step 2. 授予 CDN fill SA objectViewer 權限
#   Step 3. 移除 allUsers:objectViewer
#   完成後直接存取 storage.googleapis.com URL 會返回 403，
#   但 Cloud CDN 仍可透過 SA 正常抓取與快取內容。
#
# ⚠️  Signed URL Key 建立後即使不用 Signed URLs 功能也沒關係，
#     它只是用來觸發 CDN SA 的創建。
#
# 使用方式：
#   chmod +x scripts/gcp-storage-secure.sh
#
#   # 鎖定：移除公開讀取，只允許 CDN 存取（測試前執行）
#   ./scripts/gcp-storage-secure.sh --mode lock
#
#   # 解鎖：恢復 allUsers 公開讀取（臨時偵錯用）
#   ./scripts/gcp-storage-secure.sh --mode unlock
#
#   # 查看目前 bucket IAM 狀態
#   ./scripts/gcp-storage-secure.sh --mode status
# =============================================================================

set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-omega-pivot-488513-k6}"
BUCKET_NAME="${GCP_BUCKET_NAME:-deltacast-live-output}"
BACKEND_BUCKET="deltacast-backend"
CDN_KEY_NAME="deltacast-cdn-key"

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()     { echo -e "${RED}[ERR]${NC}   $*"; }

MODE="${1:---mode}"
MODEARG="${2:-status}"
if [[ "$MODE" == "--mode" ]]; then MODE_VALUE="$MODEARG"; else MODE_VALUE="status"; fi

gcloud config set project "$PROJECT_ID" --quiet

PROJECT_NUMBER=$(gcloud projects list \
  --filter="projectId=${PROJECT_ID}" \
  --format="value(projectNumber)" 2>/dev/null)
CDN_SA="service-${PROJECT_NUMBER}@cloud-cdn-fill.iam.gserviceaccount.com"

if [[ "$MODE_VALUE" == "status" ]]; then
  info "Bucket: gs://$BUCKET_NAME 目前 IAM 綁定："
  gsutil iam get "gs://$BUCKET_NAME"
  echo ""
  info "是否存在 allUsers（公開直接存取）："
  if gsutil iam get "gs://$BUCKET_NAME" 2>/dev/null | grep -q "allUsers"; then
    warn "⚠️  allUsers:objectViewer 存在 → 直接存取 storage.googleapis.com URL 開放中"
  else
    success "✅ allUsers 不存在 → GCS 直接 URL 已封閉"
  fi
  exit 0
fi

if [[ "$MODE_VALUE" == "lock" ]]; then
  echo ""
  echo -e "${CYAN}=====================================================${NC}"
  echo -e "${CYAN}  🔒  鎖定 GCS Bucket 直接存取${NC}"
  echo -e "${CYAN}=====================================================${NC}"
  echo ""
  echo "  Bucket       : gs://$BUCKET_NAME"
  echo "  CDN SA       : $CDN_SA"
  echo ""

  # 狀態檢查：若 allUsers 已不存在且 CDN SA 已有權限 → 已是鎖定狀態
  IAM_JSON=$(gsutil iam get "gs://$BUCKET_NAME" 2>/dev/null)
  ALREADY_NO_PUBLIC=$(echo "$IAM_JSON" | grep -q "allUsers" && echo false || echo true)
  ALREADY_CDN_SA=$(echo "$IAM_JSON" | grep -q "$CDN_SA" && echo true || echo false)
  if [[ "$ALREADY_NO_PUBLIC" == "true" && "$ALREADY_CDN_SA" == "true" ]]; then
    info "已是鎖定狀態（allUsers 不存在，CDN SA 已授權）。無需變更。"
    exit 0
  fi

  # Step 1：建立 CDN Signed URL Key 以觸發 CDN fill SA 的自動創建
  info "Step 1：初始化 CDN Signed URL Key（觸發 CDN fill SA 建立）..."
  if gcloud compute backend-buckets describe "$BACKEND_BUCKET" \
       --format="value(cdnPolicy.signedUrlKeyNames)" 2>/dev/null | grep -q "$CDN_KEY_NAME"; then
    info "CDN Key 已存在，略過建立"
  else
    # 產生隨機 16-byte base64url key
    KEY_VALUE=$(python3 -c "import os,base64; print(base64.urlsafe_b64encode(os.urandom(16)).decode().rstrip('='))")
    KEY_FILE=$(mktemp)
    echo -n "$KEY_VALUE" > "$KEY_FILE"
    gcloud compute backend-buckets add-signed-url-key "$BACKEND_BUCKET" \
      --key-name="$CDN_KEY_NAME" \
      --key-file="$KEY_FILE" \
      --quiet
    rm -f "$KEY_FILE"
    success "CDN Signed URL Key 建立完成（SA 已自動創建）"
  fi

  # Step 2：授予 CDN fill SA 讀取權限
  info "Step 2：授予 CDN fill SA 讀取 bucket 權限..."
  if gsutil iam get "gs://$BUCKET_NAME" 2>/dev/null | grep -q "$CDN_SA"; then
    info "CDN SA 已有讀取權限，略過"
  else
    gsutil iam ch "serviceAccount:${CDN_SA}:objectViewer" "gs://$BUCKET_NAME"
    success "已授予 CDN SA objectViewer"
  fi

  # Step 3：移除 allUsers
  info "Step 3：移除 allUsers:objectViewer..."
  if gsutil iam get "gs://$BUCKET_NAME" 2>/dev/null | grep -q "allUsers"; then
    gsutil iam ch -d allUsers:objectViewer "gs://$BUCKET_NAME"
    success "已移除 allUsers objectViewer"
  else
    info "allUsers 不存在，略過"
  fi

  echo ""
  echo -e "${GREEN}=====================================================${NC}"
  echo -e "${GREEN}  ✅  Bucket 已鎖定${NC}"
  echo -e "${GREEN}=====================================================${NC}"
  echo ""
  echo "  ❌  直接存取 storage.googleapis.com/$BUCKET_NAME/... → 403"
  echo "  ✅  透過 Cloud CDN 存取 → 正常（CDN SA 有讀取權限）"
  echo ""
  warn "⚠️  CDN 有快取，鎖定前已快取的內容仍可短暫存取。"
  warn "解鎖：./scripts/gcp-storage-secure.sh --mode unlock"

elif [[ "$MODE_VALUE" == "unlock" ]]; then
  echo ""
  warn "⚠️  恢復 allUsers 公開讀取，GCS 直接 URL 將可存取（暫時偵錯用）。"
  echo ""

  # 狀態檢查：若 allUsers 已存在 → 已是解鎖狀態
  if gsutil iam get "gs://$BUCKET_NAME" 2>/dev/null | grep -q "allUsers"; then
    info "已是解鎖狀態（allUsers:objectViewer 已存在）。無需變更。"
    exit 0
  fi

  info "恢復 allUsers:objectViewer..."
  gsutil iam ch allUsers:objectViewer "gs://$BUCKET_NAME"
  success "已恢復 allUsers objectViewer"

  echo ""
  echo -e "${YELLOW}=====================================================${NC}"
  echo -e "${YELLOW}  🔓  Bucket 已解鎖（偵錯模式）${NC}"
  echo -e "${YELLOW}=====================================================${NC}"
  echo ""
  warn "偵錯完成後記得重新鎖定：./scripts/gcp-storage-secure.sh --mode lock"
else
  echo "未知模式：$MODE_VALUE"
  echo "用法: ./gcp-storage-secure.sh --mode [lock|unlock|status]"
  exit 1
fi
