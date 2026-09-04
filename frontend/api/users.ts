import type { User, Role, Paginated, HistoryItem } from "@/types"
import {
  mapBackendMyProfileToFrontend,
  mapBackendPublicProfileToFrontend,
} from "@/types"
import { request, getLocalHistory } from "./client"

export interface LoginRequest {
  phone?: string
  nickname?: string
  password: string
  remember_me?: boolean
  device?: string
}

export interface RegisterRequest {
  nickname: string
  phone: string
  password: string
}

export interface UpdateProfileRequest {
  nickname: string
  avatar?: string
}

export async function login(data: { account: string; password: string }): Promise<{ access_token: string }> {
  const isNumeric = /^\d+$/.test(data.account)
  const reqBody: LoginRequest = {
    password: data.password,
    remember_me: true,
    device: "web",
  }
  if (isNumeric) {
    reqBody.phone = data.account
  } else {
    reqBody.nickname = data.account
  }

  return request<{ access_token: string }>("/user/login", {
    method: "POST",
    body: JSON.stringify(reqBody),
  })
}

export async function register(data: RegisterRequest): Promise<void> {
  await request<void>("/user/register", {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function logout(): Promise<void> {
  await request<void>("/auth/my/logout", {
    method: "POST",
  })
}

export async function getMyProfile(): Promise<User> {
  const data = await request<{
    id: number
    nickname: string
    avatar: string
    last_login_time: number
    last_login_ip: string
    role: number
  }>("/auth/my/profile")
  
  const role: Role = data.role === 2 ? "admin" : "user"

  return mapBackendMyProfileToFrontend({
    ...data,
    role,
  })
}

export async function getPublicProfile(userId: string): Promise<User> {
  const data = await request<{ id: number; nickname: string; avatar: string }>(
    `/user/profile?user_id=${userId}`,
  )
  return mapBackendPublicProfileToFrontend(data)
}

export async function updateProfile(data: UpdateProfileRequest): Promise<void> {
  await request<void>("/auth/my/profile/update", {
    method: "POST",
    body: JSON.stringify(data),
  })
}

export async function verifyPassword(oldPassword: string): Promise<string> {
  const data = await request<{ change_token: string }>("/auth/my/password/verify", {
    method: "POST",
    body: JSON.stringify({ old_password: oldPassword }),
  })

  return data.change_token
}

export async function changePassword(changeToken: string, newPassword: string): Promise<void> {
  await request<void>("/auth/my/password/change", {
    method: "POST",
    body: JSON.stringify({
      change_token: changeToken,
      new_password: newPassword,
    }),
  })
}

export async function updateAccount(phone: string): Promise<void> {
  await request<void>("/auth/my/account/update", {
    method: "POST",
    body: JSON.stringify({ phone }),
  })
}

export async function getHistory(page = 1, pageSize = 10): Promise<Paginated<HistoryItem>> {
  return getLocalHistory(page, pageSize)
}

export async function getAvatarUploadURL(fileExt: string): Promise<{ upload_url: string; object_key: string }> {
  return request<{ upload_url: string; object_key: string }>("/auth/my/avatar/upload-url", {
    method: "POST",
    body: JSON.stringify({ file_ext: fileExt }),
  })
}

export async function confirmAvatar(objectKey: string): Promise<{ avatar_url: string }> {
  return request<{ avatar_url: string }>("/auth/my/avatar/confirm", {
    method: "POST",
    body: JSON.stringify({ object_key: objectKey }),
  })
}

export async function uploadAvatar(file: File): Promise<string> {
  const ext = file.name.split(".").pop() || "jpg"
  const { upload_url, object_key } = await getAvatarUploadURL(ext)
  
  const uploadRes = await fetch(upload_url, {
    method: "PUT",
    body: file,
    headers: {
      "Content-Type": file.type,
    },
  })
  
  if (!uploadRes.ok) {
    throw new Error("文件上传到存储服务失败")
  }
  
  const { avatar_url } = await confirmAvatar(object_key)
  return avatar_url
}
