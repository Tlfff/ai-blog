"use client"

import { useState } from "react"
import Link from "next/link"
import { ArrowLeft, Lock, Phone } from "lucide-react"
import { updatePassword, updateAccount } from "@/api/users"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"

export default function AccountPage() {
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [phone, setPhone] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [activeTab, setActiveTab] = useState<"password" | "phone">("password")

  async function handlePasswordSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      alert("两次输入的密码不一致")
      return
    }
    setSubmitting(true)
    try {
      await updatePassword({ old_password: oldPassword, new_password: newPassword })
      alert("密码修改成功")
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch (error) {
      alert("修改失败，请重试")
    } finally {
      setSubmitting(false)
    }
  }

  async function handlePhoneSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!phone) {
      alert("请输入手机号")
      return
    }
    setSubmitting(true)
    try {
      await updateAccount(phone)
      alert("手机号修改成功")
      setPhone("")
    } catch (error) {
      alert("修改失败，请重试")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <SiteShell>
      <Container className="py-6">
        <div className="mx-auto max-w-md">
          <div className="mb-6 flex items-center gap-3">
            <Link href="/profile" className="flex items-center gap-1 rounded-md px-2 py-1 hover:bg-muted">
              <ArrowLeft className="size-4" />
              返回
            </Link>
            <h1 className="text-xl font-semibold">修改账号</h1>
          </div>

          <div className="flex rounded-lg border border-border overflow-hidden mb-4">
            <button
              onClick={() => setActiveTab("password")}
              className={`flex-1 py-2 text-sm font-medium transition-colors ${
                activeTab === "password" ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <Lock className="inline size-4 mr-2" />
              修改密码
            </button>
            <button
              onClick={() => setActiveTab("phone")}
              className={`flex-1 py-2 text-sm font-medium transition-colors ${
                activeTab === "phone" ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <Phone className="inline size-4 mr-2" />
              修改手机号
            </button>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                {activeTab === "password" ? <Lock className="size-4" /> : <Phone className="size-4" />}
                {activeTab === "password" ? "修改密码" : "修改手机号"}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {activeTab === "password" ? (
                <form onSubmit={handlePasswordSubmit} className="space-y-4">
                  <div>
                    <label className="mb-1.5 block text-sm font-medium">旧密码</label>
                    <input
                      type="password"
                      value={oldPassword}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => setOldPassword(e.target.value)}
                      placeholder="请输入旧密码"
                      className="flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                    />
                  </div>

                  <div>
                    <label className="mb-1.5 block text-sm font-medium">新密码</label>
                    <input
                      type="password"
                      value={newPassword}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewPassword(e.target.value)}
                      placeholder="请输入新密码"
                      className="flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                    />
                  </div>

                  <div>
                    <label className="mb-1.5 block text-sm font-medium">确认密码</label>
                    <input
                      type="password"
                      value={confirmPassword}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfirmPassword(e.target.value)}
                      placeholder="请再次输入新密码"
                      className="flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                    />
                  </div>

                  <div className="flex gap-3 pt-2">
                    <Button type="button" variant="outline" className="flex-1" onClick={() => window.history.back()}>
                      取消
                    </Button>
                    <Button type="submit" className="flex-1" disabled={submitting}>
                      {submitting ? "修改中..." : "修改"}
                    </Button>
                  </div>
                </form>
              ) : (
                <form onSubmit={handlePhoneSubmit} className="space-y-4">
                  <div>
                    <label className="mb-1.5 block text-sm font-medium">手机号</label>
                    <input
                      type="tel"
                      value={phone}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPhone(e.target.value)}
                      placeholder="请输入新手机号"
                      className="flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                    />
                  </div>

                  <div className="flex gap-3 pt-2">
                    <Button type="button" variant="outline" className="flex-1" onClick={() => window.history.back()}>
                      取消
                    </Button>
                    <Button type="submit" className="flex-1" disabled={submitting}>
                      {submitting ? "修改中..." : "修改"}
                    </Button>
                  </div>
                </form>
              )}
            </CardContent>
          </Card>
        </div>
      </Container>
    </SiteShell>
  )
}