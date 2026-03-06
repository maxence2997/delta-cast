# DeltaCast API 文件

> 系統架構、Session 狀態機與完整流程圖見 [`doc/spec.md`](../spec.md) 第 3 節。

---

## 目錄

- [DeltaCast API 文件](#deltacast-api-文件)
  - [目錄](#目錄)
  - [認證](#認證)
  - [API 端點](#api-端點)
    - [POST `/v1/live/prepare`](#post-v1liveprepare)
    - [POST `/v1/live/start`](#post-v1livestart)
    - [POST `/v1/live/stop`](#post-v1livestop)
    - [GET `/v1/live/status`](#get-v1livestatus)
    - [POST `/v1/webhook/agora/channel`](#post-v1webhookagorachannel)
    - [POST `/v1/webhook/agora/media-push`](#post-v1webhookagoramedia-push)

> Session 資料模型、Agora Media Push 模式設定與系統注意事項請參閱 [`doc/spec.md`](../spec.md)。

---

## 認證

除 Webhook 端點外，所有 API 請求必須在 Header 帶上 JWT Bearer Token：

```
Authorization: Bearer <JWT>
```

JWT 使用 **HS256** 演算法簽發，secret 對應環境變數 `JWT_SECRET`。  
JWT 必須包含 `"iss": "delta-cast"` claim，缺少此 claim 的 Token 將以 `401 Unauthorized` 拒絕，即使簽名正確。  
Webhook 端點不需要 JWT，改用 **Agora HMAC/SHA1 簽章驗證**（`Agora-Signature` Header）。

---

## API 端點

### POST `/v1/live/prepare`

預熱 GCP 與 YouTube 資源。立即回傳，資源分配在背景非同步執行（約 30–60 秒）。

**認證**：JWT Bearer Token 必填

**Request Body**：無

**Response `200 OK`** — 新建 Session：

```json
{
  "sessionId": "a1b2c3d4",
  "state": "preparing",
  "message": "resource allocation started, poll /v1/live/status for updates"
}
```

**Response `200 OK`** — Session 已存在（非 `idle`），回傳現有狀態：

```json
{
  "sessionId": "a1b2c3d4",
  "state": "ready",
  "message": "session already exists"
}
```

**Response `500 Internal Server Error`**：

```json
{
  "error": "prepare_failed",
  "message": "<錯誤描述>"
}
```

**後台非同步行為**

並行執行以下兩個 goroutine：

| 任務    | 步驟                                                                     |
| ------- | ------------------------------------------------------------------------ |
| GCP     | `CreateInput` → `CreateChannel` → `StartChannel` → `WaitForChannelReady` |
| YouTube | `CreateBroadcast` → `CreateStream` → `BindBroadcastToStream`             |

兩者皆成功 → Session 狀態轉為 `ready`；任一失敗 → Session 狀態回到 `idle`，需重新呼叫 prepare。

---

### POST `/v1/live/start`

取得 Agora Token。Session 必須為 `ready` 狀態（透過 `GET /v1/live/status` 輪詢確認後呼叫）。

> **重要**：此端點**不改變** Session 狀態。呼叫後 Session 仍維持 `ready`，前端使用取得的 Token 加入 Agora 頻道並推流，Agora 偵測到主播加入後才會透過 NCS Webhook（`eventType=103`）通知後端，後端才將狀態轉為 `live`。

**認證**：JWT Bearer Token 必填

**Request Body**：無

**Response `200 OK`**：

```json
{
  "sessionId": "a1b2c3d4",
  "agoraAppId": "your-agora-app-id",
  "agoraChannel": "deltacast-a1b2c3d4",
  "agoraToken": "<Dynamic RTC Token>",
  "agoraUid": 0
}
```

**Response `400 Bad Request`** — Session 狀態不符：

```json
{
  "error": "start_failed",
  "message": "session is in preparing state, must be ready to start"
}
```

**行為說明**：

- 僅產生 Agora RTC Token，不分配任何 GCP/YouTube 資源，**不改變 Session 狀態**
- 若 Session 已為 `live`，重複呼叫會回傳新 Token（不重複建立資源）
- Token TTL：86400 秒（24 小時）
- `agoraUid` 固定為 `0`（由 Agora 自動分配）

---

### POST `/v1/live/stop`

停止直播並依序釋放所有資源。各步驟失敗只 log，不中斷後續清理。

**認證**：JWT Bearer Token 必填

**Request Body**：無

**Response `200 OK`** — 正常停止：

```json
{
  "sessionId": "a1b2c3d4",
  "state": "idle",
  "message": "session stopped, all resources cleaned up"
}
```

**Response `200 OK`** — 無活躍 Session（state 為 `idle`）：

```json
{
  "sessionId": "",
  "state": "idle",
  "message": "no active session"
}
```

**Response `200 OK`** — Teardown 已在進行中（state 為 `stopping`）：

```json
{
  "sessionId": "",
  "state": "stopping",
  "message": "no active session"
}
```

> 呼叫端收到 `state: "stopping"` 時應繼續輪詢 `GET /v1/live/status`，直到 state 轉為 `idle`。

**Response `500 Internal Server Error`**：

```json
{
  "error": "stop_failed",
  "message": "<錯誤描述>"
}
```

**清理順序（每步失敗皆 log 並繼續）**：

| 步驟 | 動作                                            |
| ---- | ----------------------------------------------- |
| 1    | 停止 Agora Media Push Converter（GCP 目標）     |
| 2    | 停止 Agora Media Push Converter（YouTube 目標） |
| 3    | YouTube Broadcast 轉為 `complete`               |
| 4    | 停止 GCP Channel                                |
| 5    | 刪除 GCP Channel                                |
| 6    | 刪除 GCP Input                                  |

> 六步驟完成後 Session 狀態重設為 `idle`。各步驟皆有 guard 保護（資源 ID 為空則跳過），可安全用於任何非 idle/stopping 狀態。

---

### GET `/v1/live/status`

查詢當前 Session 狀態與播放 URL。

**認證**：JWT Bearer Token 必填

**Response `200 OK`**：

```json
{
  "sessionId": "a1b2c3d4",
  "state": "live",
  "gcpPlaybackUrl": "https://<cdn-domain>/channel-a1b2c3d4/main.m3u8",
  "youtubeWatchUrl": "https://www.youtube.com/watch?v=<broadcastId>"
}
```

**各狀態下欄位可用性**：

| 狀態        | `gcpPlaybackUrl`  | `youtubeWatchUrl` | 有實際內容？             |
| ----------- | ----------------- | ----------------- | ------------------------ |
| `idle`      | 省略（omitempty） | 省略（omitempty） | 否                       |
| `preparing` | 省略（omitempty） | 省略（omitempty） | 否                       |
| `ready`     | ✓ 已填入          | ✓ 已填入          | 否（資源就緒但尚未推流） |
| `live`      | ✓ 已填入          | ✓ 已填入          | **是**                   |
| `stopping`  | ✓ 已填入          | ✓ 已填入          | 停止中                   |

> 兩個 URL 欄位使用 `omitempty`，在 `idle` / `preparing` 狀態下不會出現在回應中，前端應以 optional（`?`）型別處理。

> 收播端只需輪詢此端點，在 `state === "live"` 時取用兩條 URL 即可。因為 POC 單一 Session，不需要額外的房間選擇邏輯。

---

### POST `/v1/webhook/agora/channel`

接收 Agora RTC Channel 事件（NCS，productId=1）。

**認證**：無 JWT；使用 `Agora-Signature` Header 進行 HMAC/SHA1 驗證（對應 `AGORA_CHANNEL_NCS_SECRET`）

**Request Headers**：

```
Agora-Signature: <hmac-sha1-hex>
Content-Type: application/json
```

**Request Body**：

```json
{
  "noticeId": "abc123",
  "productId": 1,
  "eventType": 103,
  "payload": {
    "uid": 12345,
    "channelName": "deltacast-a1b2c3d4",
    "clientSeq": 1
  }
}
```

| Payload 欄位  | 說明                                                                                                |
| ------------- | --------------------------------------------------------------------------------------------------- |
| `uid`         | Agora RTC UID                                                                                       |
| `channelName` | Agora 頻道名稱，用於驗證事件是否屬於當前 Session 的頻道                                             |
| `clientSeq`   | 主播端序號；`> 0` 為真實主播事件；`== 0` 為 Media Push bot 的假離開事件（eventType 104 時忽略此類） |

**Response `200 OK`**：

```json
{ "status": "ok" }
```

**Response `401 Unauthorized`** — 簽章驗證失敗：

```json
{
  "error": "unauthorized",
  "message": "invalid webhook signature"
}
```

**處理邏輯**：

- **eventType 103**（主播加入頻道）：Session 為 `ready` 時，觸發 Agora Media Push 轉發至 GCP RTMP + YouTube RTMP，Session 狀態轉為 `live`；YouTube Broadcast 透過建立時設定的 `enableAutoStart=true` 自動 transition，無需顯式呼叫
- **eventType 102**（頻道銀毀）：Session 為 `live` 時，觸發自動關播（同 Stop 流程）
- **eventType 104**（主播離開頻道）：Session 為 `live` 且 `payload.clientSeq > 0` 時，觸發自動關播；`clientSeq == 0` 為 Media Push bot 報到的假離開事件，忽略
- 其他 eventType 直接忽略並回傳 `200`
- **冪等保護**：以 Session 狀態作為 guard，已處理過的事件不重覆執行

**RTC Channel eventType 對照**：

| eventType | 說明                                           |
| --------- | ---------------------------------------------- |
| 101       | 頻道建立                                       |
| **102**   | **頻道銀毀（Session live 時觸發自動關播）**    |
| **103**   | **主播加入頻道（觸發 Media Push）**            |
| **104**   | **主播離開頻道（clientSeq>0 時觸發自動關播）** |

---

### POST `/v1/webhook/agora/media-push`

接收 Agora Media Push 事件（NCS，productId=5）。

**認證**：無 JWT；使用 `Agora-Signature` Header 進行 HMAC/SHA1 驗證（對應 `AGORA_MEDIA_PUSH_NCS_SECRET`）

**Request Headers**：

```
Agora-Signature: <hmac-sha1-hex>
Content-Type: application/json
```

**Request Body**：

```json
{
  "noticeId": "abc123",
  "productId": 5,
  "eventType": 3,
  "payload": {
    "converter": {
      "id": "converter-id-xxx",
      "state": "running"
    },
    "destroyReason": ""
  }
}
```

**Response `200 OK`**：

```json
{ "status": "ok" }
```

**Media Push eventType 對照**：

| eventType | 說明                                       |
| --------- | ------------------------------------------ |
| 1         | Converter 已建立                           |
| 2         | Converter 設定變更                         |
| 3         | Converter 狀態變更（`running` / `failed`） |
| 4         | Converter 已銷毀（`destroyReason` 填原因） |

> 目前僅做 log，不觸發額外狀態變更。

