"use client"

import { createContext, useCallback, useContext, useMemo, useState, useEffect, type ReactNode } from "react"
import type { Role, User } from "@/types"
import { getMyProfile, login, logout as logoutRequest } from "@/api/users"

interface AuthContextValue {
  user: User | null
  role: Role
  isLoggedIn: boolean
  isAdmin: boolean
  login: (account: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refreshProfile: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const refreshProfile = useCallback(async () => {
    try {
      const profile = await getMyProfile()
      setUser(profile)
    } catch (e) {
      console.error("Failed to refresh profile:", e)
    }
  }, [])

  useEffect(() => {
    const token = localStorage.getItem("access_token")
    if (token) {
      getMyProfile()
        .then(setUser)
        .catch(() => {
          localStorage.removeItem("access_token")
        })
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
    }
  }, [])

  const loginFn = useCallback(async (account: string, password: string) => {
    const response = await login({ account, password })
    localStorage.setItem("access_token", response.access_token)
    const profile = await getMyProfile()
    setUser(profile)
  }, [])

  const logout = useCallback(async () => {
    try {
      await logoutRequest()
    } finally {
      localStorage.removeItem("access_token")
      setUser(null)
    }
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      role: user?.role ?? "guest",
      isLoggedIn: !!user,
      isAdmin: user?.role === "admin",
      login: loginFn,
      logout,
      refreshProfile,
    }),
    [user, loginFn, logout, refreshProfile],
  )

  if (loading) {
    return null
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth 必须在 AuthProvider 内使用")
  return ctx
}
