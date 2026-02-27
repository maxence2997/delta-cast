# GCP 設定

> ⚠️ 此文件不存放 Service Account 金鑰。金鑰位於本機 `~/deltacast-sa-key.json`（已加入 .gitignore）。

完成後取得：`GCP_PROJECT_ID`、`GCP_REGION`、`GCP_BUCKET_NAME`、`GCP_CDN_DOMAIN`

自動化腳本：`script/gcp-setup.sh`（建立）、`script/gcp-teardown.sh`（清除）

---

## 3.1 前置準備

```bash
# 安裝 gcloud CLI（若尚未安裝）
brew install google-cloud-sdk  # macOS

# 登入與設定專案
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
```

---

## 3.2 啟用 Live Stream API

```bash
gcloud services enable livestream.googleapis.com
```

驗證：

```bash
gcloud services list --enabled | grep livestream
# 應顯示 livestream.googleapis.com
```

---

## 3.3 建立 GCS Bucket

```bash
export GCP_REGION=asia-east1
export BUCKET_NAME=<YOUR_BUCKET_NAME>

gsutil mb -l $GCP_REGION -p YOUR_PROJECT_ID gs://$BUCKET_NAME

# 設定公開讀取（Cloud CDN 需要）
gsutil iam ch allUsers:objectViewer gs://$BUCKET_NAME

# 設定 CORS（允許瀏覽器播放 HLS）
cat > /tmp/cors.json << 'EOF'
[
  {
    "origin": ["*"],
    "method": ["GET", "HEAD"],
    "responseHeader": ["Content-Type", "Range"],
    "maxAgeSeconds": 3600
  }
]
EOF
gsutil cors set /tmp/cors.json gs://$BUCKET_NAME
```

---

## 3.4 配置 Cloud CDN

```bash
# Backend Bucket
gcloud compute backend-buckets create deltacast-backend \
  --gcs-bucket-name=$BUCKET_NAME \
  --enable-cdn

# URL Map
gcloud compute url-maps create deltacast-url-map \
  --default-backend-bucket=deltacast-backend

# HTTP Proxy
gcloud compute target-http-proxies create deltacast-http-proxy \
  --url-map=deltacast-url-map

# Forwarding Rule
gcloud compute forwarding-rules create deltacast-http-rule \
  --global \
  --target-http-proxy=deltacast-http-proxy \
  --ports=80

# 查看分配的外部 IP
gcloud compute forwarding-rules describe deltacast-http-rule --global \
  --format="get(IPAddress)"
```

取得 IP 後：

- **直接使用 IP**：填入 `GCP_CDN_DOMAIN`
- **綁定域名**（建議）：DNS 設定 A Record 指向此 IP → 填入 `GCP_CDN_DOMAIN`

---

## 3.5 設定 Service Account

```bash
# 建立 SA
gcloud iam service-accounts create deltacast-server \
  --display-name="DeltaCast Server"

# 授予權限
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/livestream.editor"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin"

# 下載金鑰（不要加入版控）
gcloud iam service-accounts keys create ~/deltacast-sa-key.json \
  --iam-account=deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com

export GCP_SA_KEY_PATH=~/deltacast-sa-key.json
```

---

## 環境變數對應

| 環境變數          | 值                        | 備註                         |
| ----------------- | ------------------------- | ---------------------------- |
| `GCP_PROJECT_ID`  | `<YOUR_PROJECT_ID>`       |                              |
| `GCP_REGION`      | `asia-east1`              |                              |
| `GCP_BUCKET_NAME` | `<YOUR_BUCKET_NAME>`      |                              |
| `GCP_CDN_DOMAIN`  | `<YOUR_CDN_DOMAIN>`       | A Record 已指向 Cloud CDN IP |
| `GCP_SA_KEY_PATH` | `~/deltacast-sa-key.json` | 寫入 server/.env.local       |

---

## 快速驗證

```bash
export GCP_SA_KEY_PATH=~/deltacast-sa-key.json

# 驗證 SA 認證
gcloud auth application-default print-access-token

# 驗證 Live Stream API
gcloud beta livestream inputs list --location=asia-east1

# 驗證 CDN
curl -I http://<GCP_CDN_DOMAIN>/
```

---

## 資源管理腳本

```bash
# 建立全部靜態資源
./script/gcp-setup.sh

# 清除全部靜態資源
./script/gcp-teardown.sh

# CDN 防護（IP 白名單 / 速率限制）
./script/gcp-cdn-armor.sh --mode allowlist
```
