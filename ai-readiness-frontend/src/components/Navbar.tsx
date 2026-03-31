'use client';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAssessment } from '@/lib/store';

// Logo: hexagonal neural mark — no .app suffix
function Logo({ size = 32 }: { size?: number }) {
  // Static gradient IDs — safe because there is only ever one Navbar in the DOM
  return (
    <div className="flex items-center gap-2.5 select-none">
      <svg width={size} height={size} viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M20 2L36 11V29L20 38L4 29V11L20 2Z" fill="url(#nbg)" opacity="0.12"/>
        <path d="M20 2L36 11V29L20 38L4 29V11L20 2Z" stroke="url(#nbs)" strokeWidth="1.5" fill="none"/>
        <circle cx="20" cy="20" r="3" fill="url(#nbg)"/>
        <line x1="20"   y1="17"   x2="20"   y2="5.5"  stroke="#38bdf8" strokeWidth="1.3" strokeLinecap="round" opacity="0.9"/>
        <line x1="22.6" y1="21.5" x2="32.5" y2="27.5" stroke="#818cf8" strokeWidth="1.3" strokeLinecap="round" opacity="0.9"/>
        <line x1="17.4" y1="21.5" x2="7.5"  y2="27.5" stroke="#34d399" strokeWidth="1.3" strokeLinecap="round" opacity="0.9"/>
        <circle cx="20" cy="5"  r="2.2" fill="#38bdf8" opacity="0.95"/>
        <circle cx="33" cy="28" r="2.2" fill="#818cf8" opacity="0.95"/>
        <circle cx="7"  cy="28" r="2.2" fill="#34d399" opacity="0.95"/>
        <circle cx="20" cy="20" r="7" stroke="url(#nbs)" strokeWidth="0.7" fill="none" opacity="0.35" strokeDasharray="2.5 3"/>
        <defs>
          <linearGradient id="nbg" x1="4" y1="2" x2="36" y2="38" gradientUnits="userSpaceOnUse">
            <stop stopColor="#38bdf8"/>
            <stop offset="0.5" stopColor="#818cf8"/>
            <stop offset="1" stopColor="#34d399"/>
          </linearGradient>
          <linearGradient id="nbs" x1="4" y1="2" x2="36" y2="38" gradientUnits="userSpaceOnUse">
            <stop stopColor="#38bdf8" stopOpacity="0.9"/>
            <stop offset="1" stopColor="#34d399" stopOpacity="0.9"/>
          </linearGradient>
        </defs>
      </svg>

      {/* Wordmark — no .app */}
      <div style={{ lineHeight: 1 }}>
        <div style={{
          fontFamily: "var(--font-display), var(--font-sans), sans-serif",
          fontSize: size * 0.44,
          fontWeight: 700,
          letterSpacing: '-0.02em',
          color: 'white',
        }}>
          ai
          <span style={{
            background: 'linear-gradient(90deg, #38bdf8, #818cf8)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            backgroundClip: 'text',
          }}>
            transformation
          </span>
        </div>
      </div>
    </div>
  );
}

export function Navbar() {
  const pathname = usePathname();
  const isHome = pathname === '/';
  const { reset, state, questionBank, backendAvailable } = useAssessment();

  const totalQ = questionBank.questions.length;
  const totalAnswered = questionBank.questions.filter(
    (q) => state.answers[q.id]?.score != null
  ).length;
  const pct = totalQ > 0 ? Math.round((totalAnswered / totalQ) * 100) : 0;

  const NAV_ITEMS = [
    { href: '/assessment', label: 'Dashboard' },
    { href: '/review',     label: 'Review' },
    ...(state.result ? [{ href: '/results', label: 'Results' }] : []),
  ];

  return (
    <>
      {/*
        ALWAYS dark navy — same as the hero/page background.
        On home: fully transparent border so it blends into the hero seamlessly.
        On inner pages: a subtle bottom border separates it from content.
      */}
      <nav className={`h-[60px] flex items-center px-6 sticky top-0 z-50
        bg-[#080d1a] transition-all duration-300
        ${isHome
          ? 'border-b border-transparent'
          : 'border-b border-white/[0.08]'
        }`}>

        <Link href="/" className="mr-6 shrink-0">
          <Logo size={30} />
        </Link>

        {/* Inner page nav — white-toned links on dark bg */}
        {!isHome && (
          <div className="flex items-center gap-1 flex-1 min-w-0">
            {NAV_ITEMS.map((item) => {
              const active = pathname.startsWith(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-all shrink-0
                    ${active
                      ? 'bg-white/10 text-white font-semibold'
                      : 'text-white/60 hover:text-white hover:bg-white/[0.07]'
                    }`}>
                  {item.label}
                </Link>
              );
            })}

            {/* Progress pill */}
            {pct > 0 && (
              <div className="hidden sm:flex items-center gap-2 ml-3
                bg-white/[0.06] border border-white/[0.10] rounded-full px-3 py-1 shrink-0">
                <div className="w-16 h-1.5 bg-white/10 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-sky-400 rounded-full transition-all"
                    style={{ width: `${pct}%` }}
                  />
                </div>
                <span className="text-xs font-semibold text-white/50">{pct}%</span>
              </div>
            )}

            <div className="ml-auto flex items-center gap-2 shrink-0">
              {/* Backend status dot */}
              <span
                title={backendAvailable ? 'Backend connected' : 'Offline mode'}
                className={`w-2 h-2 rounded-full hidden md:block
                  ${backendAvailable ? 'bg-emerald-400' : 'bg-white/20'}`}
              />
              <button
                onClick={() => {
                  if (confirm('Reset assessment? All progress will be lost.')) reset();
                }}
                className="text-red-400 hover:text-red-300 hover:bg-red-500/10
                  text-sm font-medium px-3 py-2 rounded-lg transition-all">
                Reset
              </button>
            </div>
          </div>
        )}

        {/* Home page — minimal right-side links */}
        {isHome && (
          <div className="flex items-center gap-3 ml-auto">
            <Link
              href="/assessment"
              className="hidden sm:block text-white/50 hover:text-white
                text-sm font-medium transition-colors">
              Dashboard
            </Link>
            <Link
              href="/assessment"
              className="bg-blue-600 hover:bg-blue-500 text-white text-sm font-semibold
                px-5 py-2 rounded-lg transition-all shadow-lg shadow-blue-500/20">
              Start Assessment
            </Link>
          </div>
        )}
      </nav>
    </>
  );
}
