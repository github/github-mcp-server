# REPAIR PLAN (requiere aprobación — auditoría en modo read-only)
**Fecha:** 2026-06-22 (UTC). Nada de esto se ha ejecutado. Todo reversible, con backup y rollback.

## Fase A — P0 (esta semana)
| Paso | Acción | Backup | Rollback |
|---|---|---|---|
| A1 | Revocar anon en claude_sessions (erp + CONEXION): drop policy + `revoke ... from anon` | export filas antes | re-crear policy |
| A2 | Supabase Pro + habilitar PITR; backup lógico manual | — | — |
| A3 | Revocar EXECUTE anon en rls_auto_enable | snapshot grants | re-grant |
| A4 | Activar leaked-password protection (Auth) | — | desactivar |

## Fase B — P1 (30 días)
| B1 | Reconciliar migraciones: añadir al repo create_claude_sessions + funciones drift; baseline | git | revert commit |
| B2 | Unificar facturas: migrar holded_facturas→factura_holded; eliminar duplicado (tras backup) | export | restaurar tabla |
| B3 | Mover ERP a repo privado | git | — |
| B4 | Conectar 4 sociedades Holded (keys al Vault) | — | quitar key |

## Fase C — P2/P3 (90 días)
Índices FKs · RLS `(select auth.fn())` · alertas · modelo obra→capítulo→partida · usuarios reales.

> Regla Omega: cambios pequeños, un commit por cambio, sin mezclar refactor y feature, sin tocar producción sin aprobación.
