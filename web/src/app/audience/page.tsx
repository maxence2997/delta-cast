"use client";

import { useEffect } from "react";
import { useSession } from "@/lib/use-session";
import StatusBadge from "@/components/status-badge";
import HlsPlayer from "@/components/hls-player";
import YouTubePlayer from "@/components/youtube-player";

export default function AudiencePage() {
  const { session, refreshStatus, startPolling, stopPolling } = useSession();

  // Start polling on mount to pick up any active session.
  // continueOnIdle=true keeps polling even when state is idle so the audience
  // page automatically detects when the streamer starts preparing.
  useEffect(() => {
    refreshStatus();
    startPolling(true);
    return () => stopPolling();
  }, [refreshStatus, startPolling, stopPolling]);

  const state = session?.state ?? "idle";

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Audience</h1>
        <StatusBadge state={state} />
      </div>

      {state === "idle" && (
        <div className="text-center py-20 text-gray-500">
          <p className="text-lg">No active stream</p>
          <p className="text-sm mt-2">Waiting for the streamer to go live...</p>
        </div>
      )}

      {(state === "preparing" || state === "ready") && (
        <div className="text-center py-20 text-yellow-600">
          <p className="text-lg">Stream is being prepared...</p>
          <p className="text-sm mt-2">
            Resources are provisioning. The stream will appear shortly.
          </p>
        </div>
      )}

      {state === "live" && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* GCP HLS Player */}
          <div className="space-y-2">
            <h2 className="text-lg font-semibold">Cloud CDN (HLS)</h2>
            {session?.gcpPlaybackUrl ? (
              <HlsPlayer url={session.gcpPlaybackUrl} />
            ) : (
              <div className="w-full aspect-video bg-gray-100 dark:bg-gray-900 rounded-lg flex items-center justify-center text-sm text-gray-500">
                HLS URL not available yet
              </div>
            )}
          </div>

          {/* YouTube Player */}
          <div className="space-y-2">
            <h2 className="text-lg font-semibold">YouTube</h2>
            {session?.youtubeWatchUrl ? (
              <YouTubePlayer url={session.youtubeWatchUrl} />
            ) : (
              <div className="w-full aspect-video bg-gray-100 dark:bg-gray-900 rounded-lg flex items-center justify-center text-sm text-gray-500">
                YouTube URL not available yet
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
