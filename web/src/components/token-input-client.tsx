"use client";

import dynamic from "next/dynamic";

// ssr: false prevents hydration mismatches from localStorage reads in useEffect
// Must live in a Client Component — `next/dynamic` with ssr:false is not allowed in Server Components
const TokenInput = dynamic(() => import("@/components/token-input"), {
  ssr: false,
});

export default function TokenInputClient() {
  return <TokenInput />;
}
