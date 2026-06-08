-- Fase 1 — Sistema de usuarios y roles
-- Perfiles vinculados a Supabase Auth, roles (direccion/obra/comercial),
-- asignacion de promociones y funciones helper para RLS.

create table perfil (
    id        uuid primary key references auth.users(id) on delete cascade,
    nombre    text,
    email     text,
    rol       text check (rol in ('direccion','obra','comercial')),
    creado_en timestamptz default now()
);

create table acceso_promocion (
    perfil_id    uuid references perfil(id) on delete cascade,
    promocion_id uuid references promocion(id) on delete cascade,
    primary key (perfil_id, promocion_id)
);

-- Crea el perfil automaticamente al registrarse un usuario (rol pendiente de asignar).
create or replace function public.handle_new_user()
returns trigger
language plpgsql
security definer set search_path = public
as $$
begin
  insert into public.perfil (id, email, nombre)
  values (new.id, new.email, coalesce(new.raw_user_meta_data->>'full_name', new.email));
  return new;
end;
$$;

create trigger on_auth_user_created
  after insert on auth.users
  for each row execute function public.handle_new_user();

-- Funciones helper para RLS
create or replace function public.current_rol()
returns text language sql stable security definer set search_path = public
as $$ select rol from public.perfil where id = auth.uid() $$;

create or replace function public.es_direccion()
returns boolean language sql stable security definer set search_path = public
as $$ select public.current_rol() = 'direccion' $$;

create or replace function public.puede_ver_promocion(p_id uuid)
returns boolean language sql stable security definer set search_path = public
as $$
  select public.es_direccion()
      or exists (select 1 from public.acceso_promocion
                 where perfil_id = auth.uid() and promocion_id = p_id)
$$;
