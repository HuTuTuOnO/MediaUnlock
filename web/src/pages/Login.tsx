import { useState, useEffect } from "react"
import { useNavigate, useLocation } from "react-router-dom"
import { Server, Eye, EyeOff, Sun, Moon, Monitor } from "lucide-react"
import { authApi, commonApi } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useToast } from "@/components/ui/toast-context"
import { useTheme } from "@/components/theme-context"
import { cn } from "@/lib/utils"

export default function LoginPage() {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [showPwd, setShowPwd] = useState(false)
  const [loading, setLoading] = useState(false)
  const [siteTitle, setSiteTitle] = useState("MediaUnlock")

  useEffect(() => {
    commonApi.settings().then((res) => {
      if (res.data.data?.title) setSiteTitle(res.data.data.title)
    }).catch(() => {})
  }, [])
  const navigate = useNavigate()
  const location = useLocation()
  const { toast } = useToast()
  const { theme, setTheme } = useTheme()

  const themeOptions = [
    { value: "light" as const, icon: Sun, label: "浅色" },
    { value: "dark" as const, icon: Moon, label: "深色" },
    { value: "system" as const, icon: Monitor, label: "跟随系统" },
  ]

  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || "/dashboard"

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username || !password) {
      toast({ title: "请填写用户名和密码" })
      return
    }
    setLoading(true)
    try {
      const res = await authApi.login(username, password)
      if (res.data.code === 200) {
        toast({ title: "登录成功", description: `欢迎回来，${res.data.data?.user?.username ?? ""}` })
        navigate(from, { replace: true })
      } else {
        toast({ title: "登录失败", description: res.data.msg })
      }
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { msg?: string } } })?.response?.data?.msg || "网络错误，请稍后重试"
      toast({ title: "登录失败", description: msg })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-muted p-4">
      {/* Theme switcher */}
      <div className="fixed top-4 right-4 flex items-center gap-0.5 rounded-lg p-1 bg-muted/60">
        {themeOptions.map(({ value, icon: Icon, label }) => (
          <button
            key={value}
            title={label}
            onClick={() => setTheme(value)}
            className={cn(
              "flex items-center justify-center rounded-md p-1.5 transition-colors",
              theme === value
                ? "bg-background shadow text-foreground"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            <Icon className="h-4 w-4" />
          </button>
        ))}
      </div>
      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="flex flex-col items-center mb-8">
          <div className="h-14 w-14 rounded-2xl bg-primary flex items-center justify-center shadow-lg mb-4">
            <Server className="h-7 w-7 text-white" />
          </div>
          <h1 className="text-2xl font-bold text-foreground">{siteTitle}</h1>
          <p className="text-sm text-muted-foreground mt-1">流媒体解锁管理系统</p>
        </div>

        <Card className="shadow-md">
          <CardHeader className="pb-4">
            <CardTitle className="text-lg">管理员登录</CardTitle>
            <CardDescription>请输入您的账户信息</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="username">用户名</Label>
                <Input
                  id="username"
                  placeholder="请输入用户名"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">密码</Label>
                <div className="relative">
                  <Input
                    id="password"
                    type={showPwd ? "text" : "password"}
                    placeholder="请输入密码"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    className="pr-10"
                  />
                  <button
                    type="button"
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    onClick={() => setShowPwd(!showPwd)}
                  >
                    {showPwd ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? (
                  <span className="flex items-center gap-2">
                    <span className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                    登录中...
                  </span>
                ) : "登录"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
