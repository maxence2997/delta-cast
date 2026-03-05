# YouTube 設定

> ⚠️ 此文件不存放真實金鑰。金鑰填入 `server/.env.local`（本機）或 CI/CD Secret。

完成後取得：`YOUTUBE_CLIENT_ID`、`YOUTUBE_CLIENT_SECRET`、`YOUTUBE_REFRESH_TOKEN`

---

## 2.1 啟用 YouTube Data API v3

1. 前往 [Google Cloud Console](https://console.cloud.google.com/)。
2. 建立或選擇一個 GCP 專案（建議與 GCP 設定共用同一專案）。
3. 進入 **APIs & Services** → **Library**，搜尋 **YouTube Data API v3** 並 **Enable**。

---

## 2.2 建立 OAuth 2.0 憑證

1. 進入 **APIs & Services** → **Credentials** → **Create Credentials** → **OAuth client ID**。
2. Application Type 選擇 **Web application**。
3. Authorized redirect URIs 新增：`http://localhost:8080/oauth/callback`（僅用於取得 refresh token）。
4. 建立後記錄：
   - **Client ID** → 填入 `YOUTUBE_CLIENT_ID`
   - **Client Secret** → 填入 `YOUTUBE_CLIENT_SECRET`

---

## 2.3 取得 Refresh Token

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

---

## 2.4 驗證直播功能

1. 前往 [YouTube Studio](https://studio.youtube.com/) → **Settings** → **Channel** → **Feature eligibility**。
2. 確認 **Live streaming** 已啟用。
   > ⚠️ 首次啟用需等待 **24 小時**審核，請提前處理。
3. 可在 **Go live** 頁面手動測試一次串流，確認頻道可正常直播。

---

## 2.5 直播嵌入設定（必要）

> ⚠️ 若略過此步驟，DeltaCast Audience 頁面的 YouTube 播放器只有頻道擁有者的瀏覽器（已登入帳號）能正常播放；其他人會看到 YouTube iframe 內部出現播放失敗畫面（error 101 / error 150）。

### 根因

YouTube 對每場直播廣播（broadcast）獨立控管嵌入權限。頻道擁有者因帳號驗證免受此限；未登入或非擁有者的瀏覽器必須明確被允許才能嵌入播放。

### 設定步驟

1. 前往 [YouTube Studio](https://studio.youtube.com/) → **Go live** → 選取目標廣播 → 點選 **Edit（鉛筆圖示）**。
2. 切換至 **Customization** 分頁。
3. 找到 **Allow embedding** 選項，確認已勾選（預設可能為關閉）。
4. 儲存後重新整理廣播，設定立即生效。

> 若使用 YouTube Data API 程式化建立廣播（DeltaCast 正常流程），目前 API 不直接提供設定 Allow embedding 的欄位；每場直播需在 YouTube Studio 手動確認一次。

---

## 環境變數對應

### 個人帳號模式（預設）

| 環境變數                | 取自位置                                                         | 備註                                 |
| ----------------------- | ---------------------------------------------------------------- | ------------------------------------ |
| `YOUTUBE_CLIENT_ID`     | APIs & Services → Credentials → OAuth 2.0 Client → Client ID     | 結尾為 `.apps.googleusercontent.com` |
| `YOUTUBE_CLIENT_SECRET` | APIs & Services → Credentials → OAuth 2.0 Client → Client Secret |                                      |
| `YOUTUBE_REFRESH_TOKEN` | OAuth Playground 或 cURL 手動換取                                | 永不過期，換 access token 用         |

### 企業帳號模式（Google Workspace + DWD）

| 環境變數                    | 說明                                                            |
| --------------------------- | --------------------------------------------------------------- |
| `YOUTUBE_IMPERSONATE_EMAIL` | 被代理的頻道擁有者 email，設定後自動切換至 DWD 模式            |
| `YOUTUBE_SA_KEY_PATH`       | SA key JSON 檔路徑（標準 SA key 格式，不支援 WIF config）       |
| `YOUTUBE_SA_KEY_JSON`       | SA key JSON inline 內容，無法掛檔時的 fallback                  |

設定 `YOUTUBE_IMPERSONATE_EMAIL` 後，`YOUTUBE_CLIENT_ID`、`YOUTUBE_CLIENT_SECRET`、`YOUTUBE_REFRESH_TOKEN` 均可留空。

---

## 2.5 企業帳號：Domain-Wide Delegation（Google Workspace）

適用場景：公司 Google Workspace 環境，以 Service Account 代理頻道擁有者帳號操作 YouTube，
不需 OAuth2 consent flow，不需 refresh token。

### 前置條件

- 使用的 SA 已有 SA private key（`YOUTUBE_SA_KEY_PATH` 或 `YOUTUBE_SA_KEY_JSON` 指向 SA key JSON）
  - **注意**：DWD 不支援 WIF external_account config，必須是標準 SA key JSON
- 頻道擁有者帳號屬於同一 Google Workspace org

### 步驟

**1. 取得 SA 的 Client ID**

```bash
gcloud iam service-accounts describe deltacast-server@YOUR_PROJECT_ID.iam.gserviceaccount.com \
  --format="value(oauth2ClientId)"
```

**2. 在 Google Admin Console 設定 DWD**

1. 前往 [Google Admin Console](https://admin.google.com/) → **Security** → **Access and data control** → **API controls** → **Manage Domain Wide Delegation**
2. 點選 **Add new** → 填入上一步取得的 SA Client ID
3. OAuth Scopes 填入：`https://www.googleapis.com/auth/youtube`
4. 儲存

**3. 設定環境變數**

```bash
# SA key（與 GCP 共用同一個 SA 即可）
YOUTUBE_SA_KEY_PATH=/path/to/deltacast-sa-key.json

# 頻道擁有者 email（被代理的對象）
YOUTUBE_IMPERSONATE_EMAIL=channel-owner@company.com

# OAuth2 三個變數可留空
# YOUTUBE_CLIENT_ID=
# YOUTUBE_CLIENT_SECRET=
# YOUTUBE_REFRESH_TOKEN=
```

程式啟動後 log 會顯示 `youtube auth initialized mode="SA DWD" subject="channel-owner@company.com"` 確認生效。

---

## 快速驗證

```bash
# 用 Refresh Token 換 Access Token（回傳 access_token 即有效）
curl -s -X POST https://oauth2.googleapis.com/token \
  -d "client_id=$YOUTUBE_CLIENT_ID" \
  -d "client_secret=$YOUTUBE_CLIENT_SECRET" \
  -d "refresh_token=$YOUTUBE_REFRESH_TOKEN" \
  -d "grant_type=refresh_token" | jq .access_token
```

```

```
