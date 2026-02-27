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
  const { session, error, loading, prepare, startStream, stopStream } =
    useSession();

  const hostClientRef = useRef<IAgoraRTCClient | null>(null);
  const videoTrackRef = useRef<ICameraVideoTrack | null>(null);
  const audioTrackRef = useRef<IMicrophoneAudioTrack | null>(null);
  const videoContainerRef = useRef<HTMLDivElement>(null);

  const [localPreview, setLocalPreview] = useState(false);
  const [joined, setJoined] = useState(false);

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
      setLocalPreview(true);
    } catch (err) {
      console.error("Failed to start preview:", err);
    }
  }, []);

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
    await stopStream();
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

        {(state === "live" || state === "preparing" || state === "ready") && (
          <button
            onClick={handleStop}
            disabled={loading}
            className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 transition"
          >
            {loading ? "Stopping..." : "Stop"}
          </button>
        )}
      </div>

      {state === "preparing" && (
        <p className="text-sm text-yellow-600">
          Provisioning GCP &amp; YouTube resources... This may take 30-60
          seconds.
        </p>
      )}

      {joined && (
        <p className="text-sm text-green-600">
          You are live! Streaming to YouTube &amp; Cloud CDN.
        </p>
      )}

      {error && <p className="text-sm text-red-600">Error: {error}</p>}
    </div>
  );
}
