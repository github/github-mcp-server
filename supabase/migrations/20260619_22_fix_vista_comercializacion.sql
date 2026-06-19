-- Fase 2 — Fix: una promoción recién creada (sin fases/unidades) no aparecía en el
-- dashboard porque `v_comercializacion_promocion` hacía INNER JOIN a fase/unidad.
-- Se pasa a LEFT JOIN para que aparezca con los contadores a 0 en cuanto se crea.
-- Mismas columnas/orden/tipos => no rompe la vista dependiente v_resumen_grupo.

create or replace view public.v_comercializacion_promocion
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
left join public.fase f   on f.promocion_id = pr.id
left join public.unidad u on u.fase_id = f.id
group by pr.id, pr.nombre, pr.municipio, pr.estado;
