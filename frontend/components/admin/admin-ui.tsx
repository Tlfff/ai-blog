import { ChevronLeft, ChevronRight, Inbox } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export function AdminPageHeader({ eyebrow, title, description, actions }: { eyebrow: string; title: string; description: string; actions?: React.ReactNode }) {
  return (
    <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <p className="font-mono text-[0.62rem] font-semibold uppercase tracking-[0.22em] text-[var(--admin-sky-deep)]">{eyebrow}</p>
        <h1 className="mt-2 font-playful text-3xl font-bold tracking-[-0.04em] text-[var(--admin-ink)] sm:text-4xl">{title}</h1>
        <p className="mt-2 text-sm text-[var(--admin-muted)]">{description}</p>
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  )
}

export function AdminPanel({ children, className }: { children: React.ReactNode; className?: string }) {
  return <section className={cn("rounded-[1.15rem] border border-[var(--admin-border)] bg-[var(--admin-card)] shadow-[0_10px_30px_var(--admin-shadow-soft)]", className)}>{children}</section>
}

export function AdminPanelHeading({ title, description, action }: { title: string; description?: string; action?: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-[var(--admin-border)] px-5 py-4 sm:px-6">
      <div>
        <h2 className="font-playful text-xl font-bold text-[var(--admin-ink)]">{title}</h2>
        {description ? <p className="mt-1 text-xs text-[var(--admin-faint)]">{description}</p> : null}
      </div>
      {action}
    </div>
  )
}

export function AdminStatusBadge({ status }: { status: "published" | "draft" | "deleted" }) {
  const config = {
    published: ["已发表", "bg-[var(--admin-teal-soft)] text-[var(--admin-teal-deep)]"],
    draft: ["草稿", "bg-[var(--admin-yellow-soft)] text-[var(--admin-yellow-deep)]"],
    deleted: ["已删除", "bg-[var(--admin-coral-soft)] text-[var(--admin-coral)]"],
  }[status]
  return <span className={cn("inline-flex rounded-full px-2.5 py-1 text-[0.68rem] font-semibold", config[1])}>{config[0]}</span>
}

export function AdminPagination({ page, totalPages, onPageChange }: { page: number; totalPages: number; onPageChange: (page: number) => void }) {
  if (totalPages <= 1) return null
  return (
    <div className="flex items-center justify-center gap-3 border-t border-[var(--admin-border)] px-5 py-4">
      <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)} className="rounded-full border-[var(--admin-border-strong)] bg-transparent"><ChevronLeft className="size-4" />上一页</Button>
      <span className="text-xs text-[var(--admin-muted)]">第 {page} / {totalPages} 页</span>
      <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)} className="rounded-full border-[var(--admin-border-strong)] bg-transparent">下一页<ChevronRight className="size-4" /></Button>
    </div>
  )
}

export function AdminEmptyState({ title, description }: { title: string; description?: string }) {
  return (
    <div className="grid min-h-56 place-items-center px-6 py-12 text-center">
      <div>
        <span className="mx-auto grid size-12 place-items-center rounded-2xl bg-[var(--admin-sky-soft)] text-[var(--admin-sky-deep)]"><Inbox className="size-5" /></span>
        <p className="mt-4 font-playful text-lg font-bold text-[var(--admin-ink)]">{title}</p>
        {description ? <p className="mt-2 text-sm text-[var(--admin-muted)]">{description}</p> : null}
      </div>
    </div>
  )
}

export const adminInputClass = "h-10 rounded-full border border-[var(--admin-border-strong)] bg-[var(--admin-input)] px-4 text-sm text-[var(--admin-ink)] outline-none transition placeholder:text-[var(--admin-faint)] focus:border-[var(--admin-sky)] focus:ring-4 focus:ring-[var(--admin-sky-soft)]"
