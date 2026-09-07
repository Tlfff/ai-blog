"use client"

import { SystemState } from "@/components/system/system-state"

export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return <SystemState variant="error" reset={reset} />
}
