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
#   # 模式 A：IP 白名單（只有自己能看，自動偵測目前 IP）
#   ./scripts/gcp-cdn-armor.sh --mode allowlist
#   ALLOW_IPS="1.2.3.4/32,5.6.7.8/32" ./scripts/gcp-cdn-armor.sh --mode allowlist
#
#   # 模式 B：完全開放（公開測試，保留 policy 可快速切回）
#   ./scripts/gcp-cdn-armor.sh --mode allow-all
#
#   # 模式 C：完全封鎖（非測試期間使用，所有請求一律 403）
#   ./scripts/gcp-cdn-armor.sh --mode deny-all
#
#   # 移除防護規則（刪除 policy）
#   ./scripts/gcp-cdn-armor.sh --mode remove
# =============================================================================
# ⚠️  注意：此腳本針對 Cloud CDN Backend Bucket 使用 CLOUD_ARMOR_EDGE 類型。
#    Edge Policy 限制：
#      - 掛載指令：--edge-security-policy（非 --security-policy）
#      - 規則：--src-ip-ranges（非 CEL expression）
#      - 不支援 rate-based-ban（該功能只適用 Backend Service）
# =============================================================================

set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-omega-pivot-488513-k6}"
BACKEND_BUCKET="deltacast-backend"
ARMOR_POLICY="deltacast-armor"
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

# ── 建立 Edge Security Policy（Backend Bucket 專用）────────────────────────
info "建立 Cloud Armor Edge Security Policy: $ARMOR_POLICY"
if gcloud compute security-policies describe "$ARMOR_POLICY" --quiet 2>/dev/null; then
  warn "Policy 已存在，更新規則..."
else
  gcloud compute security-policies create "$ARMOR_POLICY" \
    --description="DeltaCast CDN protection" \
    --type=CLOUD_ARMOR_EDGE \
    --quiet
  success "Edge Policy 建立完成"
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
  # Edge Policy 使用 --src-ip-ranges（逗號分隔），不用 CEL expression
  gcloud compute security-policies rules create 1000 \
    --security-policy="$ARMOR_POLICY" \
    --src-ip-ranges="$ALLOW_IPS" \
    --action=allow \
    --description="Allow whitelisted IPs" \
    --quiet 2>/dev/null || \
  gcloud compute security-policies rules update 1000 \
    --security-policy="$ARMOR_POLICY" \
    --src-ip-ranges="$ALLOW_IPS" \
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

elif [[ "$MODE_VALUE" == "deny-all" ]]; then
  # ── 模式 C：完全封鎖 ──────────────────────────────────────────────────────
  CURRENT_DEFAULT=$(gcloud compute security-policies rules describe 2147483647 \
    --security-policy="$ARMOR_POLICY" --format="value(action)" 2>/dev/null || echo "unknown")
  RULE_1000=$(gcloud compute security-policies rules describe 1000 \
    --security-policy="$ARMOR_POLICY" --quiet &>/dev/null && echo true || echo false)
  if [[ "$CURRENT_DEFAULT" == "deny(403)" && "$RULE_1000" == "false" ]]; then
    info "已是 deny-all 狀態。無需變更。"
    exit 0
  fi

  info "設定完全封鎖：所有請求一律 403"

  # 刪除 priority 1000（若存在）避免有殘留 allow 規則
  gcloud compute security-policies rules delete 1000 \
    --security-policy="$ARMOR_POLICY" --quiet 2>/dev/null || true

  # Default rule 設為 deny-403
  gcloud compute security-policies rules update 2147483647 \
    --security-policy="$ARMOR_POLICY" \
    --action=deny-403 \
    --quiet
  success "Default rule 設為 deny-403（所有請求一律封鎖）"

elif [[ "$MODE_VALUE" == "allow-all" ]]; then
  # ── 模式 D：完全開放（保留 policy，方便快速切回 deny-all）───────────────
  CURRENT_DEFAULT=$(gcloud compute security-policies rules describe 2147483647 \
    --security-policy="$ARMOR_POLICY" --format="value(action)" 2>/dev/null || echo "unknown")
  RULE_1000=$(gcloud compute security-policies rules describe 1000 \
    --security-policy="$ARMOR_POLICY" --quiet &>/dev/null && echo true || echo false)
  if [[ "$CURRENT_DEFAULT" == "allow" && "$RULE_1000" == "false" ]]; then
    info "已是 allow-all 狀態。無需變更。"
    exit 0
  fi

  info "設定完全開放：所有請求一律允許"

  # 刪除 priority 1000（若存在）
  gcloud compute security-policies rules delete 1000 \
    --security-policy="$ARMOR_POLICY" --quiet 2>/dev/null || true

  # Default rule 設為 allow
  gcloud compute security-policies rules update 2147483647 \
    --security-policy="$ARMOR_POLICY" \
    --action=allow \
    --quiet
  success "Default rule 設為 allow（所有請求開放）"

elif [[ "$MODE_VALUE" == "rate-limit" ]]; then
  # ── 模式 B：速率限制（Edge Policy 不支援）────────────────────────────────
  echo -e "${RED}[ERR]${NC}  rate-limit 不支援 Backend Bucket（CLOUD_ARMOR_EDGE）。"
  echo "       rate-based-ban 只適用於 Backend Service（VM/NEG）。"
  echo "       測試期間建議改用：--mode allowlist、--mode allow-all 或 --mode deny-all"
  exit 1
else
  echo "未知模式：$MODE_VALUE"
  echo "用法: ./gcp-cdn-armor.sh --mode [allowlist|allow-all|deny-all|remove]"
  exit 1
fi

# ── 將 Edge Policy 掛到 Backend Bucket ──────────────────────────────────────
info "將 Edge Security Policy 掛到 Backend Bucket: $BACKEND_BUCKET"
gcloud compute backend-buckets update "$BACKEND_BUCKET" \
  --edge-security-policy="$ARMOR_POLICY" \
  --quiet
success "Cloud Armor Edge Policy 已啟用於 CDN"

echo ""
echo -e "${GREEN}=================================================${NC}"
echo -e "${GREEN}  ✅  Cloud Armor 設定完成（模式：${MODE_VALUE}）${NC}"
echo -e "${GREEN}=================================================${NC}"
echo ""
info "目前規則："
gcloud compute security-policies describe "$ARMOR_POLICY" \
  --format="table(rules.priority,rules.action,rules.description,rules.match.config.srcIpRanges)"
echo ""
warn "移除防護：./scripts/gcp-cdn-armor.sh --mode remove"
