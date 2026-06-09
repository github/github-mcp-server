-- Fase 2 — Cola de facturación automática hacia Holded
-- Al escriturar una compraventa se encola su factura (trigger). Un proceso externo
-- (Make/Zapier con la acción create_invoice de Holded) consume las 'pendiente',
-- crea la factura en Holded y guarda holded_factura_id + estado 'enviada'.

create table factura_pendiente (
    id            uuid primary key default gen_random_uuid(),
    contrato_id   uuid unique references contrato_venta(id) on delete cascade,
    cliente_id    uuid references cliente(id),
    importe       numeric(12,2),
    concepto      text,
    estado        text check (estado in ('pendiente','enviada','error')) default 'pendiente',
    holded_factura_id text,
    creada_en     timestamptz default now()
);

create or replace function public.encolar_factura()
returns trigger language plpgsql security definer set search_path = public
as $$
begin
  if NEW.estado = 'escriturado' then
    insert into public.factura_pendiente (contrato_id, cliente_id, importe, concepto)
    values (NEW.id, NEW.cliente_id, NEW.precio_total, 'Compraventa de unidad')
    on conflict (contrato_id) do nothing;
  end if;
  return NEW;
end;
$$;
revoke execute on function public.encolar_factura() from public;

create trigger trg_encolar_factura
  after insert or update of estado on public.contrato_venta
  for each row execute function public.encolar_factura();

alter table public.factura_pendiente enable row level security;
create policy "factura_direccion" on public.factura_pendiente for all to authenticated
  using (public.es_direccion()) with check (public.es_direccion());
create policy "factura_ver" on public.factura_pendiente for select to authenticated
  using (public.puede_ver_promocion(
    (select f.promocion_id from public.contrato_venta cv
       join public.unidad u on u.id = cv.unidad_id
       join public.fase f on f.id = u.fase_id
     where cv.id = contrato_id)));
