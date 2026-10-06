import { FormEvent, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Fingerprint, Lock, AlertCircle, ArrowRight, Sun, Moon } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useTheme } from '../context/ThemeContext';
import { Button } from '../components/common/Button';
import { BrandMark } from '../components/layout/BrandMark';
import { cn } from '../utils/format';

const inputClass =
  'h-10 w-full rounded-md border border-lineStrong bg-surface px-3 text-[14px] text-ink placeholder:text-ink-faint transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100';

export default function Login() {
  const { login, error, clearError, status } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();
  const [aadhaarId, setAadhaarId] = useState('');
  const [password, setPassword] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  const busy = status === 'loading';

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setFormError(null);
    if (!/^\d{12}$/.test(aadhaarId)) {
      setFormError('Aadhaar ID must be exactly 12 digits.');
      return;
    }
    if (!password) {
      setFormError('Enter your password.');
      return;
    }
    try {
      await login({ aadhaarId, password });
      navigate('/dashboard', { replace: true });
    } catch {
      // error is rendered from context
    }
  };

  const shownError = formError ?? error;

  return (
    <div className="flex min-h-screen">
      {/* Brand panel */}
      <div className="relative hidden w-[46%] flex-col justify-between overflow-hidden border-r border-line bg-primary-900 p-10 lg:flex">
        <div className="flex items-center gap-2.5">
          <BrandMark size={30} />
          <div className="leading-tight">
            <p className="text-[17px] font-bold tracking-tight text-white">Yomi</p>
            <p className="text-[10.5px] font-medium uppercase tracking-label text-primary-200">
              Consent ledger
            </p>
          </div>
        </div>

        <div className="max-w-md">
          <p className="label !text-primary-200">Citizen data trust infrastructure</p>
          <h1 className="mt-4 text-[30px] font-semibold leading-[1.15] tracking-tight text-white">
            Every access to your data, signed and verifiable.
          </h1>
          <p className="mt-5 text-[14.5px] leading-relaxed text-primary-100/90">
            Banks, hospitals, telecoms and government bodies must ask before they
            touch your personal data. Yomi records every request, approval and
            revocation in an immutable, hash-chained audit ledger — and it is
            yours to inspect.
          </p>

          <ul className="mt-9 space-y-4">
            {[
              'Explicit consent before any data access',
              'Signed authorization tokens, revocable anytime',
              'Hash-chained ledger you can verify yourself',
            ].map((item) => (
              <li key={item} className="flex items-center gap-3 text-[13.5px] text-primary-50">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-primary-400/50 text-[11px] text-primary-200">
                  ✓
                </span>
                {item}
              </li>
            ))}
          </ul>
        </div>

        <div className="border-t border-primary-700/60 pt-5">
          <p className="font-mono text-[11px] text-primary-300">
            ledger://genesis — every entry links to the one before it
          </p>
        </div>
      </div>

      {/* Form panel */}
      <div className="relative flex flex-1 items-center justify-center px-6 py-12">
        <button
          onClick={toggleTheme}
          aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
          title={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
          className="absolute right-5 top-5 inline-flex h-8 w-8 items-center justify-center rounded-md text-ink-muted transition-colors hover:bg-tint hover:text-ink"
        >
          {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
        </button>
        <div className="w-full max-w-[400px]">
          <div className="mb-8 lg:hidden">
            <BrandMark />
          </div>

          <h2 className="text-[22px] font-bold tracking-tight text-ink">Sign in</h2>
          <p className="mt-1 text-[13.5px] text-ink-muted">
            Use your Aadhaar-linked citizen account.
          </p>

          {shownError && (
            <div
              role="alert"
              className="mt-5 flex items-start gap-2.5 rounded-md border border-red-100 bg-red-50 px-3.5 py-3 text-[13px] text-red-700"
            >
              <AlertCircle size={16} className="mt-0.5 shrink-0" />
              <span>{shownError}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="mt-6 space-y-4" noValidate>
            <div>
              <label htmlFor="aadhaar" className="label mb-1.5 block">
                Aadhaar ID
              </label>
              <div className="relative">
                <Fingerprint size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted" />
                <input
                  id="aadhaar"
                  name="aadhaar"
                  type="text"
                  inputMode="numeric"
                  autoComplete="username"
                  placeholder="12-digit Aadhaar number"
                  value={aadhaarId}
                  onChange={(e) => {
                    setAadhaarId(e.target.value.replace(/\D/g, '').slice(0, 12));
                    clearError();
                  }}
                  className={cn(inputClass, 'pl-9 font-mono')}
                  required
                />
              </div>
            </div>

            <div>
              <label htmlFor="password" className="label mb-1.5 block">
                Password
              </label>
              <div className="relative">
                <Lock size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted" />
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  placeholder="Your password"
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    clearError();
                  }}
                  className={cn(inputClass, 'pl-9')}
                  required
                />
              </div>
            </div>

            <Button type="submit" loading={busy} className="w-full" icon={<ArrowRight size={15} />}>
              Sign in
            </Button>
          </form>

          <p className="mt-6 text-center text-[13px] text-ink-muted">
            New to Yomi?{' '}
            <Link to="/register" className="font-semibold text-primary-600 hover:text-primary-700">
              Register your citizen account
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
