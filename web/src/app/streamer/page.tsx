"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type {
  IAgoraRTCClient,
  ICameraVideoTrack,
  IMicrophoneAudioTrack,
} from "agora-rtc-sdk-ng";
import { useSession } from "@/lib/use-session";
import StatusBadge from "@/components/status-badge";

/** Lazily import Agora SDK to avoid `window is not defined` during SSR. */
async function getAgoraRTC() {
  const mod = await import("agora-rtc-sdk-ng");
  return mod.default;
}

export default function StreamerPage() {
  const {
    session,
    error,
    loading,
    prepare,
    startStream,
    stopStream,
    cancelPrepare,
  } = useSession();

  const hostClientRef = useRef<IAgoraRTCClient | null>(null);
  const videoTrackRef = useRef<ICameraVideoTrack | null>(null);
  const audioTrackRef = useRef<IMicrophoneAudioTrack | null>(null);
  const videoContainerRef = useRef<HTMLDivElement>(null);

  const [localPreview, setLocalPreview] = useState(false);
  const [joined, setJoined] = useState(false);
  const [cameraEnabled, setCameraEnabled] = useState(true);
  const [micEnabled, setMicEnabled] = useState(true);

  // Preview local camera
  const startPreview = useCallback(async () => {
    try {
      const AgoraRTC = await getAgoraRTC();
      const videoTrack = await AgoraRTC.createCameraVideoTrack();
      const audioTrack = await AgoraRTC.createMicrophoneAudioTrack();
      videoTrackRef.current = videoTrack;
      audioTrackRef.current = audioTrack;

      if (videoContainerRef.current) {
        videoTrack.play(videoContainerRef.current);
      }
      setCameraEnabled(true);
      setMicEnabled(true);
      setLocalPreview(true);
    } catch (err) {
      console.error("Failed to start preview:", err);
    }
  }, []);

  const toggleCamera = useCallback(async () => {
    if (!videoTrackRef.current) return;
    const next = !cameraEnabled;
    await videoTrackRef.current.setEnabled(next);
    setCameraEnabled(next);
    console.log(`[Device] camera ${next ? "enabled" : "disabled"}`);
  }, [cameraEnabled]);

  const toggleMic = useCallback(async () => {
    if (!audioTrackRef.current) return;
    const next = !micEnabled;
    await audioTrackRef.current.setEnabled(next);
    setMicEnabled(next);
    console.log(`[Device] mic ${next ? "enabled" : "disabled"}`);
  }, [micEnabled]);

  // Join Agora channel and publish tracks
  const joinAndPublish = useCallback(
    async (appId: string, channel: string, token: string, uid: number) => {
      const AgoraRTC = await getAgoraRTC();
      const hostClient = AgoraRTC.createClient({
        mode: "live",
        codec: "h264",
        role: "host",
      });
      hostClientRef.current = hostClient;

      await hostClient.join(appId, channel, token, uid);

      if (videoTrackRef.current && audioTrackRef.current) {
        await hostClient.publish([
          videoTrackRef.current,
          audioTrackRef.current,
        ]);
      }
      setJoined(true);
    },
    [],
  );

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      videoTrackRef.current?.close();
      audioTrackRef.current?.close();
      hostClientRef.current?.leave();
    };
  }, []);

  const handlePrepare = async () => {
    if (!localPreview) {
      await startPreview();
    }
    await prepare();
  };

  const handleStart = async () => {
    const res = await startStream();
    if (res) {
      await joinAndPublish(
        res.agoraAppId,
        res.agoraChannel,
        res.agoraToken,
        res.agoraUid,
      );
    }
  };

  const handleStop = async () => {
    videoTrackRef.current?.close();
    audioTrackRef.current?.close();
    await hostClientRef.current?.leave();
    videoTrackRef.current = null;
    audioTrackRef.current = null;
    hostClientRef.current = null;
    setJoined(false);
    setLocalPreview(false);
    setCameraEnabled(true);
    setMicEnabled(true);
    await stopStream();
  };

  const handleCancel = async () => {
    videoTrackRef.current?.close();
    audioTrackRef.current?.close();
    videoTrackRef.current = null;
    audioTrackRef.current = null;
    await cancelPrepare();
    setLocalPreview(false);
    setCameraEnabled(true);
    setMicEnabled(true);
  };

  const state = session?.state ?? "idle";

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Streamer</h1>
        <StatusBadge state={state} />
      </div>

      {/* Video preview */}
      <div
        ref={videoContainerRef}
        className="w-full aspect-video bg-black rounded-lg overflow-hidden"
      />

      {/* Device controls */}
      <div className="flex gap-3">
        {/* Camera */}
        <button
          onClick={toggleCamera}
          disabled={!localPreview}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg border font-medium text-sm transition disabled:opacity-40 disabled:cursor-not-allowed ${
            cameraEnabled
              ? "border-gray-300 bg-white text-gray-800 hover:bg-gray-100"
              : "border-red-400 bg-red-50 text-red-700 hover:bg-red-100"
          }`}
        >
          {/* camera icon */}
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
          >
            {cameraEnabled ? (
              <>
                <path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z" />
                <circle cx="12" cy="13" r="3" />
              </>
            ) : (
              <>
                <line x1="1" y1="1" x2="23" y2="23" />
                <path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z" />
                <circle cx="12" cy="13" r="3" />
              </>
            )}
          </svg>
          攝影機：{cameraEnabled ? "開啟" : "關閉"}
        </button>

        {/* Microphone */}
        <button
          onClick={toggleMic}
          disabled={!localPreview}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg border font-medium text-sm transition disabled:opacity-40 disabled:cursor-not-allowed ${
            micEnabled
              ? "border-gray-300 bg-white text-gray-800 hover:bg-gray-100"
              : "border-red-400 bg-red-50 text-red-700 hover:bg-red-100"
          }`}
        >
          {/* mic icon */}
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
          >
            {micEnabled ? (
              <>
                <rect x="9" y="2" width="6" height="13" rx="3" />
                <path d="M5 10a7 7 0 0 0 14 0" />
                <line x1="12" y1="19" x2="12" y2="23" />
                <line x1="8" y1="23" x2="16" y2="23" />
              </>
            ) : (
              <>
                <line x1="1" y1="1" x2="23" y2="23" />
                <rect x="9" y="2" width="6" height="13" rx="3" />
                <path d="M5 10a7 7 0 0 0 14 0" />
                <line x1="12" y1="19" x2="12" y2="23" />
                <line x1="8" y1="23" x2="16" y2="23" />
              </>
            )}
          </svg>
          麥克風：{micEnabled ? "開啟" : "關閉"}
        </button>
      </div>

      {/* Preparing banner */}
      {state === "preparing" && (
        <div className="rounded-lg border border-yellow-300 bg-yellow-50 p-6 flex flex-col items-center gap-3 text-center">
          <div className="h-10 w-10 rounded-full border-4 border-yellow-400 border-t-transparent animate-spin" />
          <p className="text-xl font-bold text-yellow-800">準備資源中</p>
          <p className="text-sm text-yellow-700">
            正在佈建 GCP &amp; YouTube 資源，請稍候約 30–60 秒。
          </p>
          <button
            onClick={handleCancel}
            disabled={loading}
            className="mt-2 px-5 py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 disabled:opacity-50 transition font-medium"
          >
            {loading ? "取消中..." : "取消準備"}
          </button>
        </div>
      )}

      {/* Controls */}
      <div className="flex gap-3">
        {state === "idle" && (
          <button
            onClick={handlePrepare}
            disabled={loading}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 transition"
          >
            {loading ? "Preparing..." : "Prepare"}
          </button>
        )}

        {state === "ready" && (
          <button
            onClick={handleStart}
            disabled={loading}
            className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 transition"
          >
            {loading ? "Starting..." : "Go Live"}
          </button>
        )}

        {(state === "live" || state === "ready") && (
          <button
            onClick={handleStop}
            disabled={loading}
            className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 transition"
          >
            {loading ? "Stopping..." : "Stop"}
          </button>
        )}
      </div>

      {joined && (
        <p className="text-sm text-green-600">
          You are live! Streaming to YouTube &amp; Cloud CDN.
        </p>
      )}

      {error && <p className="text-sm text-red-600">Error: {error}</p>}
    </div>
  );
}
