import { useEffect, useState } from "react"
import { Navigate, Outlet, useLocation } from "react-router-dom"
import { authApi, getToken } from "@/lib/api"

export default function ProtectedRoute() {
  // 无 token 时惰性初始化即判定未登录,避免在 effect 里同步 setState
  const [status, setStatus] = useState<"loading" | "auth" | "unauth">(() =>
    getToken() ? "loading" : "unauth"
  )
  const location = useLocation()

  useEffect(() => {
    if (!getToken()) return

    let cancelled = false
    authApi.me()
      .then(() => {
        if (!cancelled) setStatus("auth")
      })
      .catch(() => {
        if (!cancelled) setStatus("unauth")
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (status === "loading") {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (status === "unauth") {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <Outlet />
}
