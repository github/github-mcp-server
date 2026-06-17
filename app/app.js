// App ERP Grupo Tesela — frontend conectado a Supabase
const sb = supabase.createClient(window.SUPABASE_CONFIG.url, window.SUPABASE_CONFIG.anonKey);

const $ = (id) => document.getElementById(id);
const eur = (n) => (n == null ? "—" : Number(n).toLocaleString("es-ES") + " €");
const PROMOS = {};
let ROL = null;
let DETALLE_ID = null;

async function init() {
  const { data: { session } } = await sb.auth.getSession();
  if (session) showApp(session.user); else showLogin();
}
function showLogin() {
  $("login").classList.remove("hidden");
  $("app").classList.add("hidden");
}
async function showApp(user) {
  $("login").classList.add("hidden");
  $("app").classList.remove("hidden");
  let rol = "sin rol";
  const { data: perfil } = await sb.from("perfil").select("rol").eq("id", user.id).maybeSingle();
  if (perfil && perfil.rol) rol = perfil.rol;
  ROL = rol;
  $("who").innerHTML = `<b>${user.email}</b><span class="rolepill">${rol}</span>`;
  mostrarLista();
  cargar();
}
function mostrarLista() {
  $("vistaDetalle").classList.add("hidden");
  $("vistaLista").classList.remove("hidden");
}

async function cargar() {
  const grid = $("grid");
  grid.innerHTML = "";

  // Resumen del grupo (KPIs agregados)
  const { data: g } = await sb.from("v_resumen_grupo").select("*").maybeSingle();
  if (g) {
    $("resumenGrupo").innerHTML =
      kpiCard("Promociones", g.num_promociones) +
      kpiCard("Valor cartera", eur(g.valor_cartera)) +
      kpiCard("Total vendido", eur(g.total_vendido), "var(--prim)") +
      kpiCard("Margen total", eur(g.margen_total), "var(--green)") +
      kpiCard("Caja cobrada", eur(g.caja_cobrada), "var(--green)") +
      kpiCard("Caja pendiente", eur(g.caja_pendiente), "var(--amber)");
  }

  const [com, rent] = await Promise.all([
    sb.from("v_comercializacion_promocion").select("*"),
    sb.from("v_rentabilidad_promocion").select("*"),
  ]);
  if (com.error || rent.error) {
    $("appSub").textContent = "Error al cargar datos: " + (com.error?.message || rent.error?.message);
    return;
  }
  const rentById = {};
  (rent.data || []).forEach((r) => (rentById[r.id] = r));
  const promos = com.data || [];
  if (promos.length === 0) {
    $("appSub").textContent = "";
    grid.innerHTML = `<div class="empty">No tienes promociones visibles todavía.<br>
      Si acabas de registrarte, Dirección debe asignarte un rol y tus promociones.</div>`;
    return;
  }
  $("appSub").textContent = `${promos.length} promoción(es) — haz clic en una para ver el detalle.`;
  promos.sort((a, b) => (rentById[b.id]?.margen || 0) - (rentById[a.id]?.margen || 0));
  promos.forEach((p) => {
    PROMOS[p.id] = p;
    const r = rentById[p.id] || {};
    const estado = p.estado || "—";
    const eClass = estado.toLowerCase().includes("obra") ? "eobra" : "";
    const pct = p.pct_colocado != null ? Number(p.pct_colocado) : 0;
    const card = document.createElement("div");
    card.className = "promo";
    card.innerHTML = `
      <h3>${p.nombre}</h3>
      <div class="loc">${p.municipio || ""} <span class="estado ${eClass}">${estado}</span></div>
      <div class="kpis">
        <div class="kpi"><div class="l">% Colocado</div><div class="v">${pct}%</div></div>
        <div class="kpi"><div class="l">Vendidas</div><div class="v">${p.vendidas}/${p.total_unidades}</div></div>
        <div class="kpi"><div class="l">Ingresos</div><div class="v">${eur(r.ingresos_contratados)}</div></div>
        <div class="kpi"><div class="l">Margen</div><div class="v" style="color:var(--green)">${eur(r.margen)}</div></div>
      </div>
      <div class="track"><div class="fill" style="width:${Math.min(100, pct)}%"></div></div>`;
    card.addEventListener("click", () => abrirDetalle(p.id));
    grid.appendChild(card);
  });
}

const kpiBox = (l, v) => `<div class="kpi"><div class="l">${l}</div><div class="v">${v}</div></div>`;
const kpiCard = (l, v, col) => `<div class="card kpi"><div class="l">${l}</div><div class="v" style="${col ? `color:${col}` : ""}">${v}</div></div>`;

function abrirDetalle(id) {
  const p = PROMOS[id];
  if (!p) return;
  DETALLE_ID = id;
  $("vistaLista").classList.add("hidden");
  const d = $("vistaDetalle");
  d.classList.remove("hidden");
  d.innerHTML = `
    <button class="back" id="volver">← Volver a promociones</button>
    <h1>${p.nombre}</h1>
    <div class="sub">${p.municipio || ""} · ${p.estado || ""}</div>
    <div id="detKpis" class="kpis" style="grid-template-columns:repeat(4,1fr)"></div>
    <div class="section"><h2>Unidades</h2><div id="detUnidades">Cargando…</div></div>
    <div class="section"><h2>Tesorería</h2><div id="detTeso">Cargando…</div></div>
    <div class="section"><h2>Avance de obra</h2><div id="detObra">Cargando…</div></div>
    <div class="section"><h2>Documentos</h2><div id="detDocs">Cargando…</div></div>`;
  $("volver").addEventListener("click", mostrarLista);
  cargarDetalle(id);
}

async function cargarDetalle(id) {
  const [rent, com, teso] = await Promise.all([
    sb.from("v_rentabilidad_promocion").select("*").eq("id", id).maybeSingle(),
    sb.from("v_comercializacion_promocion").select("*").eq("id", id).maybeSingle(),
    sb.from("v_tesoreria_promocion").select("*").eq("id", id).maybeSingle(),
  ]);
  const r = rent.data || {}, cm = com.data || {}, t = teso.data || {};
  $("detKpis").innerHTML =
    kpiBox("% Colocado", (cm.pct_colocado ?? 0) + "%") +
    kpiBox("Ingresos", eur(r.ingresos_contratados)) +
    kpiBox("Coste real", eur(r.coste_real)) +
    kpiBox("Margen", eur(r.margen));

  // Unidades
  const fases = await sb.from("fase").select("id").eq("promocion_id", id);
  const faseIds = (fases.data || []).map((f) => f.id);
  let ud = [];
  if (faseIds.length) {
    const uni = await sb.from("unidad")
      .select("id,referencia,tipo,estado,precio_venta,contrato_venta(cliente(nombre))")
      .in("fase_id", faseIds).order("referencia");
    ud = uni.data || [];
  }
  const puedeVender = ROL === "direccion" || ROL === "comercial";
  $("detUnidades").innerHTML = ud.length ? `<table>
    <tr><th>Ref.</th><th>Tipo</th><th>Estado</th><th class="rt">Precio</th><th>Comprador</th><th></th></tr>
    ${ud.map((u) => {
      const comp = u.contrato_venta?.[0]?.cliente?.nombre || "—";
      const accion = (puedeVender && u.estado !== "vendida")
        ? `<button class="mini" onclick="registrarVenta('${u.id}',${u.precio_venta || 0},'${u.referencia}')">Vender</button>`
        : "";
      return `<tr><td>${u.referencia}</td><td>${u.tipo}</td><td>${u.estado}</td><td class="rt">${eur(u.precio_venta)}</td><td>${comp}</td><td class="rt">${accion}</td></tr>`;
    }).join("")}</table>` : `<div class="sub">Sin unidades.</div>`;

  // Tesorería
  $("detTeso").innerHTML = (t && t.total_hitos != null) ? `<table>
    <tr><th>Contratado en hitos</th><th>Cobrado</th><th>Pendiente</th><th class="rt">% cobrado</th></tr>
    <tr><td>${eur(t.total_hitos)}</td><td style="color:var(--green)">${eur(t.cobrado)}</td><td style="color:var(--amber)">${eur(t.pendiente_cobro)}</td><td class="rt">${t.pct_cobrado ?? 0}%</td></tr>
    </table>` : `<div class="sub">Sin hitos de pago registrados.</div>`;

  // Obra
  const obra = await sb.from("contrato_obra")
    .select("id,descripcion,importe_adjudicado,estado,certificacion(importe)").eq("promocion_id", id);
  const od = obra.data || [];
  const puedeCert = ROL === "direccion" || ROL === "obra";
  $("detObra").innerHTML = od.length ? `<table>
    <tr><th>Contrato</th><th class="rt">Adjudicado</th><th class="rt">Certificado</th><th class="rt">%</th><th></th></tr>
    ${od.map((o) => {
      const cert = (o.certificacion || []).reduce((s, c) => s + (Number(c.importe) || 0), 0);
      const pct = o.importe_adjudicado ? Math.round(1000 * cert / o.importe_adjudicado) / 10 : 0;
      const accion = puedeCert ? `<button class="mini" data-co="${o.id}">+ Cert.</button>` : "";
      return `<tr><td>${o.descripcion}</td><td class="rt">${eur(o.importe_adjudicado)}</td><td class="rt">${eur(cert)}</td><td class="rt">${pct}%</td><td class="rt">${accion}</td></tr>`;
    }).join("")}</table>` : `<div class="sub">Sin contratos de obra.</div>`;
  $("detObra").querySelectorAll("button.mini").forEach((b) =>
    b.addEventListener("click", () => registrarCertificacion(b.getAttribute("data-co"))));

  // Documentos
  const docs = await sb.from("documento").select("tipo,nombre").eq("promocion_id", id);
  const dd = docs.data || [];
  $("detDocs").innerHTML = dd.length ? `<table>
    <tr><th>Tipo</th><th>Nombre</th></tr>
    ${dd.map((x) => `<tr><td>${x.tipo}</td><td>${x.nombre}</td></tr>`).join("")}</table>`
    : `<div class="sub">Sin documentos.</div>`;
}

// ---- Modal + registrar venta ----
function openModal(html) { $("modalBody").innerHTML = html; $("modal").classList.remove("hidden"); }
function closeModal() { $("modal").classList.add("hidden"); $("modalBody").innerHTML = ""; }
window.closeModal = closeModal;

window.registrarVenta = async function (unidadId, precio, ref) {
  const { data: clientes } = await sb.from("cliente").select("id,nombre").order("nombre");
  const opts = (clientes || []).map((c) => `<option value="${c.id}">${c.nombre}</option>`).join("");
  openModal(`
    <h2>Registrar venta · ${ref}</h2>
    <div class="field"><label>Cliente</label><select id="mCliente">${opts}</select></div>
    <div class="field"><label>Precio (€)</label><input id="mPrecio" type="number" value="${precio}"></div>
    <div id="mMsg" class="msg"></div>
    <div class="modal-actions">
      <button class="linkbtn" onclick="closeModal()">Cancelar</button>
      <button class="btn btn-sm" onclick="confirmarVenta('${unidadId}')">Confirmar venta</button>
    </div>`);
};

window.confirmarVenta = async function (unidadId) {
  const clienteId = $("mCliente").value;
  const precio = Number($("mPrecio").value);
  const msg = $("mMsg");
  msg.className = "msg";
  msg.textContent = "Guardando…";
  if (!clienteId) { msg.className = "msg err"; msg.textContent = "Selecciona un cliente."; return; }
  const ins = await sb.from("contrato_venta").insert({
    unidad_id: unidadId, cliente_id: clienteId, precio_total: precio,
    fecha_firma: new Date().toISOString().slice(0, 10), estado: "escriturado",
  });
  if (ins.error) { msg.className = "msg err"; msg.textContent = ins.error.message; return; }
  // La unidad pasa a 'vendida' por trigger; la factura se encola por trigger.
  closeModal();
  abrirDetalle(DETALLE_ID); // recargar el detalle con los datos actualizados
};

let CO = null;
window.registrarCertificacion = async function (coId) {
  CO = coId;
  openModal(`
    <h2>Nueva certificación de obra</h2>
    <div class="field"><label>Nº de certificación</label><input id="cNum" type="number" value="1"></div>
    <div class="field"><label>Importe (€)</label><input id="cImp" type="number" placeholder="0"></div>
    <div class="field"><label>Fecha</label><input id="cFecha" type="date" value="${new Date().toISOString().slice(0, 10)}"></div>
    <div id="cMsg" class="msg"></div>
    <div class="modal-actions">
      <button class="linkbtn" onclick="closeModal()">Cancelar</button>
      <button class="btn btn-sm" onclick="confirmarCert()">Guardar</button>
    </div>`);
};
window.confirmarCert = async function () {
  const importe = Number($("cImp").value);
  const msg = $("cMsg");
  msg.className = "msg";
  if (!importe) { msg.className = "msg err"; msg.textContent = "Indica un importe."; return; }
  msg.textContent = "Guardando…";
  const ins = await sb.from("certificacion").insert({
    contrato_obra_id: CO, numero: Number($("cNum").value) || null,
    importe, fecha: $("cFecha").value,
  });
  if (ins.error) { msg.className = "msg err"; msg.textContent = ins.error.message; return; }
  closeModal();
  abrirDetalle(DETALLE_ID);
};

// Eventos de login
$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = $("loginMsg");
  msg.className = "msg";
  msg.textContent = "Entrando…";
  const { data, error } = await sb.auth.signInWithPassword({
    email: $("email").value.trim(),
    password: $("password").value,
  });
  if (error) { msg.className = "msg err"; msg.textContent = error.message; return; }
  msg.textContent = "";
  showApp(data.user);
});
$("logout").addEventListener("click", async () => { await sb.auth.signOut(); showLogin(); });

init();
