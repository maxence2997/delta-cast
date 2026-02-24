# Instruction

## Package Structure

```
delta-cast/
├── docs/                # 存放 SPEC.md, 系統架構圖, API 文件
├── server/              # Golang (Orchestrator) 專案
│   ├── cmd/
│   ├── internal/
│   └── go.mod
├── web/                 # 前端 Web 專案 (Next.js / React)
│   ├── src/
│   └── package.json
├── mobile/              # 未來的 Mobile SDK 或 Demo (Flutter/RN/Native)
│   ├── ios/
│   └── android/
├── shared/              # 存放 Proto 定義或統一的轉碼參數 JSON (若有)
├── .github/             # GitHub Actions 自動化腳本
│   └── workflows/
│       ├── deploy-server.yml
|				├── deploy-web.yml
|				├── deploy-ios.yml
│       └── deploy-android.yml
├── env.example           # 環境變數範例文件，列舉所有需要的 API 金鑰與配置項

└── docker-compose.yml   # 本地一次性啟動所有服務進行整合測試