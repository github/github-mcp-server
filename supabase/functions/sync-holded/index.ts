// sync-holded: sincroniza contactos y facturas de Holded para todas las sociedades
// cuyas keys están en Vault (RPC get_holded_keys). Invocable manualmente o por el cron diario.
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";
const BASE = "https://api.holded.com/api/invoicing/v1";

Deno.serve(async () => {
  try {
    const supabase = createClient(Deno.env.get("SUPABASE_URL")!, Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!);
    const { data: keysJson, error: kerr } = await supabase.rpc("get_holded_keys");
    if (kerr || !keysJson) return json({ error: "No se pudieron leer las keys del vault: " + (kerr?.message || "vacio") }, 400);
    let empresas: any[];
    try { empresas = JSON.parse(keysJson as string); } catch { return json({ error: "Keys mal formadas" }, 400); }

    const resumen: any[] = [];
    for (const emp of empresas) {
      if (!emp?.key || !emp?.nombre) continue;
      const { data: soc } = await supabase.from("sociedad").upsert({ nombre: emp.nombre }, { onConflict: "nombre" }).select("id").single();
      const sociedadId = soc?.id;
      const h = { key: emp.key, Accept: "application/json" };

      let nC = 0, nP = 0, nF = 0;
      const rc = await fetch(`${BASE}/contacts`, { headers: h });
      if (rc.ok) {
        const cont = await rc.json();
        if (Array.isArray(cont)) {
          const cli: any[] = [], prov: any[] = [];
          for (const c of cont) {
            const tipo = String(c.type ?? "").toLowerCase();
            const b = { nombre: c.name ?? "(sin nombre)", email: c.email ?? null, telefono: c.phone ?? null, holded_id: String(c.id), sociedad_id: sociedadId };
            if (tipo.includes("supplier") || tipo.includes("creditor")) prov.push({ ...b, cif: c.code ?? null });
            else cli.push({ ...b, nif: c.code ?? null });
          }
          if (cli.length) { await supabase.from("cliente").upsert(cli, { onConflict: "holded_id" }); nC = cli.length; }
          if (prov.length) { await supabase.from("proveedor").upsert(prov, { onConflict: "holded_id" }); nP = prov.length; }
        }
      }
      for (const tipo of ["invoice", "purchase"]) {
        const rf = await fetch(`${BASE}/documents/${tipo}`, { headers: h });
        if (!rf.ok) continue;
        const docs = await rf.json();
        if (!Array.isArray(docs)) continue;
        const rows = docs.map((d: any) => ({
          sociedad_id: sociedadId, holded_id: String(d.id), tipo,
          doc_numero: d.docNumber ?? null,
          fecha: d.date ? new Date(Number(d.date) * 1000).toISOString().slice(0, 10) : null,
          contacto_nombre: d.contactName ?? null, holded_contact_id: d.contact ?? null,
          subtotal: d.subtotal ?? null, total: d.total ?? null,
          pagada: Number(d.paymentsPending ?? 0) <= 0,
        }));
        if (rows.length) { await supabase.from("factura_holded").upsert(rows, { onConflict: "holded_id" }); nF += rows.length; }
      }
      resumen.push({ sociedad: emp.nombre, clientes: nC, proveedores: nP, facturas: nF });
    }
    return json({ ok: true, resumen });
  } catch (e) {
    return json({ error: String(e) }, 500);
  }
});
function json(b: unknown, s = 200): Response {
  return new Response(JSON.stringify(b), { status: s, headers: { "Content-Type": "application/json" } });
}
