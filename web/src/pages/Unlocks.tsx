import { useState, useEffect, useCallback, useMemo, useRef } from "react"
import { History } from "lucide-react"
import {
  unlocksApi, nodesApi, platformsApi,
  type Unlock, type Node, type Platform,
} from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { useToast } from "@/components/ui/toast-context"
import { cn } from "@/lib/utils"

// 状态 → 文案 / 格子颜色 / 徽章配色(见 需求文档 3.3 unlocks)
const STATUS_META: Record<string, { label: string; square: string; badge: string }> = {
  "1": { label: "成功", square: "bg-emerald-500", badge: "border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400" },
  "2": { label: "受限", square: "bg-amber-500", badge: "border-transparent bg-amber-500/15 text-amber-600 dark:text-amber-400" },
  "3": { label: "不支持", square: "bg-zinc-400", badge: "border-transparent bg-muted text-muted-foreground" },
  "4": { label: "封禁", square: "bg-red-600", badge: "border-transparent bg-red-500/15 text-red-600 dark:text-red-400" },
  "5": { label: "失败", square: "bg-rose-500", badge: "border-transparent bg-rose-500/15 text-rose-600 dark:text-rose-400" },
  "6": { label: "异常", square: "bg-violet-500", badge: "border-transparent bg-violet-500/15 text-violet-600 dark:text-violet-400" },
  "-1": { label: "网络错误", square: "bg-orange-500", badge: "border-transparent bg-orange-500/15 text-orange-600 dark:text-orange-400" },
  "-2": { label: "错误", square: "bg-pink-500", badge: "border-transparent bg-pink-500/15 text-pink-600 dark:text-pink-400" },
}

const STATUS_ORDER = ["1", "2", "3", "4", "5", "6", "-1", "-2"]

function metaOf(status: number) {
  return (
    STATUS_META[String(status)] ?? {
      label: `未知(${status})`,
      square: "bg-zinc-300",
      badge: "border-transparent bg-muted text-muted-foreground",
    }
  )
}

// 后端返回 RFC3339(2026-09-12T18:14:06Z)→ 本地可读时间
function fmtTime(s: string) {
  if (!s) return "—"
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  const p = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// 时间范围。key 直接作为 GET /api/unlocks 的 range 参数,由后端按 created_at 过滤。
// "all" 不发 range 参数(等价于不限时间)。
const RANGES = [
  { key: "24h", label: "24小时" },
  { key: "7d", label: "7天" },
  { key: "all", label: "全部" },
] as const

type RangeKey = (typeof RANGES)[number]["key"]

const DEFAULT_RANGE: RangeKey = "24h"

// 每次加载多少平台;滚到底部自动加载下一页。
const PLATFORM_PAGE_SIZE = 20

type HoverState = { u: Unlock; x: number; y: number } | null

// HistorySquares 某平台的检测历史格子(左旧 → 右新);无记录时显示占位文案。
// 桌面端和移动端共用,避免两套渲染逻辑走样。
function HistorySquares({
  name,
  items,
  onHover,
}: {
  name: string
  items: Unlock[]
  onHover: (h: HoverState) => void
}) {
  if (items.length === 0) {
    return <span className="text-xs text-muted-foreground">该范围内无记录</span>
  }
  return (
    // 全部展示:flex-wrap 换行铺开,每个格子固定 16px(shrink-0),放不下就换到下一行
    <div className="flex min-w-0 flex-wrap items-center gap-1">
      {items.map((u) => (
        <button
          key={u.id}
          type="button"
          aria-label={`${name} ${metaOf(u.status).label} ${fmtTime(u.created_at)}`}
          className={cn(
            // 固定 16px:shrink-0 保证不被压缩,也不用 flex-1(避免被拉伸)。放不下时整行换行。
            "h-4 w-4 shrink-0 rounded-[3px] transition-transform hover:scale-125 hover:ring-2 hover:ring-ring hover:ring-offset-1 hover:ring-offset-background",
            metaOf(u.status).square
          )}
          onMouseEnter={(e) => {
            const r = e.currentTarget.getBoundingClientRect()
            onHover({ u, x: r.left + r.width / 2, y: r.top })
          }}
          onMouseLeave={() => onHover(null)}
        />
      ))}
    </div>
  )
}

export default function UnlocksPage() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [nodeId, setNodeId] = useState<string>("")
  const [range, setRange] = useState<RangeKey>(DEFAULT_RANGE)

  // 平台分页加载
  const [platforms, setPlatforms] = useState<Platform[]>([])
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(false)

  // platform_id → 该平台的检测记录(组内旧 → 新)
  const [records, setRecords] = useState<Map<number, Unlock[]>>(new Map())

  const [hover, setHover] = useState<{ u: Unlock; x: number; y: number } | null>(null)
  const sentinelRef = useRef<HTMLDivElement | null>(null)
  // 请求代次:每次发起加载自增。响应回来时若代次已变,说明被更新的请求取代,
  // 直接丢弃 —— 否则快速切换节点/时间范围时,先发后到的旧响应会把数据盖成错的。
  const genRef = useRef(0)

  const { toast } = useToast()

  // 节点列表(ListNodes 已 Preload 该节点已解锁的平台)
  useEffect(() => {
    nodesApi.listAll()
      .then((r) => {
        const items = r.data.data.items
        setNodes(items)
        if (items.length > 0) setNodeId(String(items[0].id))
      })
      .catch(() => toast({ title: "获取节点失败" }))
  }, [toast])

  const currentNode = useMemo(
    () => nodes.find((n) => String(n.id) === nodeId) ?? null,
    [nodes, nodeId]
  )

  const unlockedIds = useMemo(
    () => new Set((currentNode?.platforms ?? []).map((p) => p.id)),
    [currentNode]
  )

  // 加载一页平台,再用 platform_ids 一次把这批平台在该节点/时间窗内的记录全拉回来
  const loadPage = useCallback(async (target: number, reset: boolean) => {
    if (!nodeId) return
    const gen = ++genRef.current
    setLoading(true)
    try {
      const pRes = await platformsApi.list({ page: target, limit: PLATFORM_PAGE_SIZE })
      if (gen !== genRef.current) return // 已被更新请求取代,丢弃

      const p = pRes.data.data

      const grouped = new Map<number, Unlock[]>()
      if (p.items.length > 0) {
        const uRes = await unlocksApi.list({
          node_id: Number(nodeId),
          platform_ids: p.items.map((x) => x.id),
          // "all" 不发 range 参数:等价于后端不限时间
          range: range === "all" ? undefined : range,
        })
        if (gen !== genRef.current) return

        // 不分页:直接就是记录数组
        for (const u of uRes) {
          const arr = grouped.get(u.platform_id)
          if (arr) arr.push(u)
          else grouped.set(u.platform_id, [u])
        }
        // 后端按 id desc 返回;分组后反转,让每组内部是旧 → 新
        grouped.forEach((arr) => arr.reverse())
      }

      setPlatforms((prev) => (reset ? p.items : [...prev, ...p.items]))
      setRecords((prev) => {
        const next = reset ? new Map<number, Unlock[]>() : new Map(prev)
        grouped.forEach((v, k) => next.set(k, v))
        return next
      })
      setPage(target)
      setHasMore(target < p.pages)
    } catch {
      if (gen === genRef.current) toast({ title: "加载失败" })
    } finally {
      if (gen === genRef.current) setLoading(false)
    }
  }, [nodeId, range, toast])

  // 节点或时间范围变化 → 清空,回到第 1 页
  const reload = useCallback(async () => {
    setPlatforms([])
    setRecords(new Map())
    setHasMore(false)
    await loadPage(1, true)
  }, [loadPage])

  useEffect(() => {
    if (!nodeId) return
    setHover(null)
    void reload()
  }, [nodeId, range, reload])

  // 滚到底部自动加载下一页
  const loadMore = useCallback(async () => {
    if (!hasMore || loading) return
    await loadPage(page + 1, false)
  }, [hasMore, loading, loadPage, page])

  useEffect(() => {
    const el = sentinelRef.current
    if (!el || !hasMore) return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) void loadMore()
      },
      { rootMargin: "120px" }
    )
    io.observe(el)
    return () => io.disconnect()
  }, [hasMore, loadMore])

  // 平台名映射(悬浮详情用):已加载的平台 + 当前节点已解锁的平台
  const platformName = useMemo(() => {
    const m = new Map<number, string>()
    platforms.forEach((p) => m.set(p.id, p.name))
    ;(currentNode?.platforms ?? []).forEach((p) => m.set(p.id, p.name))
    return m
  }, [platforms, currentNode])

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground flex items-center gap-2">
            <History className="h-6 w-6 text-primary" />
            解锁历史
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            选择节点,查看所有平台的历次检测结果(每行一个平台,格子由旧到新)
          </p>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          {/* 时间范围 */}
          <div className="flex items-center self-start rounded-lg border bg-card p-0.5">
            {RANGES.map((r) => (
              <button
                key={r.key}
                type="button"
                onClick={() => setRange(r.key)}
                aria-pressed={range === r.key}
                className={cn(
                  "rounded-md px-3 py-1.5 text-sm transition-colors",
                  range === r.key
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {r.label}
              </button>
            ))}
          </div>

          <Select value={nodeId} onValueChange={setNodeId}>
            <SelectTrigger className="w-full sm:w-[220px]">
              <SelectValue placeholder="请选择节点" />
            </SelectTrigger>
            <SelectContent>
              {nodes.map((n) => (
                <SelectItem key={n.id} value={String(n.id)}>
                  {n.name || n.alias}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {nodes.length === 0 ? (
        <div className="rounded-xl border bg-card py-16 text-center text-muted-foreground shadow-sm">
          暂无节点,请先在「节点管理」中添加
        </div>
      ) : !nodeId ? (
        <div className="rounded-xl border bg-card py-16 text-center text-muted-foreground shadow-sm">
          请选择一个节点
        </div>
      ) : (
        <Card className="border-0 bg-transparent shadow-none sm:border sm:bg-card sm:shadow">
          {/* 移动端:去掉外层 Card 的边框/背景/阴影 —— 卡片列表自带 border,再套一层会变成"卡片套卡片"的双边框,桌面端 sm: 恢复.
              另外:无 CardHeader,表格直接开始(表头第一列写"平台",第二列放图例居右),与 nodes / platforms 页一致. */}
          <CardContent className="p-0">
            {loading && platforms.length === 0 ? (
              <div className="flex items-center justify-center gap-2 py-12 text-muted-foreground">
                <span className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                加载中...
              </div>
            ) : platforms.length === 0 ? (
              <div className="py-12 text-center text-muted-foreground">
                暂无平台,请先在「平台管理」中添加
              </div>
            ) : (
              <>
                {/* ===== 桌面:Table(图例在表头第二列居右) ===== */}
                <div className="hidden sm:block">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-56">平台名称</TableHead>
                        {/* 图例 —— 仅桌面端(移动端不展示:点格子后的悬浮详情里带状态文字) */}
                        <TableHead>
                          <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
                            {STATUS_ORDER.map((s) => {
                              const m = STATUS_META[s]
                              return (
                                <span
                                  key={s}
                                  className="flex items-center gap-1.5 text-xs text-muted-foreground"
                                >
                                  <span className={cn("h-3 w-3 rounded-[3px]", m.square)} />
                                  {m.label}
                                </span>
                              )
                            })}
                          </div>
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {platforms.map((p) => {
                        const items = records.get(p.id) ?? []
                        return (
                          <TableRow key={p.id}>
                            <TableCell className="font-medium">
                              <div className="flex items-center gap-2">
                                <span
                                  className={cn(
                                    "h-1.5 w-1.5 shrink-0 rounded-full",
                                    unlockedIds.has(p.id)
                                      ? "bg-emerald-500"
                                      : "bg-zinc-300 dark:bg-zinc-600"
                                  )}
                                  title={unlockedIds.has(p.id) ? "当前已解锁" : "当前未解锁"}
                                />
                                <span className="truncate" title={p.name}>
                                  {p.name}
                                </span>
                              </div>
                            </TableCell>

                            {/* 历史格子:左旧 → 右新 */}
                            <TableCell>
                              <HistorySquares name={p.name} items={items} onHover={setHover} />
                            </TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                </div>

                {/* ===== 移动端:卡片列表,上=平台名 / 下=解锁信息(与平台管理页一致) =====
                     不展示图例:状态文字在点格子后的悬浮详情里就有,省掉一整行高度 ===== */}
                <div className="space-y-3 sm:hidden">
                  {platforms.map((p) => {
                    const items = records.get(p.id) ?? []
                    return (
                      <div
                        key={p.id}
                        className="space-y-2 rounded-xl border bg-card p-4 shadow-sm"
                      >
                        {/* 上:平台名 + 解锁状态点 */}
                        <div className="flex items-center gap-2 font-medium text-foreground">
                          <span
                            className={cn(
                              "h-1.5 w-1.5 shrink-0 rounded-full",
                              unlockedIds.has(p.id)
                                ? "bg-emerald-500"
                                : "bg-zinc-300 dark:bg-zinc-600"
                            )}
                            title={unlockedIds.has(p.id) ? "当前已解锁" : "当前未解锁"}
                          />
                          <span className="truncate">{p.name}</span>
                        </div>

                        {/* 下:解锁信息格子 */}
                        <HistorySquares name={p.name} items={items} onHover={setHover} />
                      </div>
                    )
                  })}
                </div>
              </>
            )}

            {/* 无限滚动哨兵 */}
            <div
              ref={sentinelRef}
              className="flex items-center justify-center gap-2 border-t px-6 py-3 text-sm text-muted-foreground"
            >
              {loading && platforms.length > 0 ? (
                <>
                  <span className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                  加载中...
                </>
              ) : hasMore ? (
                <button
                  type="button"
                  className="underline underline-offset-4 hover:text-foreground"
                  onClick={() => void loadMore()}
                >
                  加载更多
                </button>
              ) : platforms.length > 0 ? (
                "已经到底了"
              ) : null}
            </div>
          </CardContent>
        </Card>
      )}

      {/* 悬浮详情 */}
      {hover && (
        <div
          className="pointer-events-none fixed z-50 w-56 rounded-lg border bg-popover p-3 text-popover-foreground shadow-lg"
          style={{
            left: Math.min(Math.max(hover.x, 120), window.innerWidth - 120),
            top: hover.y - 10,
            transform: "translate(-50%, -100%)",
          }}
        >
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium">
              {platformName.get(hover.u.platform_id) ?? `#${hover.u.platform_id}`}
            </span>
            <Badge className={metaOf(hover.u.status).badge}>
              {metaOf(hover.u.status).label}
            </Badge>
          </div>
          <dl className="mt-2 space-y-1 text-xs text-muted-foreground">
            <div className="flex justify-between gap-3">
              <dt>地区</dt>
              <dd className="text-foreground">{hover.u.region || "—"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt>时间</dt>
              <dd className="text-foreground">{fmtTime(hover.u.created_at)}</dd>
            </div>
            {(hover.u.err || hover.u.info) && (
              <div className="flex justify-between gap-3">
                <dt className="mb-0.5">说明</dt>
                <dd className="break-all text-foreground">{hover.u.err || hover.u.info}</dd>
              </div>
            )}
          </dl>
        </div>
      )}
    </div>
  )
}
