#!/usr/bin/env bash
# GCP resource status check for DeltaCast -- ready for test?

set -euo pipefail

# ── 載入環境變數 ──────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
[ -f "${SCRIPT_DIR}/../.env" ] && source "${SCRIPT_DIR}/../.env"

# ── 必填變數檢查 ──────────────────────────────────────────────────────────────
MISSING=()
[ -z "${GCP_PROJECT_ID:-}" ] && MISSING+=("GCP_PROJECT_ID")
[ -z "${GCP_BUCKET_NAME:-}" ] && MISSING+=("GCP_BUCKET_NAME")
if [ ${#MISSING[@]} -gt 0 ]; then
  echo "Error: missing required env vars: ${MISSING[*]}" >&2
  echo "Copy scripts/.env.example to scripts/.env and fill in values." >&2
  exit 1
fi

PROJECT_ID="${GCP_PROJECT_ID}"
REGION="${GCP_REGION:-asia-east1}"
BUCKET_NAME="${GCP_BUCKET_NAME}"
BACKEND_BUCKET="deltacast-backend"
ARMOR_POLICY="deltacast-armor"
FORWARDING_RULE="deltacast-http-rule"
SA_NAME="deltacast-server"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
SA_KEY_PATH="${SA_KEY_PATH:-$HOME/deltacast-sa-key.json}"
ENV_FILE="server/.env.local"

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

FAIL_COUNT=0
WARN_COUNT=0

ok()      { echo -e "  ${GREEN}[OK]${NC}    $*"; }
fail_()   { echo -e "  ${RED}[FAIL]${NC}  $*"; FAIL_COUNT=$((FAIL_COUNT+1)); }
warn_()   { echo -e "  ${YELLOW}[WARN]${NC}  $*"; WARN_COUNT=$((WARN_COUNT+1)); }
section() { echo ""; echo -e "${CYAN}${BOLD}-- $* ${NC}"; }

gcloud config set project "${PROJECT_ID}" --quiet 2>/dev/null

PROJECT_NUMBER=$(gcloud projects list \
  --filter="projectId=${PROJECT_ID}" \
  --format="value(projectNumber)" 2>/dev/null)
CDN_SA="service-${PROJECT_NUMBER}@cloud-cdn-fill.iam.gserviceaccount.com"

echo ""
echo -e "${CYAN}${BOLD}=====================================================${NC}"
echo -e "${CYAN}${BOLD}  GCP Resource Status -- DeltaCast${NC}"
echo -e "${CYAN}${BOLD}=====================================================${NC}"
echo "  Project : ${PROJECT_ID}"
echo "  Region  : ${REGION}"

# ---------------------------------------------------------------------------
section "GCS Bucket"

if gsutil ls "gs://${BUCKET_NAME}" &>/dev/null; then
  ok "Bucket exists: gs://${BUCKET_NAME}"
  IAM_JSON=$(gsutil iam get "gs://${BUCKET_NAME}" 2>/dev/null)

  GCS_LOCKED=false
  if echo "${IAM_JSON}" | grep -q "allUsers"; then
    warn_ "allUsers:objectViewer present -- direct GCS URL is PUBLIC (not locked)"
  else
    GCS_LOCKED=true
    ok "allUsers removed -- direct storage.googleapis.com URL is blocked"
  fi

  if echo "${IAM_JSON}" | grep -q "${CDN_SA}"; then
    ok "CDN fill SA has objectViewer -- CDN serving works"
  else
    if [[ "${GCS_LOCKED}" == "true" ]]; then
      fail_ "CDN fill SA has no read access -- CDN broken. Run: make gcp-open"
    else
      warn_ "CDN fill SA not granted (bucket is public, CDN still works)"
    fi
  fi
else
  fail_ "Bucket gs://${BUCKET_NAME} does not exist"
fi

# ---------------------------------------------------------------------------
section "Cloud CDN"

if gcloud compute backend-buckets describe "${BACKEND_BUCKET}" --quiet &>/dev/null; then
  ok "Backend Bucket exists: ${BACKEND_BUCKET}"
  EDGE_POLICY=$(gcloud compute backend-buckets describe "${BACKEND_BUCKET}" \
    --format="value(edgeSecurityPolicy)" 2>/dev/null || true)
  if [[ -n "${EDGE_POLICY}" ]]; then
    ok "Edge Security Policy attached: $(basename "${EDGE_POLICY}")"
  else
    warn_ "No Edge Security Policy -- CDN is fully public (no Cloud Armor)"
  fi
else
  fail_ "Backend Bucket '${BACKEND_BUCKET}' does not exist"
fi

if gcloud compute forwarding-rules describe "${FORWARDING_RULE}" \
    --global --quiet &>/dev/null; then
  CDN_IP=$(gcloud compute forwarding-rules describe "${FORWARDING_RULE}" \
    --global --format="get(IPAddress)" 2>/dev/null)
  ok "Forwarding Rule exists, CDN IP: ${CDN_IP}"
else
  fail_ "Forwarding Rule '${FORWARDING_RULE}' does not exist"
fi

# ---------------------------------------------------------------------------
section "Cloud Armor"

CDN_MODE="none"
if gcloud compute security-policies describe "${ARMOR_POLICY}" --quiet &>/dev/null; then
  ok "Security Policy exists: ${ARMOR_POLICY}"

  DEFAULT_ACTION=$(gcloud compute security-policies rules describe 2147483647 \
    --security-policy="${ARMOR_POLICY}" \
    --format="value(action)" 2>/dev/null || echo "unknown")

  RULE_1000_EXISTS=false
  if gcloud compute security-policies rules describe 1000 \
      --security-policy="${ARMOR_POLICY}" --quiet &>/dev/null; then
    RULE_1000_EXISTS=true
  fi

  if [[ "${DEFAULT_ACTION}" == "deny(403)" && "${RULE_1000_EXISTS}" == "false" ]]; then
    warn_ "Mode: deny-all (all traffic blocked). Run 'make gcp-open' to allow testing."
    CDN_MODE="deny-all"
  elif [[ "${DEFAULT_ACTION}" == "deny(403)" && "${RULE_1000_EXISTS}" == "true" ]]; then
    ALLOWED_IPS=$(gcloud compute security-policies rules describe 1000 \
      --security-policy="${ARMOR_POLICY}" \
      --format="value(match.config.srcIpRanges)" 2>/dev/null || echo "unknown")
    ok "Mode: allowlist -- allowed IPs: ${ALLOWED_IPS}"
    CDN_MODE="allowlist"
  elif [[ "${DEFAULT_ACTION}" == "allow" ]]; then
    ok "Mode: allow-all (fully open)"
    CDN_MODE="allow-all"
  else
    warn_ "Mode: unknown (default action=${DEFAULT_ACTION})"
    CDN_MODE="unknown"
  fi
else
  warn_ "Cloud Armor Policy not found -- CDN is fully public (no protection)"
fi

# ---------------------------------------------------------------------------
section "Service Account"

if gcloud iam service-accounts describe "${SA_EMAIL}" --quiet &>/dev/null; then
  ok "Service Account exists: ${SA_NAME}"
else
  fail_ "Service Account '${SA_EMAIL}' does not exist"
fi

if [[ -f "${SA_KEY_PATH}" ]]; then
  ok "SA key file exists: ${SA_KEY_PATH}"
else
  fail_ "SA key file missing: ${SA_KEY_PATH} (re-run scripts/gcp-setup.sh)"
fi

if [[ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]]; then
  ok "GOOGLE_APPLICATION_CREDENTIALS set: ${GOOGLE_APPLICATION_CREDENTIALS}"
else
  warn_ "GOOGLE_APPLICATION_CREDENTIALS not set in shell (add to ~/.zshrc)"
fi

# ---------------------------------------------------------------------------
section "Live Stream API"

if gcloud services list --enabled \
    --filter="name:livestream.googleapis.com" \
    --format="value(name)" 2>/dev/null | grep -q "livestream"; then
  ok "livestream.googleapis.com enabled"
else
  fail_ "livestream.googleapis.com NOT enabled"
fi

# ---------------------------------------------------------------------------
section ".env.local"

if [[ -f "${ENV_FILE}" ]]; then
  REQUIRED_VARS=(
    AGORA_APP_ID AGORA_APP_CERTIFICATE
    AGORA_REST_KEY AGORA_REST_SECRET AGORA_NCS_SECRET
    GCP_PROJECT_ID GCP_REGION GCP_BUCKET_NAME GCP_CDN_DOMAIN
    YOUTUBE_CLIENT_ID YOUTUBE_CLIENT_SECRET YOUTUBE_REFRESH_TOKEN
    JWT_SECRET
  )
  for var in "${REQUIRED_VARS[@]}"; do
    val=$(grep "^${var}=" "${ENV_FILE}" 2>/dev/null | cut -d= -f2- | tr -d '"' | tr -d "'")
    if [[ -z "${val}" || "${val}" == your-* ]]; then
      fail_ "Missing or placeholder: ${var}"
    else
      ok "${var} is set"
    fi
  done
else
  fail_ "${ENV_FILE} not found (copy from .env.local.example and fill in values)"
fi

# ---------------------------------------------------------------------------
echo ""
echo -e "${BOLD}=====================================================${NC}"

if [[ ${FAIL_COUNT} -gt 0 ]]; then
  echo -e "  ${RED}${BOLD}NOT READY  --  ${FAIL_COUNT} issue(s) must be fixed${NC}"
elif [[ "${CDN_MODE}" == "deny-all" || "${CDN_MODE}" == "none" ]]; then
  echo -e "  ${YELLOW}${BOLD}LOCKED  --  Run 'make gcp-open' to enable test access${NC}"
elif [[ ${WARN_COUNT} -gt 0 ]]; then
  echo -e "  ${YELLOW}${BOLD}READY WITH WARNINGS (${WARN_COUNT})  --  see above${NC}"
else
  echo -e "  ${GREEN}${BOLD}READY FOR TEST${NC}"
fi

echo -e "${BOLD}=====================================================${NC}"
echo ""
