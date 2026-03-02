import { NextResponse } from "next/server";

/**
 * GET /api/token
 *
 * 在 server-side 讀取 API_TOKEN 環境變數並回傳。
 * 此端點受 Cloudflare Zero Trust 保護，只有通過驗證的使用者才能存取。
 * API_TOKEN 不會出現在 client-side bundle 中。
 */
export async function GET() {
  const token = process.env.API_TOKEN;

  if (!token) {
    return NextResponse.json(
      { error: "API_TOKEN is not configured" },
      { status: 500 },
    );
  }

  return NextResponse.json({ token });
}
