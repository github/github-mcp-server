-- Fase 2 — Hardening de seguridad final.

-- Fijar search_path en la función que faltaba
alter function public.promocion_de_path(text) set search_path = public;

-- Funciones de trigger: nadie debe invocarlas por API (se ejecutan solas)
revoke execute on function public.handle_new_user()       from anon, authenticated, public;
revoke execute on function public.encolar_factura()       from anon, authenticated, public;
revoke execute on function public.marcar_unidad_estado()  from anon, authenticated, public;

-- Helpers de RLS: quitar de anon (authenticated las necesita para evaluar las políticas)
revoke execute on function public.current_rol()             from anon;
revoke execute on function public.es_direccion()            from anon;
revoke execute on function public.puede_ver_promocion(uuid) from anon;
revoke execute on function public.promocion_de_path(text)   from anon, public;
