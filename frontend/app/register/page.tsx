"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { ArrowRight, Eye, EyeOff, LockKeyhole, Phone, UserRound } from "lucide-react"
import { register } from "@/api/users"
import { AuthField, AuthNotice, AuthShell, authInputClass } from "@/components/auth/auth-shell"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"

export default function RegisterPage() {
  const router = useRouter()
  const { isLoggedIn } = useAuth()
  const [nickname, setNickname] = useState("")
  const [phone, setPhone] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (isLoggedIn) router.replace("/")
  }, [isLoggedIn, router])

  if (isLoggedIn) return null

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    const normalizedNickname = nickname.trim()
    const normalizedPhone = phone.trim()

    if (!normalizedNickname) { setError("请输入昵称"); return }
    if (!normalizedPhone) { setError("请输入手机号"); return }
    if (!/^1[3-9]\d{9}$/.test(normalizedPhone)) { setError("请输入有效的手机号"); return }
    if (!password) { setError("请输入密码"); return }
    if (password.length < 6) { setError("密码长度至少为 6 位"); return }
    if (password !== confirmPassword) { setError("两次输入的密码不一致"); return }

    setLoading(true)
    try {
      await register({ nickname: normalizedNickname, phone: normalizedPhone, password })
      router.push("/login")
    } catch {
      setError("注册失败，请稍后重试")
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthShell variant="register">
      <p className="font-mono text-[0.62rem] font-semibold uppercase tracking-[0.22em] text-[var(--auth-teal-deep)]">create account / 01</p>
      <h2 className="auth-heading mt-3 font-playful text-4xl font-bold tracking-[-0.04em]">开始新的记录</h2>
      <p className="mt-2 text-sm text-[var(--auth-muted)]">填写基本信息即可创建账号。</p>

      <form onSubmit={handleSubmit} className="mt-7 space-y-4">
        <AuthField label="昵称" icon={<UserRound className="size-5" />}>
          <input type="text" autoComplete="nickname" value={nickname} onChange={(event) => setNickname(event.target.value.slice(0, 20))} placeholder="请输入昵称" disabled={loading} maxLength={20} className={`${authInputClass} pl-12`} />
        </AuthField>

        <AuthField label="手机号" icon={<Phone className="size-5" />}>
          <input type="tel" inputMode="numeric" autoComplete="tel" value={phone} onChange={(event) => setPhone(event.target.value.replace(/\D/g, "").slice(0, 11))} placeholder="请输入 11 位手机号" disabled={loading} className={`${authInputClass} pl-12`} />
        </AuthField>

        <AuthField label="密码" icon={<LockKeyhole className="size-5" />}>
          <input type={showPassword ? "text" : "password"} autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="至少 6 个字符" disabled={loading} className={`${authInputClass} pl-12 pr-12`} />
          <button type="button" onClick={() => setShowPassword((value) => !value)} aria-label={showPassword ? "隐藏密码" : "显示密码"} className="absolute right-4 top-1/2 -translate-y-1/2 text-[var(--auth-faint)] hover:text-[var(--auth-ink)]">{showPassword ? <EyeOff className="size-5" /> : <Eye className="size-5" />}</button>
        </AuthField>

        <AuthField label="确认密码" icon={<LockKeyhole className="size-5" />}>
          <input type={showPassword ? "text" : "password"} autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} placeholder="请再次输入密码" disabled={loading} className={`${authInputClass} pl-12`} />
        </AuthField>

        {error ? <p role="alert" className="rounded-xl bg-[var(--auth-coral-soft)] px-4 py-3 text-sm text-[var(--auth-coral)]">{error}</p> : null}

        <Button type="submit" disabled={loading} className="h-12 w-full rounded-full bg-[var(--auth-teal-deep)] text-white hover:bg-[var(--auth-sky-deep)]">
          {loading ? "注册中..." : <><span>创建账号</span><ArrowRight className="ml-auto size-4" /></>}
        </Button>
      </form>

      <div className="my-6 flex items-center gap-4"><span className="h-px flex-1 bg-[var(--auth-border)]" /><span className="text-xs text-[var(--auth-faint)]">已经有账号？</span><span className="h-px flex-1 bg-[var(--auth-border)]" /></div>
      <div className="text-center"><Link href="/login" className="inline-flex rounded-full bg-[var(--auth-teal-soft)] px-6 py-2.5 text-sm font-semibold text-[var(--auth-teal-deep)] hover:opacity-80">返回登录</Link></div>
      <div className="mt-7"><AuthNotice tone="yellow" title="注册提示">手机号仅用于账号登录，不会展示在公开页面。</AuthNotice></div>
    </AuthShell>
  )
}
