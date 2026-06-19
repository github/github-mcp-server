// emit-facturas: consume la cola `factura_pendiente` y emite las facturas en Holded.
// SEGURIDAD: requiere la SERVICE_ROLE key en Authorization (solo invocable desde el
// panel de Supabase / un proceso de confianza, nunca desde el navegador).
// SEGURO POR DEFECTO: dryRun=true => NO emite nada, solo informa de lo que haria.
// Solo procesa facturas cuyo cliente tenga holded_id real (los datos demo se ignoran).
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";
const BASE = "https://api.holded.com/api/invoicing/v1";

Deno.serve(async (req) => {
  try {
    const SERVICE = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!;
    // Autenticacion por secreto compartido: solo quien tenga la service-role key.
    if ((req.headers.get("Authorization") || "") !== `Bearer ${SERVICE}`) {
      return json({ error: "unauthorized" }, 401);
    }
    let dryRun = true;
    try { const b = await req.json(); if (b && b.dryRun === false) dryRun = false; } catch { /* sin body => dry-run */ }

    const supabase = createClient(Deno.env.get("SUPABASE_URL")!, SERVICE);

    // Mapa sociedad -> key de Holded (desde el Vault).
    const { data: keysJson, error: kerr } = await supabase.rpc("get_holded_keys");
    if (kerr) return json({ error: "No se pudieron leer las keys del vault: " + kerr.message }, 400);
    const keyMap: Record<string, string> = {};
    try { for (const e of JSON.parse((keysJson as string) || "[]")) if (e?.nombre && e?.key) keyMap[e.nombre] = e.key; }
    catch { return json({ error: "Keys del vault mal formadas" }, 400); }

    // Facturas pendientes con todo el contexto necesario.
    const { data: pend, error: perr } = await supabase
      .from("factura_pendiente")
      .select("id, importe, concepto, contrato_id, cliente:cliente_id(nombre,holded_id), contrato:contrato_id(unidad:unidad_id(referencia, fase:fase_id(promocion:promocion_id(sociedad:sociedad_id(nombre)))))")
      .eq("estado", "pendiente");
    if (perr) return json({ error: perr.message }, 400);

    const resultado = { dryRun, total: (pend || []).length, emitidas: 0, errores: 0, omitidas: 0, detalle: [] as any[] };

    for (const f of pend || []) {
      const cliente: any = f.cliente;
      const soc: any = f.contrato?.unidad?.fase?.promocion?.sociedad;
      const holdedContact = cliente?.holded_id;
      const socNombre = soc?.nombre;
      const key = socNombre ? keyMap[socNombre] : undefined;

      if (!holdedContact) { resultado.omitidas++; resultado.detalle.push({ id: f.id, accion: "omitida", motivo: "cliente sin holded_id (p.ej. dato demo)" }); continue; }
      if (!key) { resultado.omitidas++; resultado.detalle.push({ id: f.id, accion: "omitida", motivo: `sin key de Holded para la sociedad '${socNombre ?? "?"}'` }); continue; }

      const ref = f.contrato?.unidad?.referencia ?? "";
      if (dryRun) {
        resultado.detalle.push({ id: f.id, accion: "emitiria", contactId: holdedContact, importe: f.importe, concepto: `${f.concepto} ${ref}`.trim(), sociedad: socNombre });
        continue;
      }

      // EMISION REAL en Holded.
      const body = {
        contactId: holdedContact,
        date: Math.floor(Date.now() / 1000),
        items: [{ name: `${f.concepto} ${ref}`.trim(), units: 1, price: Number(f.importe) || 0 }],
      };
      const r = await fetch(`${BASE}/documents/invoice`, {
        method: "POST",
        headers: { key, "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(body),
      });
      const out = await r.json().catch(() => ({}));
      const holdedId = out?.id || out?.invoiceId;
      if (r.ok && holdedId) {
        await supabase.from("factura_pendiente").update({ estado: "enviada", holded_factura_id: String(holdedId) }).eq("id", f.id);
        await supabase.from("contrato_venta").update({ holded_factura_id: String(holdedId) }).eq("id", f.contrato_id);
        resultado.emitidas++;
        resultado.detalle.push({ id: f.id, accion: "emitida", holded_factura_id: String(holdedId) });
      } else {
        await supabase.from("factura_pendiente").update({ estado: "error" }).eq("id", f.id);
        resultado.errores++;
        resultado.detalle.push({ id: f.id, accion: "error", status: r.status, respuesta: out });
      }
    }
    return json(resultado);
  } catch (e) {
    return json({ error: String(e) }, 500);
  }
});

function json(b: unknown, s = 200): Response {
  return new Response(JSON.stringify(b), { status: s, headers: { "Content-Type": "application/json" } });
}
