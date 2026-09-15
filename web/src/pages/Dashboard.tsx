import { useCallback, useEffect, useState } from "react"
import { Server, Globe, Link2, LayoutDashboard } from "lucide-react"
import { commonApi } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useToast } from "@/components/ui/toast-context"

interface Stats {
  node_count: number
  platform_count: number
  association_count: number
  active_node_count: number
}

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(false)
  const { toast } = useToast()

  const fetchStats = useCallback(async () => {
    setLoading(true)
    try {
      const res = await commonApi.stats()
      setStats(res.data.data)
    } catch {
      toast({ title: "获取统计数据失败" })
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  const cards = [
    {
      title: "节点总数",
      value: stats ? `${stats.active_node_count} / ${stats.node_count}` : undefined,
      icon: Server,
    },
    {
      title: "平台总数",
      value: stats?.platform_count,
      icon: Globe,
    },
    {
      title: "关联总数",
      value: stats?.association_count,
      icon: Link2,
    },
  ]

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground flex items-center gap-2">
            <LayoutDashboard className="h-6 w-6 text-primary" />
            仪表盘
          </h1>
          <p className="text-sm text-muted-foreground mt-1">查看系统运行状态和统计数据</p>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {cards.map(({ title, value, icon: Icon }) => (
          <Card key={title} className="shadow-sm hover:shadow-md transition-shadow">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
              <Icon className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-foreground">
                {loading || value === undefined ? (
                  <span className="inline-block h-8 w-16 animate-pulse rounded bg-muted" />
                ) : value}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
