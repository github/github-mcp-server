// App ERP Grupo Tesela — frontend conectado a Supabase
const sb = supabase.createClient(window.SUPABASE_CONFIG.url, window.SUPABASE_CONFIG.anonKey);

const $ = (id) => document.getElementById(id);
const eur = (n) => (n == null ? "—" : Number(n).toLocaleString("es-ES") + " €");

// ---- Routing simple según sesión ----
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

  // Rol del usuario
  let rol = "sin rol";
  const { data: perfil } = await sb.from("perfil").select("rol").eq("id", user.id).maybeSingle();
  if (perfil && perfil.rol) rol = perfil.rol;
  $("who").innerHTML = `<b>${user.email}</b><span class="rolepill">${rol}</span>`;

  await cargarPromociones();
}

// ---- Carga de datos en vivo (RLS aplica automáticamente) ----
async function cargarPromociones() {
  const grid = $("grid");
  grid.innerHTML = "";

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

  $("appSub").textContent = `${promos.length} promoción(es) visibles según tu rol.`;

  promos.forEach((p) => {
    const r = rentById[p.id] || {};
    const estado = (p.estado || "—");
    const eClass = "e-" + estado.toLowerCase().replace(/\s+/g, "");
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
      <div class="track"><div class="fill" style="width:${Math.min(100, pct)}%"></div></div>
    `;
    grid.appendChild(card);
  });
}

// ---- Eventos ----
$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = $("loginMsg");
  msg.className = "msg";
  msg.textContent = "Entrando…";
  const { data, error } = await sb.auth.signInWithPassword({
    email: $("email").value.trim(),
    password: $("password").value,
  });
  if (error) {
    msg.className = "msg err";
    msg.textContent = error.message;
    return;
  }
  msg.className = "msg ok";
  msg.textContent = "";
  showApp(data.user);
});

$("logout").addEventListener("click", async () => {
  await sb.auth.signOut();
  showLogin();
});

init();
