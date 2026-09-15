import { useState, useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { Settings, Key, RefreshCw, Lock, Eye, EyeOff, Save } from "lucide-react"
import { settingsApi, authApi, type Settings as SettingsType } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle
} from "@/components/ui/dialog"
import { useToast } from "@/components/ui/toast-context"

export default function SettingsPage() {
  const [settings, setSettings] = useState<SettingsType | null>(null)
  const [title, setTitle] = useState("")
  const [retentionDays, setRetentionDays] = useState("")
  const [showToken, setShowToken] = useState(false)
  const [saving, setSaving] = useState(false)
  const [regenDialogOpen, setRegenDialogOpen] = useState(false)
  const [pwdForm, setPwdForm] = useState({ new: "", confirm: "" })
  const [pwdSaving, setPwdSaving] = useState(false)

  const { toast } = useToast()
  const navigate = useNavigate()

  useEffect(() => {
    settingsApi.get().then((res) => {
      const d = res.data.data
      setSettings(d)
      setTitle(d.title || "")
      setRetentionDays(d.retention_days || "")
    }).catch(() => {
      toast({ title: "获取配置失败" })
    })
  }, [toast])

  const handleSaveTitle = async () => {
    setSaving(true)
    try {
      await settingsApi.update({ title })
      toast({ title: "标题更新成功" })
      setSettings((prev) => prev ? { ...prev, title } : prev)
    } catch {
      toast({ title: "更新失败" })
    } finally {
      setSaving(false)
    }
  }

  const handleSaveRetention = async () => {
    const n = Number(retentionDays)
    if (!Number.isInteger(n) || n < 0) {
      toast({ title: "保留天数必须是非负整数" })
      return
    }
    setSaving(true)
    try {
      await settingsApi.update({ retention_days: String(n) })
      toast({ title: "数据保留设置已保存" })
      setSettings((prev) => prev ? { ...prev, retention_days: String(n) } : prev)
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { msg?: string } } })?.response?.data?.msg || "保存失败"
      toast({ title: msg })
    } finally {
      setSaving(false)
    }
  }

  const handleRegenToken = async () => {
    setSaving(true)
    try {
      const res = await settingsApi.regenerateToken()
      const newToken = res.data.data.token
      setSettings((prev) => prev ? { ...prev, token: newToken } : prev)
      setRegenDialogOpen(false)
      toast({ title: "Token 重新生成成功" })
    } catch {
      toast({ title: "操作失败" })
    } finally {
      setSaving(false)
    }
  }

  const handleChangePwd = async () => {
    if (!pwdForm.new || !pwdForm.confirm) {
      toast({ title: "请填写所有字段" })
      return
    }
    if (pwdForm.new !== pwdForm.confirm) {
      toast({ title: "两次输入的新密码不一致" })
      return
    }
    if (pwdForm.new.length < 6) {
      toast({ title: "新密码长度不能少于6位" })
      return
    }
    setPwdSaving(true)
    try {
      await authApi.changePassword(pwdForm.new)
      toast({ title: "密码修改成功，请重新登录" })
      setPwdForm({ new: "", confirm: "" })
      await authApi.logout()
      setTimeout(() => navigate("/login"), 1200)
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { msg?: string } } })?.response?.data?.msg || "修改失败"
      toast({ title: msg })
    } finally {
      setPwdSaving(false)
    }
  }

  const copyToken = () => {
    if (settings?.token) {
      navigator.clipboard.writeText(settings.token)
      toast({ title: "Token 已复制到剪贴板" })
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-foreground flex items-center gap-2">
          <Settings className="h-6 w-6 text-primary" />
          系统配置
        </h1>
        <p className="text-sm text-muted-foreground mt-1">管理系统基础配置和数据</p>
      </div>

      <div className="columns-1 lg:columns-2 gap-6 space-y-6">

      {/* Basic Settings */}
      <Card className="break-inside-avoid">
        <CardHeader>
          <CardTitle className="text-base">基础设置</CardTitle>
          <CardDescription>系统显示名称等基础配置</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="title">系统标题</Label>
            <div className="flex gap-2">
              <Input
                id="title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="MediaUnlock 流媒体解锁"
              />
              <Button onClick={handleSaveTitle} disabled={saving} className="gap-2 shrink-0">
                <Save className="h-4 w-4" />
                保存
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 数据保留 */}
      <Card className="break-inside-avoid">
        <CardHeader>
          <CardTitle className="text-base">数据保留</CardTitle>
          <CardDescription>
            只保留最近 N 天的检测记录，超期数据由服务端定时清理（启动清一次，之后每 6 小时一次）
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="retention">保留天数</Label>
            <div className="flex gap-2">
              <Input
                id="retention"
                type="number"
                min={0}
                value={retentionDays}
                onChange={(e) => setRetentionDays(e.target.value)}
                placeholder="7"
              />
              <Button onClick={handleSaveRetention} disabled={saving} className="gap-2 shrink-0">
                <Save className="h-4 w-4" />
                保存
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* API Token */}
      <Card className="break-inside-avoid">
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Key className="h-4 w-4" />
            API Token
          </CardTitle>
          <CardDescription>client 模式 Agent 拉取下发数据的全局只读 Token</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>当前 Token</Label>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Input
                  type={showToken ? "text" : "password"}
                  value={settings?.token || ""}
                  readOnly
                  className="pr-10 font-mono text-xs"
                />
                <button
                  type="button"
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowToken(!showToken)}
                >
                  {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <Button variant="outline" onClick={copyToken} className="shrink-0">
                复制
              </Button>
              <Button
                variant="outline"
                className="gap-2 shrink-0 text-destructive border-destructive/30 hover:bg-destructive/5"
                onClick={() => setRegenDialogOpen(true)}
              >
                <RefreshCw className="h-4 w-4" />
                重新生成
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Security */}
      <Card className="break-inside-avoid">
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Lock className="h-4 w-4" />
            安全设置
          </CardTitle>
          <CardDescription>管理账户密码</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {/* 同一行:新密码 + 确认密码 + 提交按钮(不再用弹窗)。
              sm:items-end 让提交按钮与输入框底部对齐(因为上方有 Label)。 */}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="space-y-2 sm:flex-1">
              <Label htmlFor="newPwd">输入新密码</Label>
              <Input
                id="newPwd"
                type="password"
                value={pwdForm.new}
                onChange={(e) => setPwdForm({ ...pwdForm, new: e.target.value })}
                placeholder="请输入新密码"
              />
            </div>
            <div className="space-y-2 sm:flex-1">
              <Label htmlFor="confirmPwd">确认新密码</Label>
              <Input
                id="confirmPwd"
                type="password"
                value={pwdForm.confirm}
                onChange={(e) => setPwdForm({ ...pwdForm, confirm: e.target.value })}
                placeholder="请确认新密码"
              />
            </div>
            <Button onClick={handleChangePwd} disabled={pwdSaving} className="gap-2 shrink-0">
              <Save className="h-4 w-4" />
              {pwdSaving ? "提交中..." : "提交"}
            </Button>
          </div>
        </CardContent>
      </Card>

      </div>{/* end grid */}

      {/* Regen Token Dialog */}
      <Dialog open={regenDialogOpen} onOpenChange={setRegenDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>重新生成 Token</DialogTitle>
            <DialogDescription>
              重新生成后，旧 Token 将立即失效，所有使用旧 Token 的接入方需要更新。确认继续？
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRegenDialogOpen(false)}>取消</Button>
            <Button variant="destructive" onClick={handleRegenToken} disabled={saving}>
              {saving ? "生成中..." : "确认生成"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

    </div>
  )
}
