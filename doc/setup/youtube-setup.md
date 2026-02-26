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

## 環境變數對應

| 環境變數                | 取自位置                                                         | 備註                                 |
| ----------------------- | ---------------------------------------------------------------- | ------------------------------------ |
| `YOUTUBE_CLIENT_ID`     | APIs & Services → Credentials → OAuth 2.0 Client → Client ID     | 結尾為 `.apps.googleusercontent.com` |
| `YOUTUBE_CLIENT_SECRET` | APIs & Services → Credentials → OAuth 2.0 Client → Client Secret |                                      |
| `YOUTUBE_REFRESH_TOKEN` | OAuth Playground 或 cURL 手動換取                                | 永不過期，換 access token 用         |

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
