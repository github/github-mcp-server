-- Fase 2 — Tesorería por promoción
-- Importe contratado en hitos de pago, cobrado y pendiente de cobro. Respeta RLS.

create view public.v_tesoreria_promocion
with (security_invoker = true) as
select
  pr.id, pr.nombre,
  coalesce(sum(h.importe), 0)                              as total_hitos,
  coalesce(sum(h.importe) filter (where h.pagado), 0)      as cobrado,
  coalesce(sum(h.importe) filter (where not h.pagado), 0)  as pendiente_cobro,
  round(100.0 * coalesce(sum(h.importe) filter (where h.pagado), 0)
        / nullif(sum(h.importe), 0), 1)                    as pct_cobrado
from public.promocion pr
join public.fase f            on f.promocion_id = pr.id
join public.unidad u          on u.fase_id = f.id
join public.contrato_venta cv on cv.unidad_id = u.id
join public.hito_pago h       on h.contrato_id = cv.id
group by pr.id, pr.nombre;
