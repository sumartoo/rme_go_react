import { Box, Field, FormProps, RadioField, readRows, RowsEditor, TextArea, TextInput, v, Values, num, fmt } from "./mcuFields";

// ---------- Anamnesa ----------

export function AnamnesaForm(p: FormProps) {
  return (
    <Box title="Anamnesa">
      <TextArea p={p} k="keluhan_utama" label="Keluhan utama" rows={3} placeholder="Kosongkan bila tidak ada keluhan" />
      <TextArea p={p} k="keluhan_tambahan" label="Keluhan tambahan" />
      <TextArea p={p} k="riwayat_sekarang" label="Riwayat penyakit sekarang" rows={3} />
      <TextArea p={p} k="obat" label="Obat yang sedang dikonsumsi" />
      <TextInput p={p} k="alergi" label="Alergi (obat/makanan)" wide placeholder="Tidak ada" />
    </Box>
  );
}

export function anamnesaFindings(x: Values): string[] {
  const out: string[] = [];
  if (v(x, "keluhan_utama")) out.push(`Keluhan: ${v(x, "keluhan_utama")}`);
  if (v(x, "keluhan_tambahan")) out.push(`Keluhan tambahan: ${v(x, "keluhan_tambahan")}`);
  const al = v(x, "alergi");
  if (al && !/^(tidak|tidak ada|-)$/i.test(al)) out.push(`Alergi: ${al}`);
  return out;
}

// ---------- Riwayat Kesehatan Keluarga ----------

export const PENYAKIT_KELUARGA = [
  "Hipertensi",
  "Diabetes melitus",
  "Penyakit jantung",
  "Stroke",
  "Kanker",
  "Asma / alergi",
  "TBC",
  "Penyakit ginjal",
  "Hepatitis",
  "Gangguan jiwa",
];
const HUBUNGAN = ["Ayah", "Ibu", "Saudara kandung", "Kakek / nenek"];

export function KeluargaForm(p: FormProps) {
  return (
    <Box title="Riwayat penyakit dalam keluarga">
      <div className="mcu-scroll">
        <table className="mcu-table mcu-grid">
          <thead>
            <tr>
              <th>Penyakit</th>
              {HUBUNGAN.map((h) => (
                <th key={h}>{h}</th>
              ))}
              <th>Keterangan</th>
            </tr>
          </thead>
          <tbody>
            {PENYAKIT_KELUARGA.map((d, i) => (
              <tr key={d}>
                <td>{d}</td>
                {HUBUNGAN.map((h, j) => {
                  const k = `kel.${i}.${j}`;
                  return (
                    <td key={h} className="mcu-center">
                      <input type="checkbox" aria-label={`${d} – ${h}`} checked={v(p.values, k) === "1"} onChange={(e) => p.set(k, e.target.checked ? "1" : "")} />
                    </td>
                  );
                })}
                <td>
                  <input className="select mcu-input" aria-label={`Keterangan ${d}`} value={v(p.values, `kel.${i}`)} onChange={(e) => p.set(`kel.${i}`, e.target.value)} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <TextArea p={p} k="kel.lain" label="Penyakit lain dalam keluarga" />
    </Box>
  );
}

export function keluargaFindings(x: Values): string[] {
  const out: string[] = [];
  PENYAKIT_KELUARGA.forEach((d, i) => {
    const who = HUBUNGAN.filter((_, j) => v(x, `kel.${i}.${j}`) === "1");
    if (who.length) out.push(`Riwayat keluarga ${d.toLowerCase()} (${who.join(", ").toLowerCase()})`);
  });
  if (v(x, "kel.lain")) out.push(`Riwayat keluarga lain: ${v(x, "kel.lain")}`);
  return out;
}

// ---------- Kebiasaan Pasien ----------

export function KebiasaanForm(p: FormProps) {
  const rokok = v(p.values, "rokok");
  const batang = num(p.values, "rokok.batang");
  const tahun = num(p.values, "rokok.tahun");
  const brinkman = batang && tahun ? batang * tahun : null;
  return (
    <>
      <Box title="Merokok">
        <RadioField p={p} k="rokok" label="Merokok" options={["Tidak pernah", "Ya", "Sudah berhenti"]} normal={["Tidak pernah"]} />
        {rokok && rokok !== "Tidak pernah" && (
          <>
            <TextInput p={p} k="rokok.batang" label="Jumlah" unit="batang/hari" type="number" />
            <TextInput p={p} k="rokok.tahun" label="Lama merokok" unit="tahun" type="number" />
            {rokok === "Sudah berhenti" && <TextInput p={p} k="rokok.berhenti" label="Berhenti sejak" unit="tahun" type="number" />}
            {brinkman !== null && (
              <p className={brinkman >= 200 ? "mcu-calc mcu-abnormal" : "mcu-calc"}>
                Indeks Brinkman {fmt.format(brinkman)} — {brinkman < 200 ? "perokok ringan" : brinkman < 600 ? "perokok sedang" : "perokok berat"}
              </p>
            )}
          </>
        )}
      </Box>
      <Box title="Kebiasaan lain">
        <RadioField p={p} k="alkohol" label="Alkohol" options={["Tidak", "Kadang-kadang", "Sering"]} normal={["Tidak"]} />
        <TextInput p={p} k="olahraga.kali" label="Olahraga" unit="kali/minggu" type="number" />
        <TextInput p={p} k="olahraga.menit" label="Lama olahraga" unit="menit/kali" type="number" />
        <TextInput p={p} k="kopi" label="Kopi" unit="gelas/hari" type="number" />
        <TextInput p={p} k="tidur" label="Tidur" unit="jam/malam" type="number" />
        <RadioField p={p} k="makan" label="Pola makan" options={["Teratur", "Tidak teratur"]} normal={["Teratur"]} />
        <RadioField p={p} k="shift" label="Begadang / kerja malam" options={["Tidak", "Kadang-kadang", "Sering"]} normal={["Tidak", "Kadang-kadang"]} />
        <TextArea p={p} k="kebiasaan.lain" label="Lainnya" />
      </Box>
    </>
  );
}

export function kebiasaanFindings(x: Values): string[] {
  const out: string[] = [];
  const rokok = v(x, "rokok");
  if (rokok === "Ya" || rokok === "Sudah berhenti") {
    const b = num(x, "rokok.batang");
    const t = num(x, "rokok.tahun");
    out.push(`${rokok === "Ya" ? "Perokok aktif" : "Bekas perokok"}${b ? `, ${b} batang/hari` : ""}${t ? ` selama ${t} tahun` : ""}${b && t ? ` (Brinkman ${b * t})` : ""}`);
  }
  if (v(x, "alkohol") === "Sering") out.push("Sering mengonsumsi alkohol");
  const ol = num(x, "olahraga.kali");
  if (ol !== null && ol < 3) out.push(`Olahraga kurang (${ol}×/minggu)`);
  const tidur = num(x, "tidur");
  if (tidur !== null && tidur < 6) out.push(`Kurang tidur (${tidur} jam/malam)`);
  if (v(x, "shift") === "Sering") out.push("Sering begadang / kerja malam");
  return out;
}

// ---------- Riwayat Pekerjaan ----------

export const PAPARAN = [
  "Bising",
  "Debu",
  "Bahan kimia / uap",
  "Getaran",
  "Radiasi",
  "Suhu panas / dingin",
  "Angkat beban / ergonomi",
  "Kerja shift",
  "Komputer (>4 jam/hari)",
  "Kerja di ketinggian",
  "Bahan biologis",
];

type Pekerjaan = { perusahaan: string; jabatan: string; dari: string; sampai: string; uraian: string };

export function PekerjaanForm(p: FormProps) {
  return (
    <>
      <Box title="Pekerjaan sekarang">
        <Field label="Perusahaan / jabatan">
          <span>
            {p.reg.perusahaan || "-"} · {p.reg.departemen || "-"} · {p.reg.jabatan || "-"}
          </span>
        </Field>
        <TextInput p={p} k="lama_kerja" label="Lama bekerja di sini" unit="tahun" type="number" />
        <TextInput p={p} k="jam_kerja" label="Jam kerja" unit="jam/hari" type="number" />
        <TextArea p={p} k="uraian_tugas" label="Uraian tugas" />
      </Box>
      <Box title="Riwayat pekerjaan sebelumnya">
        <RowsEditor<Pekerjaan>
          p={p}
          k="riwayat"
          columns={[
            { key: "perusahaan", label: "Perusahaan" },
            { key: "jabatan", label: "Jabatan / bagian" },
            { key: "dari", label: "Dari (tahun)", width: "90px" },
            { key: "sampai", label: "Sampai", width: "90px" },
            { key: "uraian", label: "Uraian / paparan" },
          ]}
          empty={{ perusahaan: "", jabatan: "", dari: "", sampai: "", uraian: "" }}
          addLabel="Tambah pekerjaan"
        />
      </Box>
      <Box title="Paparan bahaya di tempat kerja">
        <div className="mcu-checks">
          {PAPARAN.map((x, i) => (
            <label key={x} className="mcu-check">
              <input type="checkbox" checked={v(p.values, `paparan.${i}`) === "1"} onChange={(e) => p.set(`paparan.${i}`, e.target.checked ? "1" : "")} />
              {x}
            </label>
          ))}
        </div>
        <TextInput p={p} k="paparan.lain" label="Paparan lain" wide />
        <RadioField p={p} k="apd" label="Memakai APD" options={["Selalu", "Kadang-kadang", "Tidak"]} normal={["Selalu"]} />
        <RadioField p={p} k="kecelakaan" label="Pernah kecelakaan kerja" options={["Tidak", "Ya"]} normal={["Tidak"]} />
        {v(p.values, "kecelakaan") === "Ya" && <TextArea p={p} k="kecelakaan.ket" label="Keterangan kecelakaan" />}
      </Box>
    </>
  );
}

export function pekerjaanFindings(x: Values): string[] {
  const out: string[] = [];
  const pap = PAPARAN.filter((_, i) => v(x, `paparan.${i}`) === "1");
  if (v(x, "paparan.lain")) pap.push(v(x, "paparan.lain"));
  if (pap.length) out.push(`Paparan kerja: ${pap.join(", ").toLowerCase()}`);
  if (pap.length && v(x, "apd") && v(x, "apd") !== "Selalu") out.push(`APD ${v(x, "apd").toLowerCase()} dipakai`);
  if (v(x, "kecelakaan") === "Ya") out.push(`Riwayat kecelakaan kerja${v(x, "kecelakaan.ket") ? `: ${v(x, "kecelakaan.ket")}` : ""}`);
  return out;
}

// ---------- Riwayat Opname & Operasi ----------

type Opname = { tahun: string; jenis: string; diagnosis: string; rs: string; lama: string };

export function OpnameForm(p: FormProps) {
  const none = v(p.values, "tidak_ada") === "1";
  return (
    <Box title="Riwayat rawat inap, operasi dan kecelakaan">
      <Field label="Tidak pernah">
        <label className="mcu-check">
          <input type="checkbox" checked={none} onChange={(e) => p.set("tidak_ada", e.target.checked ? "1" : "")} />
          Tidak pernah dirawat inap, dioperasi atau mengalami kecelakaan berat
        </label>
      </Field>
      {!none && (
        <RowsEditor<Opname>
          p={p}
          k="riwayat"
          columns={[
            { key: "tahun", label: "Tahun", width: "80px" },
            { key: "jenis", label: "Jenis", width: "150px", options: ["Rawat inap", "Operasi", "Kecelakaan"] },
            { key: "diagnosis", label: "Diagnosis / tindakan" },
            { key: "rs", label: "Rumah sakit" },
            { key: "lama", label: "Lama (hari)", width: "90px" },
          ]}
          empty={{ tahun: "", jenis: "", diagnosis: "", rs: "", lama: "" }}
          addLabel="Tambah riwayat"
        />
      )}
    </Box>
  );
}

export function opnameFindings(x: Values): string[] {
  return readRows<Opname>(x, "riwayat")
    .filter((r) => r.diagnosis || r.jenis)
    .map((r) => `${r.jenis || "Riwayat"} ${r.tahun ? `(${r.tahun}) ` : ""}${r.diagnosis}`.trim());
}
