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
      console.log("[Session] polling stopped");
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const handlePollingError = useCallback(
    (err: unknown) => {
      if (err instanceof api.ApiError && err.status === 401) {
        setError(err.message);
        stopPolling();
      }
    },
    [stopPolling],
  );

  const startPolling = useCallback(
    (continueOnIdle = false) => {
      stopPolling();
      console.log("[Session] polling started");
      pollRef.current = setInterval(async () => {
        try {
          const status = await api.getStatus();
          console.log(`[Poll] state=${status.state}`, status);
          if (status.state === "idle") {
            console.log("[Session] reached idle");
            setSession(null);
            if (!continueOnIdle) {
              console.log("[Session] stopping poll (idle)");
              stopPolling();
            }
          } else {
            setSession({
              sessionId: status.sessionId,
              state: status.state as SessionState,
              gcpPlaybackUrl: status.gcpPlaybackUrl,
              youtubeWatchUrl: status.youtubeWatchUrl,
            });
          }
        } catch (error) {
          handlePollingError(error);
        }
      }, 2000);
    },
    [handlePollingError, stopPolling],
  );

  useEffect(() => {
    return () => stopPolling();
  }, [stopPolling]);

  const prepare = useCallback(async () => {
    console.log("[Session] prepare()");
    setError(null);
    setLoading(true);
    try {
      const res = await api.prepare();
      console.log("[Session] prepare response:", res);
      setSession({
        sessionId: res.sessionId,
        state: res.state as SessionState,
        gcpPlaybackUrl: undefined,
        youtubeWatchUrl: undefined,
      });
      startPolling();
    } catch (e) {
      console.error("[Session] prepare failed:", e);
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [startPolling]);

  const startStream = useCallback(async () => {
    console.log("[Session] startStream()");
    setError(null);
    setLoading(true);
    try {
      const res = await api.start();
      console.log("[Session] startStream response:", res);
      // Do NOT optimistically set state to "live" here.
      // start() only issues an Agora token; session state stays "ready".
      // State transitions to "live" only after Agora fires NCS eventType=103
      // to the backend webhook, which polling will then pick up.
      return res;
    } catch (e) {
      console.error("[Session] startStream failed:", e);
      setError((e as Error).message);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const stopStream = useCallback(async () => {
    console.log("[Session] stopStream()");
    setError(null);
    setLoading(true);
    try {
      await api.stop();
      stopPolling();
      setSession(null);
    } catch (e) {
      console.error("[Session] stopStream failed:", e);
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [stopPolling]);

  /** Cancel an in-progress prepare: stop polling and call stop API. */
  const cancelPrepare = useCallback(async () => {
    console.log("[Session] cancelPrepare()");
    stopPolling();
    setError(null);
    setLoading(true);
    try {
      await api.stop();
      setSession(null);
    } catch (e) {
      console.warn("[Session] cancelPrepare stop error (ignored):", e);
      // Ignore errors — session may not have been fully created yet
      setSession(null);
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
    } catch (error) {
      handlePollingError(error);
    }
  }, [handlePollingError]);

  return {
    session,
    error,
    loading,
    prepare,
    startStream,
    stopStream,
    cancelPrepare,
    refreshStatus,
    startPolling,
    stopPolling,
  };
}
