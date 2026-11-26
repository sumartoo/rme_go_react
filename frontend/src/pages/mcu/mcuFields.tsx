import { ReactNode, useId } from "react";
import { McuLabItem, McuRegistrasi, McuSection, McuValue } from "../../api";

export type Values = Record<string, McuValue>;

/** What every form body gets: the answers, a setter, and the context. */
export interface FormProps {
  values: Values;
  set: (key: string, value: string, note?: string) => void;
  setNote: (key: string, note: string) => void;
  reg: McuRegistrasi;
  sections: McuSection[];
  lab: McuLabItem[];
}

export const v = (values: Values, key: string) => values[key]?.value ?? "";
export const n = (values: Values, key: string) => values[key]?.note ?? "";
export const num = (values: Values, key: string): number | null => {
  const s = v(values, key).replace(",", ".").trim();
  if (s === "") return null;
  const x = Number(s);
  return Number.isFinite(x) ? x : null;
};
export const fmt = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 1 });

/** A list stored as JSON in one value. */
export function readRows<T>(values: Values, key: string): T[] {
  try {
    const x = JSON.parse(v(values, key) || "[]");
    return Array.isArray(x) ? x : [];
  } catch {
    return [];
  }
}

export function Box({ title, children, actions }: { title: string; children: ReactNode; actions?: ReactNode }) {
  return (
    <section className="mcu-box">
      <header className="mcu-box-head">
        <h3>{title}</h3>
        {actions}
      </header>
      {children}
    </section>
  );
}

export function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: ReactNode }) {
  return (
    <div className="mcu-field">
      <span className="mcu-label">{label}</span>
      <div className="mcu-control">
        {children}
        {hint && <span className="mcu-hint">{hint}</span>}
      </div>
    </div>
  );
}

export function TextInput({ p, k, label, unit, type = "text", placeholder, wide }: { p: FormProps; k: string; label: string; unit?: string; type?: "text" | "number"; placeholder?: string; wide?: boolean }) {
  return (
    <Field label={label}>
      <input
        className={`select mcu-input${wide ? " mcu-wide" : ""}`}
        type={type === "number" ? "text" : "text"}
        inputMode={type === "number" ? "decimal" : undefined}
        value={v(p.values, k)}
        placeholder={placeholder}
        aria-label={label}
        onChange={(e) => p.set(k, e.target.value)}
      />
      {unit && <span className="mcu-unit">{unit}</span>}
    </Field>
  );
}

export function TextArea({ p, k, label, rows = 2, placeholder }: { p: FormProps; k: string; label: string; rows?: number; placeholder?: string }) {
  return (
    <Field label={label}>
      <textarea className="select mcu-input mcu-wide" rows={rows} value={v(p.values, k)} placeholder={placeholder} aria-label={label} onChange={(e) => p.set(k, e.target.value)} />
    </Field>
  );
}

export function Radios({
  value,
  options,
  onChange,
  label,
  abnormal,
}: {
  value: string;
  options: string[];
  onChange: (v: string) => void;
  label: string;
  /** Marks the chosen option as a finding. */
  abnormal?: boolean;
}) {
  const name = useId();
  return (
    <div className={`mcu-radios${abnormal ? " mcu-abnormal" : ""}`} role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <label key={o} className={value === o ? "mcu-radio on" : "mcu-radio"}>
          <input type="radio" name={name} checked={value === o} onChange={() => onChange(o)} onClick={() => value === o && onChange("")} />
          {o || "—"}
        </label>
      ))}
    </div>
  );
}

export function RadioField({ p, k, label, options, normal }: { p: FormProps; k: string; label: string; options: string[]; normal?: string[] }) {
  const val = v(p.values, k);
  return (
    <Field label={label}>
      <Radios label={label} value={val} options={options} onChange={(x) => p.set(k, x)} abnormal={!!val && !!normal && !normal.includes(val)} />
    </Field>
  );
}

/** Editable table of rows stored as JSON under one key. */
export function RowsEditor<T extends Record<string, string>>({
  p,
  k,
  columns,
  empty,
  addLabel,
}: {
  p: FormProps;
  k: string;
  columns: { key: keyof T & string; label: string; width?: string; options?: string[] }[];
  empty: T;
  addLabel: string;
}) {
  const rows = readRows<T>(p.values, k);
  const save = (next: T[]) => p.set(k, next.length ? JSON.stringify(next) : "");
  return (
    <div className="mcu-rows">
      {rows.length > 0 && (
        <table className="mcu-table">
          <thead>
            <tr>
              {columns.map((c) => (
                <th key={c.key} style={c.width ? { width: c.width } : undefined}>
                  {c.label}
                </th>
              ))}
              <th aria-label="Hapus" />
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                {columns.map((c) => (
                  <td key={c.key}>
                    {c.options ? (
                      <select className="select mcu-input" aria-label={c.label} value={r[c.key] ?? ""} onChange={(e) => save(rows.map((x, j) => (j === i ? { ...x, [c.key]: e.target.value } : x)))}>
                        <option value="">—</option>
                        {c.options.map((o) => (
                          <option key={o}>{o}</option>
                        ))}
                      </select>
                    ) : (
                      <input
                        className="select mcu-input"
                        aria-label={c.label}
                        value={r[c.key] ?? ""}
                        onChange={(e) => save(rows.map((x, j) => (j === i ? { ...x, [c.key]: e.target.value } : x)))}
                      />
                    )}
                  </td>
                ))}
                <td>
                  <button type="button" className="row-action" aria-label="Hapus baris" onClick={() => save(rows.filter((_, j) => j !== i))}>
                    ×
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <button type="button" className="btn-add" onClick={() => save([...rows, { ...empty }])}>
        + {addLabel}
      </button>
    </div>
  );
}
