import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import dynamic from "next/dynamic";
import Link from "next/link";
import "./globals.css";

// ssr: false prevents hydration mismatches from localStorage reads in useEffect
const TokenInput = dynamic(() => import("@/components/token-input"), {
  ssr: false,
});

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "DeltaCast",
  description: "One-in, multi-out live streaming relay",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <nav className="border-b border-gray-200 dark:border-gray-800 px-6 py-3 flex items-center gap-6">
          <Link href="/" className="font-bold text-lg">
            DeltaCast
          </Link>
          <Link href="/streamer" className="text-sm hover:underline">
            Streamer
          </Link>
          <Link href="/audience" className="text-sm hover:underline">
            Audience
          </Link>
          <TokenInput />
        </nav>
        <main className="max-w-5xl mx-auto px-6 py-8">{children}</main>
      </body>
    </html>
  );
}
