import { Link, Route, Routes } from "react-router-dom";
import TokenInput from "@/components/token-input";
import Home from "@/pages/Home";
import Streamer from "@/pages/Streamer";
import Audience from "@/pages/Audience";

export default function App() {
    return (
        <>
            <nav className="border-b border-gray-200 dark:border-gray-800 px-6 py-3 flex items-center gap-6">
                <Link to="/" className="font-bold text-lg">
                    DeltaCast
                </Link>
                <Link to="/streamer" className="text-sm hover:underline">
                    Streamer
                </Link>
                <Link to="/audience" className="text-sm hover:underline">
                    Audience
                </Link>
                <TokenInput />
            </nav>
            <main className="max-w-5xl mx-auto px-6 py-8">
                <Routes>
                    <Route path="/" element={<Home />} />
                    <Route path="/streamer" element={<Streamer />} />
                    <Route path="/audience" element={<Audience />} />
                </Routes>
            </main>
        </>
    );
}
