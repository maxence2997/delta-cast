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

### 核心時序流程

1. **Init**: 前端向 Golang 後端發起 `POST /v1/live/start`。
2. **Resource Allocation**:
   可同步執行以下 API 呼叫：
   - 後端呼叫 YouTube API 獲取 Stream Key。
   - 後端呼叫 Google Live Stream API 初始化 Channel 並獲取 RTMP Input URL。

3. **Relay Trigger**: 後端呼叫 Agora REST API (Media Push)，將頻道內的流推送至上述兩個 RTMP 位址。
4. **Response**: 後端回傳 Agora Token 給前端，前端啟動本地採集並推流。
5. **Distribution**:

- GCP 端：Live Stream API 轉碼後存入 GCS，透過 Cloud CDN 發布 HLS (.m3u8)。
- YouTube 端：直接於頻道頁面播放。

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

- **成本控管**: Google Live Stream API 是按時段計費，`stop` 指令必須確保呼叫成功，以免產生冗餘費用。
- **狀態簡單化**: POC 階段不處理斷線重連或複雜的併發狀態，僅以「成功連通」為驗證指標。
- **延遲預期**: HLS 預期延遲約 10-30 秒，YouTube RTMP 預期延遲約 5-10 秒。
