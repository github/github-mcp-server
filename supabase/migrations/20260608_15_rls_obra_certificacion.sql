-- Fase 2 — El rol 'obra' puede registrar certificaciones de obra de sus promociones.
-- (direccion ya podía por la política direccion_todo.)

create policy "obra_certificacion" on public.certificacion for all to authenticated
  using (public.current_rol() = 'obra' and public.puede_ver_promocion(
    (select co.promocion_id from public.contrato_obra co where co.id = contrato_obra_id)))
  with check (public.current_rol() = 'obra' and public.puede_ver_promocion(
    (select co.promocion_id from public.contrato_obra co where co.id = contrato_obra_id)));
