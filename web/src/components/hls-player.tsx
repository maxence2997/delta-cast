"use client";

import { useEffect, useRef } from "react";
import videojs from "video.js";
import "video.js/dist/video-js.css";
import type Player from "video.js/dist/types/player";

interface HlsPlayerProps {
  url: string;
}

export default function HlsPlayer({ url }: HlsPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const playerRef = useRef<Player | null>(null);

  useEffect(() => {
    if (!videoRef.current) return;

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

    playerRef.current = player;

    return () => {
      if (playerRef.current) {
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, [url]);

  return (
    <div data-vjs-player>
      <video
        ref={videoRef}
        className="video-js vjs-big-play-centered w-full aspect-video rounded-lg overflow-hidden"
      />
    </div>
  );
}
