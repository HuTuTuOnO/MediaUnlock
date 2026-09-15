import { createContext, useContext } from "react"

// 主题上下文与 useTheme 单独放这里:
// 组件文件(ThemeProvider.tsx)只导出组件,才能让 react-refresh 正常工作。

export type Theme = "light" | "dark" | "system"

export interface ThemeContextType {
  theme: Theme
  setTheme: (t: Theme) => void
}

export const ThemeContext = createContext<ThemeContextType>({
  theme: "system",
  setTheme: () => {},
})

export const useTheme = () => useContext(ThemeContext)
