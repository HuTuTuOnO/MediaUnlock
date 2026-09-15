import { HashRouter, Routes, Route, Navigate } from "react-router-dom"
import { ToastProvider } from "@/components/ui/toast"
import { ThemeProvider } from "@/components/ThemeProvider"
import ProtectedRoute from "@/components/ProtectedRoute"
import Layout from "@/components/Layout"
import LoginPage from "@/pages/Login"
import DashboardPage from "@/pages/Dashboard"
import NodesPage from "@/pages/Nodes"
import PlatformsPage from "@/pages/Platforms"
import UnlocksPage from "@/pages/Unlocks"
import SettingsPage from "@/pages/Settings"

function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <HashRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route element={<ProtectedRoute />}>
              <Route element={<Layout />}>
                <Route index element={<Navigate to="/dashboard" replace />} />
                <Route path="/dashboard" element={<DashboardPage />} />
                <Route path="/nodes" element={<NodesPage />} />
                <Route path="/platforms" element={<PlatformsPage />} />
                <Route path="/unlocks" element={<UnlocksPage />} />
                <Route path="/settings" element={<SettingsPage />} />
              </Route>
            </Route>
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </HashRouter>
      </ToastProvider>
    </ThemeProvider>
  )
}

export default App

