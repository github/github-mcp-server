# Arquitectura del ERP — Grupo Tesela

> Documento vivo. v2.
> Sector: **Construcción · Estudio de arquitectura · Promoción inmobiliaria**
> Usuarios objetivo: **10** (arranque por fases, meta a corto plazo: completar **Fase 2**).
> Sistema actual: **Holded** (contabilidad, facturación, CRM básico).

---

## 1. Principios de diseño

1. **Una única fuente de verdad:** los datos del negocio viven en una base de datos central; el resto de herramientas leen/escriben contra ella.
2. **No reinventar lo que ya funciona:** Holded sigue siendo el **motor contable y fiscal** (cumple normativa española: facturación, SII/Verifactu). El ERP nuevo lo **extiende**, no lo reemplaza.
3. **Modular y por fases:** cada módulo se activa cuando toca.
4. **API-first.**
5. **Soberanía del dato:** los datos sensibles (clientes, contratos, compraventas) deben estar en la UE/España.

---

## 2. Stack elegido (decisión tomada)

| Capa | Herramienta | Por qué |
|------|-------------|---------|
| **Contabilidad y facturación (motor fiscal)** | **Holded** (se mantiene) | Ya lo usáis y cumple la normativa fiscal española. Se integra vía su API. |
| **Núcleo de datos del ERP** | **PostgreSQL** (vía Supabase, o self-host en España) | Mejor que MySQL para un ERP: integridad, JSON, extensiones, permisos por fila. Ver §6. |
| **Lógica de negocio / API** | **Supabase Edge Functions** | Reglas del ERP y conexión con Holded. |
| **Automatización principal** | **Make.com** | Sincroniza Holded ⇄ ERP ⇄ resto de apps. |
| **CRM inmobiliario / comercial** | **Attio** | Pipeline de compradores, leads, visitas. |
| **Gestión documental** | **Google Drive** + **Supabase Storage** | Planos, licencias, contratos, escrituras. |
| **Documentación / procesos** | **Notion** | Manual interno, procedimientos de obra. |
| **Datos operativos ligeros** | **Airtable** | Vistas rápidas para obra/comercial sin tocar la BD. |
| **BI / Reporting** | **Coupler.io** | Cuadros de mando (rentabilidad por promoción, tesorería). |
| **Diseño de interfaz** | **Figma** | Frontend del ERP. |
| **Código / CI/CD** | **GitHub** | Versionado y despliegues. |
| **Gestión del desarrollo** | **Linear** | Roadmap del ERP. |
| **Ofimática** | **Google Workspace** | Correo, agenda, Drive (10 usuarios). |
| **Reuniones** | **Zoom** | Comités de obra, reuniones con clientes. |

> **Marketing (Supermetrics):** queda **fuera de las Fases 0–2.** Solo tiene sentido si hacéis inversión publicitaria seria para comercializar promociones. Lo dejamos para más adelante.

---

## 3. Arquitectura por capas

```mermaid
flowchart TB
    subgraph PRES["🖥️ Presentación"]
        UI["Frontend ERP\n(Figma → código)"]
        AIR["Airtable\n(vistas obra/comercial)"]
        NOT["Notion\n(procesos)"]
    end

    subgraph APP["⚙️ Aplicación / Lógica"]
        EF["Supabase Edge Functions\n(reglas + sync Holded)"]
        API["API auto-generada"]
    end

    subgraph DATA["🗄️ Datos — FUENTE ÚNICA DE VERDAD"]
        PG[("PostgreSQL\n(Supabase / self-host ES)")]
        ST["Storage\n(planos, contratos)"]
        AUTH["Auth\n(usuarios y permisos)"]
    end

    subgraph EXT["🔌 Sistemas Externos"]
        HOL["Holded\n(contabilidad/facturas)"]
        GW["Google Workspace\n(Drive/Gmail/Calendar)"]
        ATT["Attio (CRM)"]
        ZOOM["Zoom"]
    end

    subgraph AUTO["🔄 Automatización"]
        MAKE["Make.com"]
    end

    subgraph BI["📊 BI"]
        COUP["Coupler.io\n(cuadros de mando)"]
    end

    subgraph DEV["🛠️ Desarrollo"]
        GH["GitHub"]
        LIN["Linear"]
    end

    UI --> API
    AIR --> API
    API --> EF
    EF --> PG
    EF --> ST
    EF --> AUTH
    MAKE <--> PG
    MAKE <--> HOL
    MAKE <--> ATT
    MAKE <--> GW
    PG --> COUP
    HOL --> COUP
    GH --> EF
```

---

## 4. Módulos del ERP (adaptados al sector)

```mermaid
flowchart LR
    CORE[("Núcleo\nPostgreSQL")]
    CORE --- M1["💰 Contabilidad\n(Holded - integrado)"]
    CORE --- M2["🏗️ Promociones\ny Obras"]
    CORE --- M3["🏠 Comercialización\nde viviendas"]
    CORE --- M4["📐 Proyectos de\narquitectura"]
    CORE --- M5["🧾 Compras y\nsubcontratas"]
    CORE --- M6["🤝 CRM\n(Attio)"]
    CORE --- M7["🛠️ Postventa\ny garantías"]
    CORE --- M8["📈 BI / Cuadros\nde mando"]
    CORE --- M9["📁 Documental\n(planos/licencias)"]
```

| # | Módulo | Qué cubre | Conectores | Fase |
|---|--------|-----------|-----------|------|
| 1 | **Contabilidad / Facturación** | Asientos, facturas, impuestos | **Holded** (integrado) | F1 |
| 2 | **Promociones y Obras** | Promociones, fases, parcelas/viviendas, certificaciones, control de costes vs presupuesto | Postgres + Airtable | F1–F2 |
| 3 | **Comercialización** | Reservas, arras, contratos de compraventa, estado de cada vivienda | Postgres + Attio | F2 |
| 4 | **Proyectos de arquitectura** | Encargos, fases de diseño, licencias, honorarios | Postgres + Drive | F2 |
| 5 | **Compras y subcontratas** | Proveedores, pedidos, comparativas, certificaciones de subcontrata | Postgres + Holded | F2 |
| 6 | **CRM** | Leads compradores, visitas, seguimiento | Attio | F1–F2 |
| 7 | **Postventa** | Incidencias y garantías de vivienda entregada | Postgres | F3 |
| 8 | **BI / Cuadros de mando** | Rentabilidad por promoción, tesorería, avance de obra | Coupler.io | F2 |
| 9 | **Documental** | Planos, licencias, escrituras, contratos | Drive + Storage | F1 |

---

## 5. Hoja de ruta (objetivo: completar Fase 2)

- **Fase 0 — Cimientos:** decidir BD (§6), montar PostgreSQL + Auth, conectar Holded (lectura), estructura de datos, diseño en Figma.
- **Fase 1 — Núcleo:** Promociones/Obras (base) + Documental + integración Holded + CRM básico (Attio).
- **Fase 2 — Operación (META actual):** Comercialización de viviendas + Compras/subcontratas + Proyectos de arquitectura + BI (Coupler.io).
- **Fase 3 — Más adelante:** Postventa, RRHH, marketing (Supermetrics).

---

## 6. Decisión cerrada: base de datos

**Elegido: PostgreSQL en Supabase Cloud, región UE (opción A).**

Para un ERP, Postgres gana a MySQL en integridad de datos, tipos avanzados (JSON, geolocalización de parcelas), seguridad por fila y extensiones.

- **Hoy:** Supabase Cloud en región UE → arranque inmediato y datos en la Unión Europea (cumple RGPD).
- **Futuro (si surge requisito legal estricto de datos en España):** Postgres es portable → migración a self-host en proveedor de Barcelona sin rehacer el modelo. Puerta abierta, sin coste hoy.
- **MySQL descartado:** perderíamos las ventajas de Supabase (Auth, API automática, Storage, RLS) sin ganancia real.

| Opción | Datos en | Estado |
|--------|----------|--------|
| **A. Supabase Cloud (región UE)** | UE | ✅ **ELEGIDA** |
| B. Supabase self-host en Barcelona | España | 🔓 Reservada para el futuro |
| C. MySQL en Barcelona | España | ❌ Descartada |

---

## 7. Integración con Holded

- Holded **no se sustituye.** Sigue siendo el sistema legal de contabilidad y facturación.
- El ERP **lee** de Holded (facturas, cobros, proveedores) vía su API y los cruza con cada promoción para calcular rentabilidad real.
- El ERP **escribe** en Holded cuando proceda (p.ej. generar factura desde un contrato de compraventa) — a través de Make.com.
- Así evitamos duplicar la contabilidad y mantenemos el cumplimiento fiscal.

---

## 8. Decisiones cerradas

1. ✅ **Base de datos:** PostgreSQL en Supabase Cloud (región UE). Ver §6.
2. ✅ **Holded:** se mantiene como motor contable/fiscal, integrado por API (primero lectura). Ver §7.

### Pendiente menor
- ¿Tenéis ya alguna herramienta de gestión de obra/promociones de la que haya que migrar datos? (No bloquea el arranque.)

---

## 9. Siguiente paso

**Fase 0 en marcha.** El modelo de datos inicial (tablas del ERP) está en
[`MODELO-DATOS.md`](./MODELO-DATOS.md). Una vez validado, creamos el proyecto Supabase
y aplicamos el esquema.
