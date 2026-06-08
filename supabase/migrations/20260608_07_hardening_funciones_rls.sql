-- Fase 1 — Endurecimiento de las funciones helper
-- Las funciones siguen usandose dentro de las politicas RLS, pero se retiran
-- del API REST publico para que no sean invocables por usuarios anonimos.

revoke execute on function public.handle_new_user()         from public;
revoke execute on function public.current_rol()             from public;
revoke execute on function public.es_direccion()            from public;
revoke execute on function public.puede_ver_promocion(uuid) from public;

grant execute on function public.current_rol()             to authenticated;
grant execute on function public.es_direccion()            to authenticated;
grant execute on function public.puede_ver_promocion(uuid) to authenticated;
