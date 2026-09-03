"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { ArrowLeft, User, Camera } from "lucide-react"
import { updateProfile, uploadAvatar } from "@/api/users"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Avatar } from "@/components/ui/avatar"
import { useAuth } from "@/hooks/use-auth"

export default function EditProfilePage() {
  const { user, refreshProfile } = useAuth()
  const [username, setUsername] = useState("")
  const [avatar, setAvatar] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [uploading, setUploading] = useState(false)

  useEffect(() => {
    if (user) {
      setUsername(user.username)
      setAvatar(user.avatar)
    }
  }, [user])

  async function handleAvatarChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      const url = await uploadAvatar(file)
      setAvatar(url)
      // 上传成功后立即调用确认逻辑更新当前全局上下文的用户头像
      await refreshProfile()
    } catch (error) {
      alert("头像上传失败，请重试")
    } finally {
      setUploading(false)
    }
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await updateProfile({ nickname: username, avatar })
      alert("资料修改成功")
      await refreshProfile()
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
            <h1 className="text-xl font-semibold">修改资料</h1>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <User className="size-4" />
                个人资料
              </CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit} className="space-y-4">
                <div className="flex flex-col items-center gap-3 mb-6">
                  <div 
                    className="relative group cursor-pointer overflow-hidden rounded-full size-24 border-2 border-border/80 hover:border-primary transition-all duration-300"
                    onClick={() => document.getElementById("avatar-file-input")?.click()}
                  >
                    <Avatar src={avatar} alt={username || "用户"} size={96} className="h-full w-full object-cover" />
                    <div className="absolute inset-0 bg-black/40 rounded-full flex flex-col items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity duration-300">
                      <Camera className="size-5 text-white mb-1" />
                      <span className="text-[10px] text-white font-medium">更换头像</span>
                    </div>
                  </div>
                  <input
                    id="avatar-file-input"
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={handleAvatarChange}
                    disabled={uploading || submitting}
                  />
                  {uploading && <span className="text-xs text-muted-foreground animate-pulse">上传中...</span>}
                </div>

                <div>
                  <label className="mb-1.5 block text-sm font-medium">昵称</label>
                  <input
                    type="text"
                    value={username}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setUsername(e.target.value)}
                    placeholder="请输入昵称"
                    className="flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                  />
                </div>

                <div className="flex gap-3 pt-2">
                  <Button type="button" variant="outline" className="flex-1" onClick={() => window.history.back()}>
                    取消
                  </Button>
                  <Button type="submit" className="flex-1" disabled={submitting || uploading}>
                    {submitting ? "保存中..." : "保存"}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      </Container>
    </SiteShell>
  )
}
