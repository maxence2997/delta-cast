"use client";

import { useEffect } from "react";
import { setToken, getToken } from "@/lib/api";

/**
 * 在 mount 時自動從 /api/token 取得 API Token 並儲存至 localStorage。
 * 不渲染任何 UI，純粹作為 token 初始化的機制。
 * 前端存取受 Cloudflare Zero Trust 保護，無需手動輸入。
 */
export default function AutoTokenInit() {
  useEffect(() => {
    // 若已有 token 則不重複請求
    if (getToken()) return;

    fetch("/api/token")
      .then((res) => res.json())
      .then(({ token }) => {
        if (token) setToken(token);
      })
      .catch((err) => {
        console.error("[AutoTokenInit] 無法取得 API Token:", err);
      });
  }, []);

  return null;
}
