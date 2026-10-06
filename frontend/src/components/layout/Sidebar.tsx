import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Inbox,
  History,
  ScrollText,
  User,
  ShieldCheck,
} from 'lucide-react';
import { cn } from '../../utils/format';
import { BrandMark } from './BrandMark';

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/consent/pending', label: 'Pending requests', icon: Inbox },
  { to: '/consent/history', label: 'Consent history', icon: History },
  { to: '/audit', label: 'Audit ledger', icon: ScrollText },
  { to: '/profile', label: 'Profile', icon: User },
];

/** Application sidebar: brand, primary navigation, and a ledger trust note. */
export function Sidebar() {
  return (
    <aside className="fixed inset-y-0 left-0 z-30 flex w-[236px] flex-col border-r border-line bg-surface">
      <div className="flex items-center gap-2.5 px-5 h-16 border-b border-line">
        <BrandMark />
        <div className="leading-tight">
          <p className="text-[16px] font-bold tracking-tight text-ink">Yomi</p>
          <p className="text-[10.5px] font-medium uppercase tracking-label text-ink-muted">
            Consent ledger
          </p>
        </div>
      </div>

      <nav className="flex-1 overflow-y-auto px-3 py-4" aria-label="Primary">
        <ul className="space-y-0.5">
          {navItems.map(({ to, label, icon: Icon }) => (
            <li key={to}>
              <NavLink
                to={to}
                className={({ isActive }) =>
                  cn(
                    'group relative flex items-center gap-2.5 rounded-md px-3 py-2 text-[13.5px] font-medium transition-colors duration-100',
                    isActive
                      ? 'bg-canvas text-ink'
                      : 'text-ink-secondary hover:bg-canvas hover:text-ink',
                  )
                }
              >
                {({ isActive }) => (
                  <>
                    <span
                      className={cn(
                        'absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-r bg-primary-600 transition-opacity',
                        isActive ? 'opacity-100' : 'opacity-0',
                      )}
                    />
                    <Icon size={16} strokeWidth={2} className={isActive ? 'text-primary-600' : 'text-ink-muted'} />
                    {label}
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className="border-t border-line px-4 py-4">
        <div className="flex items-start gap-2.5 rounded-md border border-line bg-tint px-3 py-2.5">
          <ShieldCheck size={15} className="mt-0.5 shrink-0 text-primary-600" strokeWidth={2} />
          <p className="text-[11.5px] leading-snug text-ink-muted">
            Every event is signed, hash-chained, and verified end to end.
          </p>
        </div>
      </div>
    </aside>
  );
}
