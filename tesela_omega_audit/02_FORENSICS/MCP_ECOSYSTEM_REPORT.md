# MCP ECOSYSTEM REPORT
**Fecha:** 2026-06-22 (UTC). Fuente: servidores MCP conectados a esta sesión (conexión intermitente observada).

## Relevantes para Grupo Tesela
| MCP | Uso real en auditoría | Riesgo/nota |
|---|---|---|
| Supabase | ✅ intensivo (lectura) | Crítico para el ERP; mantener |
| GitHub | ✅ (get_me, repos, PRs) | Scope limitado a github-mcp-server |
| Vercel | ❌ no usado | Útil si hay frontends desplegados |
| Cloudflare | ❌ | DNS/SSL |
| Notion | ❌ | SOP/PRD |
| Make / Zapier | ❌ | Automatizaciones (Holded vía Zapier) |
| Google (Drive/Gmail/Calendar) | ❌ | Documental/agenda |
| Figma | ❌ | Diseño UI (bloqueado por plan en bitácoras) |
| Airtable / Linear / Slack | ❌ | Operativa/PM/comms |

## No relacionados con el negocio (ruido/superficie)
Supermetrics, ICD-10, Metaview, MyInvestor, Era Context, Uber/Uber Eats, Expedia/Booking/Tripadvisor, Adobe, Canva, SketchUp, Three.js, PDF/Goodnotes, ZipRecruiter, Play Sheet Music, etc.

## Observaciones
1. **Inestabilidad de conexión:** múltiples servidores conectan/desconectan durante la sesión → fiabilidad de automatizaciones MCP cuestionable; no apoyar procesos críticos en MCP volátil.
2. **Superficie excesiva:** decenas de MCP activos sin relación con el negocio = más superficie, más confusión, posible coste. Recomendación: dejar habilitados solo los que se usan (Supabase, GitHub, y los que dirección decida: Vercel/Cloudflare/Notion/Holded-Zapier/Google).
3. **Riesgo combinado con R1:** agentes con estos MCP (write) + coordinación pública (claude_sessions) = por qué R1 es P0.
