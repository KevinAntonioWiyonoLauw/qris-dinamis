import { useCallback, useEffect, useRef, useState } from "react";
import * as Tooltip from "@radix-ui/react-tooltip";
import { Slot } from "@radix-ui/react-slot";
import jsQR from "jsqr";
import {
  convertQRIS as convertOffline,
  parseQRIS as parseOffline,
  strictValidate,
  validateQRIS as validateOffline,
  type ConvertOptions as OfflineConvertOptions,
  type QRISData,
  type TLVPair,
} from "./lib/qris";

const PRESETS = [10000, 20000, 25000, 50000, 100000];

// Kevinio's static QRIS base (DANA tag 26 + QRIS tag 51) for the traktir feature
const TRAKTIR_BASE = "00020101021126570011ID.DANA.WWW011893600915303430371702090343037170303UMI51440014ID.CO.QRIS.WWW0215ID10265753483660303UMI5204481453033605802ID5907Kevinio6012Kota Mataram61058311263044FF5";

const mcc: Record<string, string> = { "5411": "Grocery Store", "5812": "Restaurant", "5814": "Fast Food", "5912": "Pharmacy", "5999": "Retail Store", "7299": "Other Services", "8011": "Medical" };
const currency: Record<string, string> = { "360": "IDR (Rupiah)", "840": "USD (Dollar)" };

type HistoryItem = { code: string; merchant: string; amount: string; at: number };
type Template = { id: string; name: string; qris: string; fee?: { type: "fixed" | "percentage"; value: number } };

async function post<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const data = await response.json() as T & { error?: string };
  if (!response.ok) throw new Error(data.error || "Permintaan gagal");
  return data;
}

function Button({ asChild = false, className = "", ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { asChild?: boolean }) {
  const classes = `inline-flex items-center justify-center gap-2 rounded-xl border px-4 py-3 text-sm font-semibold transition active:scale-[.98] focus-visible:outline-2 focus-visible:outline-brand-500 ${className}`;
  return asChild ? <Slot className={classes} {...props} /> : <button className={classes} {...props} />;
}

function Header() {
  const [dark, setDark] = useState(() => localStorage.getItem("theme") !== "light");
  useEffect(() => { document.documentElement.classList.toggle("dark", dark); localStorage.setItem("theme", dark ? "dark" : "light"); }, [dark]);
  return (
    <header className="sticky top-0 z-10 border-b border-black/10 bg-[#f5f5f5]/90 text-slate-900 backdrop-blur-xl print:hidden dark:border-white/10 dark:bg-[#121212]/85 dark:text-white">
      <div className="mx-auto flex h-16 max-w-4xl items-center justify-between px-5">
        <div className="flex items-center gap-3 font-bold tracking-tight"><span className="grid size-9 place-items-center rounded-xl bg-brand-600 text-white">QR</span> QRIS Dinamis</div>
        <div className="flex items-center gap-1 text-slate-600 dark:text-white/60">
          <Tooltip.Provider delayDuration={250}>
            <Tooltip.Root><Tooltip.Trigger asChild><a className="grid size-10 place-items-center rounded-xl hover:bg-black/5 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white" href="https://github.com/KevinAntonioWiyonoLauw" target="_blank" rel="noreferrer" aria-label="Repositori GitHub"><svg viewBox="0 0 16 16" width="20" height="20" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg></a></Tooltip.Trigger><Tooltip.Portal><Tooltip.Content side="bottom" sideOffset={8} collisionPadding={12} className="relative z-[9999] rounded-lg bg-white px-2 py-1 text-xs text-black shadow-lg">Repositori GitHub</Tooltip.Content></Tooltip.Portal></Tooltip.Root>
            <Tooltip.Root><Tooltip.Trigger asChild><button className="grid size-10 place-items-center rounded-xl hover:bg-black/5 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white" onClick={() => setDark(!dark)} aria-label="Ganti tema">{dark ? "☼" : "☾"}</button></Tooltip.Trigger><Tooltip.Portal><Tooltip.Content side="bottom" sideOffset={8} collisionPadding={12} className="relative z-[9999] rounded-lg bg-white px-2 py-1 text-xs text-black shadow-lg">Ganti tema</Tooltip.Content></Tooltip.Portal></Tooltip.Root>
          </Tooltip.Provider>
        </div>
      </div>
    </header>
  );
}

function TLVTree({ list }: { list: TLVPair[] }) {
  return (
    <ul className="space-y-1 font-mono text-xs">
      {list.map((item, index) => (
        <li key={`${item.tag}-${index}`}>
          <div className="flex gap-2"><span className="w-12 shrink-0 text-brand-500">{item.tag}</span><span className="w-8 shrink-0 text-slate-400 dark:text-white/30">{item.length}</span><span className="break-all text-slate-700 dark:text-white/70">{item.value || item.name}</span></div>
          {item.children && <div className="ml-6 border-l border-black/10 pl-3 dark:border-white/10"><TLVTree list={item.children} /></div>}
        </li>
      ))}
    </ul>
  );
}

function InfoCard({ data }: { data: QRISData }) {
  const [showTLV, setShowTLV] = useState(false);
  const rows = [["Merchant", data.merchantName], ["Kota", data.merchantCity], ["Kode Pos", data.postalCode], ["Penerbit", data.merchantAccountInfo?.[0]?.globallyUniqueId || "-"], ["Kategori", mcc[data.merchantCategoryCode] || data.merchantCategoryCode], ["Mata uang", currency[data.currency] || data.currency]];
  return (
    <section className="animate-enter rounded-2xl border border-black/10 bg-[#ffffff] p-5 shadow-[0_18px_45px_rgb(23_23_23/0.06)] dark:border-white/10 dark:bg-[#171717]">
      <h2 className="mb-4 text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-white/50">Informasi QRIS</h2>
      <div className="grid gap-x-5 sm:grid-cols-2">
        {rows.map(([label, value]) => (
          <div className="border-t border-black/10 py-3 first:border-t-0 sm:nth-[2]:border-t-0 dark:border-white/10" key={label}>
            <p className="mb-1 text-xs text-slate-400 dark:text-white/40">{label}</p>
            <p className="truncate text-sm font-semibold">{value}</p>
          </div>
        ))}
        <div className="border-t border-black/10 py-3 dark:border-white/10"><p className="mb-1 text-xs text-slate-400 dark:text-white/40">Metode</p><span className={`rounded-full px-2.5 py-1 text-xs font-bold ${data.method === "static" ? "bg-amber-400/15 text-amber-600 dark:text-amber-300" : "bg-slate-500/15 text-slate-600 dark:text-slate-300"}`}>{data.method === "static" ? "Statis" : "Dinamis"}</span></div>
        {data.amount && <div className="border-t border-black/10 py-3 dark:border-white/10"><p className="mb-1 text-xs text-slate-400 dark:text-white/40">Jumlah</p><p className="text-sm font-semibold">Rp {Number(data.amount).toLocaleString("id-ID")}</p></div>}
      </div>
      <button type="button" onClick={() => setShowTLV(!showTLV)} className="mt-4 w-full rounded-xl border border-black/10 px-4 py-2.5 text-sm font-semibold text-slate-600 transition hover:bg-black/5 dark:border-white/10 dark:text-white/60 dark:hover:bg-white/5">{showTLV ? "Sembunyikan detail TLV" : "Lihat detail TLV"}</button>
      {showTLV && <div className="mt-3 max-h-72 overflow-auto rounded-xl bg-black/[.03] p-3 dark:bg-white/[.04]"><TLVTree list={data.raw} /></div>}
    </section>
  );
}

function Scanner({ onValue }: { onValue: (value: string) => void }) {
  const input = useRef<HTMLInputElement>(null); const video = useRef<HTMLVideoElement>(null); const canvas = useRef<HTMLCanvasElement>(null); const stream = useRef<MediaStream | null>(null); const raf = useRef(0); const [scanning, setScanning] = useState(false);
  const stop = useCallback(() => { stream.current?.getTracks().forEach((track) => track.stop()); stream.current = null; cancelAnimationFrame(raf.current); setScanning(false); }, []);
  const decode = (file: File) => { const reader = new FileReader(); reader.onload = () => { const image = new Image(); image.onload = () => { const c = document.createElement("canvas"); c.width = image.width; c.height = image.height; const ctx = c.getContext("2d"); if (!ctx) return; ctx.drawImage(image, 0, 0); const result = jsQR(ctx.getImageData(0, 0, c.width, c.height).data, c.width, c.height); if (result) onValue(result.data); else alert("QR tidak ditemukan di gambar. Coba gambar lain."); }; image.src = String(reader.result); }; reader.readAsDataURL(file); };
  const camera = async () => { if (scanning) return stop(); try { stream.current = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" } }); setScanning(true); if (!video.current || !canvas.current) return; video.current.srcObject = stream.current; await video.current.play(); const ctx = canvas.current.getContext("2d"); if (!ctx) return; const loop = () => { if (!stream.current) return; if (video.current!.readyState >= 2) { canvas.current!.width = video.current!.videoWidth; canvas.current!.height = video.current!.videoHeight; ctx.drawImage(video.current!, 0, 0); const result = jsQR(ctx.getImageData(0, 0, canvas.current!.width, canvas.current!.height).data, canvas.current!.width, canvas.current!.height); if (result) { onValue(result.data); stop(); return; } } raf.current = requestAnimationFrame(loop); }; loop(); } catch { alert("Akses kamera ditolak atau tidak tersedia."); stop(); } };
  useEffect(() => () => stop(), [stop]);
  return (
    <div className="mt-5">
      <div className="grid gap-3 sm:grid-cols-2">
        <Button className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]" onClick={() => input.current?.click()}>▧ Unggah gambar</Button>
        <Button className={scanning ? "border-red-400/40 bg-red-500/15 text-red-200" : "border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]"} onClick={camera}>{scanning ? "Hentikan kamera" : "◉ Scan kamera"}</Button>
      </div>
      <input ref={input} className="hidden" type="file" accept="image/*" onChange={(event) => { const file = event.target.files?.[0]; if (file) decode(file); event.target.value = ""; }} />
      {scanning && <div className="relative mt-4 overflow-hidden rounded-2xl border border-black/10 bg-black dark:border-white/15"><video ref={video} className="block aspect-video w-full object-cover" playsInline muted /><div className="pointer-events-none absolute inset-0 grid place-items-center"><div className="size-40 rounded-2xl border-2 border-white/80 shadow-[0_0_0_999px_rgb(0_0_0/0.25)]" /></div><p className="absolute bottom-3 left-0 right-0 text-center text-xs font-medium text-white drop-shadow">Arahkan kamera ke kode QRIS</p></div>}
      <canvas ref={canvas} className="hidden" />
    </div>
  );
}

function Templates({ onPick, active }: { onPick: (t: Template) => void; active: Template | null }) {
  const [templates, setTemplates] = useState<Template[]>(() => { try { return JSON.parse(localStorage.getItem("templates") || "[]"); } catch { return []; } });
  const [name, setName] = useState("");
  const [qris, setQris] = useState("");
  useEffect(() => localStorage.setItem("templates", JSON.stringify(templates)), [templates]);
  const save = () => { if (!name.trim() || !qris.trim()) return; setTemplates((prev) => [...prev, { id: crypto.randomUUID(), name: name.trim(), qris: qris.trim() }]); setName(""); setQris(""); };
  const remove = (id: string) => setTemplates((prev) => prev.filter((t) => t.id !== id));
  return (
    <section className="rounded-2xl border border-black/10 bg-[#ffffff] p-5 shadow-[0_18px_45px_rgb(23_23_23/0.06)] dark:border-white/10 dark:bg-[#171717]">
      <h2 className="mb-4 text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-white/50">Template merchant</h2>
      <div className="grid gap-2 sm:grid-cols-2">
        <input value={name} onChange={(event) => setName(event.target.value)} placeholder="Nama template" className="rounded-xl border border-black/10 bg-[#fafafa] px-4 py-2.5 text-sm outline-none focus:border-brand-500 dark:border-white/15 dark:bg-[#121212]" />
        <input value={qris} onChange={(event) => setQris(event.target.value)} placeholder="String QRIS statis" className="rounded-xl border border-black/10 bg-[#fafafa] px-4 py-2.5 font-mono text-xs outline-none focus:border-brand-500 dark:border-white/15 dark:bg-[#121212]" />
      </div>
      <Button onClick={save} className="mt-2 w-full border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Simpan template</Button>
      <div className="mt-3 space-y-2">
        {templates.length === 0 && <p className="text-sm text-slate-400 dark:text-white/30">Belum ada template.</p>}
        {templates.map((t) => (
          <div key={t.id} className="flex items-center justify-between gap-2 rounded-xl border border-black/10 px-3 py-2 dark:border-white/10">
            <button type="button" onClick={() => onPick(t)} className="flex-1 truncate text-left text-sm font-semibold hover:text-brand-500">{t.name}</button>
            {active?.id === t.id && <span className="rounded-full bg-brand-500/15 px-2 py-0.5 text-[10px] font-bold text-brand-600 dark:text-brand-300">Aktif</span>}
            <button type="button" onClick={() => remove(t.id)} aria-label={`Hapus ${t.name}`} className="px-2 text-slate-400 hover:text-red-500 dark:text-white/30">×</button>
          </div>
        ))}
      </div>
    </section>
  );
}

function History({ onPick }: { onPick: (code: string) => void }) {
  const [history, setHistory] = useState<HistoryItem[]>(() => { try { return JSON.parse(localStorage.getItem("history") || "[]"); } catch { return []; } });
  useEffect(() => localStorage.setItem("history", JSON.stringify(history)), [history]);
  const remove = (at: number) => setHistory((prev) => prev.filter((item) => item.at !== at));
  const clear = () => setHistory([]);
  return (
    <section className="rounded-2xl border border-black/10 bg-[#ffffff] p-5 shadow-[0_18px_45px_rgb(23_23_23/0.06)] dark:border-white/10 dark:bg-[#171717]">
      <div className="mb-3 flex items-center justify-between"><h2 className="text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-white/50">Riwayat</h2>{history.length > 0 && <button type="button" onClick={clear} className="text-xs text-slate-400 hover:text-red-500 dark:text-white/30">Hapus semua</button>}</div>
      <div className="space-y-2">
        {history.length === 0 && <p className="text-sm text-slate-400 dark:text-white/30">Belum ada konversi.</p>}
        {history.map((item) => (
          <div key={item.at} className="flex items-center justify-between gap-2 rounded-xl border border-black/10 px-3 py-2 dark:border-white/10">
            <button type="button" onClick={() => onPick(item.code)} className="flex-1 truncate text-left text-sm"><span className="font-semibold">{item.merchant || "QRIS"}</span> <span className="text-slate-400 dark:text-white/30">· Rp {Number(item.amount).toLocaleString("id-ID")}</span></button>
            <span className="shrink-0 text-[10px] text-slate-400 dark:text-white/30">{new Date(item.at).toLocaleString("id-ID")}</span>
            <button type="button" onClick={() => remove(item.at)} aria-label="Hapus riwayat" className="px-2 text-slate-400 hover:text-red-500 dark:text-white/30">×</button>
          </div>
        ))}
      </div>
    </section>
  );
}

type AdminTransaction = { id: number; reference: string; amount: string; merchant: string; status: string; provider?: string; created_at: string };

function AdminPage() {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [authenticated, setAuthenticated] = useState(false);
  const [transactions, setTransactions] = useState<AdminTransaction[]>([]);
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const loadTransactions = useCallback(async () => {
    const response = await fetch("/api/transactions/list", { credentials: "same-origin" });
    const data = await response.json() as { transactions?: AdminTransaction[]; error?: string };
    if (!response.ok) throw new Error(data.error || "Gagal memuat transaksi");
    setTransactions(data.transactions || []);
  }, []);

  const login = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setLoading(true); setError("");
    try {
      const response = await fetch("/api/admin/login", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) });
      const data = await response.json() as { error?: string };
      if (!response.ok) throw new Error(data.error || "Login gagal");
      setAuthenticated(true); setPassword(""); await loadTransactions();
    } catch (err) { setError(err instanceof Error ? err.message : "Login gagal"); }
    finally { setLoading(false); }
  };

  const createKey = async () => {
    setError("");
    try {
      const response = await fetch("/api/admin/keys", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ label: "POS utama" }) });
      const data = await response.json() as { api_key?: string; error?: string };
      if (!response.ok) throw new Error(data.error || "Gagal membuat API key");
      setApiKey(data.api_key || "");
    } catch (err) { setError(err instanceof Error ? err.message : "Gagal membuat API key"); }
  };

  if (!authenticated) return <><Header /><main className="mx-auto flex min-h-[calc(100dvh-8rem)] w-full max-w-md items-center px-5 py-12"><form onSubmit={login} className="w-full rounded-2xl border border-black/10 bg-white p-6 shadow-xl dark:border-white/10 dark:bg-[#171717]"><p className="mb-2 font-mono text-xs uppercase tracking-[.18em] text-slate-400 dark:text-white/40">Area admin</p><h1 className="text-2xl font-extrabold tracking-tight">Masuk ke dashboard</h1><label className="mt-6 block text-sm font-semibold" htmlFor="admin-user">Username</label><input id="admin-user" value={username} onChange={(event) => setUsername(event.target.value)} className="mt-2 w-full rounded-xl border border-black/15 bg-[#f5f5f5] px-4 py-3 text-sm outline-none focus:border-slate-700 dark:border-white/15 dark:bg-[#121212]" autoComplete="username" /><label className="mt-4 block text-sm font-semibold" htmlFor="admin-pass">Password</label><input id="admin-pass" value={password} onChange={(event) => setPassword(event.target.value)} className="mt-2 w-full rounded-xl border border-black/15 bg-[#f5f5f5] px-4 py-3 text-sm outline-none focus:border-slate-700 dark:border-white/15 dark:bg-[#121212]" type="password" autoComplete="current-password" required /><button className="mt-6 w-full rounded-xl bg-neutral-800 px-4 py-3 text-sm font-bold text-white transition hover:bg-neutral-950 disabled:opacity-50 dark:bg-white dark:text-neutral-900 dark:hover:bg-neutral-200" disabled={loading}>{loading ? "Memeriksa..." : "Masuk"}</button>{error && <p className="mt-4 rounded-xl border border-red-300 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">{error}</p>}</form></main></>;

  return <><Header /><main className="mx-auto min-h-[calc(100dvh-8rem)] w-full max-w-5xl px-5 py-10"><div className="flex flex-wrap items-end justify-between gap-4"><div><p className="font-mono text-xs uppercase tracking-[.18em] text-slate-400 dark:text-white/40">Admin</p><h1 className="mt-2 text-3xl font-extrabold tracking-tight">Transaksi</h1></div><div className="flex gap-2"><button onClick={() => loadTransactions().catch((err) => setError(err.message))} className="rounded-xl border border-black/10 bg-white px-4 py-2 text-sm font-semibold hover:bg-black/5 dark:border-white/10 dark:bg-[#171717] dark:hover:bg-white/5">Refresh</button><a href="/" className="rounded-xl bg-neutral-800 px-4 py-2 text-sm font-semibold text-white hover:bg-neutral-950 dark:bg-white dark:text-neutral-900">Converter</a></div></div><section className="mt-6 rounded-2xl border border-black/10 bg-white p-5 shadow-sm dark:border-white/10 dark:bg-[#171717]"><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-bold">API key POS</h2><p className="mt-1 text-sm text-slate-500 dark:text-white/45">Buat sekali, simpan nilainya. Key tidak ditampilkan ulang.</p></div><button onClick={createKey} className="rounded-xl bg-neutral-800 px-4 py-2.5 text-sm font-semibold text-white hover:bg-neutral-950 dark:bg-white dark:text-neutral-900">Buat API key</button></div>{apiKey && <code className="mt-4 block break-all rounded-xl bg-[#f5f5f5] p-4 font-mono text-xs text-slate-700 dark:bg-[#121212] dark:text-white/70">{apiKey}</code>}</section>{error && <p className="mt-4 rounded-xl border border-red-300 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">{error}</p>}<section className="mt-6 overflow-hidden rounded-2xl border border-black/10 bg-white shadow-sm dark:border-white/10 dark:bg-[#171717]"><div className="border-b border-black/10 px-5 py-4 dark:border-white/10"><h2 className="font-bold">Daftar transaksi</h2></div>{transactions.length === 0 ? <p className="p-5 text-sm text-slate-500 dark:text-white/45">Belum ada transaksi.</p> : <div className="overflow-x-auto"><table className="w-full min-w-[650px] text-left text-sm"><thead className="bg-[#f5f5f5] text-xs uppercase tracking-wide text-slate-500 dark:bg-[#121212] dark:text-white/40"><tr><th className="px-5 py-3">Reference</th><th className="px-5 py-3">Merchant</th><th className="px-5 py-3">Amount</th><th className="px-5 py-3">Status</th><th className="px-5 py-3">Created</th></tr></thead><tbody>{transactions.map((txn) => <tr key={txn.id} className="border-t border-black/10 dark:border-white/10"><td className="px-5 py-3 font-mono text-xs">{txn.reference}</td><td className="px-5 py-3 font-semibold">{txn.merchant}</td><td className="px-5 py-3">Rp {Number(txn.amount).toLocaleString("id-ID")}</td><td className="px-5 py-3"><span className="rounded-full bg-slate-500/15 px-2.5 py-1 text-xs font-semibold text-slate-700 dark:text-slate-300">{txn.status}</span></td><td className="px-5 py-3 text-xs text-slate-500 dark:text-white/45">{new Date(txn.created_at).toLocaleString("id-ID")}</td></tr>)}</tbody></table></div>}</section></main></>;
}

function App() {
  const [qris, setQris] = useState("");
  const [data, setData] = useState<QRISData | null>(null);
  const [errors, setErrors] = useState<string[]>([]);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [amount, setAmount] = useState("");
  const [feeType, setFeeType] = useState<"none" | "fixed" | "percentage">("none");
  const [fee, setFee] = useState("");
  const [result, setResult] = useState<{ code: string; data: QRISData } | null>(null);
   const [batchInput, setBatchInput] = useState("");
   const [batchResult, setBatchResult] = useState<{ nominal: string; code: string }[]>([]);
   const [batchOpen, setBatchOpen] = useState(false);
  const [traktirAmount, setTraktirAmount] = useState("");
  const [traktirResult, setTraktirResult] = useState<string | null>(null);
  const [offline, setOffline] = useState(() => typeof navigator !== "undefined" ? !navigator.onLine : false);
  const [history, setHistory] = useState<HistoryItem[]>(() => { try { return JSON.parse(localStorage.getItem("history") || "[]"); } catch { return []; } });
  const [templates, setTemplates] = useState<Template[]>(() => { try { return JSON.parse(localStorage.getItem("templates") || "[]"); } catch { return []; } });
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => {
    const online = () => setOffline(false);
    const offlineEvent = () => setOffline(true);
    window.addEventListener("online", online);
    window.addEventListener("offline", offlineEvent);
    return () => { window.removeEventListener("online", online); window.removeEventListener("offline", offlineEvent); };
  }, []);

  useEffect(() => localStorage.setItem("history", JSON.stringify(history)), [history]);
  useEffect(() => localStorage.setItem("templates", JSON.stringify(templates)), [templates]);

  const addHistory = (code: string) => {
    try {
      const parsed = parseOffline(code);
      setHistory((prev) => [{ code, merchant: parsed.merchantName, amount: parsed.amount || "0", at: Date.now() }, ...prev.filter((item) => item.code !== code)].slice(0, 50));
    } catch { /* noop */ }
  };

  const inspect = (value: string) => {
    setQris(value); setData(null); setResult(null); setErrors([]); setWarnings([]); window.clearTimeout(timer.current);
    if (!value.trim()) return;
    const valueTrim = value.trim();
    timer.current = window.setTimeout(async () => {
      try {
        let parsed: QRISData;
        let check: { valid: boolean; errors: string[] };
        if (offline || navigator.onLine === false) {
          parsed = parseOffline(valueTrim);
          check = validateOffline(valueTrim);
        } else {
          try {
            const serverValidate = await post<{ valid: boolean; errors: string[] }>("/api/validate", { qris: valueTrim });
            check = serverValidate;
            const serverParse = await post<{ data: QRISData }>("/api/parse", { qris: valueTrim });
            parsed = serverParse.data;
          } catch {
            parsed = parseOffline(valueTrim);
            check = validateOffline(valueTrim);
            setOffline(true);
          }
        }
        if (!check.valid) { setErrors(check.errors); return; }
        const strict = strictValidate(valueTrim);
        if (strict.warnings.length > 0) setWarnings(strict.warnings);
        setData(parsed);
      } catch (error) { setErrors([error instanceof Error ? error.message : "Permintaan gagal"]); }
    }, 280);
  };

  const convert = async () => {
    const value = Number.parseInt(amount, 10);
    if (!data || !Number.isInteger(value) || value <= 0) return;
    const options: OfflineConvertOptions = { amount: String(value), ...(feeType !== "none" && fee ? { fee: { type: feeType, value: Number.parseFloat(fee) } } : {}) };
    try {
      let codeResult: string;
      let parsedResult: QRISData;
      if (offline || navigator.onLine === false) {
        codeResult = convertOffline(qris.trim(), options);
        parsedResult = parseOffline(codeResult);
      } else {
        try {
          const response = await post<{ result: string; parsed: QRISData }>("/api/convert", { qris: qris.trim(), amount: String(value), ...(options.fee ? { fee: options.fee } : {}) });
          codeResult = response.result; parsedResult = response.parsed;
        } catch {
          setOffline(true);
          codeResult = convertOffline(qris.trim(), options);
          parsedResult = parseOffline(codeResult);
        }
      }
      setResult({ code: codeResult, data: parsedResult });
      addHistory(codeResult);
    } catch (error) { setErrors([error instanceof Error ? error.message : "Konversi gagal"]); }
  };

  const runBatch = async () => {
    if (!data) return;
    const nominals = batchInput.split(/[\s,;\n]+/).map((s) => s.trim()).filter(Boolean).map(Number).filter((n) => Number.isInteger(n) && n > 0);
    if (nominals.length === 0) { setErrors(["Tidak ada nominal valid di input batch."]); return; }
    const items: { nominal: string; code: string }[] = [];
    for (const nominal of nominals) {
      const converted = convertOffline(qris.trim(), { amount: String(nominal) });
      items.push({ nominal: String(nominal), code: converted });
    }
    setBatchResult(items);
    items.forEach((item) => addHistory(item.code));
  };

  const downloadBatchCSV = () => {
    if (batchResult.length === 0) return;
    const lines = ["merchant,nominal,qris"];
    for (const item of batchResult) lines.push(`"${data?.merchantName ?? ""}","${item.nominal}","${item.code}"`);
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8" });
    const a = document.createElement("a"); a.href = URL.createObjectURL(blob); a.download = "batch-qris.csv"; a.click(); URL.revokeObjectURL(a.href);
  };

  const downloadBatchZIP = async () => {
    if (batchResult.length === 0) return;
    const response = await fetch("/api/batch-zip", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ qris: qris.trim(), items: batchResult.map((i) => ({ nominal: i.nominal, code: i.code })) }) });
    if (!response.ok) { const err = await response.json().catch(() => ({})); setErrors([err.error || "Gagal download ZIP"]); return; }
    const blob = await response.blob();
    const a = document.createElement("a"); a.href = URL.createObjectURL(blob); a.download = "batch-qris.zip"; a.click(); URL.revokeObjectURL(a.href);
  };

  const downloadPDF = async () => {
    if (!result) return;
    const response = await fetch("/api/pdf", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ qris: result.code }) });
    if (!response.ok) { const err = await response.json().catch(() => ({})); setErrors([err.error || "Gagal generate PDF"]); return; }
    const blob = await response.blob();
    const a = document.createElement("a"); a.href = URL.createObjectURL(blob); a.download = `qris-${result.data.merchantName.toLowerCase().replace(/\s+/g, "-")}.pdf`; a.click(); URL.revokeObjectURL(a.href);
  };

  const share = async () => {
    if (!result) return;
    const qrUrl = `${location.origin}/api/qr?data=${encodeURIComponent(result.code)}&size=280`;
    const text = `QRIS ${result.data.merchantName} Rp ${Number(result.data.amount || 0).toLocaleString("id-ID")}\nCek di page: ${location.origin}`;
    const file = await qrToPngFile(result.code, "qris.png");
    try {
      // try sharing with image attached (WhatsApp/Android share sheets)
      const payload = { title: "QRIS Dinamis", text, files: [file] };
      if ((navigator as any).canShare?.(payload)) { await navigator.share(payload); return; }
    } catch { /* image share unsupported → fall back below */ }
    try {
      if (navigator.share) { await navigator.share({ title: "QRIS Dinamis", text }); return; }
      await navigator.clipboard.writeText(text);
      alert("QRIS disalin ke clipboard.");
    } catch { /* cancel */ }
  };

  // fetch SVG QR from /api/qr, rasterize to PNG (white background) for share/download
  const qrToPngFile = async (qrText: string, filename: string) => {
    const res = await fetch(`/api/qr?data=${encodeURIComponent(qrText)}&size=560`);
    const svg = await res.text();
    const blobUrl = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
    const img = new Image();
    await new Promise<void>((resolve, reject) => { img.onload = () => resolve(); img.onerror = () => reject(new Error("QR load failed")); img.src = blobUrl; });
    const canvas = document.createElement("canvas");
    canvas.width = 560; canvas.height = 560;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("canvas unavailable");
    ctx.fillStyle = "#ffffff"; ctx.fillRect(0, 0, 560, 560);
    ctx.drawImage(img, 0, 0, 560, 560);
    URL.revokeObjectURL(blobUrl);
    const png = await new Promise<Blob | null>((r) => canvas.toBlob(r, "image/png"));
    if (!png) throw new Error("png encode failed");
    return new File([png], filename, { type: "image/png" });
  };

  const downloadQrPng = async (qrText: string, filename: string) => {
    try {
      const file = await qrToPngFile(qrText, filename);
      const url = URL.createObjectURL(file);
      const a = document.createElement("a");
      a.href = url; a.download = filename; a.click();
      URL.revokeObjectURL(url);
    } catch (error) { setErrors([error instanceof Error ? error.message : "Gagal unduh QR"]); }
  };

  const generateTraktir = () => {
    const value = Number.parseInt(traktirAmount, 10);
    if (!Number.isInteger(value) || value <= 0) { setTraktirResult(null); return; }
    try {
      setTraktirResult(convertOffline(TRAKTIR_BASE, { amount: String(value) }));
    } catch (error) { setErrors([error instanceof Error ? error.message : "Gagal generate QR traktir"]); }
  };

  const pickFromHistory = (code: string) => { inspect(code); };
  const pickTemplate = (template: Template) => { inspect(template.qris); };

  if (window.location.pathname === "/admin") return <AdminPage />;
  return (
    <><Header />
    <main className="relative z-[1] mx-auto w-full max-w-4xl px-5 pb-16 print:max-w-none print:px-0">
      <section className="py-14 print:hidden">
        <p className="mb-3 font-mono text-xs uppercase tracking-[.2em] text-brand-500">Pembayaran QR Indonesia</p>
        <h1 className="max-w-2xl text-4xl font-extrabold leading-[1.05] tracking-[-.04em] sm:text-5xl">Satu string. QRIS dinamis siap bayar.</h1>
        <p className="mt-5 max-w-xl text-base leading-7 text-slate-500 dark:text-white/55">Validasi QRIS statis, tambahkan nominal, lalu buat kode baru dengan checksum yang benar. Bisa jalan offline tanpa server.</p>
        {offline && <div className="mt-4 inline-flex items-center gap-2 rounded-xl border border-amber-400/40 bg-amber-400/10 px-3 py-2 text-xs font-semibold text-amber-600 dark:text-amber-300">Mode offline aktif: konversi berjalan di perangkat ini.</div>}
      </section>

       <section className={`rounded-2xl border border-black/10 bg-[#ffffff] p-5 text-slate-900 shadow-[0_18px_45px_rgb(23_23_23/0.06)] sm:p-6 print:border-0 print:shadow-none dark:border-white/10 dark:bg-[#171717] dark:text-white ${result ? "print:hidden" : ""}`}>
        <div className="mb-3 flex items-center justify-between print:hidden">
          <label htmlFor="qris" className="text-sm font-bold">Tempel string QRIS</label>
          <div className="flex gap-2"><span className="font-mono text-[11px] text-slate-400 dark:text-white/35">TLV / CRC16</span>{offline && <span className="rounded-full bg-amber-400/15 px-2 py-0.5 text-[10px] font-bold text-amber-600 dark:text-amber-300">Offline</span>}</div>
        </div>
         <textarea id="qris" value={qris} onChange={(event) => inspect(event.target.value)} placeholder="000201010211..." spellCheck={false} className={`min-h-36 w-full resize-y rounded-xl border border-dashed border-black/20 bg-[#fafafa] px-4 py-4 font-mono text-sm leading-6 text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-brand-500 focus:ring-4 focus:ring-brand-500/10 dark:border-white/20 dark:bg-[#121212] dark:text-white dark:placeholder:text-white/45 ${result ? "print:min-h-0 print:border-0 print:bg-transparent" : ""}`} />
        <div className="print:hidden"><Scanner onValue={inspect} /></div>
        {errors.length > 0 && <div className="mt-4 rounded-xl border border-red-400/25 bg-red-500/10 p-4 text-sm text-red-200"><ul className="space-y-1">{errors.map((error) => <li key={error}>× {error}</li>)}</ul></div>}
        {warnings.length > 0 && <div className="mt-4 rounded-xl border border-amber-400/25 bg-amber-400/10 p-4 text-sm text-amber-200"><ul className="space-y-1">{warnings.map((warning) => <li key={warning}>! {warning}</li>)}</ul></div>}
      </section>

      {data && <div className="mt-4 space-y-4 print:hidden"><InfoCard data={data} /></div>}

       <div className="mt-6 grid gap-4 lg:grid-cols-2 print:hidden">
        {templates.length > 0 && <div className="space-y-4"><Templates onPick={pickTemplate} active={null} /></div>}
      </div>

      {data && data.method === "static" && (
         <section className={`animate-enter rounded-2xl border border-black/10 bg-[#ffffff] p-5 text-slate-900 shadow-[0_18px_45px_rgb(23_23_23/0.06)] print:hidden dark:border-white/10 dark:bg-[#171717] dark:text-white`}>
           <div className="mb-5 flex items-center justify-between gap-3">
             <h2 className="text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-white/50">Buat nominal</h2>
             <button type="button" onClick={() => setBatchOpen((open) => !open)} className="rounded-xl border border-black/10 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-black/5 dark:border-white/15 dark:text-white/70 dark:hover:bg-white/5">{batchOpen ? "Tutup batch" : "Buat batch"}</button>
           </div>
          <label className="mb-2 block text-sm font-semibold" htmlFor="amount">Jumlah rupiah</label>
           <div className="relative"><span className="absolute left-4 top-1/2 -translate-y-1/2 font-mono text-sm text-slate-500 dark:text-white/55">Rp</span><input id="amount" value={amount} onChange={(event) => setAmount(event.target.value)} type="number" min="1" placeholder="Masukkan nominal" className="w-full rounded-xl border border-black/10 bg-[#fafafa] py-3 pl-11 pr-4 text-base text-slate-900 outline-none placeholder:text-slate-400 focus:border-brand-500 focus:ring-4 focus:ring-brand-500/10 dark:border-white/15 dark:bg-[#121212] dark:text-white dark:placeholder:text-white/45" /></div>
          <div className="mt-3 flex flex-wrap gap-2">
            {PRESETS.map((preset) => <button type="button" key={preset} onClick={() => setAmount(String(preset))} className={`rounded-xl border px-3.5 py-2 text-sm font-semibold transition ${amount === String(preset) ? "border-brand-500 bg-brand-500/10 text-brand-600 dark:text-brand-300" : "border-black/10 text-slate-500 hover:bg-black/5 dark:border-white/10 dark:text-white/50 dark:hover:bg-white/5"}`}>Rp {preset.toLocaleString("id-ID")}</button>)}
          </div>
          <div className="mt-5"><span className="mb-2 block text-sm font-semibold">Biaya layanan</span><div className="grid grid-cols-3 gap-2">{(["none", "fixed", "percentage"] as const).map((type) => <button type="button" key={type} onClick={() => { setFeeType(type); setFee(""); }} className={`rounded-xl border px-2 py-2.5 text-xs font-semibold transition ${feeType === type ? "border-brand-500 bg-brand-500/15 text-brand-600 dark:text-brand-300" : "border-black/10 text-slate-500 hover:bg-black/5 dark:border-white/10 dark:text-white/50 dark:hover:bg-white/5"}`}>{type === "none" ? "Tanpa" : type === "fixed" ? "Tetap (Rp)" : "Persen (%)"}</button>)}</div></div>
           {feeType !== "none" && <input value={fee} onChange={(event) => setFee(event.target.value)} type="number" min="0" step={feeType === "percentage" ? ".1" : "1"} placeholder={feeType === "fixed" ? "Masukkan biaya tetap" : "Masukkan persentase"} className="mt-3 w-full rounded-xl border border-black/10 bg-[#fafafa] px-4 py-3 text-sm text-slate-900 outline-none placeholder:text-slate-400 focus:border-brand-500 dark:border-white/15 dark:bg-[#121212] dark:text-white dark:placeholder:text-white/45" />}
          <Button onClick={convert} className="mt-5 w-full border-brand-600 bg-brand-600 text-white hover:bg-brand-700">Konversi ke QRIS dinamis</Button>
        </section>
      )}

       {data && data.method !== "static" && (
        <div className="mt-6 rounded-2xl border border-slate-500/25 bg-slate-500/10 p-5 text-sm text-slate-500 print:hidden dark:bg-slate-500/10 dark:text-slate-200">
          <p>QRIS ini sudah dinamis dengan nominal Rp {Number(data.amount || 0).toLocaleString("id-ID")}.</p>
          {qris.trim() && (
            <div className="mx-auto my-5 w-72 rounded-xl bg-white p-4 shadow-md">
              <img className="w-full" src={`/api/qr?data=${encodeURIComponent(qris.trim())}&size=280`} alt="QRIS dinamis" />
            </div>
          )}
        </div>
      )}

       {data && data.method === "static" && batchOpen && (
         <section className="mt-6 rounded-2xl border border-black/10 bg-[#ffffff] p-5 text-slate-900 shadow-[0_18px_45px_rgb(23_23_23/0.06)] print:hidden dark:border-white/10 dark:bg-[#171717] dark:text-white">
          <h2 className="mb-3 text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-white/50">Konversi batch</h2>
           <textarea value={batchInput} onChange={(event) => setBatchInput(event.target.value)} rows={3} placeholder="25000&#10;50000&#10;100000&#10;(pisah dengan baris, koma, atau titik koma)" className="w-full rounded-xl border border-black/10 bg-[#fafafa] px-4 py-3 font-mono text-xs text-slate-900 outline-none placeholder:text-slate-400 focus:border-brand-500 dark:border-white/15 dark:bg-[#121212] dark:text-white dark:placeholder:text-white/45" />
          <div className="mt-3 flex flex-wrap gap-2">
            <Button onClick={runBatch} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Generate batch</Button>
            {batchResult.length > 0 && <><Button onClick={downloadBatchCSV} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">CSV</Button><Button onClick={downloadBatchZIP} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">ZIP QR</Button></>}
          </div>
          {batchResult.length > 0 && <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3"><div key="header" className="rounded-xl border border-black/10 px-3 py-2 text-xs font-bold uppercase text-slate-400 dark:border-white/10 dark:text-white/30">Nominal</div><div className="rounded-xl border border-black/10 px-3 py-2 text-xs font-bold uppercase text-slate-400 dark:border-white/10 dark:text-white/30">Hasil</div><div className="hidden rounded-xl border border-black/10 px-3 py-2 text-xs font-bold uppercase text-slate-400 dark:border-white/10 dark:text-white/30 lg:block">Aksi</div>{batchResult.map((item) => <div key={item.nominal} className="flex items-center gap-2 rounded-xl border border-black/10 px-3 py-2 text-xs dark:border-white/10"><span className="w-20 shrink-0 font-bold">Rp {Number(item.nominal).toLocaleString("id-ID")}</span><span className="truncate font-mono text-[10px] text-slate-500 dark:text-white/40">{item.code}</span><button type="button" onClick={() => inspect(item.code)} className="ml-auto shrink-0 rounded-lg px-2 py-1 text-slate-500 hover:bg-black/5 dark:text-white/40 dark:hover:bg-white/5">Pakai</button></div>)}</div>}
        </section>
      )}

      {result && (
         <section className="animate-enter mt-6 rounded-2xl border border-brand-500/25 bg-slate-500/5 p-5 text-center dark:bg-brand-500/10">
          <h2 className="print:hidden text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-brand-300">QRIS siap digunakan</h2>
          <div className="mx-auto my-5 w-72 rounded-xl bg-white p-4 shadow-md"><img className="w-full" src={`/api/qr?data=${encodeURIComponent(result.code)}&size=280`} alt="QRIS dinamis" /></div>
          <p className="text-sm font-semibold">{result.data.merchantName}</p>
          <p className="mt-1 text-3xl font-extrabold text-slate-800 dark:text-brand-300">Rp {Number(result.data.amount || 0).toLocaleString("id-ID")}</p>
          {result.data.tipIndicator === "fixed" && result.data.tipFixed && <p className="mt-1 text-xs text-slate-400 dark:text-white/40">+ biaya Rp {Number(result.data.tipFixed).toLocaleString("id-ID")}</p>}
          {result.data.tipIndicator === "percentage" && result.data.tipPercentage && <p className="mt-1 text-xs text-slate-400 dark:text-white/40">+ biaya {result.data.tipPercentage}%</p>}
          <code className="mt-5 block max-h-28 overflow-auto rounded-xl bg-black/10 p-4 text-left font-mono text-xs leading-5 text-slate-600 dark:bg-black/20 dark:text-white/60">{result.code}</code>
          <div className="mt-4 grid gap-2 print:hidden sm:grid-cols-2"><Button onClick={() => navigator.clipboard.writeText(result.code)} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Salin string</Button><Button onClick={share} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Bagikan QRIS</Button><Button onClick={() => downloadQrPng(result.code, `qris-dynamic-${result.data.merchantName.toLowerCase().replace(/\s+/g, "-")}.png`)} className="border-brand-600 bg-brand-600 text-white hover:bg-brand-700">Unduh QR</Button><Button onClick={downloadPDF} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Export PDF</Button><Button onClick={() => window.print()} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Cetak</Button></div>
        </section>
      )}

      <div className="mt-6 grid gap-4 lg:grid-cols-2 print:hidden"><Templates onPick={pickTemplate} active={null} /><History onPick={pickFromHistory} /></div>

      <section className="mt-6 rounded-2xl border border-black/10 bg-[#ffffff] p-5 text-slate-900 shadow-[0_18px_45px_rgb(23_23_23/0.06)] print:hidden dark:border-white/10 dark:bg-[#171717] dark:text-white">
        <h2 className="text-sm font-bold uppercase tracking-[.16em] text-slate-400 dark:text-white/50">Beliin kopi dong bang ☕</h2>
        <p className="mt-2 text-sm text-slate-500 dark:text-white/55">Dukung developer — bayar lewat QRIS DANA. Masukkan nominal, QR bayar langsung muncul.</p>
        <div className="mt-4 flex flex-wrap gap-2">
          <input value={traktirAmount} onChange={(event) => setTraktirAmount(event.target.value)} type="number" min="1" placeholder="Nominal (Rp)" className="w-full min-w-0 flex-1 rounded-xl border border-black/10 bg-[#fafafa] px-4 py-3 text-sm text-slate-900 outline-none placeholder:text-slate-400 focus:border-brand-500 dark:border-white/15 dark:bg-[#121212] dark:text-white dark:placeholder:text-white/45" />
          <Button onClick={generateTraktir} className="border-brand-600 bg-brand-600 text-white hover:bg-brand-700">Buat QR traktir</Button>
        </div>
        {traktirResult && (
          <div className="mt-5">
            <div className="mx-auto w-64 rounded-xl bg-white p-4 shadow-md">
              <img className="w-full" src={`/api/qr?data=${encodeURIComponent(traktirResult)}&size=280`} alt="QR traktir" />
            </div>
            <p className="mt-3 text-center text-sm font-semibold">Rp {Number(traktirAmount).toLocaleString("id-ID")}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2">
              <Button onClick={() => navigator.clipboard.writeText(traktirResult)} className="border-black/10 bg-white text-slate-700 hover:bg-black/5 dark:border-white/15 dark:bg-white/[.04] dark:text-white/80 dark:hover:bg-white/[.08]">Salin string</Button>
              <Button onClick={() => downloadQrPng(traktirResult, `traktir-${Number(traktirAmount).toLocaleString("id-ID").replace(/\./g, "")}.png`)} className="border-brand-600 bg-brand-600 text-white hover:bg-brand-700">Download QR</Button>
            </div>
          </div>
        )}
      </section>

    </main>
    <footer className="border-t border-black/10 py-7 text-center text-xs text-slate-400 print:hidden dark:border-white/10 dark:text-white/35">QRIS adalah standar kode QR pembayaran Bank Indonesia<br /><span className="text-slate-300 dark:text-white/20">Open source · MIT License</span></footer></>
  );
}

export default App;
