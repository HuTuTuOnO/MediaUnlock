import { useState, useEffect, useCallback } from "react"
import {
  Plus, Search, Pencil, Trash2, Globe, ChevronLeft, ChevronRight, Eye
} from "lucide-react"
import { platformsApi, type Platform, type Node } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow
} from "@/components/ui/table"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle
} from "@/components/ui/dialog"
import { useToast } from "@/components/ui/toast-context"
import { cn, fmtTime } from "@/lib/utils"

const EMPTY_FORM = { name: "", rules: "", status: 1 }

export default function PlatformsPage() {
  const [platforms, setPlatforms] = useState<Platform[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [loading, setLoading] = useState(false)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [detailDialogOpen, setDetailDialogOpen] = useState(false)
  const [editingPlatform, setEditingPlatform] = useState<Platform | null>(null)
  const [deletingPlatform, setDeletingPlatform] = useState<Platform | null>(null)
  const [detailPlatform, setDetailPlatform] = useState<Platform | null>(null)
  const [detailNodes, setDetailNodes] = useState<Node[]>([])
  const [form, setForm] = useState({ ...EMPTY_FORM })
  const [saving, setSaving] = useState(false)

  const { toast } = useToast()
  const limit = 10

  const fetchPlatforms = useCallback(async () => {
    setLoading(true)
    try {
      const res = await platformsApi.list({ page, limit, search })
      const d = res.data.data
      setPlatforms(d.items)
      setTotal(d.total)
      setPages(d.pages)
    } catch {
      toast({ title: "获取平台失败" })
    } finally {
      setLoading(false)
    }
  }, [page, search, toast])

  useEffect(() => { fetchPlatforms() }, [fetchPlatforms])

  const handleSearch = () => {
    setSearch(searchInput)
    setPage(1)
  }

  const openCreate = () => {
    setEditingPlatform(null)
    setForm({ ...EMPTY_FORM })
    setDialogOpen(true)
  }

  const openEdit = (p: Platform) => {
    setEditingPlatform(p)
    setForm({ name: p.name, rules: p.rules || "", status: p.status })
    setDialogOpen(true)
  }

  const openDelete = (p: Platform) => {
    setDeletingPlatform(p)
    setDeleteDialogOpen(true)
  }

  const openDetail = (p: Platform) => {
    setDetailPlatform(p)
    setDetailNodes(p.nodes)
    setDetailDialogOpen(true)
  }

  const handleSave = async () => {
    if (!form.name) {
      toast({ title: "请填写平台名称" })
      return
    }
    setSaving(true)
    try {
      if (editingPlatform) {
        await platformsApi.update(editingPlatform.id, form)
        toast({ title: "平台更新成功" })
      } else {
        await platformsApi.create(form)
        toast({ title: "平台创建成功" })
      }
      setDialogOpen(false)
      fetchPlatforms()
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { msg?: string } } })?.response?.data?.msg || "操作失败"
      toast({ title: msg })
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deletingPlatform) return
    setSaving(true)
    try {
      await platformsApi.delete(deletingPlatform.id)
      toast({ title: "平台删除成功" })
      setDeleteDialogOpen(false)
      fetchPlatforms()
    } catch {
      toast({ title: "删除失败" })
    } finally {
      setSaving(false)
    }
  }

  const parseRules = (rules: string) =>
    rules ? rules.split(",").map((r) => r.trim()).filter(Boolean) : []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground flex items-center gap-2">
            <Globe className="h-6 w-6 text-primary" />
            平台管理
          </h1>
          <p className="text-sm text-muted-foreground mt-1">共 {total} 个流媒体平台</p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <div className="flex gap-2">
            <div className="relative flex-1 sm:flex-none">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-9 sm:w-56"
                placeholder="搜索平台名称或规则..."
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSearch()}
              />
            </div>
            <Button variant="outline" onClick={handleSearch}>搜索</Button>
            {search && (
              <Button variant="ghost" onClick={() => { setSearch(""); setSearchInput(""); setPage(1) }}>清除</Button>
            )}
          </div>
          <Button onClick={openCreate} className="gap-2 w-full sm:w-auto">
            <Plus className="h-4 w-4" />
            添加平台
          </Button>
        </div>
      </div>

      {/* Table - desktop */}
      <div className="hidden sm:block rounded-xl border bg-card shadow-sm">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">ID</TableHead>
              <TableHead>平台名称</TableHead>
              <TableHead>路由规则</TableHead>
              <TableHead className="text-center">状态</TableHead>
              <TableHead>关联节点</TableHead>
              <TableHead>更新时间</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-12 text-muted-foreground">
                  <div className="flex items-center justify-center gap-2">
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                    加载中...
                  </div>
                </TableCell>
              </TableRow>
            ) : platforms.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-12 text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              platforms.map((p) => {
                const rules = parseRules(p.rules)
                return (
<TableRow key={p.id}>
                  <TableCell className="text-muted-foreground text-xs">{p.id}</TableCell>
                  <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1 max-w-[300px]">
                        {rules.slice(0, 1).map((r) => (
                          <Badge key={r} variant="outline" className="text-xs font-mono">{r}</Badge>
                        ))}
                        {rules.length > 1 && (
                          <Badge variant="secondary" className="text-xs">+{rules.length - 1}</Badge>
                        )}
                        {rules.length === 0 && <span className="text-muted-foreground text-xs">无规则</span>}
                      </div>
                    </TableCell>
                  <TableCell className="text-center">
                    <Badge variant={p.status === 1 ? "default" : "outline"}>
                      {p.status === 1 ? "开启" : "关闭"}
                    </Badge>
                  </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1 max-w-[200px]">
                        {p.nodes && p.nodes.length > 0 ? (
                          <>
                            {p.nodes.slice(0, 1).map((n) => (
                              <Badge key={n.id} variant="secondary" className="text-xs">{n.name || n.alias}</Badge>
                            ))}
                            {p.nodes.length > 1 && (
                              <Badge variant="outline" className="text-xs">+{p.nodes.length - 1}</Badge>
                            )}
                          </>
                        ) : (
                          <span className="text-muted-foreground text-xs">无</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">{fmtTime(p.updated_at)}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:text-foreground"
                          onClick={() => openDetail(p)}
                        >
                          <Eye className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:text-foreground"
                          onClick={() => openEdit(p)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:text-destructive"
                          onClick={() => openDelete(p)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Card list - mobile only */}
      <div className="sm:hidden space-y-3">
        {loading ? (
          <div className="flex items-center justify-center py-12 text-muted-foreground gap-2">
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
            加载中...
          </div>
        ) : platforms.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground">暂无数据</div>
        ) : (
          platforms.map((p) => {
            const rules = parseRules(p.rules)
            return (
              <div key={p.id} className="rounded-xl border bg-card p-4 space-y-2 shadow-sm">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2 font-medium text-foreground">
                    <span>{p.name}</span>
                    <Badge variant={p.status === 1 ? "default" : "outline"}>
                      {p.status === 1 ? "开启" : "关闭"}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground" onClick={() => openDetail(p)}>
                      <Eye className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground" onClick={() => openEdit(p)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => openDelete(p)}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
                {rules.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {rules.slice(0, 4).map((r) => (
                      <Badge key={r} variant="outline" className="text-xs font-mono">{r}</Badge>
                    ))}
                    {rules.length > 4 && <Badge variant="secondary" className="text-xs">+{rules.length - 4}</Badge>}
                  </div>
                )}
                {p.nodes && p.nodes.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {p.nodes.slice(0, 3).map((n) => (
                      <Badge key={n.id} variant="secondary" className="text-xs">{n.name || n.alias}</Badge>
                    ))}
                    {p.nodes.length > 3 && <Badge variant="outline" className="text-xs">+{p.nodes.length - 3}</Badge>}
                  </div>
                )}
              </div>
            )
          })
        )}
      </div>

      {/* Pagination */}
      {pages > 1 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>第 {page} / {pages} 页，共 {total} 条</span>
          <div className="flex gap-1">
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            {Array.from({ length: Math.min(5, pages) }, (_, i) => {
              const p = Math.max(1, Math.min(pages - 4, page - 2)) + i
              return p <= pages ? (
                <Button
                  key={p}
                  variant={p === page ? "default" : "outline"}
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => setPage(p)}
                >
                  {p}
                </Button>
              ) : null
            })}
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              disabled={page >= pages}
              onClick={() => setPage(page + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{editingPlatform ? "编辑平台" : "添加平台"}</DialogTitle>
            <DialogDescription>
              {editingPlatform ? "修改平台配置信息" : "添加新的流媒体平台"}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="pname">平台名称 <span className="text-destructive">*</span></Label>
              <Input
                id="pname"
                placeholder="如：Netflix"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="rules">
                路由规则
                <span className="text-muted-foreground font-normal ml-1">（逗号分隔）</span>
              </Label>
              <Textarea
                id="rules"
                placeholder={"geosite:netflix,domain:google.com"}
                value={form.rules}
                onChange={(e) => setForm({ ...form, rules: e.target.value })}
                rows={4}
                className="font-mono text-xs"
              />
              {form.rules && (
                <p className="text-xs text-muted-foreground">
                  共 {parseRules(form.rules).length} 条规则
                </p>
              )}
            </div>
            <div className="space-y-2">
              <Label>平台状态</Label>
              <div className="flex h-9 items-center gap-3 rounded-md border border-input px-3">
                <span className={cn("flex-1 text-sm", form.status === 1 ? "text-foreground" : "text-muted-foreground")}>
                  {form.status === 1 ? "开启" : "关闭"}
                </span>
                <Switch
                  checked={form.status === 1}
                  onCheckedChange={(checked) => setForm({ ...form, status: checked ? 1 : 0 })}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? "保存中..." : "保存"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认删除</DialogTitle>
            <DialogDescription>
              确定要删除平台 <span className="font-semibold text-foreground">"{deletingPlatform?.name}"</span> 吗？此操作不可撤销。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>取消</Button>
            <Button variant="destructive" onClick={handleDelete} disabled={saving}>
              {saving ? "删除中..." : "确认删除"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Detail Dialog */}
      <Dialog open={detailDialogOpen} onOpenChange={setDetailDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{detailPlatform?.name}</DialogTitle>
            <DialogDescription>平台详情信息</DialogDescription>
          </DialogHeader>
          {detailPlatform && (
            <div className="space-y-4">
              <div>
                <p className="text-sm font-medium mb-2">路由规则</p>
                <div className="flex flex-wrap gap-1.5 p-3 bg-muted/50 rounded-lg">
                  {parseRules(detailPlatform.rules).map((r) => (
                    <Badge key={r} variant="outline" className="font-mono text-xs">{r}</Badge>
                  ))}
                  {parseRules(detailPlatform.rules).length === 0 && (
                    <span className="text-muted-foreground text-sm">暂无规则</span>
                  )}
                </div>
              </div>
              <div>
                <p className="text-sm font-medium mb-2">关联节点（{detailNodes.length}）</p>
                <div className="flex flex-wrap gap-1.5">
                  {detailNodes.map((n) => (
                    <Badge key={n.id} variant="secondary">{n.name || n.alias}</Badge>
                  ))}
                  {detailNodes.length === 0 && (
                    <span className="text-muted-foreground text-sm">暂无关联节点</span>
                  )}
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setDetailDialogOpen(false)}>关闭</Button>
            <Button onClick={() => { setDetailDialogOpen(false); openEdit(detailPlatform!) }}>
              编辑
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
