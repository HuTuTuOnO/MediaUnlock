import * as React from "react"
import { ToastContext, type ToastProps } from "@/components/ui/toast-context"

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = React.useState<ToastProps[]>([])

  const toast = React.useCallback((props: Omit<ToastProps, "id">) => {
    const id = Math.random().toString(36).slice(2)
    setToasts((prev) => [...prev, { ...props, id }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 4000)
  }, [])

  return (
    <ToastContext.Provider value={{ toasts, toast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-[200] flex flex-col gap-2 w-80">
        {toasts.map((t) => (
          <div
            key={t.id}
            className="rounded-lg border p-4 shadow-lg animate-in slide-in-from-right-full bg-background text-foreground"
          >
            {t.title && <p className="font-semibold text-sm">{t.title}</p>}
            {t.description && <p className="text-sm opacity-90 mt-0.5">{t.description}</p>}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}
