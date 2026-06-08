// Edge Function: sync-holded (multi-sociedad)
// Sincroniza los CONTACTOS de varias empresas de Holded hacia las tablas
// `cliente` y `proveedor` del ERP (carga masiva inicial, modo LECTURA).
// Cada contacto se asocia a su `sociedad` y guarda su `holded_id`.
//
// Configuración (secret en Supabase, NUNCA en el repo):
//   HOLDED_SOCIEDADES = JSON con la lista de empresas y sus API keys, p.ej.:
//   [
//     {"nombre":"Tesela Promociones SL","key":"<api_key_1>"},
//     {"nombre":"Tesela Construccion SL","key":"<api_key_2>"}
//   ]
//
// Secrets inyectados por Supabase: SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY
//
// NOTA: el mapeo de campos de Holded debe validarse contra developers.holded.com.

import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

const HOLDED_API_BASE = "https://api.holded.com/api/invoicing/v1";

interface Empresa {
  nombre: string;
  key: string;
}

Deno.serve(async () => {
  try {
    const raw = Deno.env.get("HOLDED_SOCIEDADES");
    if (!raw) return json({ error: "Falta el secreto HOLDED_SOCIEDADES" }, 400);

    let empresas: Empresa[];
    try {
      empresas = JSON.parse(raw);
      if (!Array.isArray(empresas) || !empresas.length) throw new Error();
    } catch {
      return json({ error: "HOLDED_SOCIEDADES no es un JSON válido (array de {nombre,key})" }, 400);
    }

    const supabase = createClient(
      Deno.env.get("SUPABASE_URL")!,
      Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!,
    );

    const resumen: Record<string, unknown>[] = [];

    for (const emp of empresas) {
      if (!emp?.key || !emp?.nombre) {
        resumen.push({ sociedad: emp?.nombre ?? "(sin nombre)", error: "Entrada incompleta" });
        continue;
      }

      // 1) Upsert de la sociedad por nombre -> obtener su id
      const { data: soc, error: socErr } = await supabase
        .from("sociedad")
        .upsert({ nombre: emp.nombre }, { onConflict: "nombre" })
        .select("id")
        .single();
      if (socErr || !soc) {
        resumen.push({ sociedad: emp.nombre, error: `No se pudo crear la sociedad: ${socErr?.message}` });
        continue;
      }
      const sociedadId = soc.id;

      // 2) Traer contactos de esta empresa de Holded
      const res = await fetch(`${HOLDED_API_BASE}/contacts`, {
        headers: { key: emp.key, "Content-Type": "application/json" },
      });
      if (!res.ok) {
        resumen.push({ sociedad: emp.nombre, error: `Holded respondió ${res.status}` });
        continue;
      }
      const contactos = await res.json();
      if (!Array.isArray(contactos)) {
        resumen.push({ sociedad: emp.nombre, error: "Respuesta inesperada de Holded" });
        continue;
      }

      // 3) Clasificar y preparar filas
      const clientes: Record<string, unknown>[] = [];
      const proveedores: Record<string, unknown>[] = [];
      for (const c of contactos) {
        const tipo = String(c.type ?? "").toLowerCase();
        const base = {
          nombre: c.name ?? "(sin nombre)",
          email: c.email ?? null,
          telefono: c.phone ?? null,
          holded_id: String(c.id),
          sociedad_id: sociedadId,
        };
        const esProveedor = tipo.includes("supplier") || tipo.includes("creditor");
        const esCliente = tipo.includes("client") || tipo.includes("debtor") || tipo.includes("lead");
        if (esProveedor) proveedores.push({ ...base, cif: c.code ?? null });
        if (esCliente || (!esProveedor && !esCliente)) clientes.push({ ...base, nif: c.code ?? null });
      }

      // 4) Upsert por holded_id (no duplica)
      let nC = 0, nP = 0;
      if (clientes.length) {
        const { error } = await supabase.from("cliente").upsert(clientes, { onConflict: "holded_id" });
        if (error) { resumen.push({ sociedad: emp.nombre, error: `clientes: ${error.message}` }); continue; }
        nC = clientes.length;
      }
      if (proveedores.length) {
        const { error } = await supabase.from("proveedor").upsert(proveedores, { onConflict: "holded_id" });
        if (error) { resumen.push({ sociedad: emp.nombre, error: `proveedores: ${error.message}` }); continue; }
        nP = proveedores.length;
      }

      resumen.push({
        sociedad: emp.nombre,
        total_contactos: contactos.length,
        clientes_sincronizados: nC,
        proveedores_sincronizados: nP,
      });
    }

    return json({ ok: true, sociedades_procesadas: empresas.length, detalle: resumen });
  } catch (e) {
    return json({ error: "Excepción no controlada", detail: String(e) }, 500);
  }
});

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
