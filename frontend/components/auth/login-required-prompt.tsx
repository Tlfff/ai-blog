"use client"

import { useEffect } from "react"
import Link from "next/link"
import { LogIn, X } from "lucide-react"
import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export function LoginRequiredPrompt({ redirectTo, onClose }: { redirectTo: string; onClose: () => void }) {
  useEffect(() => {
    const timer = window.setTimeout(onClose, 5000)
    return () => window.clearTimeout(timer)
  }, [onClose])

  return (
    <div className="fixed inset-x-4 bottom-6 z-[100] mx-auto flex max-w-sm items-center gap-3 rounded-2xl border border-border bg-paper px-4 py-3 text-ink shadow-[0_18px_55px_rgba(22,28,38,0.22)]" role="status" aria-live="polite">
      <span className="grid size-9 shrink-0 place-items-center rounded-full bg-sakura-wash text-sakura-deep">
        <LogIn className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="font-playful text-sm font-bold">请先登录</p>
        <p className="mt-0.5 text-xs text-muted-foreground">登录后即可继续当前操作</p>
      </div>
      <Link href={`/login?redirect=${encodeURIComponent(redirectTo)}`} onClick={onClose} className={cn(buttonVariants({ size: "sm" }), "rounded-full")}>去登录</Link>
      <button type="button" onClick={onClose} aria-label="关闭登录提示" className="grid size-7 shrink-0 place-items-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground">
        <X className="size-3.5" />
      </button>
    </div>
  )
}
