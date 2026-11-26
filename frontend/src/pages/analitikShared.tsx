import { ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { Bot, Send } from "lucide-react";
import { AnalitikChatMessage, api, ApiError } from "../api";
import { useAuth } from "../auth/AuthContext";

export const rupiah = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
export const num = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 1 });

/** Turns API failures into a message; a 401 logs out. */
export function useFail() {
  const { logout } = useAuth();
  return useCallback(
    (err: unknown, fallback: string) => {
      if (err instanceof ApiError && err.status === 401) {
        logout("Your session has expired. Please log in again.");
        return "";
      }
      return err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : fallback;
    },
    [logout],
  );
}

/** Renders the assistant's light Markdown (headings, bullets, bold) safely. */
export function AiText({ text }: { text: string }) {
  const inline = (s: string) =>
    s.split(/(\*\*[^*]+\*\*)/g).map((part, i) => (part.startsWith("**") && part.endsWith("**") ? <strong key={i}>{part.slice(2, -2)}</strong> : part));
  const blocks: ReactNode[] = [];
  let list: { ordered: boolean; items: string[] } | null = null;
  const flush = () => {
    if (!list) return;
    const items = list.items.map((it, i) => <li key={i}>{inline(it)}</li>);
    blocks.push(list.ordered ? <ol key={blocks.length}>{items}</ol> : <ul key={blocks.length}>{items}</ul>);
    list = null;
  };
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    const bullet = line.match(/^[-*•]\s+(.*)$/);
    const ordered = line.match(/^\d+[.)]\s+(.*)$/);
    if (bullet || ordered) {
      const isOrdered = !!ordered;
      if (list && list.ordered !== isOrdered) flush();
      if (!list) list = { ordered: isOrdered, items: [] };
      list.items.push((bullet ?? ordered)![1]);
      continue;
    }
    flush();
    if (!line) continue;
    const h = line.match(/^#{1,6}\s+(.*)$/);
    blocks.push(h ? <h3 key={blocks.length}>{inline(h[1])}</h3> : <p key={blocks.length}>{inline(line)}</p>);
  }
  flush();
  return <div className="an-ai-text">{blocks}</div>;
}

export function AiDisabledNotice() {
  return (
    <p className="alert info" role="note">
      <Bot aria-hidden="true" /> Asisten AI belum aktif. Admin perlu mengisi <code>DEEPSEEK_API_KEY</code> di konfigurasi backend. Fitur lain tetap
      berfungsi tanpa AI.
    </p>
  );
}

/** The assistant chat: suggestions, the conversation and the question box. */
export function ChatBox({ cabang, suggestions, placeholder }: { cabang?: string; suggestions: string[]; placeholder: string }) {
  const { token } = useAuth();
  const fail = useFail();
  const [messages, setMessages] = useState<AnalitikChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const endRef = useRef<HTMLDivElement>(null);

  // Block body on purpose: newer browsers return a Promise from scrollIntoView,
  // and React would call a returned value as the effect's cleanup.
  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "nearest" });
  }, [messages, busy]);

  const ask = (question: string) => {
    const q = question.trim();
    if (!token || !q || busy) return;
    const next: AnalitikChatMessage[] = [...messages, { role: "user", content: q }];
    setMessages(next);
    setDraft("");
    setBusy(true);
    setError("");
    api
      .analitikChat(token, { cabang, messages: next.slice(-20) })
      .then((r) => setMessages([...next, r.message]))
      .catch((err) => {
        setError(fail(err, "Gagal mendapat jawaban"));
        setMessages(messages); // drop the unanswered question so the turns stay alternating
        setDraft(q);
      })
      .finally(() => setBusy(false));
  };

  return (
    <>
      <div className="an-chat-log" aria-live="polite">
        {messages.length === 0 && (
          <div className="an-suggest">
            {suggestions.map((s) => (
              <button key={s} type="button" className="btn-add" onClick={() => ask(s)}>
                {s}
              </button>
            ))}
          </div>
        )}
        {messages.map((m, i) => (
          <div key={i} className={`an-msg an-msg-${m.role}`}>
            {m.role === "assistant" ? <AiText text={m.content} /> : <p>{m.content}</p>}
          </div>
        ))}
        {busy && <p className="muted">Asisten sedang membaca data…</p>}
        <div ref={endRef} />
      </div>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      <form
        className="an-chat-form"
        onSubmit={(e) => {
          e.preventDefault();
          ask(draft);
        }}
      >
        <textarea
          className="select"
          rows={2}
          maxLength={2000}
          value={draft}
          placeholder={placeholder}
          aria-label="Pertanyaan"
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              ask(draft);
            }
          }}
        />
        <button type="submit" className="btn-add" disabled={busy || !draft.trim()} aria-label="Kirim">
          <Send aria-hidden="true" />
        </button>
      </form>
      <p className="muted an-foot">
        Yang dikirim ke AI hanya angka agregat (nama obat, kode ICD, jumlah, pendapatan), tanpa data pasien. Jawaban AI bisa keliru; cek angka di
        tab lain.
      </p>
    </>
  );
}
