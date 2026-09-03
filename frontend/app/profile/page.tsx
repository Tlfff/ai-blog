"use client"

import { useState, useEffect } from "react"
import useSWR from "swr"
import Link from "next/link"
import { User, Calendar, MapPin, Settings, Mail, Phone, Edit3 } from "lucide-react"
import { getPublicProfile, getMyProfile } from "@/api/users"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { formatDate } from "@/lib/format"
import { useAuth } from "@/hooks/use-auth"
import { LoadingState } from "@/components/ui/spinner"

export default function ProfilePage({ searchParams }: { searchParams: Promise<{ userId?: string }> }) {
  const { user: currentUser } = useAuth()
  const [userId, setUserId] = useState<string | undefined>(undefined)

  useEffect(() => {
    searchParams.then((p) => setUserId(p.userId))
  }, [searchParams])

  const targetUserId = userId ?? currentUser?.id
  const isViewingSelf = !userId && currentUser?.id
  const { data: user, isLoading: userLoading } = useSWR(targetUserId ? ["user", targetUserId] : null, () =>
    isViewingSelf ? getMyProfile() : getPublicProfile(targetUserId!),
  )

  if (userLoading) return <LoadingState />
  if (!user) return <div className="py-12 text-center">用户不存在</div>

  const isCurrentUser = currentUser?.id === user.id
  const isAdmin = user.role === "admin"

  return (
    <SiteShell>
      <Container className="py-6">
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_320px]">
          <div>
            <div className="flex flex-col gap-6">
              <div className="flex gap-6">
                <Avatar src={user.avatar} alt={user.username} size={100} />
                <div className="flex flex-col justify-center">
                  <div className="flex items-center gap-2">
                    <h1 className="text-2xl font-bold">{user.username}</h1>
                    {isAdmin && <Badge variant="primary">管理员</Badge>}
                  </div>
                  {user.location && <p className="mt-1 text-muted-foreground">{user.location}</p>}
                  <div className="mt-3 flex items-center gap-4 text-sm text-muted-foreground">
                    <span className="flex items-center gap-1">
                      <MapPin className="size-4" />
                      {user.location || "未知"}
                    </span>
                    <span className="flex items-center gap-1">
                      <Calendar className="size-4" />
                      {formatDate(user.joinedAt)} 加入
                    </span>
                  </div>
                </div>
              </div>

              {isCurrentUser && (
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                      <Settings className="size-4" />
                      账户设置
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-col gap-3">
                      <Link href="/profile/edit">
                        <Button variant="outline" className="w-full justify-start gap-2">
                          <Edit3 className="size-4" />
                          修改资料
                        </Button>
                      </Link>
                      <Link href="/profile/account">
                        <Button variant="outline" className="w-full justify-start gap-2">
                          <Mail className="size-4" />
                          修改账号
                        </Button>
                      </Link>
                    </div>
                  </CardContent>
                </Card>
              )}

              {isAdmin && (
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                      <Edit3 className="size-4" />
                      快捷操作
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-col gap-3">
                      <Link href="/editor">
                        <Button className="w-full justify-center gap-2">
                          <Edit3 className="size-4" />
                          写文章
                        </Button>
                      </Link>
                      <Link href="/admin">
                        <Button variant="outline" className="w-full justify-center gap-2">
                          <Settings className="size-4" />
                          管理后台
                        </Button>
                      </Link>
                    </div>
                  </CardContent>
                </Card>
              )}
            </div>
          </div>

          <aside className="hidden lg:block">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <User className="size-4" />
                  {isCurrentUser ? "我的" : `${user.username} 的`}信息
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">角色</span>
                    <span>{isAdmin ? "管理员" : "普通用户"}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">加入时间</span>
                    <span>{formatDate(user.joinedAt)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">IP归属地</span>
                    <span>{user.location || "未知"}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </aside>
        </div>
      </Container>
    </SiteShell>
  )
}