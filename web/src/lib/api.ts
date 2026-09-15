import axios, { type AxiosResponse } from "axios"

const TOKEN_KEY = "token"

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

const api = axios.create({
  baseURL: "/api",
  headers: { "Content-Type": "application/json" },
})

// 请求:附带 JWT
api.interceptors.request.use((config) => {
  const token = getToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// 响应:401 清 token 并跳登录(HashRouter → #/login)
api.interceptors.response.use(
  (res) => res,
  (err) => {
    const url: string = err.config?.url || ""
    if (err.response?.status === 401 && !url.includes("/auth/login")) {
      localStorage.removeItem(TOKEN_KEY)
      if (!window.location.hash.startsWith("#/login")) {
        window.location.href = "/#/login"
      }
    }
    return Promise.reject(err)
  }
)

export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T
}

export interface PaginatedData<T> {
  items: T[]
  total: number
  page: number
  limit: number
  pages: number
}

// 后端分页原始结构 { total, limit, offset, items }
interface RawPaged<T> {
  total: number
  limit: number
  offset: number
  items: T[]
}

// adaptPage 把后端 offset 分页转换成前端使用的 page/pages 结构。
function adaptPage<T>(
  res: AxiosResponse<ApiResponse<RawPaged<T>>>,
  page: number
): AxiosResponse<ApiResponse<PaginatedData<T>>> {
  const d = res.data.data
  // 后端 limit < 0(回 -1)表示"未限制":不计算 pages,直接当 1 页
  const limit = d?.limit ?? 1
  const total = d?.total ?? 0
  const adapted = res as unknown as AxiosResponse<ApiResponse<PaginatedData<T>>>
  adapted.data.data = {
    items: d?.items ?? [],
    total,
    page,
    limit,
    pages: limit > 0 ? Math.max(1, Math.ceil(total / limit)) : 1,
  }
  return adapted
}

export interface Node {
  id: number
  name: string
  alias: string
  type: string
  host: string
  port: number
  value1: string | null
  value2: string | null
  value3: string | null
  value4: string | null
  value5: string | null
  value6: string | null
  status: number
  /** 节点独立的 Agent 认证 Token(后端已放开,列表接口直接下发) */
  token: string
  report_at: string | null
  platforms: Platform[]
  created_at: string
  updated_at: string
}

export interface Platform {
  id: number
  name: string
  rules: string
  nodes: Node[]
  status: number
  created_at: string
  updated_at: string
}

export interface Settings {
  title: string
  token: string
  /** unlocks 保留天数(字符串形式;0 = 不自动清理)*/
  retention_days?: string
}

export interface Stats {
  node_count: number
  platform_count: number
  association_count: number
  active_node_count: number
}

export interface Unlock {
  id: number
  node_id: number
  platform_id: number
  status: number
  region: string
  info: string
  err: string
  created_at: string
  updated_at: string
}

export const authApi = {
  login: async (username: string, password: string) => {
    const res = await api.post<
      ApiResponse<{ token: string; user: { id: number; username: string } }>
    >("/auth/login", { username, password })
    const token = res.data?.data?.token
    if (token) localStorage.setItem(TOKEN_KEY, token)
    return res
  },
  logout: () => {
    localStorage.removeItem(TOKEN_KEY)
    return Promise.resolve()
  },
  me: () => api.get<ApiResponse<{ user_id: number }>>("/userinfo"),
  changePassword: (new_password: string) =>
    api.post<ApiResponse>("/auth/change-password", { new_password }),
}

export const nodesApi = {
  list: async (params?: { page?: number; limit?: number; search?: string }) => {
    const page = params?.page ?? 1
    const limit = params?.limit ?? 10
    const res = await api.get<ApiResponse<RawPaged<Node>>>("/nodes", {
      params: { limit, offset: (page - 1) * limit, search: params?.search || undefined },
    })
    return adaptPage(res, page)
  },
  get: (id: number) => api.get<ApiResponse<Node>>(`/nodes/${id}`),
  listAll: async () => {
    const res = await api.get<ApiResponse<RawPaged<Node>>>("/nodes", {
      params: { limit: 500, offset: 0 },
    })
    return adaptPage(res, 1)
  },
  create: (data: Partial<Node>) =>
    api.post<ApiResponse<{ node: Node; token: string }>>("/nodes", data),
  update: (id: number, data: Partial<Node>) =>
    api.put<ApiResponse<Node>>(`/nodes/${id}`, data),
  delete: (id: number) => api.delete<ApiResponse>(`/nodes/${id}`),
  // 重新生成该节点的 Agent 认证 Token(旧的立即失效)
  retoken: (id: number) =>
    api.post<ApiResponse<{ token: string }>>(`/nodes/${id}/retoken`),
}

export const platformsApi = {
  list: async (params?: { page?: number; limit?: number; search?: string }) => {
    const page = params?.page ?? 1
    const limit = params?.limit ?? 10
    const res = await api.get<ApiResponse<RawPaged<Platform>>>("/platforms", {
      params: { limit, offset: (page - 1) * limit, search: params?.search || undefined },
    })
    return adaptPage(res, page)
  },
  listAll: async () => {
    const res = await api.get<ApiResponse<RawPaged<Platform>>>("/platforms", {
      params: { limit: 1000, offset: 0 },
    })
    return adaptPage(res, 1)
  },
  get: (id: number) => api.get<ApiResponse<Platform>>(`/platforms/${id}`),
  create: (data: Partial<Platform>) =>
    api.post<ApiResponse<Platform>>("/platforms", data),
  update: (id: number, data: Partial<Platform>) =>
    api.put<ApiResponse<Platform>>(`/platforms/${id}`, data),
  delete: (id: number) => api.delete<ApiResponse>(`/platforms/${id}`),
}

export const unlocksApi = {
  // 解锁历史:按 node_id / platform_ids / range 筛选,后端按 id 倒序。
  // 不分页 —— 直接返回符合条件的全部记录(数组)。
  // range 形如 "24h" / "7d" / "30d",由后端按 created_at 过滤;不传则不限时间。
  list: async (params?: {
    node_id?: number
    platform_ids?: number[]
    range?: string
  }): Promise<Unlock[]> => {
    const res = await api.get<ApiResponse<Unlock[]>>("/unlocks", {
      params: {
        node_id: params?.node_id || undefined,
        // 后端收逗号分隔的多个 id;axios 默认会把数组序列化成 ids[]=1&ids[]=2,所以手动 join
        platform_ids: params?.platform_ids?.length
          ? params.platform_ids.join(",")
          : undefined,
        range: params?.range || undefined,
      },
    })
    return res.data.data ?? []
  },
}

export const settingsApi = {
  get: () => api.get<ApiResponse<Settings>>("/settings"),
  update: (data: Partial<Settings>) => api.put<ApiResponse<Settings>>("/settings", data),
  regenerateToken: () => api.post<ApiResponse<{ token: string }>>("/settings/retoken"),
}

export const commonApi = {
  settings: () => api.get<ApiResponse<{ title: string }>>("/common/settings"),
  stats: () => api.get<ApiResponse<Stats>>("/common/stats"),
}
