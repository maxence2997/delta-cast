# Agora 設定

> ⚠️ 此文件不存放真實金鑰。金鑰填入 `server/.env.local`（本機）或 CI/CD Secret。

完成後取得：`AGORA_APP_ID`、`AGORA_APP_CERTIFICATE`、`AGORA_REST_KEY`、`AGORA_REST_SECRET`、`AGORA_CHANNEL_NCS_SECRET`、`AGORA_MEDIA_PUSH_NCS_SECRET`

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

## 1.3 配置 RTC Channel Event Callbacks (NCS)

> **重要**：必須訂閱 **103** 事件，這是觸發 Media Push 的關鍵。

1. 在 Console 左側選單點選 **Notifications (NCS)**。
2. 選擇產品 **RTC**，點選 **Enable**，至少勾選以下事件：
   - **102** — channel destroy（頻道銷毀，用於自動停止）
   - **103** — broadcaster joins channel（**主播加入頻道，觸發 Media Push！**）
   - **104** — user offline（使用者離線，用於自動停止）
3. 設定 Webhook URL：`https://<YOUR_SERVER_DOMAIN>/v1/webhook/agora/channel`
   > 本地開發時可使用 [ngrok](https://ngrok.com/) 產生公開 URL，記得將 ngrok URL 填入此處。
4. 設定完成後會顯示 **NCS Secret**，記錄：
   - **Secret** → 填入 `AGORA_CHANNEL_NCS_SECRET`

---

## 1.4 配置 Media Push Restful API Notifications (NCS)

> 這是第二個獨立的 NCS，用來接收 Media Push（RTMP Converter）狀態回調。

1. 在 Console **Notifications (NCS)** 頁面，選擇產品 **Media Push**，點選 **Enable**。
2. 至少勾選以下事件：
   - **3** — Converter 狀態變更（running / failed）
   - **4** — Converter 已銷毀
3. 設定 Webhook URL：`https://<YOUR_SERVER_DOMAIN>/v1/webhook/agora/media-push`
4. 設定完成後會顯示 **NCS Secret**，記錄：
   - **Secret** → 填入 `AGORA_MEDIA_PUSH_NCS_SECRET`

---

## 1.5 啟用 Media Push (RTMP Converter)

1. 在 Console 的專案頁面，找到 **Extensions / Marketplace**。
2. 搜尋 **Media Push** 並啟用。
3. 無需額外金鑰，此功能透過 REST API 呼叫（已在 1.2 配置）。

---

## 環境變數對應

| 環境變數                      | 取自位置                                  | 備註                                |
| ----------------------------- | ----------------------------------------- | ----------------------------------- |
| `AGORA_APP_ID`                | Project Management → App ID               | 32 字元 hex                         |
| `AGORA_APP_CERTIFICATE`       | Project Management → App Certificate      | 點選 View 後顯示                    |
| `AGORA_REST_KEY`              | RESTful API → Customer ID                 | 即 Basic Auth username              |
| `AGORA_REST_SECRET`           | RESTful API → Customer Secret             | 即 Basic Auth password              |
| `AGORA_CHANNEL_NCS_SECRET`    | Notifications (NCS) → RTC → Secret        | 驗證 `/v1/webhook/agora/channel`    |
| `AGORA_MEDIA_PUSH_NCS_SECRET` | Notifications (NCS) → Media Push → Secret | 驗證 `/v1/webhook/agora/media-push` |

---

## 快速驗證

```bash
# 驗證 REST API 憑證是否正確（回傳 200 即有效）
curl -u "$AGORA_REST_KEY:$AGORA_REST_SECRET" \
  https://api.agora.io/dev/v1/projects
```
