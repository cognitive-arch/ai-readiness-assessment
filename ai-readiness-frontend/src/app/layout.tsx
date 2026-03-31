// app/layout.tsx
import type { Metadata } from 'next';
import { DM_Sans, DM_Mono, Syne } from 'next/font/google';
import './globals.css';
import { AssessmentProvider } from '@/lib/store';
import { Navbar } from '@/components/Navbar';

// All fonts loaded server-side via next/font — zero hydration issues
const dmSans = DM_Sans({ subsets: ['latin'], variable: '--font-sans' });
const dmMono = DM_Mono({ subsets: ['latin'], weight: ['400', '500'], variable: '--font-mono' });
const syne   = Syne({ subsets: ['latin'], weight: ['600', '700', '800'], variable: '--font-display' });

export const metadata: Metadata = {
  title: 'AI Readiness Assessment — Enterprise AI Transformation Framework',
  description: "Benchmark your organization's AI readiness across 6 domains with 72 weighted questions. Get a maturity score, risk flags, and a prioritized transformation roadmap.",
};

function EnvDebugBanner() {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (!url) {
    return (
      <div style={{
        background: '#7f1d1d', color: '#fca5a5',
        padding: '6px 16px', fontSize: '12px',
        fontFamily: 'monospace', display: 'flex', gap: '8px', alignItems: 'center',
      }}>
        <strong>⚠</strong>{' '}
        NEXT_PUBLIC_API_BASE_URL not set — running offline.
        Create <code>.env.local</code> with{' '}
        <code>NEXT_PUBLIC_API_BASE_URL=http://localhost:8080</code> then restart.
      </div>
    );
  }
  return null;
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className={`${dmSans.variable} ${dmMono.variable} ${syne.variable} min-h-screen`}>
        <EnvDebugBanner />
        <AssessmentProvider>
          <Navbar />
          <main>{children}</main>
        </AssessmentProvider>
      </body>
    </html>
  );
}
