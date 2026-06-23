# SCALABILITY REPORT
**Fecha:** 2026-06-22 (UTC). Respuestas con evidencia o N/D.

| Pregunta | Respuesta | Base |
|---|---|---|
| ¿1.000 clientes? | Sí técnicamente | Postgres sobra; falta índices FKs |
| ¿10.000 clientes? | Sí con índices + Pro | advisors unindexed_fk; Free→Pro |
| ¿100 empleados? | Parcial | RLS por rol listo; falta validar con usuarios reales, MFA org |
| ¿20 obras simultáneas? | No aún | modelo obra→capítulo→partida incompleto |
| ¿100 obras? | No | requiere modelo de coste + BI consolidado |
| ¿Promotora 100M€? | No aún | gobierno de datos + única fuente + auditoría |
| ¿Reporting consolidado? | No fiable hoy | datos repartidos en 3 Supabase + DuckDB |
| ¿Auditoría externa? | No aún | IaC con deriva, sin trazabilidad completa |
| ¿Continuidad si se va una persona? | No | bus factor = 1, coordinación artesanal |
| ¿Ciberincidente? | Frágil | sin PITR, sin alertas; claude_sessions abierto |

**Conclusión:** escala técnica del motor OK; lo que no escala es el **gobierno de datos, el modelo de coste y la continuidad**. Resolver eso desbloquea 20→100 obras.
