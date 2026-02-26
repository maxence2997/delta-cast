#!/usr/bin/env bash
# =============================================================================
# Cloud Armor 防護腳本 — DeltaCast CDN
#
# 為 Cloud CDN Backend Bucket 加上 Cloud Armor 安全策略：
#   1. IP 白名單（只允許指定 IP 存取），其他一律 403
#   2. 速率限制（每 IP 每分鐘超過 N 次請求則暫時封鎖）
#   3. 可選：地區封鎖（封鎖非預期地區的請求）
#
# 使用方式：
#   chmod +x scripts/gcp-cdn-armor.sh
#
#   # 模式 A：全球公開 + 速率限制（推薦測試期使用）
#   ./scripts/gcp-cdn-armor.sh --mode rate-limit
#
#   # 模式 B：IP 白名單（只有自己能看，最嚴格）
#   ALLOW_IPS="1.2.3.4,5.6.7.8" ./scripts/gcp-cdn-armor.sh --mode allowlist
#
#   # 移除防護規則
#   ./scripts/gcp-cdn-armor.sh --mode remove
# =============================================================================

set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-omega-pivot-488513-k6}"
BACKEND_BUCKET="deltacast-backend"
ARMOR_POLICY="deltacast-armor"
# 每 IP 每 60 秒允許的最大請求數（超過則返回 429）
RATE_LIMIT_THRESHOLD="${RATE_LIMIT_THRESHOLD:-60}"
# 逗號分隔的白名單 IP（支援 CIDR），例如 "1.2.3.4/32,5.6.7.0/24"
ALLOW_IPS="${ALLOW_IPS:-}"

MODE="${1:---mode}"
MODEARG="${2:-rate-limit}"
if [[ "$MODE" == "--mode" ]]; then MODE_VALUE="$MODEARG"; else MODE_VALUE="rate-limit"; fi

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }

gcloud config set project "$PROJECT_ID" --quiet

# ── 移除防護 ─────────────────────────────────────────────────────────────────
if [[ "$MODE_VALUE" == "remove" ]]; then
  info "移除 Cloud Armor policy 與 Backend Bucket 關聯..."
  gcloud compute backend-buckets update "$BACKEND_BUCKET" \
    --no-security-policy --quiet && success "已解除關聯"
  gcloud compute security-policies delete "$ARMOR_POLICY" --quiet 2>/dev/null && \
    success "已刪除 policy $ARMOR_POLICY" || warn "Policy 不存在，略過"
  exit 0
fi

# ── 建立基礎 Security Policy ─────────────────────────────────────────────────
info "建立 Cloud Armor Security Policy: $ARMOR_POLICY"
if gcloud compute security-policies describe "$ARMOR_POLICY" --quiet 2>/dev/null; then
  warn "Policy 已存在，更新規則..."
else
  gcloud compute security-policies create "$ARMOR_POLICY" \
    --description="DeltaCast CDN protection" \
    --quiet
  success "Policy 建立完成"
fi

if [[ "$MODE_VALUE" == "allowlist" ]]; then
  # ── 模式 A：IP 白名單 ─────────────────────────────────────────────────────
  if [[ -z "$ALLOW_IPS" ]]; then
    # 自動偵測目前出口 IP
    MY_IP=$(curl -sf https://api.ipify.org || curl -sf https://ifconfig.me)
    ALLOW_IPS="${MY_IP}/32"
    warn "未設定 ALLOW_IPS，自動使用目前 IP：$MY_IP"
  fi

  info "設定 IP 白名單：$ALLOW_IPS"
  # Rule 1000：允許白名單 IP
  IFS=',' read -ra IP_LIST <<< "$ALLOW_IPS"
  EXPR=""
  for ip in "${IP_LIST[@]}"; do
    [[ -n "$EXPR" ]] && EXPR="${EXPR} || "
    EXPR="${EXPR}inIpRange(origin.ip, '${ip}')"
  done

  gcloud compute security-policies rules create 1000 \
    --security-policy="$ARMOR_POLICY" \
    --expression="$EXPR" \
    --action=allow \
    --description="Allow whitelisted IPs" \
    --quiet 2>/dev/null || \
  gcloud compute security-policies rules update 1000 \
    --security-policy="$ARMOR_POLICY" \
    --expression="$EXPR" \
    --action=allow \
    --description="Allow whitelisted IPs" \
    --quiet
  success "白名單規則設定完成（Priority 1000）"

  # Rule 2147483647（default）：拒絕所有其他
  gcloud compute security-policies rules update 2147483647 \
    --security-policy="$ARMOR_POLICY" \
    --action=deny-403 \
    --quiet
  success "Default rule 設為 deny-403（白名單以外一律擋）"

elif [[ "$MODE_VALUE" == "rate-limit" ]]; then
  # ── 模式 B：速率限制 ─────────────────────────────────────────────────────
  info "設定速率限制：每 IP 每 60 秒最多 $RATE_LIMIT_THRESHOLD 次請求"

  # Rule 1000：速率超標時返回 429
  gcloud compute security-policies rules create 1000 \
    --security-policy="$ARMOR_POLICY" \
    --expression="true" \
    --action=rate-based-ban \
    --rate-limit-threshold-count="$RATE_LIMIT_THRESHOLD" \
    --rate-limit-threshold-interval-sec=60 \
    --ban-duration-sec=300 \
    --conform-action=allow \
    --exceed-action=deny-429 \
    --enforce-on-key=IP \
    --description="Rate limit: ${RATE_LIMIT_THRESHOLD} req/min per IP, ban 5min" \
    --quiet 2>/dev/null || \
  gcloud compute security-policies rules update 1000 \
    --security-policy="$ARMOR_POLICY" \
    --expression="true" \
    --action=rate-based-ban \
    --rate-limit-threshold-count="$RATE_LIMIT_THRESHOLD" \
    --rate-limit-threshold-interval-sec=60 \
    --ban-duration-sec=300 \
    --conform-action=allow \
    --exceed-action=deny-429 \
    --enforce-on-key=IP \
    --description="Rate limit: ${RATE_LIMIT_THRESHOLD} req/min per IP, ban 5min" \
    --quiet
  success "速率限制規則設定完成（Priority 1000）"
else
  echo "未知模式：$MODE_VALUE"
  echo "用法: ./gcp-cdn-armor.sh --mode [rate-limit|allowlist|remove]"
  exit 1
fi

# ── 將 Policy 掛到 Backend Bucket ────────────────────────────────────────────
info "將 Security Policy 掛到 Backend Bucket: $BACKEND_BUCKET"
gcloud compute backend-buckets update "$BACKEND_BUCKET" \
  --security-policy="$ARMOR_POLICY" \
  --quiet
success "Cloud Armor 已啟用於 CDN"

echo ""
echo -e "${GREEN}=================================================${NC}"
echo -e "${GREEN}  ✅  Cloud Armor 設定完成（模式：$MODE_VALUE）${NC}"
echo -e "${GREEN}=================================================${NC}"
echo ""
info "目前規則："
gcloud compute security-policies rules list --security-policy="$ARMOR_POLICY"
echo ""
warn "移除防護：./scripts/gcp-cdn-armor.sh --mode remove"
