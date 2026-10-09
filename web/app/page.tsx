"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

export default function Home() {
  const router = useRouter();
  useEffect(() => {
    const token = localStorage.getItem("pw_token");
    router.replace(token ? "/monitors" : "/login");
  }, [router]);
  return <p className="text-sm text-zinc-500">Loading...</p>;
}
