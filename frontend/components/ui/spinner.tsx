import { Loader2 } from "lucide-react"
import { cn } from "@/lib/utils"

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn("size-4 animate-spin text-muted-foreground", className)} />
}

export function LoadingState({ label = "加载中..." }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
      <Spinner />
      <span>{label}</span>
    </div>
  )
}

export function EmptyState({ label = "暂无数据" }: { label?: string }) {
  return (
    <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">{label}</div>
  )
}
