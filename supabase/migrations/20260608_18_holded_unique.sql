-- Fase 2 — Índices UNIQUE normales sobre holded_id (permiten varios NULL) para que
-- el upsert por holded_id funcione desde la Edge Function.

drop index if exists cliente_holded_id_key;
drop index if exists proveedor_holded_id_key;
alter table public.cliente   add constraint cliente_holded_id_uniq   unique (holded_id);
alter table public.proveedor add constraint proveedor_holded_id_uniq unique (holded_id);
