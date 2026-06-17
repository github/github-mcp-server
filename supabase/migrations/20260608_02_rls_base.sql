-- Fase 0 — RLS base: bloquea el acceso anónimo, permite a usuarios autenticados.
-- (En la Fase 1 estas políticas se sustituyen por las finas por rol.)
do $$
declare t text;
begin
  foreach t in array array[
    'sociedad','promocion','fase','unidad','cliente','proveedor','reserva',
    'contrato_venta','hito_pago','presupuesto','partida','contrato_obra',
    'certificacion','documento'
  ]
  loop
    execute format('alter table public.%I enable row level security;', t);
    execute format($p$create policy "acceso_autenticados" on public.%I
      for all to authenticated using (true) with check (true);$p$, t);
  end loop;
end $$;
