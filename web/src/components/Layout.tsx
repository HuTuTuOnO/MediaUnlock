import { useState, useEffect } from "react"
import { NavLink, Outlet, useNavigate } from "react-router-dom"
import { Server, Globe, Settings, LogOut, Menu, X, ChevronRight, LayoutDashboard, History, Sun, Moon, Monitor } from "lucide-react"
import { authApi, commonApi } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { useTheme } from "@/components/theme-context"
import { cn } from "@/lib/utils"

const navItems = [
  { to: "/dashboard", icon: LayoutDashboard, label: "仪表盘" },
  { to: "/nodes", icon: Server, label: "节点管理" },
  { to: "/platforms", icon: Globe, label: "平台管理" },
  { to: "/unlocks", icon: History, label: "解锁历史" },
  { to: "/settings", icon: Settings, label: "系统配置" },
]

const themeOptions = [
  { value: "light" as const, icon: Sun, label: "浅色" },
  { value: "dark" as const, icon: Moon, label: "深色" },
  { value: "system" as const, icon: Monitor, label: "跟随系统" },
]

// Sidebar 必须定义在模块级:写在组件内部会导致每次渲染重建、状态丢失
function Sidebar({
  siteTitle,
  mobile = false,
  onNavigate,
}: {
  siteTitle: string
  mobile?: boolean
  onNavigate: () => void
}) {
  return (
    <div
      className={cn(
        "flex flex-col h-full bg-card border-r border-border",
        mobile ? "w-64" : "w-60"
      )}
    >
      {/* Logo */}
      <div className="flex h-14 items-center gap-2 px-4 border-b border-border">
        <div className="h-7 w-7 rounded-lg bg-primary flex items-center justify-center">
          <Server className="h-4 w-4 text-white" />
        </div>
        <span className="font-bold text-base text-foreground">{siteTitle}</span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 p-3 space-y-1">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            onClick={onNavigate}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              )
            }
          >
            {({ isActive }) => (
              <>
                <Icon className="h-4 w-4" />
                <span className="flex-1">{label}</span>
                {isActive && <ChevronRight className="h-3 w-3" />}
              </>
            )}
          </NavLink>
        ))}
      </nav>
    </div>
  )
}

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [siteTitle, setSiteTitle] = useState("MediaUnlock")
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()

  useEffect(() => {
    commonApi.settings().then((res) => {
      if (res.data.data?.title) setSiteTitle(res.data.data.title)
    }).catch(() => {})
  }, [])

  const handleLogout = async () => {
    await authApi.logout()
    navigate("/login")
  }

  return (
    <div className="flex h-screen bg-background">
      {/* Desktop Sidebar */}
      <div className="hidden md:flex flex-shrink-0">
        <Sidebar siteTitle={siteTitle} onNavigate={() => setSidebarOpen(false)} />
      </div>

      {/* Mobile Sidebar Overlay */}
      {sidebarOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div
            className="absolute inset-0 bg-black/50"
            onClick={() => setSidebarOpen(false)}
          />
          <div className="relative z-50 h-full">
            <Sidebar mobile siteTitle={siteTitle} onNavigate={() => setSidebarOpen(false)} />
          </div>
        </div>
      )}

      {/* Main Content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top Header (desktop + mobile) */}
        <header className="flex h-14 items-center justify-between border-b border-border bg-card px-4">
          {/* Mobile: hamburger + logo */}
          <div className="flex items-center gap-3 md:hidden">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setSidebarOpen(!sidebarOpen)}
            >
              {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
            </Button>
            <div className="flex items-center gap-2">
              <div className="h-6 w-6 rounded-md bg-primary flex items-center justify-center">
                <Server className="h-3.5 w-3.5 text-white" />
              </div>
              <span className="font-bold text-sm">{siteTitle}</span>
            </div>
          </div>
          {/* Desktop: spacer */}
          <div className="hidden md:block" />

          {/* Right: theme + logout */}
          <div className="flex items-center gap-2">
            {/* Theme switcher */}
            <div className="flex items-center gap-0.5 rounded-lg p-1 bg-muted/60">
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
            {/* Logout */}
            <Button
              variant="ghost"
              size="sm"
              className="gap-2 text-muted-foreground hover:text-destructive"
              onClick={handleLogout}
            >
              <LogOut className="h-4 w-4" />
              <span className="hidden sm:inline">退出登录</span>
            </Button>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
