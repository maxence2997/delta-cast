#!/usr/bin/env bash
# =============================================================================
# GCP Setup Script — DeltaCast
#
# 一鍵建立所有 DeltaCast 所需的靜態 GCP 資源：
#   1. 啟用 Required APIs
#   2. 建立 GCS Bucket（公開讀取 + CORS）
#   3. 建立 Cloud CDN（Backend Bucket + URL Map + HTTP Proxy + Forwarding Rule）
#   4. 建立 Service Account 並授予 IAM 角色
#   5. 下載 SA 金鑰 → ~/deltacast-sa-key.json
#
# 前置需求：gcloud CLI 已登入（gcloud auth login）
#
# 執行方式：
#   chmod +x scripts/gcp-setup.sh
#   GCP_PROJECT_ID=omega-pivot-488513-k6 ./scripts/gcp-setup.sh
#
# 完成後輸出外部 IP，記得在 DNS 設定 A Record 並填入 .env 的 GCP_CDN_DOMAIN。
# =============================================================================

set -euo pipefail

# ── 設定 ──────────────────────────────────────────────────────────────────────
PROJECT_ID="${GCP_PROJECT_ID:-omega-pivot-488513-k6}"
REGION="${GCP_REGION:-asia-east1}"
BUCKET_NAME="${GCP_BUCKET_NAME:-deltacast-live-output}"
SA_NAME="deltacast-server"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
SA_KEY_PATH="${SA_KEY_PATH:-$HOME/deltacast-sa-key.json}"

# Cloud CDN 資源名稱
BACKEND_BUCKET="deltacast-backend"
URL_MAP="deltacast-url-map"
HTTP_PROXY="deltacast-http-proxy"
FORWARDING_RULE="deltacast-http-rule"

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

echo ""
echo -e "${CYAN}=====================================================${NC}"
echo -e "${CYAN}  GCP Setup — DeltaCast${NC}"
echo -e "${CYAN}=====================================================${NC}"
echo ""
echo "  Project : $PROJECT_ID"
echo "  Region  : $REGION"
echo "  Bucket  : $BUCKET_NAME"
echo "  SA Key  : $SA_KEY_PATH"
echo ""

gcloud config set project "$PROJECT_ID" --quiet

# ── Step 1：啟用 APIs ────────────────────────────────────────────────────────
info "════ Step 1：啟用 Required APIs ════"
for api in livestream.googleapis.com storage.googleapis.com compute.googleapis.com; do
  if gcloud services list --enabled --filter="name:$api" --format="value(name)" 2>/dev/null | grep -q "$api"; then
    skip "$api 已啟用"
  else
    gcloud services enable "$api" --quiet
    success "已啟用 $api"
  fi
done

# ── Step 2：建立 GCS Bucket ──────────────────────────────────────────────────
echo ""
info "════ Step 2：建立 GCS Bucket ════"

if gsutil ls "gs://$BUCKET_NAME" 2>/dev/null; then
  skip "Bucket gs://$BUCKET_NAME 已存在"
else
  gsutil mb -l "$REGION" -p "$PROJECT_ID" "gs://$BUCKET_NAME"
  success "Bucket 建立完成：gs://$BUCKET_NAME"
fi

info "設定公開讀取（Cloud CDN 需要）..."
gsutil iam ch allUsers:objectViewer "gs://$BUCKET_NAME" 2>/dev/null || \
  warn "allUsers objectViewer 已設定或需手動確認 uniform bucket-level access"

info "設定 CORS..."
cat > /tmp/deltacast-cors.json << 'EOF'
[
  {
    "origin": ["*"],
    "method": ["GET", "HEAD"],
    "responseHeader": ["Content-Type", "Range"],
    "maxAgeSeconds": 3600
  }
]
EOF
gsutil cors set /tmp/deltacast-cors.json "gs://$BUCKET_NAME"
success "CORS 設定完成"
rm -f /tmp/deltacast-cors.json

# ── Step 3：建立 Cloud CDN ───────────────────────────────────────────────────
echo ""
info "════ Step 3：建立 Cloud CDN ════"

# Backend Bucket
if gcloud compute backend-buckets describe "$BACKEND_BUCKET" --quiet 2>/dev/null; then
  skip "Backend Bucket $BACKEND_BUCKET 已存在"
else
  gcloud compute backend-buckets create "$BACKEND_BUCKET" \
    --gcs-bucket-name="$BUCKET_NAME" \
    --enable-cdn \
    --quiet
  success "Backend Bucket 建立完成"
fi

# URL Map
if gcloud compute url-maps describe "$URL_MAP" --quiet 2>/dev/null; then
  skip "URL Map $URL_MAP 已存在"
else
  gcloud compute url-maps create "$URL_MAP" \
    --default-backend-bucket="$BACKEND_BUCKET" \
    --quiet
  success "URL Map 建立完成"
fi

# HTTP Proxy
if gcloud compute target-http-proxies describe "$HTTP_PROXY" --quiet 2>/dev/null; then
  skip "HTTP Proxy $HTTP_PROXY 已存在"
else
  gcloud compute target-http-proxies create "$HTTP_PROXY" \
    --url-map="$URL_MAP" \
    --quiet
  success "HTTP Proxy 建立完成"
fi

# Forwarding Rule
if gcloud compute forwarding-rules describe "$FORWARDING_RULE" --global --quiet 2>/dev/null; then
  skip "Forwarding Rule $FORWARDING_RULE 已存在"
else
  gcloud compute forwarding-rules create "$FORWARDING_RULE" \
    --global \
    --target-http-proxy="$HTTP_PROXY" \
    --ports=80 \
    --quiet
  success "Forwarding Rule 建立完成"
fi

CDN_IP=$(gcloud compute forwarding-rules describe "$FORWARDING_RULE" \
  --global --format="get(IPAddress)")
success "Cloud CDN 外部 IP：$CDN_IP"

# ── Step 4：建立 Service Account ─────────────────────────────────────────────
echo ""
info "════ Step 4：建立 Service Account ════"

if gcloud iam service-accounts describe "$SA_EMAIL" --quiet 2>/dev/null; then
  skip "Service Account $SA_NAME 已存在"
else
  gcloud iam service-accounts create "$SA_NAME" \
    --display-name="DeltaCast Server" \
    --quiet
  success "Service Account 建立完成"
fi

info "授予 IAM 角色..."
for role in "roles/livestream.editor" "roles/storage.objectAdmin"; do
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SA_EMAIL" \
    --role="$role" \
    --quiet 2>/dev/null
  success "已授予 $role"
done

# ── Step 5：下載 SA 金鑰 ──────────────────────────────────────────────────────
echo ""
info "════ Step 5：下載 Service Account 金鑰 ════"

if [[ -f "$SA_KEY_PATH" ]]; then
  warn "金鑰檔已存在：$SA_KEY_PATH（略過下載，若要重新產生請先手動刪除）"
else
  gcloud iam service-accounts keys create "$SA_KEY_PATH" \
    --iam-account="$SA_EMAIL" \
    --quiet
  success "SA 金鑰已下載至：$SA_KEY_PATH"
fi

# ── 完成摘要 ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}  ✅  GCP Setup 完成${NC}"
echo -e "${GREEN}=====================================================${NC}"
echo ""
echo "請將以下值填入 server/.env.local："
echo ""
echo "  GCP_PROJECT_ID=$PROJECT_ID"
echo "  GCP_REGION=$REGION"
echo "  GCP_BUCKET_NAME=$BUCKET_NAME"
echo "  GCP_CDN_DOMAIN=<YOUR_DOMAIN_OR_IP>"
echo ""
echo -e "${CYAN}Cloud CDN IP：${NC}${CDN_IP}"
echo ""
echo "  → 若使用 IP 直接：GCP_CDN_DOMAIN=$CDN_IP"
echo "  → 若綁定域名：在 DNS 設定 A Record 指向 $CDN_IP"
echo ""
echo "系統環境變數（加入 ~/.zshrc 或 ~/.bashrc）："
echo "  export GOOGLE_APPLICATION_CREDENTIALS=$SA_KEY_PATH"
echo ""
warn "⚠️  $SA_KEY_PATH 是敏感檔案，勿加入版本控制。"
warn "⚠️  CDN IP 可能需要幾分鐘才能生效。"
