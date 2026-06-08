-- Fase 1 — Almacenamiento documental (Supabase Storage)
-- Bucket privado para planos, licencias, escrituras, contratos.
-- Convención de rutas: "<promocion_id>/<carpeta>/<archivo>" para aplicar seguridad por promoción.

insert into storage.buckets (id, name, public)
values ('documentos', 'documentos', false)
on conflict (id) do nothing;

-- Extrae el promocion_id del path (primer segmento). NULL si no es válido.
create or replace function public.promocion_de_path(p text)
returns uuid language plpgsql immutable
as $$
begin
  return (storage.foldername(p))[1]::uuid;
exception when others then
  return null;
end;
$$;
revoke execute on function public.promocion_de_path(text) from public;
grant execute on function public.promocion_de_path(text) to authenticated;

create policy "docs_ver" on storage.objects for select to authenticated
  using (bucket_id = 'documentos' and public.puede_ver_promocion(public.promocion_de_path(name)));
create policy "docs_subir" on storage.objects for insert to authenticated
  with check (bucket_id = 'documentos' and public.puede_ver_promocion(public.promocion_de_path(name)));
create policy "docs_actualizar" on storage.objects for update to authenticated
  using (bucket_id = 'documentos' and public.puede_ver_promocion(public.promocion_de_path(name)));
create policy "docs_borrar" on storage.objects for delete to authenticated
  using (bucket_id = 'documentos' and public.es_direccion());
