-- Fase 2 — Arreglo del timeout del cron de sincronización de Holded.
-- pg_net abandonaba la llamada a sync-holded a los 5 s (default), pero la función
-- tarda ~31 s en completar → cada ejecución quedaba marcada como `timed_out` aunque
-- el sync sí terminaba. Resultado: monitorización ciega (nunca se sabía si fallaba de
-- verdad). Se sube el timeout a 90 s para que pg_net espere la respuesta real (200/error).

select cron.unschedule('holded-sync-diario');

select cron.schedule(
  'holded-sync-diario',
  '0 6 * * *',
  $$ select net.http_post(
       url := 'https://jpojckqnhepiuwefyvdr.supabase.co/functions/v1/sync-holded',
       headers := '{"Content-Type":"application/json"}'::jsonb,
       body := '{}'::jsonb,
       timeout_milliseconds := 90000
     ) $$
);
