-- Fase 2 — Sincronización automática diaria de Holded (pg_cron + pg_net).
-- Invoca la Edge Function sync-holded cada día a las 06:00 UTC.

create extension if not exists pg_cron;

select cron.schedule(
  'holded-sync-diario',
  '0 6 * * *',
  $$ select net.http_post(
       url := 'https://jpojckqnhepiuwefyvdr.supabase.co/functions/v1/sync-holded',
       headers := '{"Content-Type":"application/json"}'::jsonb,
       body := '{}'::jsonb
     ) $$
);
