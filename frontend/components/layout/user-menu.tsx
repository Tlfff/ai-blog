"use client"

import { useEffect, useRef, useState } from "react"
import Link from "next/link"
import { LayoutDashboard, LogOut, PenSquare, User as UserIcon, Clock } from "lucide-react"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { useAuth } from "@/hooks/use-auth"

export function UserMenu() {
  const { user, isAdmin, logout } = useAuth()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener("mousedown", onClick)
    return () => document.removeEventListener("mousedown", onClick)
  }, [])

  if (!user) return null

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <Avatar src={user.avatar} alt={user.username} size={32} />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 top-11 z-50 w-56 rounded-xl border border-border bg-popover p-1.5 text-popover-foreground shadow-lg"
        >
          <div className="flex items-center gap-3 px-2.5 py-2">
            <Avatar src={user.avatar} alt={user.username} size={36} />
            <div className="min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="truncate text-sm font-medium">{user.username}</span>
                {isAdmin && <Badge variant="primary">管理员</Badge>}
              </div>
              <p className="truncate text-xs text-muted-foreground">{user.location} · IP 归属地</p>
            </div>
          </div>
          <div className="my-1 h-px bg-border" />
          <MenuLink href="/profile" icon={<UserIcon className="size-4" />} label="个人中心" onClick={() => setOpen(false)} />
          <MenuLink href="/history" icon={<Clock className="size-4" />} label="浏览历史" onClick={() => setOpen(false)} />
          <MenuLink href="/editor" icon={<PenSquare className="size-4" />} label="写文章" onClick={() => setOpen(false)} />
          {isAdmin && (
            <MenuLink
              href="/admin"
              icon={<LayoutDashboard className="size-4" />}
              label="管理后台"
              onClick={() => setOpen(false)}
            />
          )}
          <div className="my-1 h-px bg-border" />
          <button
            type="button"
            onClick={() => {
              void logout()
              setOpen(false)
            }}
            className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <LogOut className="size-4" />
            退出登录
          </button>
        </div>
      )}
    </div>
  )
}

function MenuLink({
  href,
  icon,
  label,
  onClick,
}: {
  href: string
  icon: React.ReactNode
  label: string
  onClick: () => void
}) {
  return (
    <Link
      href={href}
      onClick={onClick}
      className="flex items-center gap-2 rounded-lg px-2.5 py-2 text-sm hover:bg-muted"
      role="menuitem"
    >
      {icon}
      {label}
    </Link>
  )
}
