import ReactPlayer from "react-player";

interface YouTubePlayerProps {
  url: string;
}

export default function YouTubePlayer({ url }: YouTubePlayerProps) {
  return (
    <div className="w-full aspect-video rounded-lg overflow-hidden">
      <ReactPlayer
        src={url}
        playing
        controls
        width="100%"
        height="100%"
        config={{
          youtube: {
            playerVars: {
              // Declare the embedding origin so the YouTube IFrame API can
              // validate it. Without this, some browsers (especially those
              // that block third-party cookies) see a configuration error
              // even when the video itself allows embedding.
              origin: window.location.origin,
              // Do not show related videos after the stream ends.
              rel: 0,
            },
          },
        }}
      />
    </div>
  );
}
