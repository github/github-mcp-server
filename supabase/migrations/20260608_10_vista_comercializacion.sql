-- Fase 2 (adelanto) — Cuadro de comercialización por promoción
-- Unidades por estado, % colocado, importe vendido y valor total.
-- security_invoker => respeta el RLS del usuario.

create view public.v_comercializacion_promocion
with (security_invoker = true) as
select
  pr.id, pr.nombre, pr.municipio,
  count(u.*)                                            as total_unidades,
  count(*) filter (where u.estado = 'vendida')          as vendidas,
  count(*) filter (where u.estado = 'reservada')        as reservadas,
  count(*) filter (where u.estado = 'disponible')       as disponibles,
  round(100.0 * count(*) filter (where u.estado in ('vendida','reservada'))
        / nullif(count(u.*), 0), 1)                     as pct_colocado,
  coalesce(sum(u.precio_venta) filter (where u.estado = 'vendida'), 0) as importe_vendido,
  coalesce(sum(u.precio_venta), 0)                      as valor_total,
  pr.estado                                             as estado
from public.promocion pr
join public.fase f   on f.promocion_id = pr.id
join public.unidad u on u.fase_id = f.id
group by pr.id, pr.nombre, pr.municipio, pr.estado;
