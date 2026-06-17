-- Fase 2 — Lectura segura de las keys de Holded desde Vault.
-- La key se guarda cifrada en Vault (vault.create_secret 'holded_sociedades'), NUNCA en el repo.
-- Solo el service_role (Edge Function sync-holded) puede ejecutar esta función.

create or replace function public.get_holded_keys()
returns text language sql security definer set search_path = public, vault
as $$ select decrypted_secret from vault.decrypted_secrets where name = 'holded_sociedades' limit 1 $$;

revoke execute on function public.get_holded_keys() from public, anon, authenticated;
grant execute on function public.get_holded_keys() to service_role;
