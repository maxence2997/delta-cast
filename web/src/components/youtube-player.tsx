"use client";

import dynamic from "next/dynamic";

const ReactPlayer = dynamic(() => import("react-player"), {
  ssr: false,
});

interface YouTubePlayerProps {
  url: string;
}

export default function YouTubePlayer({ url }: YouTubePlayerProps) {
  return (
    <div className="w-full aspect-video rounded-lg overflow-hidden">
      <ReactPlayer src={url} playing controls width="100%" height="100%" />
    </div>
  );
}
