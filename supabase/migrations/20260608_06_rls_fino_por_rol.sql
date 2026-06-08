-- Fase 1 — RLS fino por rol y promocion
-- Sustituye las politicas provisionales por permisos reales:
--   direccion: acceso total | obra/comercial: lectura por promocion asignada
--   + escritura en su dominio (obra: presupuestos/obra; comercial: ventas).

alter table public.perfil enable row level security;
alter table public.acceso_promocion enable row level security;

create policy "perfil_ver" on public.perfil for select to authenticated
  using (id = auth.uid() or public.es_direccion());
create policy "perfil_direccion" on public.perfil for all to authenticated
  using (public.es_direccion()) with check (public.es_direccion());

create policy "acceso_ver" on public.acceso_promocion for select to authenticated
  using (perfil_id = auth.uid() or public.es_direccion());
create policy "acceso_direccion" on public.acceso_promocion for all to authenticated
  using (public.es_direccion()) with check (public.es_direccion());

-- Quitar politicas provisionales permisivas de Fase 0
do $$
declare t text;
begin
  foreach t in array array['sociedad','promocion','fase','unidad','cliente','proveedor',
    'reserva','contrato_venta','hito_pago','presupuesto','partida','contrato_obra',
    'certificacion','documento']
  loop execute format('drop policy if exists "acceso_autenticados" on public.%I;', t); end loop;
end $$;

-- Direccion: acceso total
do $$
declare t text;
begin
  foreach t in array array['sociedad','promocion','fase','unidad','cliente','proveedor',
    'reserva','contrato_venta','hito_pago','presupuesto','partida','contrato_obra',
    'certificacion','documento']
  loop execute format($p$create policy "direccion_todo" on public.%I
    for all to authenticated using (public.es_direccion()) with check (public.es_direccion());$p$, t); end loop;
end $$;

-- Lectura de catalogos compartidos
create policy "ver_sociedad"  on public.sociedad  for select to authenticated using (true);
create policy "ver_cliente"   on public.cliente   for select to authenticated using (true);
create policy "ver_proveedor" on public.proveedor for select to authenticated using (true);

-- Lectura por promocion accesible (arbol de promociones)
create policy "ver_promocion" on public.promocion for select to authenticated
  using (public.puede_ver_promocion(id));
create policy "ver_fase" on public.fase for select to authenticated
  using (public.puede_ver_promocion(promocion_id));
create policy "ver_unidad" on public.unidad for select to authenticated
  using (public.puede_ver_promocion((select f.promocion_id from public.fase f where f.id = fase_id)));
create policy "ver_presupuesto" on public.presupuesto for select to authenticated
  using (public.puede_ver_promocion(promocion_id));
create policy "ver_partida" on public.partida for select to authenticated
  using (public.puede_ver_promocion((select pr.promocion_id from public.presupuesto pr where pr.id = presupuesto_id)));
create policy "ver_contrato_obra" on public.contrato_obra for select to authenticated
  using (public.puede_ver_promocion(promocion_id));
create policy "ver_certificacion" on public.certificacion for select to authenticated
  using (public.puede_ver_promocion((select co.promocion_id from public.contrato_obra co where co.id = contrato_obra_id)));
create policy "ver_reserva" on public.reserva for select to authenticated
  using (public.puede_ver_promocion((select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id)));
create policy "ver_contrato_venta" on public.contrato_venta for select to authenticated
  using (public.puede_ver_promocion((select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id)));
create policy "ver_hito_pago" on public.hito_pago for select to authenticated
  using (public.puede_ver_promocion((select f.promocion_id from public.contrato_venta cv
            join public.unidad u on u.id = cv.unidad_id
            join public.fase f on f.id = u.fase_id where cv.id = contrato_id)));
create policy "ver_documento" on public.documento for select to authenticated
  using (public.puede_ver_promocion(coalesce(promocion_id,
            (select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id))));

-- Escritura por dominio: OBRA
create policy "obra_presupuesto" on public.presupuesto for all to authenticated
  using (public.current_rol() = 'obra' and public.puede_ver_promocion(promocion_id))
  with check (public.current_rol() = 'obra' and public.puede_ver_promocion(promocion_id));
create policy "obra_contrato_obra" on public.contrato_obra for all to authenticated
  using (public.current_rol() = 'obra' and public.puede_ver_promocion(promocion_id))
  with check (public.current_rol() = 'obra' and public.puede_ver_promocion(promocion_id));

-- Escritura por dominio: COMERCIAL
create policy "comercial_reserva" on public.reserva for all to authenticated
  using (public.current_rol() = 'comercial' and public.puede_ver_promocion((select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id)))
  with check (public.current_rol() = 'comercial' and public.puede_ver_promocion((select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id)));
create policy "comercial_contrato_venta" on public.contrato_venta for all to authenticated
  using (public.current_rol() = 'comercial' and public.puede_ver_promocion((select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id)))
  with check (public.current_rol() = 'comercial' and public.puede_ver_promocion((select f.promocion_id from public.unidad u join public.fase f on f.id = u.fase_id where u.id = unidad_id)));
