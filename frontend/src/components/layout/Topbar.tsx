import { useNavigate } from 'react-router-dom';
import { LogOut, Sun, Moon } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { useTheme } from '../../context/ThemeContext';
import { maskAadhaar } from '../../utils/format';

/** Top bar: current citizen identity and session actions. */
export function Topbar() {
  const { citizen, logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/login', { replace: true });
  };

  const initials = citizen
    ? citizen.fullName
        .split(' ')
        .map((p) => p[0])
        .slice(0, 2)
        .join('')
        .toUpperCase()
    : '--';

  return (
    <header className="flex h-16 items-center justify-between border-b border-line bg-surface px-6">
      <div className="flex items-center gap-3">
        <span className="hidden text-[12px] font-medium text-ink-muted sm:inline">
          Citizen data trust infrastructure
        </span>
      </div>

      <div className="flex items-center gap-4">
        <button
          onClick={toggleTheme}
          aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
          title={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-ink-muted transition-colors hover:bg-tint hover:text-ink"
        >
          {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
        </button>
        {citizen && (
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary-100 text-[12px] font-bold text-primary-700">
              {initials}
            </div>
            <div className="leading-tight text-right">
              <p className="text-[13px] font-semibold text-ink">{citizen.fullName}</p>
              <p className="font-mono text-[11px] text-ink-muted">{maskAadhaar(citizen.aadhaarId)}</p>
            </div>
          </div>
        )}
        <button
          onClick={handleLogout}
          className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium text-ink-muted transition-colors hover:bg-tint hover:text-ink"
        >
          <LogOut size={15} />
          Sign out
        </button>
      </div>
    </header>
  );
}
