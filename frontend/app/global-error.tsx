"use client"

import { SystemState } from "@/components/system/system-state"
import "./globals.css"

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return <html lang="zh-CN"><body><SystemState variant="error" reset={reset} /></body></html>
}
