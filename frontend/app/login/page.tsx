"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { ArrowRight, Eye, EyeOff, LockKeyhole, UserRound } from "lucide-react"
import { AuthField, AuthNotice, AuthShell, authInputClass } from "@/components/auth/auth-shell"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"

export default function LoginPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { login, isLoggedIn } = useAuth()
  const [account, setAccount] = useState("")
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const redirectTo = searchParams.get("redirect") || "/"

  useEffect(() => {
    if (isLoggedIn) router.replace(redirectTo)
  }, [isLoggedIn, redirectTo, router])

  if (isLoggedIn) return null

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    if (!account.trim()) { setError("请输入手机号或昵称"); return }
    if (!password.trim()) { setError("请输入密码"); return }

    setLoading(true)
    try {
      await login(account.trim(), password.trim())
      router.push(redirectTo)
    } catch {
      setError("登录失败，请检查账号或密码是否正确")
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthShell variant="login">
      <p className="font-mono text-[0.62rem] font-semibold uppercase tracking-[0.22em] text-[var(--auth-sky-deep)]">sign in / 01</p>
      <h2 className="auth-heading mt-3 font-playful text-4xl font-bold tracking-[-0.04em]">欢迎回来</h2>
      <p className="mt-2 text-sm text-[var(--auth-muted)]">使用手机号或昵称登录你的账号。</p>

      <form onSubmit={handleSubmit} className="mt-8 space-y-5">
        <AuthField label="账号" icon={<UserRound className="size-5" />}>
          <input type="text" autoComplete="username" value={account} onChange={(event) => setAccount(event.target.value)} placeholder="手机号或昵称" disabled={loading} className={`${authInputClass} pl-12`} />
        </AuthField>

        <AuthField label="密码" icon={<LockKeyhole className="size-5" />}>
          <input type={showPassword ? "text" : "password"} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="请输入密码" disabled={loading} className={`${authInputClass} pl-12 pr-12`} />
          <button type="button" onClick={() => setShowPassword((value) => !value)} aria-label={showPassword ? "隐藏密码" : "显示密码"} className="absolute right-4 top-1/2 -translate-y-1/2 text-[var(--auth-faint)] hover:text-[var(--auth-ink)]">{showPassword ? <EyeOff className="size-5" /> : <Eye className="size-5" />}</button>
        </AuthField>

        {error ? <p role="alert" className="rounded-xl bg-[var(--auth-coral-soft)] px-4 py-3 text-sm text-[var(--auth-coral)]">{error}</p> : null}

        <Button type="submit" disabled={loading} className="h-12 w-full rounded-full bg-[var(--auth-navy)] text-white hover:bg-[var(--auth-sky-deep)]">
          {loading ? "登录中..." : <><span>登录</span><ArrowRight className="ml-auto size-4" /></>}
        </Button>
      </form>

      <div className="my-7 flex items-center gap-4"><span className="h-px flex-1 bg-[var(--auth-border)]" /><span className="text-xs text-[var(--auth-faint)]">还没有账号？</span><span className="h-px flex-1 bg-[var(--auth-border)]" /></div>
      <div className="text-center"><Link href="/register" className="inline-flex rounded-full bg-[var(--auth-sky-soft)] px-6 py-2.5 text-sm font-semibold text-[var(--auth-sky-deep)] hover:opacity-80">创建新账号</Link></div>
      <div className="mt-8"><AuthNotice title="安全登录">登录状态由现有会话机制管理，不保存明文密码。</AuthNotice></div>
    </AuthShell>
  )
}
