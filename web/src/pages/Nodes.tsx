import { useState, useEffect, useCallback } from "react"
import {
  Plus, Search, Pencil, Trash2, Server, ChevronLeft, ChevronRight,
  Copy, RefreshCw, Eye
} from "lucide-react"
import { nodesApi, type Node } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
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

// 节点类型只支持 socks5 / http —— 与 Server 端 models 常量、agent 的 GOST handler 一一对应。
// (曾出现过 ss / trojan 选项,但 agent 侧没有对应实现,选了也起不来,已移除。)
const NODE_CONFIG: Record<string, {
  label: string
  fields: { key: string; label: string; type?: string; placeholder?: string; options?: readonly string[] }[]
}> = ({
  socks5: {
    label: "SOCKS5",
    fields: [
      { key: "value1", label: "账号", placeholder: "代理用户名（可选）" },
      { key: "value2", label: "密码", placeholder: "代理密码（可选）" },
    ],
  },
  http: {
    label: "HTTP/HTTPS",
    fields: [
      { key: "value1", label: "账号", placeholder: "代理用户名（可选）" },
      { key: "value2", label: "密码", placeholder: "代理密码（可选）" },
    ],
  },
})

const EMPTY_FORM = {
  name: "", alias: "", type: "socks5", host: "", port: "",
  value1: "", value2: "", value3: "", value4: "", value5: "", value6: "",
  status: 1,
}

export default function NodesPage() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [loading, setLoading] = useState(false)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [editingNode, setEditingNode] = useState<Node | null>(null)
  const [deletingNode, setDeletingNode] = useState<Node | null>(null)
  // 详情弹窗:列表不显示 Token,点「查看详情」才看(含复制与重置)
  const [detailNode, setDetailNode] = useState<Node | null>(null)
  // 重置 Token 的二次确认弹窗(不用浏览器原生 window.confirm)
  const [retokenNode, setRetokenNode] = useState<Node | null>(null)
  const [form, setForm] = useState({ ...EMPTY_FORM })
  const [saving, setSaving] = useState(false)

  const { toast } = useToast()
  const limit = 10

  const fetchNodes = useCallback(async () => {
    setLoading(true)
    try {
      const res = await nodesApi.list({ page, limit, search })
      const d = res.data.data
      setNodes(d.items)
      setTotal(d.total)
      setPages(d.pages)
    } catch {
      toast({ title: "获取节点失败" })
    } finally {
      setLoading(false)
    }
  }, [page, search, toast])

  useEffect(() => { fetchNodes() }, [fetchNodes])

  const handleSearch = () => {
    setSearch(searchInput)
    setPage(1)
  }

  const openCreate = () => {
    setEditingNode(null)
    setForm({ ...EMPTY_FORM })
    setDialogOpen(true)
  }

  const openEdit = (node: Node) => {
    setEditingNode(node)
    setForm({
      name: node.name || "",
      alias: node.alias || "",
      type: node.type || "socks5",
      host: node.host || "",
      port: String(node.port) || "",
      value1: node.value1 || "",
      value2: node.value2 || "",
      value3: node.value3 || "",
      value4: node.value4 || "",
      value5: node.value5 || "",
      value6: node.value6 || "",
      status: node.status,
    })
    setDialogOpen(true)
  }

  const openDelete = (node: Node) => {
    setDeletingNode(node)
    setDeleteDialogOpen(true)
  }

  const handleSave = async () => {
    if (!form.name || !form.alias || !form.type || !form.host || !form.port) {
      toast({ title: "请填写必填字段" })
      return
    }
    setSaving(true)
    try {
      const payload = { ...form, port: Number(form.port) }
      if (editingNode) {
        await nodesApi.update(editingNode.id, payload)
        toast({ title: "节点更新成功" })
      } else {
        await nodesApi.create(payload)
        toast({ title: "节点创建成功" })
      }
      setDialogOpen(false)
      fetchNodes()
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { msg?: string } } })?.response?.data?.msg || "操作失败"
      toast({ title: msg })
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deletingNode) return
    setSaving(true)
    try {
      await nodesApi.delete(deletingNode.id)
      toast({ title: "节点删除成功" })
      setDeleteDialogOpen(false)
      fetchNodes()
    } catch {
      toast({ title: "删除失败" })
    } finally {
      setSaving(false)
    }
  }

  // 复制节点的 Agent Token 到剪贴板
  const copyToken = (token: string) => {
    if (!token) return
    navigator.clipboard.writeText(token)
    toast({ title: "Token 已复制到剪贴板" })
  }

  // 重置节点 Token:旧的立即失效,该节点上的 Agent 需要换用新 Token。
  // 重置后同步刷新详情弹窗里显示的值,并重新拉列表。
  const handleRetoken = async () => {
    if (!retokenNode) return
    setSaving(true)
    try {
      const res = await nodesApi.retoken(retokenNode.id)
      const newToken = res.data.data?.token
      toast({ title: "Token 重置成功", description: newToken ? `新 Token：${newToken}` : undefined })
      // 详情弹窗开着的话同步显示新值
      setDetailNode((prev) =>
        prev && prev.id === retokenNode.id ? { ...prev, token: newToken ?? prev.token } : prev
      )
      setRetokenNode(null)
      fetchNodes()
    } catch {
      toast({ title: "重置失败" })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground flex items-center gap-2">
            <Server className="h-6 w-6 text-primary" />
            节点管理
          </h1>
          <p className="text-sm text-muted-foreground mt-1">共 {total} 个节点</p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          {/* Search */}
          <div className="flex gap-2">
            <div className="relative flex-1 sm:flex-none">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-9 sm:w-56"
                placeholder="搜索名称、标识、地址..."
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
            添加节点
          </Button>
        </div>
      </div>

      {/* Table - desktop / Card list - mobile */}
      <div className="hidden sm:block rounded-xl border bg-card shadow-sm">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">ID</TableHead>
              <TableHead>节点名称</TableHead>
              <TableHead>标识</TableHead>
              <TableHead className="text-center">类型</TableHead>
              <TableHead>地址</TableHead>
              <TableHead className="text-center">状态</TableHead>
              <TableHead>关联平台</TableHead>
              <TableHead>上传时间</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={9} className="text-center py-12 text-muted-foreground">
                  <div className="flex items-center justify-center gap-2">
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                    加载中...
                  </div>
                </TableCell>
              </TableRow>
            ) : nodes.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="text-center py-12 text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              nodes.map((node) => (
                <TableRow key={node.id}>
                  <TableCell className="text-muted-foreground text-xs">{node.id}</TableCell>
                  <TableCell className="font-medium">{node.name}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{node.alias}</TableCell>
                  <TableCell className="text-center">
                    <Badge variant="secondary" className="text-xs">{NODE_CONFIG[node.type]?.label ?? node.type}</Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">{node.host}:{node.port}</TableCell>
                  <TableCell className="text-center">
                    <Badge variant={node.status === 1 ? "default" : "outline"}>
                      {node.status === 1 ? "开启" : "关闭"}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1 max-w-[200px]">
                      {node.platforms.length > 0 ? (
                        node.platforms.slice(0, 1).map((p) => (
                          <Badge key={p.id} variant="outline" className="text-xs">{p.name}</Badge>
                        ))
                      ) : (
                        <span className="text-muted-foreground text-xs">无</span>
                      )}
                      {node.platforms.length > 1 && (
                        <Badge variant="outline" className="text-xs">+{node.platforms.length - 1}</Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {fmtTime(node.report_at)}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={() => setDetailNode(node)}
                        title="查看详情"
                      >
                        <Eye className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={() => openEdit(node)}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={() => openDelete(node)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
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
        ) : nodes.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground">暂无数据</div>
        ) : (
          nodes.map((node) => (
            <div key={node.id} className="rounded-xl border bg-card p-4 space-y-2 shadow-sm">
              <div className="flex items-start justify-between">
                <div>
                  <div className="font-medium text-foreground">{node.name}</div>
                  <div className="text-xs text-muted-foreground font-mono mt-0.5">{node.alias}</div>
                </div>
                <div className="flex items-center gap-1">
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground" onClick={() => setDetailNode(node)} title="查看详情">
                    <Eye className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground" onClick={() => openEdit(node)}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => openDelete(node)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <Badge variant="secondary">{NODE_CONFIG[node.type]?.label ?? node.type}</Badge>
                <Badge variant={node.status === 1 ? "default" : "outline"}>{node.status === 1 ? "开启" : "关闭"}</Badge>
                <span>{node.host}:{node.port}</span>
              </div>
              {node.platforms.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {node.platforms.slice(0, 3).map((p) => (
                    <Badge key={p.id} variant="outline" className="text-xs">{p.name}</Badge>
                  ))}
                  {node.platforms.length > 3 && (
                    <Badge variant="outline" className="text-xs">+{node.platforms.length - 3}</Badge>
                  )}
                </div>
              )}
            </div>
          ))
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
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingNode ? "编辑节点" : "添加节点"}</DialogTitle>
            <DialogDescription>
              {editingNode ? "修改节点配置信息" : "填写新节点的配置信息"}
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="name">节点名称 <span className="text-destructive">*</span></Label>
              <Input
                id="name"
                placeholder="如：AKILE-香港-01"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="alias">节点标识 <span className="text-destructive">*</span></Label>
              <Input
                id="alias"
                placeholder="如：HKAK1"
                value={form.alias}
                onChange={(e) => setForm({ ...form, alias: e.target.value })}
                disabled={!!editingNode}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="type">节点类型 <span className="text-destructive">*</span></Label>
              <Select
                value={form.type}
                onValueChange={(v) => setForm({ ...form, type: v, value1: "", value2: "", value3: "", value4: "", value5: "", value6: "" })}
              >
                <SelectTrigger id="type">
                  <SelectValue placeholder="选择类型" />
                </SelectTrigger>
                <SelectContent>
                  {Object.keys(NODE_CONFIG).map((t) => (
                    <SelectItem key={t} value={t}>{NODE_CONFIG[t].label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>节点状态</Label>
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
            <div className="space-y-2">
              <Label htmlFor="host">节点地址 <span className="text-destructive">*</span></Label>
              <Input
                id="host"
                placeholder="如：akhk01.example.com"
                value={form.host}
                onChange={(e) => setForm({ ...form, host: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="port">端口 <span className="text-destructive">*</span></Label>
              <Input
                id="port"
                type="number"
                placeholder="1-65535"
                value={form.port}
                onChange={(e) => setForm({ ...form, port: e.target.value })}
              />
            </div>
            {(NODE_CONFIG[form.type]?.fields ?? []).map((field) => (
              <div key={field.key} className="space-y-2">
                <Label htmlFor={field.key}>{field.label}</Label>
                {field.type === "checkbox" ? (
                  <div className="flex h-9 items-center gap-3 rounded-md border border-input px-3">
                    <span className={cn("flex-1 text-sm", form[field.key as keyof typeof form] === "true" ? "text-foreground" : "text-muted-foreground")}>
                      {form[field.key as keyof typeof form] === "true" ? "是" : "否"}
                    </span>
                    <Switch
                      checked={form[field.key as keyof typeof form] === "true"}
                      onCheckedChange={(checked) => setForm({ ...form, [field.key]: checked ? "true" : "false" })}
                    />
                  </div>
                ) : field.options ? (
                  <Select
                    value={form[field.key as keyof typeof form] as string}
                    onValueChange={(v) => setForm({ ...form, [field.key]: v })}
                  >
                    <SelectTrigger id={field.key}>
                      <SelectValue placeholder="选择加密方式" />
                    </SelectTrigger>
                    <SelectContent>
                      {field.options.map((opt) => (
                        <SelectItem key={opt} value={opt}>{opt}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Input
                    id={field.key}
                    placeholder={field.placeholder}
                    value={form[field.key as keyof typeof form] as string}
                    onChange={(e) => setForm({ ...form, [field.key]: e.target.value })}
                  />
                )}
              </div>
            ))}
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
              确定要删除节点 <span className="font-semibold text-foreground">"{deletingNode?.name}"</span> 吗？此操作不可撤销。
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

      {/* 详情弹窗:列表不显示 Token,这里才展示,并且 Token 后带刷新图标可重置 */}
      <Dialog open={!!detailNode} onOpenChange={(open) => { if (!open) setDetailNode(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>节点详情</DialogTitle>
            <DialogDescription>{detailNode?.name}</DialogDescription>
          </DialogHeader>

          {/* 详情只放 Token:值 + 复制图标 + 刷新图标(重置)。值不用 code 包裹。 */}
          {detailNode && (
            <>
              <p className="text-sm font-medium">Token</p>
              <div className="flex items-center gap-1">
                <Input
                  value={detailNode.token || ""}
                  disabled
                  readOnly
                  className="flex-1 font-mono text-xs"
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
                  onClick={() => copyToken(detailNode.token)}
                  title="复制 Token"
                >
                  <Copy className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground hover:text-primary"
                  onClick={() => setRetokenNode(detailNode)}
                  disabled={saving}
                  title="重置 Token"
                >
                  <RefreshCw className={cn("h-4 w-4", saving && "animate-spin")} />
                </Button>
              </div>
            </>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setDetailNode(null)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 重置 Token 的二次确认 */}
      <Dialog open={!!retokenNode} onOpenChange={(open) => { if (!open) setRetokenNode(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>重置 Token</DialogTitle>
            <DialogDescription>
              确定要重置节点 <span className="font-semibold text-foreground">"{retokenNode?.name}"</span> 的 Token 吗？
            </DialogDescription>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            重置后旧 Token 立即失效,该节点上正在运行的 Agent 需要更新配置才能继续上报。
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRetokenNode(null)}>取消</Button>
            <Button variant="destructive" onClick={handleRetoken} disabled={saving}>
              {saving ? "重置中..." : "确认重置"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

    </div>
  )
}
