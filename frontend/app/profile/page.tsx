"use client"

import { useEffect, useRef, useState } from "react"
import Image from "next/image"
import Link from "next/link"
import { useRouter } from "next/navigation"
import {
  Camera,
  Check,
  Clock3,
  KeyRound,
  LogOut,
  MapPin,
  Moon,
  Palette,
  Phone,
  Save,
  ShieldCheck,
  Sun,
} from "lucide-react"
import { updateProfile, uploadAvatar } from "@/api/users"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"

type ThemeChoice = "light" | "dark"

const THEME_STORAGE_KEY = "site-theme"

export default function ProfilePage() {
  const router = useRouter()
  const { user, isLoggedIn, refreshProfile, logout } = useAuth()
  const [username, setUsername] = useState("")
  const [avatar, setAvatar] = useState("")
  const [theme, setTheme] = useState<ThemeChoice>("light")
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [message, setMessage] = useState("")
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!isLoggedIn) router.replace("/login")
  }, [isLoggedIn, router])

  useEffect(() => {
    if (!user) return
    setUsername(user.username)
    setAvatar(user.avatar)
  }, [user])

  useEffect(() => {
    setTheme(document.documentElement.classList.contains("dark") ? "dark" : "light")
  }, [])

  if (!user) return <LoadingState />

  const roleLabel = user.role === "admin" ? "管理员" : "普通用户"
  const loginLocation = user.location || user.lastLoginIp || "暂未识别"

  async function handleAvatarChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return

    setUploading(true)
    setMessage("")
    try {
      const nextAvatar = await uploadAvatar(file)
      setAvatar(nextAvatar)
      await refreshProfile()
      setMessage("头像已更新")
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "头像上传失败，请重试")
    } finally {
      setUploading(false)
      event.target.value = ""
    }
  }

  async function handleProfileSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nickname = username.trim()
    if (!nickname) {
      setMessage("昵称不能为空")
      return
    }

    setSaving(true)
    setMessage("")
    try {
      await updateProfile({ nickname, avatar })
      await refreshProfile()
      setMessage("个人资料已保存")
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存失败，请重试")
    } finally {
      setSaving(false)
    }
  }

  function chooseTheme(nextTheme: ThemeChoice) {
    const root = document.documentElement
    const dark = nextTheme === "dark"
    root.classList.toggle("dark", dark)
    root.dataset.theme = nextTheme
    localStorage.setItem(THEME_STORAGE_KEY, nextTheme)
    setTheme(nextTheme)
  }

  async function handleLogout() {
    await logout()
    router.push("/")
  }

  return (
    <SiteShell immersiveHeader>
      <div className="personal-center-page min-h-screen overflow-hidden bg-[var(--personal-background)] text-[var(--personal-ink)]">
        <section className="relative isolate min-h-[330px] overflow-hidden text-white sm:min-h-[390px]">
          <Image
            src="/kv/bq-1.png"
            alt="蓝天与云朵下的校园屋顶"
            fill
            priority
            sizes="100vw"
            className="object-cover object-[center_30%]"
          />
          <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(10,57,91,0.84)_0%,rgba(19,101,139,0.48)_48%,rgba(26,121,143,0.28)_100%)]" aria-hidden />
          <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[#165f77]/65 to-transparent" aria-hidden />

          <Container className="relative flex min-h-[330px] items-center pb-16 pt-24 sm:min-h-[390px] sm:pb-24">
            <div className="max-w-[1180px]">
              <p className="font-mono text-[0.66rem] font-semibold uppercase tracking-[0.28em] text-white/82">
                personal space / 个人中心
              </p>
              <h1 className="mt-4 max-w-[1120px] font-playful text-[clamp(2.35rem,5vw,4.3rem)] font-bold leading-[1.08] tracking-[-0.045em] [text-shadow:0_3px_18px_rgba(0,32,50,0.3)]">
                把自己的小小空间，照看妥当。
              </h1>
              <p className="mt-5 max-w-xl text-sm leading-7 text-white/88 sm:text-base">
                更新资料、保护账号，也选择更舒服的阅读方式。
              </p>
            </div>
          </Container>

          <div className="pointer-events-none absolute inset-x-0 bottom-0 z-10 h-20 sm:h-28" aria-hidden>
            <svg viewBox="0 0 1440 112" preserveAspectRatio="none" className="h-full w-full">
              <path d="M0 48C210 82 398 77 590 61C815 42 994 27 1168 51C1285 67 1366 70 1440 60V112H0Z" fill="var(--personal-wave)" />
              <path d="M0 72C188 58 353 99 588 87C818 75 1002 47 1190 70C1290 82 1365 88 1440 80V112H0Z" fill="var(--personal-background)" />
            </svg>
          </div>
        </section>

        <Container className="relative z-20 -mt-14 max-w-[1290px] pb-16 sm:-mt-20 sm:pb-24">
          <div className="personal-center-shell overflow-hidden rounded-[1.4rem] border border-[var(--personal-border)] bg-[var(--personal-surface)] shadow-[0_24px_70px_var(--personal-shadow)] lg:grid lg:grid-cols-[292px_minmax(0,1fr)]">
            <aside className="border-b border-[var(--personal-border)] bg-[var(--personal-rail)] p-5 sm:p-7 lg:border-b-0 lg:border-r lg:p-8">
              <div className="flex items-center gap-4 text-left lg:flex-col lg:text-center">
                <div className="relative shrink-0">
                  <span className="absolute -inset-2 rounded-full bg-[var(--personal-teal-soft)]" aria-hidden />
                  <Avatar src={avatar} alt={user.username} size={112} className="relative border-4 border-[var(--personal-surface)] shadow-[0_10px_28px_var(--personal-shadow-soft)]" />
                  <button
                    type="button"
                    aria-label="更换头像"
                    onClick={() => fileInputRef.current?.click()}
                    disabled={uploading}
                    className="absolute bottom-0 right-0 grid size-9 place-items-center rounded-full border-4 border-[var(--personal-rail)] bg-[var(--personal-sky)] text-white shadow-md transition-transform hover:scale-105 disabled:cursor-wait disabled:opacity-70"
                  >
                    <Camera className="size-4" />
                  </button>
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    onChange={handleAvatarChange}
                    className="hidden"
                  />
                </div>

                <div className="min-w-0 lg:mt-4">
                  <h2 className="truncate font-playful text-2xl font-bold tracking-[-0.03em] text-[var(--personal-ink)]">
                    {user.username}
                  </h2>
                  <p className="mt-1 text-xs text-[var(--personal-muted)]">{roleLabel} · {loginLocation}</p>
                  <Badge className="mt-3 rounded-full border-0 bg-[var(--personal-teal-soft)] text-[var(--personal-teal-deep)]">
                    <span className="mr-1.5 size-1.5 rounded-full bg-[var(--personal-teal)]" />
                    账号状态正常
                  </Badge>
                </div>
              </div>

              <div className="mt-8 hidden border-t border-[var(--personal-border)] pt-6 lg:block">
                <p className="text-xs text-[var(--personal-faint)]">最近登录</p>
                <p className="mt-2 text-sm font-semibold text-[var(--personal-muted)]">{formatLoginTime(user.lastLoginTime)}</p>
                <p className="mt-1 text-xs text-[var(--personal-faint)]">IP 属地 · {loginLocation}</p>
                <button
                  type="button"
                  onClick={handleLogout}
                  className="mt-5 inline-flex items-center gap-2 text-sm font-medium text-[var(--personal-coral)] hover:opacity-80"
                >
                  <LogOut className="size-4" />
                  退出登录
                </button>
              </div>
            </aside>

            <main className="min-w-0 p-5 sm:p-8 lg:p-10 xl:p-12">
              <form id="profile-info" onSubmit={handleProfileSubmit} className="scroll-mt-24">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <p className="font-mono text-[0.66rem] font-semibold uppercase tracking-[0.22em] text-[var(--personal-sky)]">profile / 01</p>
                    <h2 className="mt-2 font-playful text-3xl font-bold tracking-[-0.04em] text-[var(--personal-ink)]">个人资料</h2>
                    <p className="mt-2 text-sm text-[var(--personal-muted)]">这些信息会用于你的站内身份展示。</p>
                  </div>
                  <Button
                    type="submit"
                    disabled={saving || uploading}
                    className="rounded-full bg-[var(--personal-navy)] px-5 text-white hover:bg-[var(--personal-sky-deep)]"
                  >
                    <Save className="size-4" />
                    {saving ? "保存中..." : "保存修改"}
                  </Button>
                </div>

                <div className="mt-6 grid gap-6 rounded-2xl border border-[var(--personal-border)] bg-[var(--personal-card)] p-5 shadow-[0_10px_30px_var(--personal-shadow-soft)] sm:p-6 md:grid-cols-[230px_minmax(0,1fr)] md:items-center md:gap-8">
                  <div className="flex items-center gap-4 md:border-r md:border-[var(--personal-border)] md:pr-7">
                    <Avatar src={avatar} alt={user.username} size={76} className="border-2 border-[var(--personal-surface)]" />
                    <div>
                      <p className="text-sm font-semibold text-[var(--personal-ink)]">头像</p>
                      <p className="mt-1 text-xs leading-5 text-[var(--personal-faint)]">JPG、PNG 或 WebP</p>
                      <button type="button" onClick={() => fileInputRef.current?.click()} className="mt-2 text-xs font-semibold text-[var(--personal-sky-deep)] hover:opacity-75">
                        {uploading ? "上传中..." : "更换头像"}
                      </button>
                    </div>
                  </div>

                  <label className="block">
                    <span className="text-sm font-semibold text-[var(--personal-ink)]">昵称</span>
                    <input
                      value={username}
                      onChange={(event) => setUsername(event.target.value)}
                      minLength={2}
                      maxLength={20}
                      className="mt-2 h-12 w-full rounded-xl border border-[var(--personal-border-strong)] bg-[var(--personal-input)] px-4 text-sm text-[var(--personal-ink)] outline-none transition focus:border-[var(--personal-sky)] focus:ring-4 focus:ring-[var(--personal-sky-soft)]"
                    />
                    <span className="mt-2 block text-xs text-[var(--personal-faint)]">2–20 个字符，修改后会同步到评论与通知中。</span>
                  </label>
                </div>

                <p className="mt-3 min-h-5 text-sm text-[var(--personal-teal-deep)]" aria-live="polite">{message}</p>
              </form>

              <div className="mt-4 grid gap-4 xl:grid-cols-2">
                <section id="account-security" className="scroll-mt-24 rounded-2xl border border-[var(--personal-border)] bg-[var(--personal-card)] p-5 shadow-[0_10px_30px_var(--personal-shadow-soft)] sm:p-6">
                  <SectionTitle icon={<ShieldCheck className="size-5" />} tone="teal" title="账号与安全" description="保护登录凭证与联系方式" />
                  <SettingRow icon={<Phone className="size-4" />} label="手机号" value="已绑定" href="/profile/account#phone" />
                  <SettingRow icon={<KeyRound className="size-4" />} label="登录密码" value="已设置" href="/profile/account#password" />
                </section>

                <section id="login-info" className="scroll-mt-24 rounded-2xl border border-[var(--personal-border)] bg-[var(--personal-card)] p-5 shadow-[0_10px_30px_var(--personal-shadow-soft)] sm:p-6">
                  <SectionTitle icon={<Clock3 className="size-5" />} tone="coral" title="最近登录" description="确认最近一次登录是否由你完成" />
                  <InfoRow label="登录时间" value={formatLoginTime(user.lastLoginTime)} />
                  <InfoRow label="IP 属地" value={loginLocation} icon={<MapPin className="size-3.5" />} />
                </section>
              </div>

              <section id="appearance" className="mt-4 scroll-mt-24 rounded-2xl border border-[var(--personal-border)] bg-[var(--personal-card)] p-5 shadow-[0_10px_30px_var(--personal-shadow-soft)] sm:p-6">
                <div className="grid gap-5 xl:grid-cols-[minmax(220px,1fr)_minmax(0,1.5fr)] xl:items-center">
                  <SectionTitle icon={<Palette className="size-5" />} tone="lavender" title="外观偏好" description="日间与深夜模式都保持清爽易读" compact />
                  <div className="grid gap-3 sm:grid-cols-2">
                    <ThemeChoiceCard
                      active={theme === "light"}
                      title="日间模式"
                      description="明亮天空与云白"
                      icon={<Sun className="size-5" />}
                      onClick={() => chooseTheme("light")}
                      variant="light"
                    />
                    <ThemeChoiceCard
                      active={theme === "dark"}
                      title="深夜模式"
                      description="藏蓝背景与柔和高光"
                      icon={<Moon className="size-5" />}
                      onClick={() => chooseTheme("dark")}
                      variant="dark"
                    />
                  </div>
                </div>
              </section>

              <button
                type="button"
                onClick={handleLogout}
                className="mt-6 inline-flex items-center gap-2 text-sm font-medium text-[var(--personal-coral)] lg:hidden"
              >
                <LogOut className="size-4" />
                退出登录
              </button>
            </main>
          </div>
        </Container>
      </div>
    </SiteShell>
  )
}

function SectionTitle({ icon, tone, title, description, compact = false }: { icon: React.ReactNode; tone: "teal" | "coral" | "lavender"; title: string; description: string; compact?: boolean }) {
  const toneClass = {
    teal: "bg-[var(--personal-teal-soft)] text-[var(--personal-teal-deep)]",
    coral: "bg-[var(--personal-coral-soft)] text-[var(--personal-coral)]",
    lavender: "bg-[var(--personal-lavender-soft)] text-[var(--personal-lavender)]",
  }[tone]

  return (
    <div className={`flex items-start gap-3 ${compact ? "" : "border-b border-[var(--personal-border)] pb-4"}`}>
      <span className={`grid size-10 shrink-0 place-items-center rounded-xl ${toneClass}`}>{icon}</span>
      <div>
        <h3 className="font-playful text-xl font-bold text-[var(--personal-ink)]">{title}</h3>
        <p className="mt-0.5 text-xs text-[var(--personal-faint)]">{description}</p>
      </div>
    </div>
  )
}

function SettingRow({ icon, label, value, href }: { icon: React.ReactNode; label: string; value: string; href: string }) {
  return (
    <div className="flex items-center gap-3 border-b border-[var(--personal-border)] py-4 last:border-b-0 last:pb-0">
      <span className="text-[var(--personal-muted)]">{icon}</span>
      <span className="text-sm text-[var(--personal-muted)]">{label}</span>
      <strong className="ml-auto text-sm text-[var(--personal-ink)]">{value}</strong>
      <Link href={href} className="ml-2 rounded-full bg-[var(--personal-sky-soft)] px-3 py-1.5 text-xs font-semibold text-[var(--personal-sky-deep)] hover:opacity-75">
        修改
      </Link>
    </div>
  )
}

function InfoRow({ label, value, icon }: { label: string; value: string; icon?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 border-b border-[var(--personal-border)] py-4 last:border-b-0 last:pb-0">
      <span className="text-sm text-[var(--personal-muted)]">{label}</span>
      <strong className="ml-auto inline-flex items-center gap-1.5 text-right text-sm text-[var(--personal-ink)]">
        {icon}
        {value}
      </strong>
    </div>
  )
}

function ThemeChoiceCard({ active, title, description, icon, onClick, variant }: { active: boolean; title: string; description: string; icon: React.ReactNode; onClick: () => void; variant: ThemeChoice }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`relative flex min-h-20 items-center gap-3 rounded-2xl border p-4 text-left transition-transform hover:-translate-y-0.5 ${
        variant === "dark"
          ? "border-[#345071] bg-[#1e385c] text-white"
          : "border-[#acd8e9] bg-[#eef9fd] text-[#245e80]"
      } ${active ? "ring-2 ring-[var(--personal-sky)] ring-offset-2 ring-offset-[var(--personal-card)]" : ""}`}
    >
      <span className={variant === "dark" ? "text-[#ddd4ff]" : "text-[#e9b93f]"}>{icon}</span>
      <span>
        <strong className="block text-sm">{title}</strong>
        <small className={`mt-1 block text-xs ${variant === "dark" ? "text-white/62" : "text-[#6d8998]"}`}>{description}</small>
      </span>
      {active ? <Check className="absolute right-3 top-3 size-4" /> : null}
    </button>
  )
}

function formatLoginTime(timestamp?: number): string {
  if (!timestamp) return "暂无记录"
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(timestamp * 1000)).replaceAll("/", "-")
}
