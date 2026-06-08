-- Fase 2 (adelanto) — Cuadro de rentabilidad por promoción
-- Cruza ingresos contratados (compraventas) con costes (presupuesto previsto y coste real:
-- partidas ejecutadas + certificaciones de obra). security_invoker => respeta el RLS del usuario.

create view public.v_rentabilidad_promocion
with (security_invoker = true) as
select
  pr.id,
  pr.nombre,
  pr.municipio,
  pr.estado,
  coalesce(ing.ingresos_contratados, 0)            as ingresos_contratados,
  coalesce(cp.coste_previsto, 0)                   as coste_previsto,
  coalesce(cr.coste_real, 0)                       as coste_real,
  coalesce(ing.ingresos_contratados, 0) - coalesce(cr.coste_real, 0) as margen
from public.promocion pr
left join (
  select f.promocion_id, sum(cv.precio_total) as ingresos_contratados
  from public.contrato_venta cv
  join public.unidad u on u.id = cv.unidad_id
  join public.fase f   on f.id = u.fase_id
  group by f.promocion_id
) ing on ing.promocion_id = pr.id
left join (
  select pp.promocion_id, sum(p.importe_prev) as coste_previsto
  from public.presupuesto pp
  join public.partida p on p.presupuesto_id = pp.id
  group by pp.promocion_id
) cp on cp.promocion_id = pr.id
left join (
  select promocion_id, sum(coste) as coste_real from (
    select pp.promocion_id, sum(p.importe_real) as coste
    from public.presupuesto pp join public.partida p on p.presupuesto_id = pp.id
    group by pp.promocion_id
    union all
    select co.promocion_id, sum(c.importe)
    from public.contrato_obra co join public.certificacion c on c.contrato_obra_id = co.id
    group by co.promocion_id
  ) x group by promocion_id
) cr on cr.promocion_id = pr.id;
