import { FormEvent, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { User, Fingerprint, Mail, Phone, Lock, AlertCircle, ArrowRight } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { Button } from '../components/common/Button';
import { BrandMark } from '../components/layout/BrandMark';
import { cn } from '../utils/format';

const inputClass =
  'h-10 w-full rounded-md border border-lineStrong bg-surface px-3 text-[14px] text-ink placeholder:text-ink-faint transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100';

function Field({
  id,
  label,
  icon,
  mono,
  ...rest
}: {
  id: string;
  label: string;
  icon: React.ReactNode;
  mono?: boolean;
} & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div>
      <label htmlFor={id} className="label mb-1.5 block">
        {label}
      </label>
      <div className="relative">
        <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted">{icon}</span>
        <input id={id} className={cn(inputClass, 'pl-9', mono && 'font-mono')} {...rest} />
      </div>
    </div>
  );
}

export default function Register() {
  const { register, error, clearError, status } = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState({
    fullName: '',
    aadhaarId: '',
    email: '',
    phone: '',
    password: '',
    confirm: '',
  });
  const [formError, setFormError] = useState<string | null>(null);
  const busy = status === 'loading';

  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = key === 'aadhaarId' ? e.target.value.replace(/\D/g, '').slice(0, 12) : e.target.value;
    setForm((f) => ({ ...f, [key]: value }));
    clearError();
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setFormError(null);

    if (form.fullName.trim().length < 2) return setFormError('Enter your full name.');
    if (!/^\d{12}$/.test(form.aadhaarId)) return setFormError('Aadhaar ID must be exactly 12 digits.');
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) return setFormError('Enter a valid email address.');
    if (form.phone && !/^[6-9]\d{9}$/.test(form.phone)) return setFormError('Enter a valid 10-digit Indian mobile number.');
    if (form.password.length < 8) return setFormError('Password must be at least 8 characters.');
    if (form.password !== form.confirm) return setFormError('Passwords do not match.');

    try {
      await register({
        fullName: form.fullName.trim(),
        aadhaarId: form.aadhaarId,
        email: form.email.trim().toLowerCase(),
        phone: form.phone || undefined,
        password: form.password,
      });
      navigate('/login', { replace: true, state: { registered: true } });
    } catch {
      // error rendered from context
    }
  };

  const shownError = formError ?? error;

  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas px-6 py-12">
      <div className="w-full max-w-[440px]">
        <div className="mb-8 flex items-center gap-2.5">
          <BrandMark />
          <div className="leading-tight">
            <p className="text-[16px] font-bold tracking-tight text-ink">Yomi</p>
            <p className="text-[10.5px] font-medium uppercase tracking-label text-ink-muted">Consent ledger</p>
          </div>
        </div>

        <h2 className="text-[22px] font-bold tracking-tight text-ink">Create your citizen account</h2>
        <p className="mt-1 text-[13.5px] text-ink-muted">
          Your identity is verified against your Aadhaar number. You keep control of everything else.
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
          <Field
            id="fullName"
            label="Full name"
            icon={<User size={15} />}
            placeholder="As per Aadhaar"
            autoComplete="name"
            value={form.fullName}
            onChange={set('fullName')}
            required
          />
          <Field
            id="aadhaar"
            label="Aadhaar ID"
            icon={<Fingerprint size={15} />}
            mono
            placeholder="12-digit Aadhaar number"
            inputMode="numeric"
            autoComplete="username"
            value={form.aadhaarId}
            onChange={set('aadhaarId')}
            required
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field
              id="email"
              label="Email"
              type="email"
              icon={<Mail size={15} />}
              placeholder="you@example.com"
              autoComplete="email"
              value={form.email}
              onChange={set('email')}
              required
            />
            <Field
              id="phone"
              label="Phone (optional)"
              type="tel"
              icon={<Phone size={15} />}
              mono
              placeholder="10-digit mobile"
              inputMode="numeric"
              autoComplete="tel"
              value={form.phone}
              onChange={set('phone')}
            />
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field
              id="password"
              label="Password"
              type="password"
              icon={<Lock size={15} />}
              placeholder="Min 8 characters"
              autoComplete="new-password"
              value={form.password}
              onChange={set('password')}
              required
            />
            <Field
              id="confirm"
              label="Confirm password"
              type="password"
              icon={<Lock size={15} />}
              placeholder="Repeat password"
              autoComplete="new-password"
              value={form.confirm}
              onChange={set('confirm')}
              required
            />
          </div>

          <Button type="submit" loading={busy} className="w-full" icon={<ArrowRight size={15} />}>
            Create account
          </Button>
        </form>

        <p className="mt-6 text-center text-[13px] text-ink-muted">
          Already registered?{' '}
          <Link to="/login" className="font-semibold text-primary-600 hover:text-primary-700">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
