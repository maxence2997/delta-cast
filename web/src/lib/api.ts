const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/** Stored JWT token for API authentication. */
let jwtToken: string | null = null;

/** Set the JWT token used for authenticated API calls. */
export function setToken(token: string) {
  jwtToken = token;
  if (typeof window !== "undefined") {
    localStorage.setItem("delta-cast-token", token);
  }
}

/** Get the current JWT token, restoring from localStorage if needed. */
export function getToken(): string | null {
  if (!jwtToken && typeof window !== "undefined") {
    jwtToken = localStorage.getItem("delta-cast-token");
  }
  return jwtToken;
}

/** Clear the stored JWT token. */
export function clearToken() {
  jwtToken = null;
  if (typeof window !== "undefined") {
    localStorage.removeItem("delta-cast-token");
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.message ?? `API error ${res.status}`);
  }

  return res.json() as Promise<T>;
}

// ---- API Types (mirrors server/internal/model/api.go) ----

export interface PrepareResponse {
  sessionId: string;
  state: string;
  message: string;
}

export interface StartResponse {
  sessionId: string;
  agoraAppId: string;
  agoraChannel: string;
  agoraToken: string;
  agoraUid: number;
}

export interface StopResponse {
  sessionId: string;
  state: string;
  message: string;
}

export interface StatusResponse {
  sessionId: string;
  state: string;
  gcpPlaybackUrl?: string;
  youtubeWatchUrl?: string;
}

// ---- API Methods ----

export function prepare(): Promise<PrepareResponse> {
  return request<PrepareResponse>("/v1/live/prepare", { method: "POST" });
}

export function start(): Promise<StartResponse> {
  return request<StartResponse>("/v1/live/start", { method: "POST" });
}

export function stop(): Promise<StopResponse> {
  return request<StopResponse>("/v1/live/stop", { method: "POST" });
}

export function getStatus(): Promise<StatusResponse> {
  return request<StatusResponse>("/v1/live/status");
}
