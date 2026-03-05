import { Link } from "react-router-dom";

export default function Home() {
    return (
        <div className="space-y-6">
            <h1 className="text-3xl font-bold">DeltaCast</h1>
            <p className="text-gray-600 dark:text-gray-400">
                One-in, multi-out live streaming relay. Push once via Agora, watch
                everywhere — YouTube &amp; Cloud CDN HLS.
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <Link
                    to="/streamer"
                    className="block p-6 border rounded-lg hover:bg-gray-50 dark:hover:bg-gray-900 transition"
                >
                    <h2 className="text-xl font-semibold mb-2">Streamer</h2>
                    <p className="text-sm text-gray-500">
                        Start a live stream using your camera &amp; microphone via Agora
                        RTC.
                    </p>
                </Link>

                <Link
                    to="/audience"
                    className="block p-6 border rounded-lg hover:bg-gray-50 dark:hover:bg-gray-900 transition"
                >
                    <h2 className="text-xl font-semibold mb-2">Audience</h2>
                    <p className="text-sm text-gray-500">
                        Watch the live stream via Cloud CDN (HLS) or YouTube.
                    </p>
                </Link>
            </div>
        </div>
    );
}
