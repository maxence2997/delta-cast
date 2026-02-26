# Agora 設定

> ⚠️ 此文件不存放真實金鑰。金鑰填入 `server/.env.local`（本機）或 CI/CD Secret。

完成後取得：`AGORA_APP_ID`、`AGORA_APP_CERTIFICATE`、`AGORA_REST_KEY`、`AGORA_REST_SECRET`、`AGORA_NCS_SECRET`

---

## 1.1 建立專案

1. 前往 [Agora Console](https://console.agora.io/) 並登入（或註冊帳號）。
2. 點選 **Project Management** → **Create**。
3. 輸入專案名稱（如 `DeltaCast`），Authentication Mechanism 選擇 **Secured mode: APP ID + Token**。
4. 建立完成後，記錄：
   - **App ID** → 填入 `AGORA_APP_ID`
   - **App Certificate** → 填入 `AGORA_APP_CERTIFICATE`

---

## 1.2 啟用 REST API

1. 在 Console 左側選單點選 **RESTful API**。
2. 點選 **Add Secret**，生成一組 Customer ID / Customer Secret。
3. 記錄：
   - **Customer ID** → 填入 `AGORA_REST_KEY`
   - **Customer Secret** → 填入 `AGORA_REST_SECRET`

---

## 1.3 配置 Notification Callback Service (NCS)

1. 在 Console 左側選單點選 **Notifications (NCS)**。
2. 點選 **Enable**，至少勾選事件：
   - **101** — channel create（有使用者加入頻道）
   - **102** — channel destroy（頻道銷毀）
3. 設定 Webhook URL：`https://<YOUR_SERVER_DOMAIN>/v1/webhook/agora`
   > 本地開發時可使用 [ngrok](https://ngrok.com/) 產生公開 URL。
4. 設定完成後會顯示 **NCS Secret**，記錄：
   - **NCS Secret** → 填入 `AGORA_NCS_SECRET`

---

## 1.4 啟用 Media Push (RTMP Converter)

1. 在 Console 的專案頁面，找到 **Extensions / Marketplace**。
2. 搜尋 **Media Push** 並啟用。
3. 無需額外金鑰，此功能透過 REST API 呼叫（已在 1.2 配置）。

---

## 環境變數對應

| 環境變數                | 取自位置                             | 備註                   |
| ----------------------- | ------------------------------------ | ---------------------- |
| `AGORA_APP_ID`          | Project Management → App ID          | 32 字元 hex            |
| `AGORA_APP_CERTIFICATE` | Project Management → App Certificate | 點選 View 後顯示       |
| `AGORA_REST_KEY`        | RESTful API → Customer ID            | 即 Basic Auth username |
| `AGORA_REST_SECRET`     | RESTful API → Customer Secret        | 即 Basic Auth password |
| `AGORA_NCS_SECRET`      | Notifications (NCS) → Secret         | 用於驗證 Webhook HMAC  |

---

## 快速驗證

```bash
# 驗證 REST API 憑證是否正確（回傳 200 即有效）
curl -u "$AGORA_REST_KEY:$AGORA_REST_SECRET" \
  https://api.agora.io/dev/v1/projects
```
