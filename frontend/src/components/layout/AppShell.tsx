import { ReactNode } from 'react';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

/** Page shell: fixed sidebar + topbar with a scrolling content column. */
export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen">
      <Sidebar />
      <div className="pl-[236px]">
        <Topbar />
        <main className="page-enter mx-auto max-w-[1080px] px-6 py-8">{children}</main>
      </div>
    </div>
  );
}
