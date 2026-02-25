"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import * as api from "./api";

export type SessionState = "idle" | "preparing" | "ready" | "live" | "stopping";

export interface SessionInfo {
  sessionId: string;
  state: SessionState;
  gcpPlaybackUrl?: string;
  youtubeWatchUrl?: string;
}

/** Hook that manages the live session lifecycle and polls status. */
export function useSession() {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const startPolling = useCallback(() => {
    stopPolling();
    pollRef.current = setInterval(async () => {
      try {
        const status = await api.getStatus();
        setSession({
          sessionId: status.sessionId,
          state: status.state as SessionState,
          gcpPlaybackUrl: status.gcpPlaybackUrl,
          youtubeWatchUrl: status.youtubeWatchUrl,
        });
        // Stop polling once we reach a stable state
        if (status.state === "idle") {
          stopPolling();
          setSession(null);
        }
      } catch {
        // Silently ignore polling errors
      }
    }, 2000);
  }, [stopPolling]);

  useEffect(() => {
    return () => stopPolling();
  }, [stopPolling]);

  const prepare = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const res = await api.prepare();
      setSession({
        sessionId: res.sessionId,
        state: res.state as SessionState,
        gcpPlaybackUrl: undefined,
        youtubeWatchUrl: undefined,
      });
      startPolling();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [startPolling]);

  const startStream = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const res = await api.start();
      setSession((prev) =>
        prev
          ? { ...prev, state: "live" }
          : {
              sessionId: res.sessionId,
              state: "live",
            },
      );
      return res;
    } catch (e) {
      setError((e as Error).message);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const stopStream = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      await api.stop();
      stopPolling();
      setSession(null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [stopPolling]);

  const refreshStatus = useCallback(async () => {
    try {
      const status = await api.getStatus();
      if (status.state === "idle") {
        setSession(null);
      } else {
        setSession({
          sessionId: status.sessionId,
          state: status.state as SessionState,
          gcpPlaybackUrl: status.gcpPlaybackUrl,
          youtubeWatchUrl: status.youtubeWatchUrl,
        });
      }
    } catch {
      // ignore
    }
  }, []);

  return {
    session,
    error,
    loading,
    prepare,
    startStream,
    stopStream,
    refreshStatus,
    startPolling,
    stopPolling,
  };
}
