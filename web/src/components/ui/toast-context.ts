import * as React from "react"

// Toast 上下文与 useToast 单独放这里:
// 组件文件(toast.tsx)只导出组件,才能让 react-refresh 正常工作。

export interface ToastProps {
  id: string
  title?: string
  description?: string
}

export interface ToastContextType {
  toasts: ToastProps[]
  toast: (props: Omit<ToastProps, "id">) => void
}

export const ToastContext = React.createContext<ToastContextType | null>(null)

export function useToast() {
  const ctx = React.useContext(ToastContext)
  if (!ctx) throw new Error("useToast must be used within ToastProvider")
  return ctx
}
