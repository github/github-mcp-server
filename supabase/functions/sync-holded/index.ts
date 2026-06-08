// Edge Function: sync-holded
// Sincroniza los CONTACTOS de Holded hacia las tablas `cliente` y `proveedor`
// del ERP (modo LECTURA desde Holded). Pobla el campo `holded_id` para
// mantener el vínculo sin duplicar la contabilidad.
//
// Requiere los siguientes secretos configurados en Supabase:
//   - HOLDED_API_KEY        -> API key de Holded (Configuración > Desarrolladores > API)
//   - SUPABASE_URL          -> (inyectada automáticamente por Supabase)
//   - SUPABASE_SERVICE_ROLE_KEY -> (inyectada automáticamente por Supabase)
//
// Uso: invocar por HTTP (POST). Devuelve un resumen de la sincronización.
//
// NOTA: el mapeo de campos de Holded debe validarse contra la documentación
// vigente de su API (https://developers.holded.com). Esta versión usa los
// campos habituales: id, name, code (NIF/CIF), email, phone, type.

import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

const HOLDED_API_BASE = "https://api.holded.com/api/invoicing/v1";

Deno.serve(async (req) => {
  try {
    const holdedKey = Deno.env.get("HOLDED_API_KEY");
    if (!holdedKey) {
      return json({ error: "Falta el secreto HOLDED_API_KEY" }, 400);
    }

    const supabase = createClient(
      Deno.env.get("SUPABASE_URL")!,
      Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!,
    );

    // 1) Traer contactos de Holded
    const res = await fetch(`${HOLDED_API_BASE}/contacts`, {
      headers: { key: holdedKey, "Content-Type": "application/json" },
    });
    if (!res.ok) {
      return json({ error: `Holded respondió ${res.status}`, detail: await res.text() }, 502);
    }
    const contactos = await res.json();
    if (!Array.isArray(contactos)) {
      return json({ error: "Respuesta inesperada de Holded", detail: contactos }, 502);
    }

    // 2) Clasificar en clientes y proveedores según el tipo de Holded
    const clientes: Record<string, unknown>[] = [];
    const proveedores: Record<string, unknown>[] = [];

    for (const c of contactos) {
      const tipo = String(c.type ?? "").toLowerCase();
      const fila = {
        nombre: c.name ?? "(sin nombre)",
        email: c.email ?? null,
        telefono: c.phone ?? null,
        holded_id: String(c.id),
      };
      const esProveedor = tipo.includes("supplier") || tipo.includes("creditor");
      const esCliente = tipo.includes("client") || tipo.includes("debtor") || tipo.includes("lead");

      if (esProveedor) {
        proveedores.push({ ...fila, cif: c.code ?? null });
      }
      if (esCliente || (!esProveedor && !esCliente)) {
        // Si Holded no especifica tipo, lo tratamos como cliente por defecto.
        clientes.push({ ...fila, nif: c.code ?? null });
      }
    }

    // 3) Upsert por holded_id (no duplica si ya existe)
    let nClientes = 0, nProveedores = 0;
    if (clientes.length) {
      const { error } = await supabase.from("cliente").upsert(clientes, { onConflict: "holded_id" });
      if (error) return json({ error: "Error al guardar clientes", detail: error.message }, 500);
      nClientes = clientes.length;
    }
    if (proveedores.length) {
      const { error } = await supabase.from("proveedor").upsert(proveedores, { onConflict: "holded_id" });
      if (error) return json({ error: "Error al guardar proveedores", detail: error.message }, 500);
      nProveedores = proveedores.length;
    }

    return json({
      ok: true,
      total_contactos: contactos.length,
      clientes_sincronizados: nClientes,
      proveedores_sincronizados: nProveedores,
    });
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
