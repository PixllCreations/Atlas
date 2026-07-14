import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { NewProjectPage } from './pages/NewProjectPage'
import { ProjectPage } from './pages/ProjectPage'
import { ProjectSettingsPage } from './pages/ProjectSettingsPage'
import { ProjectsPage } from './pages/ProjectsPage'
import { StatusPage } from './pages/StatusPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<ProjectsPage />} />
          <Route path="new" element={<NewProjectPage />} />
          <Route path="projects/:id" element={<ProjectPage />} />
          <Route path="projects/:id/settings" element={<ProjectSettingsPage />} />
          <Route path="system" element={<StatusPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
