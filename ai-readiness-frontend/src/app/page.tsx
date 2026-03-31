'use client';
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';

// Nodes pushed to perimeter so text center stays clear
const DOMAIN_NODES = [
  { label: 'Strategic',    color: '#3b82f6', x: 0.50, y: 0.06 },
  { label: 'Technology',   color: '#8b5cf6', x: 0.90, y: 0.30 },
  { label: 'Data',         color: '#06b6d4', x: 0.84, y: 0.80 },
  { label: 'Organization', color: '#f59e0b', x: 0.16, y: 0.80 },
  { label: 'Security',     color: '#ef4444', x: 0.10, y: 0.30 },
  { label: 'UseCase',      color: '#10b981', x: 0.50, y: 0.94 },
];

const EDGES: [number, number][] = [
  [0,1],[0,4],[1,2],[2,3],[3,4],[4,5],[5,2],[5,1],[0,5],[3,1],
];

interface CNode { x:number;y:number;baseX:number;baseY:number;r:number;color:string;label:string;phase:number; }
interface Pulse  { edge:[number,number];t:number;speed:number;color:string; }

function NeuralCanvas() {
  const cvs = useRef<HTMLCanvasElement>(null);
  const raf = useRef<number>(0);
  const nodes = useRef<CNode[]>([]);
  const pulses = useRef<Pulse[]>([]);

  useEffect(() => {
    const c = cvs.current; if (!c) return;
    const ctx = c.getContext('2d')!;

    const resize = () => {
      c.width = c.offsetWidth; c.height = c.offsetHeight;
      DOMAIN_NODES.forEach((d,i) => {
        if (nodes.current[i]) { nodes.current[i].baseX = d.x*c.width; nodes.current[i].baseY = d.y*c.height; }
      });
    };
    resize();
    window.addEventListener('resize', resize);

    nodes.current = DOMAIN_NODES.map(d => ({
      x: d.x*c.width, y: d.y*c.height,
      baseX: d.x*c.width, baseY: d.y*c.height,
      r: 9 + Math.random()*4,
      color: d.color, label: d.label,
      phase: Math.random()*Math.PI*2,
    }));

    const spawnInterval = setInterval(() => {
      const ei = Math.floor(Math.random()*EDGES.length);
      const [a,b] = EDGES[ei];
      pulses.current.push({ edge:[a,b], t:0, speed: 0.003+Math.random()*0.003, color: nodes.current[a]?.color??'#fff' });
    }, 650);

    const draw = () => {
      const W=c.width, H=c.height;
      ctx.clearRect(0,0,W,H);
      const ns = nodes.current;
      ns.forEach(n => { n.phase+=0.007; n.x=n.baseX+Math.sin(n.phase)*14; n.y=n.baseY+Math.cos(n.phase*.8)*10; });

      // Edges
      EDGES.forEach(([a,b]) => {
        const na=ns[a],nb=ns[b]; if(!na||!nb) return;
        const g=ctx.createLinearGradient(na.x,na.y,nb.x,nb.y);
        g.addColorStop(0,na.color+'40'); g.addColorStop(1,nb.color+'40');
        ctx.beginPath(); ctx.moveTo(na.x,na.y); ctx.lineTo(nb.x,nb.y);
        ctx.strokeStyle=g; ctx.lineWidth=1; ctx.stroke();
      });

      // Pulses
      pulses.current = pulses.current.filter(p=>p.t<=1);
      pulses.current.forEach(p => {
        p.t+=p.speed;
        const [a,b]=p.edge; const na=ns[a],nb=ns[b]; if(!na||!nb) return;
        const px=na.x+(nb.x-na.x)*p.t, py=na.y+(nb.y-na.y)*p.t;
        const al=Math.sin(p.t*Math.PI);
        const h=Math.round(al*255).toString(16).padStart(2,'0');
        const h2=Math.round(al*60).toString(16).padStart(2,'0');
        ctx.beginPath(); ctx.arc(px,py,3.5,0,Math.PI*2); ctx.fillStyle=p.color+h; ctx.fill();
        ctx.beginPath(); ctx.arc(px,py,8,0,Math.PI*2); ctx.fillStyle=p.color+h2; ctx.fill();
      });

      // Nodes
      ns.forEach(n => {
        const ga=0.10+Math.sin(n.phase*1.4)*0.05;
        const grd=ctx.createRadialGradient(n.x,n.y,n.r,n.x,n.y,n.r*4.5);
        grd.addColorStop(0,n.color+Math.round(ga*255).toString(16).padStart(2,'0'));
        grd.addColorStop(1,n.color+'00');
        ctx.beginPath(); ctx.arc(n.x,n.y,n.r*4.5,0,Math.PI*2); ctx.fillStyle=grd; ctx.fill();
        ctx.beginPath(); ctx.arc(n.x,n.y,n.r,0,Math.PI*2); ctx.fillStyle=n.color+'dd'; ctx.fill();
        ctx.beginPath(); ctx.arc(n.x-n.r*.3,n.y-n.r*.3,n.r*.35,0,Math.PI*2); ctx.fillStyle='#ffffff22'; ctx.fill();
        ctx.font='600 10px "DM Sans",system-ui'; ctx.textAlign='center'; ctx.fillStyle='#ffffff80';
        ctx.fillText(n.label,n.x,n.y+n.r+15);
      });

      raf.current = requestAnimationFrame(draw);
    };
    raf.current = requestAnimationFrame(draw);
    return () => { cancelAnimationFrame(raf.current); clearInterval(spawnInterval); window.removeEventListener('resize',resize); };
  }, []);

  return <canvas ref={cvs} className="absolute inset-0 w-full h-full" style={{opacity:0.92}} />;
}

function AnimatedCounter({ target, suffix='' }: { target:number; suffix?:string }) {
  // Start at `target` so server HTML matches initial client render (no hydration mismatch).
  // After mount, reset to 0 and animate up when scrolled into view.
  const [count, setCount] = useState(target);
  const [ready, setReady] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setCount(0);
    setReady(true);
  }, []);

  useEffect(() => {
    if (!ready) return;
    const el = ref.current; if (!el) return;
    const obs = new IntersectionObserver(([e]) => {
      if (!e.isIntersecting) return; obs.disconnect();
      let s = 0;
      const step = (ts: number) => {
        if (!s) s = ts;
        const p = Math.min((ts - s) / 1400, 1);
        setCount(Math.round((1 - Math.pow(1 - p, 3)) * target));
        if (p < 1) requestAnimationFrame(step);
      };
      requestAnimationFrame(step);
    }, { threshold: 0.3 });
    obs.observe(el);
    return () => obs.disconnect();
  }, [ready, target]);

  return <div ref={ref}>{count}{suffix}</div>;
}

// ── Logo ──────────────────────────────────────────────────────────────────────
function Logo({ size=32, showText=true }: { size?:number; showText?:boolean }) {
  const uid = size.toString();
  return (
    <div className="flex items-center gap-2.5 select-none">
      <svg width={size} height={size} viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M20 2L36 11V29L20 38L4 29V11L20 2Z" fill={`url(#lg${uid})`} opacity="0.12"/>
        <path d="M20 2L36 11V29L20 38L4 29V11L20 2Z" stroke={`url(#ls${uid})`} strokeWidth="1.5" fill="none"/>
        <circle cx="20" cy="20" r="3" fill={`url(#lg${uid})`}/>
        <line x1="20" y1="17" x2="20" y2="5.5"  stroke="#38bdf8" strokeWidth="1.3" strokeLinecap="round" opacity="0.85"/>
        <line x1="22.6" y1="21.5" x2="32.5" y2="27.5" stroke="#818cf8" strokeWidth="1.3" strokeLinecap="round" opacity="0.85"/>
        <line x1="17.4" y1="21.5" x2="7.5"  y2="27.5" stroke="#34d399" strokeWidth="1.3" strokeLinecap="round" opacity="0.85"/>
        <circle cx="20"   cy="5"    r="2.2" fill="#38bdf8" opacity="0.95"/>
        <circle cx="33"   cy="28"   r="2.2" fill="#818cf8" opacity="0.95"/>
        <circle cx="7"    cy="28"   r="2.2" fill="#34d399" opacity="0.95"/>
        <circle cx="20"   cy="20"   r="7" stroke={`url(#ls${uid})`} strokeWidth="0.7" fill="none" opacity="0.4" strokeDasharray="2.5 3"/>
        <defs>
          <linearGradient id={`lg${uid}`} x1="4" y1="2" x2="36" y2="38" gradientUnits="userSpaceOnUse">
            <stop stopColor="#38bdf8"/><stop offset="0.5" stopColor="#818cf8"/><stop offset="1" stopColor="#34d399"/>
          </linearGradient>
          <linearGradient id={`ls${uid}`} x1="4" y1="2" x2="36" y2="38" gradientUnits="userSpaceOnUse">
            <stop stopColor="#38bdf8" stopOpacity="0.9"/><stop offset="1" stopColor="#34d399" stopOpacity="0.9"/>
          </linearGradient>
        </defs>
      </svg>
      {showText && (
        <div style={{lineHeight:1}}>
          <div className="font-display font-bold tracking-tight text-white" style={{fontSize:size*0.46,letterSpacing:'-0.02em'}}>
            ai<span style={{background:'linear-gradient(90deg,#38bdf8,#818cf8)',WebkitBackgroundClip:'text',WebkitTextFillColor:'transparent',backgroundClip:'text'}}>transformation</span>
          </div>

        </div>
      )}
    </div>
  );
}

// ── Deliverable SVG icons ────────────────────────────────────────────────────
const DI = {
  score:(
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <circle cx="14" cy="14" r="11.5" stroke="url(#ds1)" strokeWidth="1.3" fill="none"/>
      <path d="M14 14L14 6.5" stroke="url(#ds1)" strokeWidth="2" strokeLinecap="round"/>
      <path d="M14 14L20.5 18" stroke="#818cf8" strokeWidth="2" strokeLinecap="round"/>
      <circle cx="14" cy="14" r="2.5" fill="url(#ds1)"/>
      <circle cx="14" cy="6" r="1.8" fill="#38bdf8"/>
      <circle cx="21" cy="18.5" r="1.8" fill="#818cf8"/>
      <defs><linearGradient id="ds1" x1="2" y1="2" x2="26" y2="26" gradientUnits="userSpaceOnUse"><stop stopColor="#38bdf8"/><stop offset="1" stopColor="#818cf8"/></linearGradient></defs>
    </svg>
  ),
  radar:(
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <polygon points="14,3 25,9.5 25,18.5 14,25 3,18.5 3,9.5" stroke="url(#ds2)" strokeWidth="1.2" fill="none" opacity="0.5"/>
      <polygon points="14,7 21,10.8 21,17.2 14,21 7,17.2 7,10.8" stroke="url(#ds2)" strokeWidth="1" fill="none" opacity="0.4"/>
      <polygon points="14,10.5 17.5,12.5 17.5,15.5 14,17.5 10.5,15.5 10.5,12.5" fill="url(#ds2)" opacity="0.2" stroke="url(#ds2)" strokeWidth="1"/>
      <line x1="14" y1="3" x2="14" y2="25" stroke="#38bdf8" strokeWidth="0.6" opacity="0.25"/>
      <line x1="3" y1="9.5" x2="25" y2="18.5" stroke="#38bdf8" strokeWidth="0.6" opacity="0.25"/>
      <line x1="3" y1="18.5" x2="25" y2="9.5" stroke="#38bdf8" strokeWidth="0.6" opacity="0.25"/>
      <circle cx="14" cy="14" r="1.5" fill="#38bdf8"/>
      <defs><linearGradient id="ds2" x1="3" y1="3" x2="25" y2="25" gradientUnits="userSpaceOnUse"><stop stopColor="#38bdf8"/><stop offset="1" stopColor="#34d399"/></linearGradient></defs>
    </svg>
  ),
  heatmap:(
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <rect x="3"  y="3"  width="6.5" height="6.5" rx="1.5" fill="#ef4444" opacity="0.9"/>
      <rect x="11" y="3"  width="6.5" height="6.5" rx="1.5" fill="#f97316" opacity="0.8"/>
      <rect x="19" y="3"  width="6.5" height="6.5" rx="1.5" fill="#eab308" opacity="0.7"/>
      <rect x="3"  y="11" width="6.5" height="6.5" rx="1.5" fill="#f97316" opacity="0.7"/>
      <rect x="11" y="11" width="6.5" height="6.5" rx="1.5" fill="#eab308" opacity="0.9"/>
      <rect x="19" y="11" width="6.5" height="6.5" rx="1.5" fill="#22c55e" opacity="0.8"/>
      <rect x="3"  y="19" width="6.5" height="6.5" rx="1.5" fill="#eab308" opacity="0.5"/>
      <rect x="11" y="19" width="6.5" height="6.5" rx="1.5" fill="#22c55e" opacity="0.7"/>
      <rect x="19" y="19" width="6.5" height="6.5" rx="1.5" fill="#10b981" opacity="0.95"/>
    </svg>
  ),
  risk:(
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <path d="M14 3L26.5 24H1.5L14 3Z" stroke="url(#ds3)" strokeWidth="1.4" fill="url(#ds3)" fillOpacity="0.08"/>
      <line x1="14" y1="11" x2="14" y2="18" stroke="#ef4444" strokeWidth="2.2" strokeLinecap="round"/>
      <circle cx="14" cy="21.5" r="1.6" fill="#ef4444"/>
      <defs><linearGradient id="ds3" x1="2" y1="3" x2="26" y2="24" gradientUnits="userSpaceOnUse"><stop stopColor="#f97316"/><stop offset="1" stopColor="#ef4444"/></linearGradient></defs>
    </svg>
  ),
  roadmap:(
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <path d="M4 22 C8 22 8 7 14 7 C20 7 20 22 24 22" stroke="url(#ds4)" strokeWidth="1.8" fill="none" strokeLinecap="round"/>
      <circle cx="4"  cy="22" r="2.5" fill="#ef4444" opacity="0.9"/>
      <circle cx="14" cy="7"  r="2.5" fill="#eab308" opacity="0.9"/>
      <circle cx="24" cy="22" r="2.5" fill="#10b981" opacity="0.9"/>
      <defs><linearGradient id="ds4" x1="4" y1="22" x2="24" y2="22" gradientUnits="userSpaceOnUse"><stop stopColor="#ef4444"/><stop offset="0.5" stopColor="#eab308"/><stop offset="1" stopColor="#10b981"/></linearGradient></defs>
    </svg>
  ),
  export:(
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <rect x="4" y="3" width="14" height="22" rx="2.5" stroke="url(#ds5)" strokeWidth="1.3" fill="none"/>
      <line x1="8" y1="10" x2="14" y2="10" stroke="url(#ds5)" strokeWidth="1.2" strokeLinecap="round" opacity="0.6"/>
      <line x1="8" y1="14" x2="14" y2="14" stroke="url(#ds5)" strokeWidth="1.2" strokeLinecap="round" opacity="0.6"/>
      <line x1="8" y1="18" x2="12" y2="18" stroke="url(#ds5)" strokeWidth="1.2" strokeLinecap="round" opacity="0.6"/>
      <path d="M18 19 L23 23 M23 23 L18 27 M23 23 H15" stroke="#34d399" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"/>
      <defs><linearGradient id="ds5" x1="4" y1="3" x2="18" y2="25" gradientUnits="userSpaceOnUse"><stop stopColor="#38bdf8"/><stop offset="1" stopColor="#818cf8"/></linearGradient></defs>
    </svg>
  ),
};

const DELIVERABLES = [
  { key:'score',   icon:DI.score,   color:'#38bdf8', title:'Overall Maturity Score',  desc:'A weighted 0–100 composite with domain breakdown, confidence rating, and maturity level classification — board-ready at a glance.' },
  { key:'radar',   icon:DI.radar,   color:'#34d399', title:'Domain Radar Chart',      desc:'Visual radar mapping relative strength and weakness across all 6 capability domains — instantly reveals where to concentrate investment.' },
  { key:'heatmap', icon:DI.heatmap, color:'#f97316', title:'Gap Heatmap Analysis',    desc:'Question-level color mapping exposing critical gaps (1–2) and improvement areas (3) across every domain and question simultaneously.' },
  { key:'risk',    icon:DI.risk,    color:'#ef4444', title:'Risk Flag Detection',      desc:'Automated identification of systemic risk patterns invisible to standard scoring — with targeted remediation guidance per detected flag.' },
  { key:'roadmap', icon:DI.roadmap, color:'#eab308', title:'3-Phase Roadmap',          desc:'Sequenced transformation plan across 0–3 months, 3–9 months, and 9–18 months — prioritized by risk exposure and strategic impact.' },
  { key:'export',  icon:DI.export,  color:'#818cf8', title:'Exportable Reports',       desc:'Download as JSON, Markdown, or PDF — precision-formatted for executive presentations, strategy documents, and board-level reporting.' },
];

const DOMAINS = [
  { label:'Strategic',    color:'#3b82f6', bg:'#1e3a5f', icon:'◈', w:'20%', desc:'AI vision, executive sponsorship, governance frameworks, and strategic roadmaps aligned to business outcomes.' },
  { label:'Technology',   color:'#8b5cf6', bg:'#2d1f5e', icon:'⬡', w:'20%', desc:'ML infrastructure, MLOps pipelines, model registries, observability, and engineering maturity for production AI.' },
  { label:'Data',         color:'#06b6d4', bg:'#0f3344', icon:'◎', w:'20%', desc:'Data governance, quality pipelines, lineage, privacy compliance, and readiness for AI-grade data access.' },
  { label:'Organization', color:'#f59e0b', bg:'#3d2e00', icon:'⊛', w:'15%', desc:'AI talent strategy, Center of Excellence, change management, cross-functional squads, and cultural adoption.' },
  { label:'Security',     color:'#ef4444', bg:'#3d1a1a', icon:'⬟', w:'15%', desc:'AI threat modeling, adversarial robustness, PII handling, incident response, and regulatory compliance.' },
  { label:'Use Cases',    color:'#10b981', bg:'#0f3027', icon:'◇', w:'10%', desc:'Use case prioritization, production deployments, ROI measurement, and value delivery from AI initiatives.' },
];

const MATURITY = [
  { level:'Foundational Risk Zone', range:'0–39',   color:'#ef4444', desc:'Critical gaps across core AI capabilities. Urgent intervention required before any AI scaling.' },
  { level:'AI Emerging',            range:'40–59',  color:'#f97316', desc:'Ad-hoc AI activity with limited governance. Inconsistent results and accumulating technical debt.' },
  { level:'AI Structured',          range:'60–74',  color:'#eab308', desc:'Defined processes and early production AI. Foundation being established for reliable operations.' },
  { level:'AI Advanced',            range:'75–89',  color:'#3b82f6', desc:'Managed AI at scale with strong data and security posture. Competitive differentiation emerging.' },
  { level:'AI-Native',              range:'90–100', color:'#10b981', desc:'AI is a core organizational competency driving sustained competitive advantage and innovation.' },
];

const HOW = [
  { step:'01', c:'#3b82f6', title:'Select Your Domain',  desc:'Work through 6 capability domains at your pace. Progress saves automatically — resume anytime across sessions.' },
  { step:'02', c:'#8b5cf6', title:'Score 72 Questions',  desc:'Rate each capability on a 1–5 maturity scale. Add evidence notes and reference URLs to support your scores.' },
  { step:'03', c:'#06b6d4', title:'Compute Your Score',  desc:'Weighted engine evaluates domain maturity, detects risk flags, and maps your position on the AI Readiness spectrum.' },
  { step:'04', c:'#10b981', title:'Act on Your Roadmap', desc:'Receive a prioritized 3-phase roadmap with recommendations, risk analysis, and an exportable report for leadership.' },
];

const RISKS = [
  { flag:'CRITICAL_GAPS',      color:'#ef4444', desc:'Critical-weight questions scored ≤ 2 — foundational capability gaps that block reliable AI scaling.' },
  { flag:'DATA_HIGH_RISK',     color:'#f97316', desc:'Data domain below 50 — AI programs cannot scale reliably without robust data foundations.' },
  { flag:'SECURITY_HIGH_RISK', color:'#ef4444', desc:'Security domain below 50 — unacceptable risk exposure for any production AI system.' },
  { flag:'MATURITY_CAPPED',    color:'#eab308', desc:'Maturity hard-capped at AI Structured despite high overall score due to critical domain gaps.' },
];

export default function LandingPage() {
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  return (
    <>
<div className="bg-[#080d1a] text-white min-h-screen overflow-x-hidden">

        {/* ═══ HERO ═══════════════════════════════════════════════════════════ */}
        <section className="relative min-h-[calc(100vh-60px)] flex flex-col hero-mask overflow-hidden">
          <div className="absolute inset-0 bg-gradient-to-br from-[#0b1028] via-[#080d1a] to-[#030608]"/>
          <div className="absolute inset-0 bg-grid"/>

          {/* Edge accent halos to complement canvas nodes */}
          <div className="absolute top-0 inset-x-0 h-48 bg-gradient-to-b from-blue-600/10 to-transparent pointer-events-none"/>
          <div className="absolute bottom-0 inset-x-0 h-48 bg-gradient-to-t from-teal-600/8 to-transparent pointer-events-none"/>
          <div className="absolute inset-y-0 left-0 w-32 bg-gradient-to-r from-red-600/6 to-transparent pointer-events-none"/>
          <div className="absolute inset-y-0 right-0 w-32 bg-gradient-to-l from-violet-600/8 to-transparent pointer-events-none"/>

          {mounted && <div className="absolute inset-0 z-0"><NeuralCanvas/></div>}

          {/* Content — z-10 renders ABOVE the ::after vignette pseudo-element (z-1) */}
          <div className="relative z-10 flex flex-col items-center justify-center flex-1 px-6 py-20 text-center">

            <div className="fu d1 inline-flex items-center gap-2.5 mb-8
              bg-white/[0.07] border border-white/[0.12] backdrop-blur-md
              rounded-full px-5 py-2 text-xs font-semibold tracking-widest uppercase text-sky-300">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 pdot"/>
              Enterprise AI Transformation Framework · v2.0
            </div>

            <h1 className="font-display fu d2
              text-5xl sm:text-6xl md:text-7xl lg:text-[5.5rem]
              font-bold leading-[1.04] tracking-tight mb-6 max-w-4xl">
              <span className="text-white">Is Your Organization</span>
              <br/><span className="text-grad">AI-Ready?</span>
            </h1>

            <p className="fu d3 text-lg sm:text-xl text-slate-300 max-w-2xl mb-4 leading-relaxed">
              A rigorous, evidence-based assessment across{' '}
              <span className="text-white font-semibold">6 capability domains</span> and{' '}
              <span className="text-white font-semibold">72 weighted questions</span> — giving leaders
              a precise maturity score, risk flags, and a prioritized transformation roadmap.
            </p>
            <p className="fu d3 text-sm text-slate-500 mb-10">
              Trusted by CIOs, CDOs, and AI Strategy teams to benchmark readiness and drive action.
            </p>

            <div className="fu d4 flex gap-4 flex-wrap justify-center">
              <Link href="/assessment"
                className="btn-sh inline-flex items-center gap-2.5 text-white font-bold
                  px-9 py-4 rounded-xl text-base shadow-lg shadow-blue-500/20
                  transition-all hover:scale-105 hover:shadow-blue-500/35">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                Begin Assessment
              </Link>
              <a href="#how-it-works"
                className="inline-flex items-center gap-2.5 bg-white/[0.07] hover:bg-white/[0.13]
                  border border-white/[0.15] hover:border-white/30 text-white/80 hover:text-white
                  font-semibold px-9 py-4 rounded-xl text-base transition-all backdrop-blur-sm">
                How It Works
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="bounce">
                  <path d="M12 5v14M5 12l7 7 7-7"/>
                </svg>
              </a>
            </div>

            <div className="fu d5 mt-16 grid grid-cols-2 sm:grid-cols-4 gap-x-10 gap-y-6">
              {[{val:72,suffix:'',label:'Assessment Questions'},{val:6,suffix:'',label:'Capability Domains'},{val:5,suffix:'',label:'Maturity Levels'},{val:10,suffix:'min',label:'Estimated Time'}]
                .map(({val,suffix,label}) => (
                  <div key={label} className="text-center">
                    <div className="font-display text-4xl sm:text-5xl font-bold text-grad leading-none mb-1">
                      <AnimatedCounter target={val} suffix={suffix}/>
                    </div>
                    <div className="text-xs text-slate-500 uppercase tracking-widest font-semibold mt-2">{label}</div>
                  </div>
                ))}
            </div>
          </div>

          <div className="relative z-10 flex justify-center pb-8">
            <div className="flex flex-col items-center gap-1 text-white/20 text-[10px] tracking-widest uppercase">
              <span>Scroll</span>
              <svg className="w-3.5 h-3.5 bounce" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 5v14M5 12l7 7 7-7"/></svg>
            </div>
          </div>
        </section>

        {/* ═══ 6 DOMAINS ══════════════════════════════════════════════════════ */}
        <section className="py-24 px-6 bg-[#090e21] border-t border-white/[0.06]">
          <div className="max-w-6xl mx-auto">
            <div className="text-center mb-16">
              <p className="text-sky-400 text-xs font-bold tracking-widest uppercase mb-4">What We Measure</p>
              <h2 className="font-display text-4xl sm:text-5xl font-bold text-white mb-4">Six Domains of AI Readiness</h2>
              <p className="text-slate-400 max-w-xl mx-auto text-base leading-relaxed">
                Each domain is assessed through 12 weighted questions. Domain scores combine using calibrated weights to produce your overall score.
              </p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {DOMAINS.map(({label,color,bg,icon,w,desc}) => (
                <div key={label}
                  className="rounded-2xl p-6 border transition-all duration-200 hover:-translate-y-1 cursor-default"
                  style={{background:bg+'bb',borderColor:color+'28'}}
                  onMouseEnter={e=>(e.currentTarget.style.boxShadow=`0 8px 32px -8px ${color}50`)}
                  onMouseLeave={e=>(e.currentTarget.style.boxShadow='none')}>
                  <div className="flex items-center gap-3 mb-4">
                    <div className="w-10 h-10 rounded-xl flex items-center justify-center font-display text-base font-bold"
                      style={{background:color+'20',border:`1px solid ${color}40`,color}}>
                      {icon}
                    </div>
                    <div>
                      <div className="font-display font-bold text-white text-sm">{label}</div>
                      <div className="text-xs mt-0.5 font-semibold" style={{color}}>12 questions · {w} weight</div>
                    </div>
                  </div>
                  <p className="text-slate-400 text-sm leading-relaxed">{desc}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ═══ HOW IT WORKS ═══════════════════════════════════════════════════ */}
        <section id="how-it-works" className="py-24 px-6 bg-[#06091a]">
          <div className="max-w-5xl mx-auto">
            <div className="text-center mb-16">
              <p className="text-emerald-400 text-xs font-bold tracking-widest uppercase mb-4">The Process</p>
              <h2 className="font-display text-4xl sm:text-5xl font-bold text-white">From Zero to Roadmap in 4 Steps</h2>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              {HOW.map(({step,c,title,desc}) => (
                <div key={step}
                  className="p-6 rounded-2xl border border-white/[0.07] bg-white/[0.025]
                    hover:bg-white/[0.055] hover:border-white/[0.12] transition-all">
                  <div className="flex items-start gap-4">
                    <div className="w-10 h-10 rounded-xl flex-shrink-0 flex items-center justify-center
                      font-display font-bold text-sm"
                      style={{background:c+'20',border:`1px solid ${c}40`,color:c}}>
                      {step}
                    </div>
                    <div>
                      <h3 className="font-display font-bold text-white text-base mb-2">{title}</h3>
                      <p className="text-slate-400 text-sm leading-relaxed">{desc}</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ═══ MATURITY SPECTRUM ══════════════════════════════════════════════ */}
        <section className="py-24 px-6 bg-[#080c1e] border-t border-white/[0.05]">
          <div className="max-w-5xl mx-auto">
            <div className="text-center mb-16">
              <p className="text-violet-400 text-xs font-bold tracking-widest uppercase mb-4">Scoring Framework</p>
              <h2 className="font-display text-4xl sm:text-5xl font-bold text-white mb-4">The AI Maturity Spectrum</h2>
              <p className="text-slate-400 max-w-lg mx-auto text-base">Your 0–100 score maps to one of five levels, each with distinct strategic implications.</p>
            </div>
            <div className="space-y-3">
              {MATURITY.map(({level,range,color,desc},i) => (
                <div key={level}
                  className="flex items-center gap-5 p-5 rounded-2xl border border-white/[0.05]
                    bg-white/[0.02] hover:bg-white/[0.05] transition-all group cursor-default">
                  <div className="flex-shrink-0 w-[70px] text-center">
                    <div className="text-xs font-bold px-2 py-1.5 rounded-lg font-mono"
                      style={{background:color+'18',color,border:`1px solid ${color}28`}}>
                      {range}
                    </div>
                  </div>
                  <div className="flex-shrink-0 w-0.5 self-stretch rounded-full my-1"
                    style={{background:`linear-gradient(to bottom,${color}00,${color}bb,${color}00)`}}/>
                  <div className="flex-1 min-w-0">
                    <div className="font-display font-bold text-white text-sm mb-1 group-hover:text-sky-100 transition-colors">{level}</div>
                    <div className="text-slate-500 text-sm">{desc}</div>
                  </div>
                  <div className="flex-shrink-0 font-display text-5xl font-bold leading-none
                    opacity-[0.07] group-hover:opacity-[0.16] transition-opacity select-none" style={{color}}>
                    {i+1}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ═══ RISK FLAGS ═════════════════════════════════════════════════════ */}
        <section className="py-24 px-6 bg-[#07091a]">
          <div className="max-w-5xl mx-auto">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-16 items-center">
              <div>
                <p className="text-red-400 text-xs font-bold tracking-widest uppercase mb-4">Intelligent Detection</p>
                <h2 className="font-display text-4xl sm:text-5xl font-bold text-white mb-6 leading-tight">
                  Automated Risk<br/><span className="text-grad">Flag Detection</span>
                </h2>
                <p className="text-slate-400 text-base leading-relaxed mb-8">
                  Beyond the score, our engine identifies systemic risk patterns invisible to standard scoring —
                  even in otherwise high-scoring organizations. Risk flags can permanently cap your maturity classification.
                </p>
                <Link href="/assessment" className="inline-flex items-center gap-2 text-sky-400 hover:text-sky-300 font-semibold text-sm transition-colors group">
                  Run your assessment to detect risks
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"
                    className="group-hover:translate-x-1 transition-transform"><path d="M5 12h14M12 5l7 7-7 7"/></svg>
                </Link>
              </div>
              <div className="space-y-3">
                {RISKS.map(({flag,color,desc}) => (
                  <div key={flag} className="flex gap-4 items-start p-4 rounded-xl border bg-white/[0.02] hover:bg-white/[0.05] transition-all"
                    style={{borderColor:color+'28'}}>
                    <div className="flex-shrink-0 w-2.5 h-2.5 rounded-full mt-1.5"
                      style={{background:color,boxShadow:`0 0 10px ${color}80`}}/>
                    <div>
                      <div className="font-mono text-xs font-bold mb-1.5" style={{color}}>{flag}</div>
                      <div className="text-slate-400 text-sm leading-relaxed">{desc}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        {/* ═══ DELIVERABLES — premium redesign ════════════════════════════════ */}
        <section className="py-24 px-6 bg-[#060918] border-t border-white/[0.05]">
          <div className="max-w-6xl mx-auto">
            <div className="text-center mb-16">
              <p className="text-amber-400 text-xs font-bold tracking-widest uppercase mb-4">Your Deliverables</p>
              <h2 className="font-display text-4xl sm:text-5xl font-bold text-white mb-4">What You Get at the End</h2>
              <p className="text-slate-400 max-w-xl mx-auto text-base leading-relaxed">
                Every completed assessment generates a complete suite of actionable intelligence —
                precision-crafted for executive review and strategic planning.
              </p>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
              {DELIVERABLES.map(({key,icon,color,title,desc}) => (
                <div key={key}
                  className="deliv-card rounded-2xl p-6 text-left border border-white/[0.08]"
                  style={{
                    '--cg': color+'14',
                    '--cs': color+'35',
                    background: `linear-gradient(145deg, ${color}0a 0%, rgba(255,255,255,0.02) 100%)`,
                  } as React.CSSProperties}>

                  {/* Icon container */}
                  <div className="relative mb-5 inline-block">
                    <div className="w-14 h-14 rounded-2xl flex items-center justify-center"
                      style={{
                        background:`radial-gradient(circle at 30% 30%,${color}28,${color}0a)`,
                        border:`1px solid ${color}35`,
                        boxShadow:`0 0 20px -6px ${color}40, inset 0 1px 0 ${color}25`,
                      }}>
                      {icon}
                    </div>
                    {/* Accent glow dot */}
                    <div className="absolute -top-1 -right-1 w-3 h-3 rounded-full border-2 border-[#080d1a]"
                      style={{background:color,boxShadow:`0 0 8px ${color}`}}/>
                  </div>

                  {/* Colored accent bar + title */}
                  <div className="flex items-start gap-2.5 mb-3">
                    <div className="w-0.5 h-5 rounded-full flex-shrink-0 mt-0.5" style={{background:color}}/>
                    <h3 className="font-display font-bold text-white text-[0.95rem] leading-snug">{title}</h3>
                  </div>

                  <p className="text-slate-400 text-sm leading-relaxed">{desc}</p>

                  {/* Bottom dot-progress accent */}
                  <div className="mt-5 pt-4 border-t border-white/[0.06] flex items-center gap-2.5">
                    <div className="flex gap-1">
                      {[0,1,2,3,4].map(i=>(
                        <div key={i} className="w-1 h-1 rounded-full" style={{background:color,opacity:0.25+i*0.17}}/>
                      ))}
                    </div>
                    <span className="text-[11px] text-slate-600 font-medium">Included in every assessment</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ═══ FINAL CTA ══════════════════════════════════════════════════════ */}
        <section className="py-32 px-6 relative overflow-hidden bg-[#050812]">
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
            <div className="w-[800px] h-[400px] bg-blue-600/8 rounded-full blur-[120px]"/>
          </div>
          <div className="relative z-10 max-w-3xl mx-auto text-center">
            <div className="inline-flex items-center gap-2 text-xs font-bold tracking-widest uppercase
              text-emerald-400 mb-6 bg-emerald-400/10 border border-emerald-400/20 rounded-full px-5 py-2">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 pdot"/>
              Free · No account required · Results in &lt;10 min
            </div>
            <h2 className="font-display text-5xl sm:text-6xl md:text-[4.5rem] font-bold text-white mb-6 leading-[1.06]">
              Start Your<br/><span className="text-grad">AI Transformation</span><br/>Today.
            </h2>
            <p className="text-slate-400 text-lg mb-12 leading-relaxed">
              Join the organizations that have benchmarked their AI readiness and built
              clear, evidence-based paths to competitive advantage.
            </p>
            <Link href="/assessment"
              className="btn-sh inline-flex items-center gap-3 text-white font-bold
                px-12 py-5 rounded-2xl text-lg shadow-2xl shadow-blue-500/25
                transition-all hover:scale-105">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
              Begin Your Assessment
            </Link>
            <div className="mt-8 text-slate-600 text-xs">Progress saves automatically · Resume anytime · Export your results</div>
          </div>
        </section>

        {/* Footer */}
        <footer className="border-t border-white/[0.06] py-8 px-6 bg-[#040610]">
          <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
            <Logo size={28}/>
            <div className="text-slate-600 text-xs">72 questions · 6 domains · 5 maturity levels · Weighted scoring</div>
          </div>
        </footer>

      </div>
    </>
  );
}
