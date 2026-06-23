# ACCESS_REQUESTS — accesos que faltan para completar la auditoría

**Fecha:** 2026-06-22 (UTC). Generado porque varias capas del Master Prompt no son verificables desde esta sesión.
Sin estos accesos, esas capas quedan marcadas `N/D` (no se inventan hallazgos).

| Sistema | Acceso necesario | Permiso mínimo | Para qué | Riesgo si no se concede |
|---|---|---|---|---|
| **Vercel** | Token API / conector activo | Read-only | Despliegues, dominios, env names del frontend | Previews huérfanos / builds rotos sin detectar |
| **Cloudflare / DNS** | Acceso zona | Read-only | SPF/DKIM/DMARC, subdominios, SSL | Spoofing/shadow-IT sin detectar |
| **Holded (panel + API)** | 4 API keys restantes | Read-only | Conciliar datos maestros y facturas | BI no fiable; 4/5 sociedades sin sync |
| **Notion** | Conector workspace Tesela | Read | SOP/PRD duplicados/contradictorios | Procesos sin gobierno |
| **Google Workspace** | Drive/Gmail/Calendar | Read-only | Permisos, archivos públicos, taxonomía | PII expuesta / dependencia de correos |
| **n8n / Make / Zapier** | Workflows + historial | Read-only | Inventario de automatizaciones | Procesos que escriben en ERP sin control |
| **OpenAI / Anthropic** | Panel organización | Read billing/usage | Coste IA, modelos obsoletos | Burn-rate IA desconocido |
| **GitHub (ampliado)** | Repos Tesela privados en el conector | Read | Forense por repo (SOT/Tesela-iA/Cerebro/command-center) | Sprawl no cuantificable |
| **Supabase (billing)** | Org uladuspfccwdyrmyklnk | Read billing | Plan/coste/PITR de los 3 proyectos | Coste/continuidad aproximados |

> Conceder = añadir repo/credencial al conector, o ejecutar esa capa desde una sesión con ese acceso y volcar el resultado aquí.
