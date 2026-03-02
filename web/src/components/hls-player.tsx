"use client";

import { useEffect, useRef, useState } from "react";
import videojs from "video.js";
import "video.js/dist/video-js.css";
import type Player from "video.js/dist/types/player";

interface HlsPlayerProps {
  url: string;
}

const POLL_INTERVAL_MS = 3000;

export default function HlsPlayer({ url }: HlsPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const playerRef = useRef<Player | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // isAvailable: true once the .m3u8 URL responds with 2xx.
  // Starts as false so we poll before handing the URL to video.js.
  const [isAvailable, setIsAvailable] = useState(false);

  // Poll the .m3u8 URL with a HEAD request until it returns 2xx.
  // Uses cache: 'no-store' to bypass Cloudflare CDN caching of the initial 404.
  useEffect(() => {
    if (isAvailable) return;

    const check = async () => {
      try {
        const res = await fetch(url, { method: "HEAD", cache: "no-store" });
        if (res.ok) {
          setIsAvailable(true);
        }
      } catch {
        // network error — keep retrying
      }
    };

    check(); // immediate first check
    pollRef.current = setInterval(check, POLL_INTERVAL_MS);

    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [url, isAvailable]);

  // Initialise video.js only after the manifest is confirmed available.
  useEffect(() => {
    if (!isAvailable || !videoRef.current) return;

    const player = videojs(videoRef.current, {
      autoplay: true,
      controls: true,
      fluid: true,
      liveui: true,
      sources: [
        {
          src: url,
          type: "application/x-mpegURL",
        },
      ],
    });

    player.on("error", () => {
      // On playback error (e.g. stream interrupted), tear down the player and
      // fall back to the polling placeholder so we automatically recover.
      if (playerRef.current) {
        playerRef.current.reset();
        playerRef.current.dispose();
        playerRef.current = null;
      }
      setIsAvailable(false);
    });

    playerRef.current = player;

    return () => {
      if (playerRef.current) {
        // Reset source before disposing to prevent in-flight XHR from
        // triggering a MEDIA_ERR_SRC_NOT_SUPPORTED error on unmount.
        playerRef.current.reset();
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, [url, isAvailable]);

  if (!isAvailable) {
    return (
      <div className="w-full aspect-video bg-gray-900 rounded-lg flex flex-col items-center justify-center gap-3 text-gray-400">
        <svg
          className="animate-spin h-8 w-8 text-gray-500"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
          />
        </svg>
        <span className="text-sm">Connecting to stream...</span>
      </div>
    );
  }

  return (
    <div data-vjs-player>
      <video
        ref={videoRef}
        className="video-js vjs-big-play-centered w-full aspect-video rounded-lg overflow-hidden"
      />
    </div>
  );
}
