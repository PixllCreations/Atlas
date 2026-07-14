import { NavLink, Outlet } from 'react-router-dom'

export function Layout() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <NavLink to="/" className="brand">
          <svg className="brand-mark" viewBox="0 0 32 32" fill="none" aria-hidden>
            <rect width="32" height="32" rx="8" fill="#0f1419" />
            <path d="M8 22 L16 8 L24 22 Z" stroke="#3dcdb8" strokeWidth="2.2" />
            <path d="M12 22 H20" stroke="#3dcdb8" strokeWidth="2.2" />
          </svg>
          Atlas
        </NavLink>
        <nav className="nav">
          <NavLink to="/" end>
            Projects
          </NavLink>
          <NavLink to="/new">New</NavLink>
          <NavLink to="/system">Status</NavLink>
        </nav>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
