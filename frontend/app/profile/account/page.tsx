"use client"

import { useEffect, useState } from "react"
import Image from "next/image"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { ArrowLeft, Check, Eye, EyeOff, KeyRound, Phone, ShieldCheck } from "lucide-react"
import { changePassword, updateAccount, verifyPassword } from "@/api/users"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"

type AccountTab = "password" | "phone"
type PasswordStep = "verify" | "change"

export default function AccountPage() {
  const router = useRouter()
  const { isLoggedIn } = useAuth()
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [changeToken, setChangeToken] = useState("")
  const [passwordStep, setPasswordStep] = useState<PasswordStep>("verify")
  const [phone, setPhone] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [activeTab, setActiveTab] = useState<AccountTab>("password")
  const [showPassword, setShowPassword] = useState(false)
  const [message, setMessage] = useState("")

  useEffect(() => {
    if (!isLoggedIn) router.replace("/login")
  }, [isLoggedIn, router])

  useEffect(() => {
    const syncHash = () => setActiveTab(window.location.hash === "#phone" ? "phone" : "password")
    syncHash()
    window.addEventListener("hashchange", syncHash)
    return () => window.removeEventListener("hashchange", syncHash)
  }, [])

  function resetPasswordFlow() {
    setPasswordStep("verify")
    setChangeToken("")
    setOldPassword("")
    setNewPassword("")
    setConfirmPassword("")
    setShowPassword(false)
  }

  function selectTab(tab: AccountTab) {
    if (tab !== "password") resetPasswordFlow()
    setActiveTab(tab)
    setMessage("")
    window.history.replaceState(null, "", `#${tab}`)
  }

  async function handlePasswordVerify(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!oldPassword) {
      setMessage("请输入当前密码")
      return
    }

    setSubmitting(true)
    setMessage("")
    try {
      const token = await verifyPassword(oldPassword)
      setChangeToken(token)
      setOldPassword("")
      setShowPassword(false)
      setPasswordStep("change")
      window.history.replaceState(null, "", "#password-new")
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "当前密码验证失败，请重试")
    } finally {
      setSubmitting(false)
    }
  }

  async function handlePasswordChange(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!changeToken) {
      resetPasswordFlow()
      setMessage("验证凭证已失效，请重新验证当前密码")
      window.history.replaceState(null, "", "#password")
      return
    }
    if (newPassword.length < 6) {
      setMessage("新密码至少需要 6 个字符")
      return
    }
    if (newPassword !== confirmPassword) {
      setMessage("两次输入的密码不一致")
      return
    }

    setSubmitting(true)
    setMessage("")
    try {
      await changePassword(changeToken, newPassword)
      setChangeToken("")
      setNewPassword("")
      setConfirmPassword("")
      setMessage("密码修改成功，请在下次登录时使用新密码")
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "密码修改失败，凭证可能已过期")
    } finally {
      setSubmitting(false)
    }
  }

  async function handlePhoneSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!/^\d{11}$/.test(phone)) {
      setMessage("请输入 11 位手机号")
      return
    }

    setSubmitting(true)
    setMessage("")
    try {
      await updateAccount(phone)
      setPhone("")
      setMessage("手机号修改成功")
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "手机号修改失败，请重试")
    } finally {
      setSubmitting(false)
    }
  }

  const inputClass = "mt-2 h-12 w-full rounded-xl border border-[var(--personal-border-strong)] bg-[var(--personal-input)] px-4 text-sm text-[var(--personal-ink)] outline-none transition placeholder:text-[var(--personal-faint)] focus:border-[var(--personal-sky)] focus:ring-4 focus:ring-[var(--personal-sky-soft)]"
  const passwordTitle = passwordStep === "verify" ? "验证当前密码" : "设置新密码"
  const passwordDescription = passwordStep === "verify"
    ? "先确认是你本人操作。验证成功后，会获得一个 10 分钟内有效的一次性凭证。"
    : "身份验证已通过，请设置新密码。提交成功后，一次性凭证会立即失效。"

  return (
    <SiteShell immersiveHeader>
      <div className="personal-center-page min-h-screen bg-[var(--personal-background)] text-[var(--personal-ink)]">
        <section className="relative isolate min-h-[260px] overflow-hidden text-white sm:min-h-[300px]">
          <Image src="/kv/bq-1.png" alt="蓝天与云朵下的校园屋顶" fill priority sizes="100vw" className="object-cover object-[center_28%]" />
          <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(9,51,82,0.88),rgba(19,99,137,0.48),rgba(24,112,136,0.3))]" aria-hidden />
          <Container className="relative flex min-h-[260px] items-center pb-10 pt-24 sm:min-h-[300px]">
            <div>
              <p className="font-mono text-[0.65rem] font-semibold uppercase tracking-[0.25em] text-white/78">account / security</p>
              <h1 className="mt-3 font-playful text-4xl font-bold tracking-[-0.04em] sm:text-5xl">账号与安全</h1>
              <p className="mt-4 text-sm text-white/82">更新手机号与登录密码，保护你的账号。</p>
            </div>
          </Container>
          <div className="absolute inset-x-0 bottom-0 h-16" aria-hidden>
            <svg viewBox="0 0 1440 64" preserveAspectRatio="none" className="h-full w-full">
              <path d="M0 31C238 57 392 43 605 34C823 25 1005 16 1192 34C1292 44 1368 45 1440 39V64H0Z" fill="var(--personal-wave)" />
              <path d="M0 46C222 34 383 61 610 53C823 45 1007 31 1197 44C1296 51 1372 55 1440 51V64H0Z" fill="var(--personal-background)" />
            </svg>
          </div>
        </section>

        <Container className="relative z-10 -mt-8 max-w-4xl pb-20 sm:-mt-12">
          <Link href="/profile" className="mb-4 inline-flex items-center gap-2 rounded-full bg-[var(--personal-card)] px-4 py-2 text-sm font-medium text-[var(--personal-muted)] shadow-[0_8px_24px_var(--personal-shadow-soft)] hover:text-[var(--personal-sky-deep)]">
            <ArrowLeft className="size-4" />
            返回个人中心
          </Link>

          <div className="overflow-hidden rounded-[1.4rem] border border-[var(--personal-border)] bg-[var(--personal-surface)] shadow-[0_24px_70px_var(--personal-shadow)]">
            <div className="grid grid-cols-2 border-b border-[var(--personal-border)] bg-[var(--personal-rail)] p-2">
              <AccountTabButton active={activeTab === "password"} onClick={() => selectTab("password")} icon={<KeyRound className="size-4" />}>修改密码</AccountTabButton>
              <AccountTabButton active={activeTab === "phone"} onClick={() => selectTab("phone")} icon={<Phone className="size-4" />}>修改手机号</AccountTabButton>
            </div>

            <div className="grid gap-8 p-5 sm:p-8 md:grid-cols-[220px_minmax(0,1fr)] md:p-10">
              <aside>
                <span className="grid size-12 place-items-center rounded-2xl bg-[var(--personal-teal-soft)] text-[var(--personal-teal-deep)]">
                  <ShieldCheck className="size-6" />
                </span>
                <h2 className="mt-5 font-playful text-2xl font-bold text-[var(--personal-ink)]">
                  {activeTab === "password" ? passwordTitle : "绑定新手机号"}
                </h2>
                <p className="mt-3 text-sm leading-7 text-[var(--personal-muted)]">
                  {activeTab === "password"
                    ? passwordDescription
                    : "手机号用于账号登录，请确认填写的是你长期使用的号码。"}
                </p>
              </aside>

              <div>
                {activeTab === "password" ? (
                  <div id="password">
                    <PasswordSteps current={passwordStep} />
                    {passwordStep === "verify" ? (
                      <form onSubmit={handlePasswordVerify} className="mt-6 space-y-5">
                        <PasswordField label="当前密码" value={oldPassword} onChange={setOldPassword} show={showPassword} inputClass={inputClass} autoComplete="current-password" />
                        <PasswordVisibilityToggle show={showPassword} onClick={() => setShowPassword((value) => !value)} />
                        <SubmitArea submitting={submitting} message={message} label="验证当前密码" loadingLabel="验证中..." />
                      </form>
                    ) : (
                      <form onSubmit={handlePasswordChange} className="mt-6 space-y-5">
                        <PasswordField label="新密码" value={newPassword} onChange={setNewPassword} show={showPassword} inputClass={inputClass} autoComplete="new-password" />
                        <PasswordField label="确认新密码" value={confirmPassword} onChange={setConfirmPassword} show={showPassword} inputClass={inputClass} autoComplete="new-password" />
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <PasswordVisibilityToggle show={showPassword} onClick={() => setShowPassword((value) => !value)} />
                          <button
                            type="button"
                            onClick={() => {
                              resetPasswordFlow()
                              setMessage("")
                              window.history.replaceState(null, "", "#password")
                            }}
                            className="text-xs font-medium text-[var(--personal-muted)] hover:text-[var(--personal-sky-deep)]"
                          >
                            返回重新验证
                          </button>
                        </div>
                        <SubmitArea submitting={submitting} message={message} label="确认修改密码" />
                      </form>
                    )}
                  </div>
                ) : (
                  <form id="phone" onSubmit={handlePhoneSubmit} className="space-y-5">
                    <label className="block text-sm font-semibold text-[var(--personal-ink)]">
                      新手机号
                      <input type="tel" inputMode="numeric" autoComplete="tel" value={phone} onChange={(event) => setPhone(event.target.value.replace(/\D/g, "").slice(0, 11))} placeholder="请输入 11 位手机号" className={inputClass} />
                    </label>
                    <SubmitArea submitting={submitting} message={message} label="确认修改手机号" />
                  </form>
                )}
              </div>
            </div>
          </div>
        </Container>
      </div>
    </SiteShell>
  )
}

function AccountTabButton({ active, onClick, icon, children }: { active: boolean; onClick: () => void; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className={`flex items-center justify-center gap-2 rounded-xl px-4 py-3 text-sm font-semibold transition-colors ${active ? "bg-[var(--personal-surface)] text-[var(--personal-sky-deep)] shadow-sm" : "text-[var(--personal-muted)] hover:text-[var(--personal-ink)]"}`}>
      {icon}
      {children}
    </button>
  )
}

function PasswordSteps({ current }: { current: PasswordStep }) {
  return (
    <ol className="grid grid-cols-[1fr_auto_1fr] items-center gap-3" aria-label="修改密码进度">
      <PasswordStepItem number="1" label="验证当前密码" active={current === "verify"} complete={current === "change"} />
      <span className="h-px w-7 bg-[var(--personal-border-strong)] sm:w-12" aria-hidden />
      <PasswordStepItem number="2" label="设置新密码" active={current === "change"} />
    </ol>
  )
}

function PasswordStepItem({ number, label, active, complete = false }: { number: string; label: string; active: boolean; complete?: boolean }) {
  return (
    <li className={`flex min-w-0 items-center gap-2 text-xs font-semibold ${active || complete ? "text-[var(--personal-sky-deep)]" : "text-[var(--personal-faint)]"}`}>
      <span className={`grid size-7 shrink-0 place-items-center rounded-full border ${active || complete ? "border-[var(--personal-sky)] bg-[var(--personal-sky-soft)]" : "border-[var(--personal-border-strong)]"}`}>
        {complete ? <Check className="size-3.5" /> : number}
      </span>
      <span className="leading-5">{label}</span>
    </li>
  )
}

function PasswordField({ label, value, onChange, show, inputClass, autoComplete }: { label: string; value: string; onChange: (value: string) => void; show: boolean; inputClass: string; autoComplete: "current-password" | "new-password" }) {
  return (
    <label className="block text-sm font-semibold text-[var(--personal-ink)]">
      {label}
      <input type={show ? "text" : "password"} autoComplete={autoComplete} value={value} onChange={(event) => onChange(event.target.value)} className={inputClass} />
    </label>
  )
}

function PasswordVisibilityToggle({ show, onClick }: { show: boolean; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="inline-flex items-center gap-2 text-xs text-[var(--personal-muted)] hover:text-[var(--personal-sky-deep)]">
      {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
      {show ? "隐藏密码" : "显示密码"}
    </button>
  )
}

function SubmitArea({ submitting, message, label, loadingLabel = "提交中..." }: { submitting: boolean; message: string; label: string; loadingLabel?: string }) {
  return (
    <div className="pt-2">
      <p className="mb-3 min-h-5 text-sm text-[var(--personal-teal-deep)]" aria-live="polite">{message}</p>
      <Button type="submit" disabled={submitting} className="w-full rounded-full bg-[var(--personal-navy)] text-white hover:bg-[var(--personal-sky-deep)] sm:w-auto sm:px-6">
        {submitting ? loadingLabel : label}
      </Button>
    </div>
  )
}
