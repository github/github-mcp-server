-- Fase 2 — Al registrar un contrato de venta, la unidad pasa a 'vendida' automáticamente.
-- Evita que el comercial necesite permiso de escritura sobre `unidad`.

create or replace function public.marcar_unidad_estado()
returns trigger language plpgsql security definer set search_path = public
as $$
begin
  if NEW.estado in ('arras','escriturado') then
    update public.unidad set estado = 'vendida' where id = NEW.unidad_id;
  end if;
  return NEW;
end;
$$;
revoke execute on function public.marcar_unidad_estado() from public;

create trigger trg_marcar_unidad
  after insert or update of estado on public.contrato_venta
  for each row execute function public.marcar_unidad_estado();
