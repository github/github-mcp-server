-- Fase 0/1 — Soporte multi-sociedad: unicidad de nombre + vínculo de terceros a sociedad.
alter table public.sociedad add constraint sociedad_nombre_key unique (nombre);
alter table public.cliente   add column if not exists sociedad_id uuid references public.sociedad(id);
alter table public.proveedor add column if not exists sociedad_id uuid references public.sociedad(id);
