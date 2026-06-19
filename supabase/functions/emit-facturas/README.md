# Edge Function `emit-facturas` — emisión de facturas del ERP hacia Holded

Consume la cola `factura_pendiente` (las facturas que el trigger encola al escriturar una
compraventa) y las **emite como factura real en Holded** vía la API directa, guardando el
`holded_factura_id` y marcando la fila como `enviada`.

Es el **consumidor** que faltaba: hasta ahora las facturas se encolaban pero nadie las emitía.

## Seguridad (importante: emite documentos contables reales)

- **Autenticación por secreto compartido:** solo se ejecuta si la cabecera
  `Authorization: Bearer <SERVICE_ROLE_KEY>` coincide con la service-role key del proyecto.
  No es invocable desde el navegador ni por usuarios normales.
- **Seguro por defecto (`dryRun=true`):** sin body, o con `{"dryRun": true}`, **NO emite nada**:
  solo devuelve qué facturas emitiría. Para emitir de verdad: `{"dryRun": false}`.
- **Ignora datos no reales:** solo procesa facturas cuyo cliente tenga `holded_id`; las
  omitidas se reportan con su motivo (cliente sin `holded_id`, o sociedad sin key en el Vault).

## Uso

```bash
# Ver qué se emitiría (dry-run, no escribe en Holded)
curl -s https://jpojckqnhepiuwefyvdr.supabase.co/functions/v1/emit-facturas \
  -H "Authorization: Bearer <SERVICE_ROLE_KEY>"

# Emitir de verdad
curl -s https://jpojckqnhepiuwefyvdr.supabase.co/functions/v1/emit-facturas \
  -H "Authorization: Bearer <SERVICE_ROLE_KEY>" \
  -H "Content-Type: application/json" -d '{"dryRun": false}'
```

## Pendiente de afinar antes de uso real

- **IVA/impuestos:** ahora envía `items` sin `tax`; revisar el tipo impositivo que aplica.
- **Numeración/serie** de factura en Holded.
- Cuando esté validado, se puede programar en un cron (como `sync-holded`) — pero solo tras
  confirmar que la emisión automática es deseada.
