# Project Spec: DeltaCast Live Streaming Relay (2026)

## 1. 專案目標

驗證「一進多出」直播流轉發架構的可行性。直播主透過 Agora 推流至頻道，由後端（Golang）協調將流轉發（Media Push）至 YouTube 與 Google Cloud Live Stream API，最終實現跨平台（Web, YouTube, Mobile）的收視與存檔。

> [!IMPORTANT]
> **POC 限制：單一 Session，無房間概念**
> 本系統目前僅支援**單一活躍 Session**，不具備多房間或多頻道的概念。同一時間只能有一組 `prepare → start → stop` 流程在運作，前端無需也不存在「選擇房間」的邏輯。所有 API 操作均針對這唯一的全域 Session 進行。

---

## 2. 技術棧 (Tech Stack)

- **後端控制中心**: Golang (1.26+)
- **直播基礎設施**: Agora RTC SDK & Media Push (RTMP to CDN)
- **轉碼與分發**: Google Live Stream API, Google Cloud Storage (GCS), Cloud CDN
- **第三方平台**: YouTube Live Data API (RTMP Input)
- **推流端**:
  - Web: Agora Web SDK (name: `agora-rtc-sdk-ng`)
  - Mobile(N2H):
    - iOS: Agora iOS SDK (Swift)
    - Android: Agora Android SDK (Kotlin)
- **展示端**:
  - Web:
    - GCP 來源:
      - 框架: Next.js 16 + Tailwind CSS(簡潔為主)
      - 播放器: video.js (核心) + react-video-js-player (包裝後的 React 元件) + Cloud CDN HLS URL
    - YouTube 來源：react-player(優先) 或 YouTube IFrame Player API(次要)
  - Mobile(N2H):
    - iOS(Swift):
      - GCP 來源：AVPlayer (原生 HLS 支援)。
      - YouTube 來源：YouTubePlayerKit (內嵌式播放)。
    - Android(Kotlin):
      - GCP 來源：Media3 ExoPlayer (Google 官方最新推薦)。
      - YouTube 來源：android-youtube-player。

---

## 3. 系統架構與流程

後端採用 **Proxy/Orchestrator** 模式，前端不直接接觸第三方密鑰，所有指令由後端代理執行。

### 3.1 認證機制

- 所有 API 端點（Webhook 除外）需帶上 `Authorization: Bearer <JWT>` 標頭。
- JWT 使用 HS256 簽發，POC 階段以固定 secret 驗證，不做使用者系統。
- Agora Webhook 端點透過 Agora 簽章驗證（Agora Notification Callback Service 簽章機制）。

### 3.2 Session 管理

- POC 階段僅支援**單一活躍 Session**，後端以 in-memory state 追蹤。
- 若已有活躍 Session，重複呼叫 `POST /v1/live/prepare` 將回傳現有 Session 資訊（不重複建立資源）。
- 若 Session 已為 `live`，重複呼叫 `POST /v1/live/start` 會回傳新 Token，不重複建立資源。
- Session 狀態機：

```mermaid
stateDiagram-v2
    [*] --> idle
    idle --> preparing : POST /v1/live/prepare
    preparing --> ready : GCP + YouTube 資源就緒（非同步，約 30-60s）
    preparing --> idle : 資源分配失敗
    ready --> live : Agora NCS Webhook (eventType=103)
    live --> stopping : POST /v1/live/stop
    stopping --> idle : 清理完成
```

| 狀態        | 說明                                    |
| ----------- | --------------------------------------- |
| `idle`      | 無活躍 Session                          |
| `preparing` | GCP 與 YouTube 資源建立中（背景非同步） |
| `ready`     | 資源就緒，播放 URL 已填入，等待推流     |
| `live`      | 串流進行中，有實際內容                  |
| `stopping`  | 資源清理中                              |

### 3.3 GCP 資源生命週期

GCP Live Stream API 的 Channel 建立需要 **30-60 秒**，為降低開播延遲，採用 **兩階段式預熱（Pre-warm）** 策略：

- **Prepare 階段**：前端呼叫 `POST /v1/live/prepare`，後端非同步建立 GCP Input + Channel 與 YouTube Broadcast。此階段耗時較長，前端可顯示「準備中」狀態。
- **Start 階段**：資源就緒後前端呼叫 `POST /v1/live/start`，後端僅需回傳 Agora Token，前端即可立即開始推流，無需等待資源分配。

資源清理：`POST /v1/live/stop` 時一併刪除 GCP Input + Channel，避免閒置計費。

### 3.4 核心時序流程

#### 開播流程

```mermaid
sequenceDiagram
    actor Streamer as 推流端
    participant API as DeltaCast API
    participant GCP as GCP Live Stream API
    participant YT as YouTube Data API
    participant AgoraRTC as Agora RTC SDK
    participant AgoraMP as Agora Media Push
    participant NCS as Agora NCS (Webhook)

    Streamer->>API: POST /v1/live/prepare (JWT)
    API-->>Streamer: 200 { state: "preparing" }
    Note over API: 背景非同步分配資源（約 30-60 秒）

    par 背景並行資源分配
        API->>GCP: CreateInput → CreateChannel → StartChannel → WaitForChannelReady
        GCP-->>API: inputURI
    and
        API->>YT: CreateBroadcast → CreateStream → BindBroadcastToStream
        YT-->>API: broadcastID, rtmpURL, streamKey
    end

    Note over API: Session state: preparing → ready

    loop 每 2 秒輪詢直到 state = "ready"
        Streamer->>API: GET /v1/live/status (JWT)
        API-->>Streamer: { state: "preparing" | "ready" }
    end

    Streamer->>API: POST /v1/live/start (JWT)
    API-->>Streamer: { agoraToken, agoraChannel, agoraAppId }
    Note over API: Session state 維持 "ready"（start 不改變狀態）

    Streamer->>AgoraRTC: joinChannel(agoraToken) + publish(H.264)
    Note over AgoraRTC: Agora 偵測到主播加入頻道

    AgoraRTC-->>NCS: 觸發 RTC Channel Event (eventType=103)
    NCS->>API: POST /v1/webhook/agora/channel
    Note over API: Session state: ready → live

    par 觸發轉發
        API->>AgoraMP: StartMediaPush → GCP RTMP Input URI
        AgoraMP-->>API: gcpConverterID
    and
        API->>AgoraMP: StartMediaPush → YouTube RTMP URL+Key
        AgoraMP-->>API: ytConverterID
    end

    API->>YT: TransitionBroadcast("live")

    Note over GCP: RTMP → 轉碼 → HLS → GCS → Cloud CDN
    Note over YT: YouTube 直播開始播放
```

#### 收播端流程

收播端不需要呼叫 `prepare` / `start` / `stop`，只需輪詢 `GET /v1/live/status`。因為 POC 是單一 Session，不需要房間選擇邏輯。

`gcpPlaybackUrl` 與 `youtubeWatchUrl` 從 `ready` 狀態起即填入，但只有 `live` 狀態才有實際串流內容。各狀態下的欄位可用性詳見 [`doc/api.md`](api.md) 的 `GET /v1/live/status` 章節。

```mermaid
sequenceDiagram
    actor Audience as 收播端
    participant API as DeltaCast API
    participant CDN as Cloud CDN (HLS)
    participant YT as YouTube

    Note over Audience: 頁面載入後開始輪詢
    loop 每 2 秒，直到 state = "live"
        Audience->>API: GET /v1/live/status (JWT)
        API-->>Audience: { state, gcpPlaybackUrl, youtubeWatchUrl }
    end

    Note over Audience: state = "live" → 取用播放 URL
    Audience->>CDN: HLS 播放（gcpPlaybackUrl → *.m3u8）延遲約 10-30 秒
    Audience->>YT: YouTube 嵌入播放（youtubeWatchUrl）延遲約 5-10 秒

    loop 持續輪詢偵測關播
        Audience->>API: GET /v1/live/status (JWT)
        API-->>Audience: { state: "idle" }
    end
    Note over Audience: state = "idle" → 顯示「無直播」
```

#### 關播流程

```mermaid
sequenceDiagram
    actor Streamer as 推流端
    participant API as DeltaCast API
    participant AgoraMP as Agora Media Push
    participant YT as YouTube Data API
    participant GCP as GCP Live Stream API

    Streamer->>API: POST /v1/live/stop (JWT)
    Note over API: Session state → "stopping"

    API->>AgoraMP: StopMediaPush(gcpConverterID)
    API->>AgoraMP: StopMediaPush(ytConverterID)
    API->>YT: TransitionBroadcast("complete")
    API->>GCP: StopChannel
    API->>GCP: DeleteChannel
    API->>GCP: DeleteInput

    Note over API: Session state → "idle"
    API-->>Streamer: { state: "idle", message: "session stopped..." }
```

> **容錯**：Stop 流程每一步驟失敗只 log，不中斷後續清理，確保 GCP 資源完整釋放（GCP 按時段計費）。

### 3.5 API 端點總覽

| Method | Path                           | 說明                                              |
| ------ | ------------------------------ | ------------------------------------------------- |
| POST   | `/v1/live/prepare`             | 預熱資源（GCP + YouTube），回傳 Session 與狀態    |
| POST   | `/v1/live/start`               | 回傳 Agora Token（不改變 Session 狀態）           |
| POST   | `/v1/live/stop`                | 關播並釋放所有資源                                |
| GET    | `/v1/live/status`              | 查詢當前 Session 狀態與播放 URL                   |
| POST   | `/v1/webhook/agora/channel`    | Agora RTC Channel NCS（無需 JWT，Agora 簽章驗證） |
| POST   | `/v1/webhook/agora/media-push` | Agora Media Push NCS（無需 JWT，Agora 簽章驗證）  |

詳細 request/response 規格見 [`doc/api.md`](api.md)。

---

## 4. Agora Media Push 轉碼模式設定

### 4.1 無轉碼直推模式（預設，POC 建議）

Agora Media Push 以 **raw relay** 模式運作，不對串流重新編碼，直接將 Agora 頻道的原始音視訊封包推送至 RTMP 目標（GCP 與 YouTube）。由於 GCP Live Stream API 與 YouTube 均可直接接收符合規格的 RTMP 串流，省去轉碼步驟可大幅降低 Agora 費用。

| 項目         | 說明                                                                             |
| ------------ | -------------------------------------------------------------------------------- |
| **費用**     | 僅 Agora Media Push 傳輸費，不計轉碼費用                                         |
| **前提條件** | 推流端（Agora RTC SDK）需輸出 GCP 與 YouTube 可接受的格式（H.264/AAC RTMP）      |
| **啟用方式** | 預設即為此模式，無需設定任何環境變數（`AGORA_TRANSCODING_ENABLED` 預設 `false`） |

### 4.2 轉碼模式（可選）

設定環境變數 `AGORA_TRANSCODING_ENABLED=true` 可切換為轉碼模式。Agora 會在推送前對串流執行 H.264/AAC 重新編碼，確保輸出格式標準化。適合推流端輸出格式不確定或需要統一規格的場景。

| 參數項目           | 設定值                               |
| ------------------ | ------------------------------------ |
| **編碼格式**       | H.264 (Video), AAC (Audio)           |
| **解析度**         | 1280 x 720 (720p)                    |
| **幀率 (FPS)**     | 30 fps                               |
| **碼率 (Bitrate)** | 2500 kbps (Video) + 128 kbps (Audio) |
| **關鍵幀間隔**     | 2 seconds                            |
| **啟用方式**       | `AGORA_TRANSCODING_ENABLED=true`     |

### 4.3 未來升級參考配置（1080p）

> **注意**：升級至 1080p 前，請評估 Agora Media Push 傳輸費用與 GCP Live Stream API 轉碼費用的增加，以及觀眾端頻寬需求。建議架構驗證穩定後再執行升級。

| 參數項目           | 設定值                           | 備註                              |
| ------------------ | -------------------------------- | --------------------------------- |
| **編碼格式**       | H.264 (Video), AAC (Audio)       | 不變                              |
| **解析度**         | 1920 x 1080 (1080p)              | 從 1280x720 調整                  |
| **幀率 (FPS)**     | 30 fps                           | 不變                              |
| **碼率 (Bitrate)** | 4500 - 6000 kbps                 | YouTube 官方 1080p/30fps 建議範圍 |
| **關鍵幀間隔**     | 2 seconds                        | 不變                              |
| **播放協議**       | HLS (GCP 輸出) / RTMP (轉發輸入) | 不變                              |

**升級時須同步修改的位置：**

- `server/internal/provider/agora_media_push.go`：更新轉碼模式的 `videoOptions` 解析度與碼率參數（`width`、`height`、`bitrate`）。
- `doc/spec.md`（本檔）：將 4.2 節的轉碼設定替換為 1080p 數值，並移除本節。

---

## 5. 注意事項

- **成本控管**: Google Live Stream API 是按時段計費，Stop 流程每一步驟失敗需 log 但不中斷後續清理，確保資源完整釋放。
- **Session 單一性**: POC 階段僅支援單一活躍 Session，重複呼叫 Start 回傳現有 Session，不重複建立資源。
- **狀態簡單化**: POC 階段不處理斷線重連或複雜的併發狀態，僅以「成功連通」為驗證指標。
- **延遲預期**: HLS 預期延遲約 10-30 秒，YouTube RTMP 預期延遲約 5-10 秒。
- **認證**: 所有客戶端請求使用 JWT Bearer Token（HS256），Webhook 使用 Agora 簽章驗證。
- **Webhook 可靠性**: Agora NCS 可能重複發送事件，後端需做冪等處理（以 Session 狀態判斷是否已處理過）。
