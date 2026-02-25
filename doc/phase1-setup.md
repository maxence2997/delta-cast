# Phase 1: 基礎設施設定指南

本文件對應 `task-tracking.md` Phase 1 的三個項目，逐步指引在 Agora、YouTube、GCP 上完成所有資源配置。完成後將獲得填入 `.env` 所需的全部金鑰與設定值。

> **預估時間**：約 1–2 小時（YouTube 直播功能審核需額外等待 24 小時）。

---

## 目錄

1. [Agora 設定](#1-agora-設定)
2. [YouTube 設定](#2-youtube-設定)
3. [GCP 設定](#3-gcp-設定)
4. [驗證清單](#4-驗證清單)

---

## 1. Agora 設定

> 完成後取得：`AGORA_APP_ID`、`AGORA_APP_CERTIFICATE`、`AGORA_REST_KEY`、`AGORA_REST_SECRET`、`AGORA_NCS_SECRET`

### 1.1 建立專案

1. 前往 [Agora Console](https://console.agora.io/) 並登入（或註冊帳號）。
2. 點選 **Project Management** → **Create**。
3. 輸入專案名稱（如 `DeltaCast`），Authentication Mechanism 選擇 **Secured mode: APP ID + Token**。
4. 建立完成後，記錄：
   - **App ID** → 填入 `AGORA_APP_ID`
   - **App Certificate** → 填入 `AGORA_APP_CERTIFICATE`

### 1.2 啟用 REST API

1. 在 Console 左側選單點選 **RESTful API**。
2. 點選 **Add Secret**，生成一組 Customer ID / Customer Secret。
3. 記錄：
   - **Customer ID** → 填入 `AGORA_REST_KEY`
   - **Customer Secret** → 填入 `AGORA_REST_SECRET`

### 1.3 配置 Notification Callback Service (NCS)

1. 在 Console 左側選單點選 **Notifications** (NCS)。
2. 點選 **Enable**，選擇需要接收的事件類型，至少勾選：
   - **101** — channel create（有使用者加入頻道）
   - **102** — channel destroy（頻道銷毀）
3. 設定 Webhook URL：`https://<YOUR_SERVER_DOMAIN>/v1/webhook/agora`
   > 本地開發時可使用 [ngrok](https://ngrok.com/) 產生公開 URL。
4. 設定完成後會顯示 **NCS Secret**，記錄：
   - **NCS Secret** → 填入 `AGORA_NCS_SECRET`

### 1.4 啟用 Media Push (RTMP Converter)

1. 在 Console 的專案頁面，找到 **Extensions** / **Marketplace**。
2. 搜尋 **Media Push** 並啟用。
3. 無需額外金鑰，此功能透過 REST API 呼叫（已在 1.2 配置）。

---

## 2. YouTube 設定

> 完成後取得：`YOUTUBE_CLIENT_ID`、`YOUTUBE_CLIENT_SECRET`、`YOUTUBE_REFRESH_TOKEN`

### 2.1 啟用 YouTube Data API v3

1. 前往 [Google Cloud Console](https://console.cloud.google.com/)。
2. 建立或選擇一個 GCP 專案（建議與 Phase 1.3 共用同一專案）。
3. 進入 **APIs & Services** → **Library**，搜尋 **YouTube Data API v3** 並 **Enable**。

### 2.2 建立 OAuth 2.0 憑證

1. 進入 **APIs & Services** → **Credentials** → **Create Credentials** → **OAuth client ID**。
2. Application Type 選擇 **Web application**。
3. Authorized redirect URIs 新增：`http://localhost:8080/oauth/callback`（僅用於取得 refresh token）。
4. 建立後記錄：
   - **Client ID** → 填入 `YOUTUBE_CLIENT_ID`
   - **Client Secret** → 填入 `YOUTUBE_CLIENT_SECRET`

### 2.3 取得 Refresh Token

使用 [OAuth 2.0 Playground](https://developers.google.com/oauthplayground/) 或手動流程取得 Refresh Token：

**方法 A：OAuth Playground（推薦）**

1. 前往 [OAuth 2.0 Playground](https://developers.google.com/oauthplayground/)。
2. 點右上角齒輪 ⚙️ → 勾選 **Use your own OAuth credentials** → 填入 Client ID & Secret。
3. 左側 Step 1：在 scope 中輸入 `https://www.googleapis.com/auth/youtube` → **Authorize APIs**。
4. 完成登入授權後，Step 2：點選 **Exchange authorization code for tokens**。
5. 記錄 **Refresh Token** → 填入 `YOUTUBE_REFRESH_TOKEN`

**方法 B：cURL 手動流程**

```bash
# Step 1: 在瀏覽器開啟以下 URL 登入授權
open "https://accounts.google.com/o/oauth2/auth?client_id=YOUR_CLIENT_ID&redirect_uri=http://localhost:8080/oauth/callback&response_type=code&scope=https://www.googleapis.com/auth/youtube&access_type=offline&prompt=consent"

# Step 2: 從 redirect URL 取得 authorization code，交換 refresh token
curl -X POST https://oauth2.googleapis.com/token \
  -d "code=YOUR_AUTH_CODE" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "redirect_uri=http://localhost:8080/oauth/callback" \
  -d "grant_type=authorization_code"
```

### 2.4 驗證直播功能

1. 前往 [YouTube Studio](https://studio.youtube.com/) → **Settings** → **Channel** → **Feature eligibility**。
2. 確認 **Live streaming** 已啟用。
   > ⚠️ 首次啟用需等待 **24 小時**審核，請提前處理。
3. 可在 **Go live** 頁面手動測試一次串流，確認頻道可正常直播。

---

## 3. GCP 設定

> 完成後取得：`GCP_PROJECT_ID`、`GCP_REGION`、`GCP_BUCKET_NAME`、`GCP_CDN_DOMAIN`

### 3.1 前置準備

```bash
# 安裝 gcloud CLI（若尚未安裝）
brew install google-cloud-sdk  # macOS

# 登入與設定專案
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
```

### 3.2 啟用 Live Stream API

```bash
gcloud services enable livestream.googleapis.com
```

驗證：

```bash
gcloud services list --enabled | grep livestream
# 應顯示 livestream.googleapis.com
```

### 3.3 建立 GCS Bucket

```bash
# 建立 Bucket（區域建議與 Live Stream API 一致）
export GCP_REGION=us-central1
export BUCKET_NAME=deltacast-live-output  # 自訂名稱

gsutil mb -l $GCP_REGION gs://$BUCKET_NAME

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

記錄：

- **Bucket Name** → 填入 `GCP_BUCKET_NAME`

### 3.4 配置 Cloud CDN

```bash
# 建立 Backend Bucket
gcloud compute backend-buckets create deltacast-backend \
  --gcs-bucket-name=$BUCKET_NAME \
  --enable-cdn

# 建立 URL Map
gcloud compute url-maps create deltacast-url-map \
  --default-backend-bucket=deltacast-backend

# 建立 HTTP Proxy
gcloud compute target-http-proxies create deltacast-http-proxy \
  --url-map=deltacast-url-map

# 建立 Forwarding Rule（取得外部 IP）
gcloud compute forwarding-rules create deltacast-http-rule \
  --global \
  --target-http-proxy=deltacast-http-proxy \
  --ports=80

# 查看分配的外部 IP
gcloud compute forwarding-rules describe deltacast-http-rule --global \
  --format="get(IPAddress)"
```

記錄分配的 IP。你可以：

- **直接使用 IP**：將 IP 填入 `GCP_CDN_DOMAIN`（如 `34.120.xxx.xxx`）
- **綁定域名**（建議）：在 DNS 設定一筆 A Record 指向此 IP，如 `cdn.deltacast.example.com` → 填入 `GCP_CDN_DOMAIN`

### 3.5 設定 Service Account（應用程式認證）

```bash
# 建立 Service Account
gcloud iam service-accounts create deltacast-server \
  --display-name="DeltaCast Server"

# 授予 Live Stream API 權限
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/livestream.editor"

# 授予 GCS 寫入權限（Live Stream API 輸出用）
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin"

# 下載金鑰檔
gcloud iam service-accounts keys create ~/deltacast-sa-key.json \
  --iam-account=deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com

# 設定環境變數（本地開發用）
export GOOGLE_APPLICATION_CREDENTIALS=~/deltacast-sa-key.json
```

> ⚠️ `deltacast-sa-key.json` 是敏感檔案，不要加入版本控制。

記錄：

- **Project ID** → 填入 `GCP_PROJECT_ID`
- **Region** → 填入 `GCP_REGION`（預設 `us-central1`）

---

## 4. 驗證清單

全部完成後，確認 `.env` 中以下變數都已填入真實值：

```
✅ AGORA_APP_ID
✅ AGORA_APP_CERTIFICATE
✅ AGORA_REST_KEY
✅ AGORA_REST_SECRET
✅ AGORA_NCS_SECRET
✅ GCP_PROJECT_ID
✅ GCP_REGION
✅ GCP_BUCKET_NAME
✅ GCP_CDN_DOMAIN
✅ YOUTUBE_CLIENT_ID
✅ YOUTUBE_CLIENT_SECRET
✅ YOUTUBE_REFRESH_TOKEN
✅ GOOGLE_APPLICATION_CREDENTIALS（系統環境變數，指向 SA 金鑰檔）
```

快速驗證指令：

```bash
# 驗證 GCP 認證
gcloud auth application-default print-access-token

# 驗證 Live Stream API 可用
gcloud beta livestream inputs list --location=$GCP_REGION

# 驗證 YouTube API（用 refresh token 換 access token）
curl -X POST https://oauth2.googleapis.com/token \
  -d "client_id=$YOUTUBE_CLIENT_ID" \
  -d "client_secret=$YOUTUBE_CLIENT_SECRET" \
  -d "refresh_token=$YOUTUBE_REFRESH_TOKEN" \
  -d "grant_type=refresh_token"
```

全部通過後，Phase 1 完成！可以回到 `task-tracking.md` 勾選三個項目，並開始 Phase 2 的開發工作。
