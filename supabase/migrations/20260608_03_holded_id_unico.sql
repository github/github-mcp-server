-- Fase 0 — Índices únicos parciales sobre holded_id (para upsert sin duplicar).
-- (En la migración 18 se sustituyen por UNIQUE normal.)
create unique index if not exists cliente_holded_id_key
  on public.cliente (holded_id) where holded_id is not null;
create unique index if not exists proveedor_holded_id_key
  on public.proveedor (holded_id) where holded_id is not null;
