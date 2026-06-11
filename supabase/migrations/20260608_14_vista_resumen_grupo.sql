-- Fase 2 — Resumen agregado del grupo (una fila) para el dashboard de Dirección.
-- security_invoker => suma solo las promociones visibles por el usuario (RLS).

create view public.v_resumen_grupo
with (security_invoker = true) as
select
  count(distinct p.id)                          as num_promociones,
  coalesce(sum(c.valor_total), 0)               as valor_cartera,
  coalesce(sum(c.importe_vendido), 0)           as total_vendido,
  coalesce(sum(r.coste_real), 0)                as coste_real,
  coalesce(sum(r.margen), 0)                    as margen_total,
  coalesce(sum(t.cobrado), 0)                   as caja_cobrada,
  coalesce(sum(t.pendiente_cobro), 0)           as caja_pendiente
from public.promocion p
left join public.v_comercializacion_promocion c on c.id = p.id
left join public.v_rentabilidad_promocion r on r.id = p.id
left join public.v_tesoreria_promocion t on t.id = p.id;
