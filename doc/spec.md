# Project Spec: DeltaCast Live Streaming Relay (2026)

## 1. 專案目標

驗證「一進多出」直播流轉發架構的可行性。直播主透過 Agora 推流至頻道，由後端（Golang）協調將流轉發（Media Push）至 YouTube 與 Google Cloud Live Stream API，最終實現跨平台（Web, YouTube, Mobile）的收視與存檔。

---

## 2. 技術棧 (Tech Stack)

- **後端控制中心**: Golang (1.24+)
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
- 若已有活躍 Session，重複呼叫 `POST /v1/live/start` 將回傳現有 Session 資訊（不重複建立資源）。
- Session 狀態機：`idle` → `preparing` → `ready` → `live` → `stopping` → `idle`。

### 3.3 GCP 資源生命週期

GCP Live Stream API 的 Channel 建立需要 **30-60 秒**，為降低開播延遲，採用 **兩階段式預熱（Pre-warm）** 策略：

- **Prepare 階段**：前端呼叫 `POST /v1/live/prepare`，後端非同步建立 GCP Input + Channel 與 YouTube Broadcast。此階段耗時較長，前端可顯示「準備中」狀態。
- **Start 階段**：資源就緒後前端呼叫 `POST /v1/live/start`，後端僅需回傳 Agora Token，前端即可立即開始推流，無需等待資源分配。

資源清理：`POST /v1/live/stop` 時一併刪除 GCP Input + Channel，避免閒置計費。

### 3.4 核心時序流程

#### 開播流程 (Start)

1. **Prepare**: 前端呼叫 `POST /v1/live/prepare`。
2. **Resource Allocation**: 後端並行執行：
   - 呼叫 Google Live Stream API 建立 Input + Channel 並獲取 RTMP Input URL。
   - 呼叫 YouTube Data API 建立 Broadcast + Stream 並獲取 Stream Key。
3. **Ready**: 資源就緒後，Session 狀態轉為 `ready`。前端可透過 `GET /v1/live/status` 輪詢或 prepare 回應中得知。
4. **Start**: 前端呼叫 `POST /v1/live/start`，後端回傳 Agora Token，前端加入頻道並開始推流。
5. **Webhook**: Agora Notification Callback Service 發送 **channel create (101)** 事件至 `POST /v1/webhook/agora`，後端確認有流進入。
6. **Relay Trigger**: 後端收到 Webhook 後，呼叫 Agora Media Push REST API，將頻道內的流推送至 YouTube RTMP + GCP RTMP 兩個目標位址。
7. **Distribution**:
   - GCP 端：Live Stream API 轉碼後存入 GCS，透過 Cloud CDN 發布 HLS (.m3u8)。
   - YouTube 端：直接於頻道頁面播放。

#### 關播流程 (Stop)

1. **Init**: 前端呼叫 `POST /v1/live/stop`。
2. **停止轉發**: 後端停止 Agora Media Push（兩個 RTMP 目標）。
3. **停止 YouTube**: 呼叫 YouTube API 將 Broadcast 狀態轉為 `complete`。
4. **停止 GCP**: 停止 GCP Live Stream Channel，刪除 Channel + Input 資源。
5. **清理**: Session 狀態回到 `idle`。
6. **容錯**: 每一步驟若失敗，記錄錯誤但繼續執行後續步驟，確保資源盡可能被完整釋放。

### 3.5 API 端點總覽

| Method | Path                | 說明                                                    |
| ------ | ------------------- | ------------------------------------------------------- |
| POST   | `/v1/live/prepare`  | 預熱資源（GCP + YouTube），回傳 Session 與狀態          |
| POST   | `/v1/live/start`    | 開始推流，回傳 Agora Token                              |
| POST   | `/v1/live/stop`     | 關播並釋放所有資源                                      |
| GET    | `/v1/live/status`   | 查詢當前 Session 狀態與各平台就緒情形                   |
| POST   | `/v1/webhook/agora` | Agora NCS Webhook 接收端（無需 JWT，用 Agora 簽章驗證） |

---

## 4. 統一轉碼配置 (Best Practice)

為確保跨平台相容性與延遲控制，所有推流節點必須統一採用以下參數：

| 參數項目           | 設定值                           |
| ------------------ | -------------------------------- |
| **編碼格式**       | H.264 (Video), AAC (Audio)       |
| **解析度**         | 1280 x 720 (720p)                |
| **幀率 (FPS)**     | 30 fps                           |
| **碼率 (Bitrate)** | 2500 - 3000 kbps                 |
| **關鍵幀間隔**     | 2 seconds                        |
| **播放協議**       | HLS (GCP 輸出) / RTMP (轉發輸入) |

---

## 5. 注意事項

- **成本控管**: Google Live Stream API 是按時段計費，Stop 流程每一步驟失敗需 log 但不中斷後續清理，確保資源完整釋放。
- **Session 單一性**: POC 階段僅支援單一活躍 Session，重複呼叫 Start 回傳現有 Session，不重複建立資源。
- **狀態簡單化**: POC 階段不處理斷線重連或複雜的併發狀態，僅以「成功連通」為驗證指標。
- **延遲預期**: HLS 預期延遲約 10-30 秒，YouTube RTMP 預期延遲約 5-10 秒。
- **認證**: 所有客戶端請求使用 JWT Bearer Token（HS256），Webhook 使用 Agora 簽章驗證。
- **Webhook 可靠性**: Agora NCS 可能重複發送事件，後端需做冪等處理（以 Session 狀態判斷是否已處理過）。
