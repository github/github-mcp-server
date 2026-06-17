-- Fase 2 — Facturas reales importadas de Holded (ventas y compras).
-- Datos contables reales para análisis. Solo dirección (financiero sensible).

create table factura_holded (
  id                uuid primary key default gen_random_uuid(),
  sociedad_id       uuid references sociedad(id),
  holded_id         text unique,
  tipo              text,            -- 'invoice' (venta) | 'purchase' (compra)
  doc_numero        text,
  fecha             date,
  contacto_nombre   text,
  holded_contact_id text,
  subtotal          numeric(14,2),
  total             numeric(14,2),
  pagada            boolean default false,
  creada_en         timestamptz default now()
);
alter table factura_holded enable row level security;
create policy "factura_holded_direccion" on factura_holded for all to authenticated
  using (public.es_direccion()) with check (public.es_direccion());
