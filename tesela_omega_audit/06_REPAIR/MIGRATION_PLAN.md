# MIGRATION PLAN
**Fecha:** 2026-06-22 (UTC)

1. **Baseline de esquema real:** `supabase db pull` (o equivalente) para capturar el estado vivo (incluye claude_sessions, holded_facturas, funciones drift).
2. **Reconciliar repo:** añadir migraciones faltantes; el repo debe reconstruir 1:1 la BD.
3. **Unificación de facturas:** migración que copie holded_facturas→factura_holded (mapeo de columnas) y elimine la tabla duplicada (tras backup).
4. **Modelo de coste:** migraciones para capítulo y trazabilidad (presupuesto→partida→compra→certificación→factura→cobro).
5. **Política:** a partir de aquí, **cero DDL directo**; todo por migración revisada.
