"use client";

import { useState } from "react";
import { getToken, setToken, clearToken } from "@/lib/api";

/** Inline JWT token manager rendered in the nav bar. */
// This component is loaded with ssr:false, so localStorage is always available at init time.
export default function TokenInput() {
  const [input, setInput] = useState<string>(() => getToken() ?? "");
  const [saved, setSaved] = useState<boolean>(() => !!getToken());

  const handleSave = () => {
    const trimmed = input.trim();
    if (!trimmed) return;
    setToken(trimmed);
    setSaved(true);
  };

  const handleClear = () => {
    clearToken();
    setInput("");
    setSaved(false);
  };

  return (
    <div className="flex items-center gap-2 ml-auto">
      <input
        type="password"
        value={input}
        onChange={(e) => {
          setInput(e.target.value);
          setSaved(false);
        }}
        onKeyDown={(e) => e.key === "Enter" && handleSave()}
        placeholder="JWT token"
        className="text-sm px-2 py-1 border rounded-md w-44 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:bg-gray-900 dark:border-gray-700"
      />
      {saved ? (
        <button
          onClick={handleClear}
          className="text-xs px-2 py-1 rounded-md bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900 dark:text-red-300 transition"
        >
          Clear
        </button>
      ) : (
        <button
          onClick={handleSave}
          disabled={!input.trim()}
          className="text-xs px-2 py-1 rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-40 transition"
        >
          Save
        </button>
      )}
      {saved && (
        <span className="text-xs text-green-600 dark:text-green-400">✓</span>
      )}
    </div>
  );
}
