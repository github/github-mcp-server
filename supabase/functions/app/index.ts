// Edge Function `app`: sirve la web del ERP (login + dashboard) como HTML autocontenido.
// Desplegada con verify_jwt=false (web pública; el login lo hace supabase-js, datos por RLS).
// Los valores importados de Holded se escapan con esc() antes de renderizarse (anti-XSS).
const HTML = String.raw`<!DOCTYPE html>
<html lang="es"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>ERP Grupo Tesela</title>
<style>
:root{--bg:#f4f6f8;--card:#fff;--prim:#0f766e;--dark:#111827;--mut:#6b7280;--green:#16a34a;--amber:#d97706;--red:#dc2626;--line:#e5e7eb;}
*{box-sizing:border-box;margin:0;padding:0}body{font-family:Inter,system-ui,-apple-system,Segoe UI,Roboto,sans-serif;background:var(--bg);color:var(--dark);}
.hidden{display:none!important;}
.login-wrap{min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px;}
.login{width:100%;max-width:400px;text-align:center;}.brand{font-size:30px;font-weight:800;color:var(--prim);}.brand-sub{color:var(--mut);font-size:14px;margin:2px 0 24px;}
.card{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:24px;}
.field{text-align:left;margin-bottom:16px;}.field label{font-size:12px;font-weight:600;display:block;margin-bottom:6px;}
.field input,.field select{width:100%;border:1px solid var(--line);border-radius:10px;padding:12px 14px;font-size:14px;outline:none;}
.btn{width:100%;background:var(--prim);color:#fff;font-weight:600;border:none;padding:13px;border-radius:10px;font-size:15px;cursor:pointer;}
.btn-sm{width:auto;padding:10px 16px;}
.msg{font-size:13px;margin-top:12px;min-height:18px;}.msg.err{color:var(--red);}.foot{color:var(--mut);font-size:12px;margin-top:18px;}
.topbar{background:#fff;border-bottom:1px solid var(--line);padding:16px 32px;display:flex;align-items:center;gap:12px;}
.topbar .logo{font-weight:800;color:var(--prim);font-size:18px;}.spacer{flex:1;}.who{font-size:13px;color:var(--mut);}.who b{color:var(--dark);}
.navlink{color:var(--mut);font-size:14px;font-weight:600;cursor:pointer;margin-left:8px;}.navlink:hover{color:var(--prim);}
.rolepill{display:inline-block;background:var(--prim);color:#fff;font-size:11px;font-weight:600;padding:2px 9px;border-radius:999px;margin-left:6px;text-transform:capitalize;}
.linkbtn{background:none;border:1px solid var(--line);border-radius:8px;padding:8px 12px;font-size:13px;cursor:pointer;}
.content{padding:32px;max-width:1100px;margin:0 auto;}h1{font-size:26px;font-weight:800;margin-bottom:14px;}.sub{color:var(--mut);font-size:14px;margin-bottom:20px;}
.resumen{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:16px;}.resumen .card{padding:18px;}.seccion{font-size:20px;font-weight:700;margin:28px 0 6px;}
.seccion-head{display:flex;align-items:center;gap:14px;margin:28px 0 6px;}.seccion-head .seccion{margin:0;}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(330px,1fr));gap:20px;}
.promo{background:#fff;border:1px solid var(--line);border-radius:14px;padding:20px;cursor:pointer;transition:box-shadow .15s;}.promo:hover{box-shadow:0 6px 20px rgba(0,0,0,.09);}
.promo h3{font-size:18px;font-weight:700;}.promo .loc{color:var(--mut);font-size:13px;margin-bottom:14px;}
.estado{font-size:11px;font-weight:600;padding:2px 9px;border-radius:999px;text-transform:capitalize;background:#f3f4f6;color:var(--mut);}.eobra{background:#ecfdf5!important;color:var(--green)!important;}
.kpis{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:8px;}.kpi .l{font-size:11px;color:var(--mut);text-transform:uppercase;font-weight:600;}.kpi .v{font-size:18px;font-weight:700;margin-top:2px;}
.track{height:8px;background:var(--line);border-radius:4px;overflow:hidden;margin-top:14px;}.fill{height:100%;background:var(--prim);border-radius:4px;}
.empty{background:#fff;border:1px dashed var(--line);border-radius:14px;padding:40px;text-align:center;color:var(--mut);}
.section{background:#fff;border:1px solid var(--line);border-radius:14px;padding:20px;margin-top:20px;}.section h2{font-size:16px;font-weight:600;margin-bottom:6px;}
.back{background:none;border:none;color:var(--prim);font-weight:600;cursor:pointer;font-size:14px;margin-bottom:12px;padding:0;}
table{width:100%;border-collapse:collapse;margin-top:6px;}th{font-size:11px;color:var(--mut);text-transform:uppercase;text-align:left;padding:6px 0;border-bottom:1px solid var(--line);}td{padding:8px 0;font-size:13px;border-bottom:1px solid var(--line);}tr:last-child td{border-bottom:none;}.rt{text-align:right;}
.mini{background:var(--prim);color:#fff;border:none;border-radius:7px;padding:5px 10px;font-size:12px;font-weight:600;cursor:pointer;}
.modal-bg{position:fixed;inset:0;background:rgba(17,24,39,.5);display:flex;align-items:center;justify-content:center;padding:24px;z-index:50;}
.modal-box{background:#fff;border-radius:14px;padding:24px;width:100%;max-width:420px;}.modal-box h2{font-size:18px;font-weight:700;margin-bottom:16px;}
.modal-actions{display:flex;justify-content:flex-end;gap:10px;margin-top:10px;}
</style></head><body>
<div id="login" class="login-wrap"><div class="login"><div class="brand">Grupo Tesela</div><div class="brand-sub">ERP de Promocion inmobiliaria</div>
<form id="loginForm" class="card"><div class="field"><label>Email</label><input id="email" type="email" placeholder="tu@grupotesela.com" required></div>
<div class="field"><label>Contrasena</label><input id="password" type="password" placeholder="********" required></div>
<button class="btn" type="submit">Entrar</button><div id="loginMsg" class="msg"></div></form>
<div class="foot">Acceso restringido - Grupo Tesela</div></div></div>
<div id="app" class="hidden"><div class="topbar"><span class="logo">Grupo Tesela ERP</span><a class="navlink" id="navPromos">Promociones</a><a class="navlink" id="navContactos">Contactos</a><a class="navlink" id="navFacturas">Facturas</a><span class="spacer"></span><span class="who" id="who"></span><button class="linkbtn" id="logout">Salir</button></div>
<div class="content"><div id="vistaLista"><h1>Resumen del grupo</h1><div id="resumenGrupo" class="resumen"></div><div class="seccion-head"><h2 class="seccion">Promociones</h2><button id="btnNuevaPromo" class="btn-sm hidden">+ Nueva promocion</button></div><div class="sub" id="appSub">Cargando...</div><div id="grid" class="grid"></div></div><div id="vistaDetalle" class="hidden"></div><div id="vistaContactos" class="hidden"></div><div id="vistaFacturas" class="hidden"></div></div>
<div id="modal" class="modal-bg hidden"><div class="modal-box" id="modalBody"></div></div></div>
<script src="https://cdn.jsdelivr.net/npm/@supabase/supabase-js@2"></script>
<script>
var CFG={url:"https://jpojckqnhepiuwefyvdr.supabase.co",anonKey:"sb_publishable_EbxhofgF-N88vMSzPIuVNw_eSVdgV8j"};
var sb=supabase.createClient(CFG.url,CFG.anonKey);var PROMOS={};var ROL=null;var DETALLE_ID=null;var VU=null;var CO=null;
var TIPOS=["residencial","terciario","mixto"];var ESTADOS=["suelo","proyecto","licencia","obra","entregada"];
function $(id){return document.getElementById(id);}
function eur(n){return n==null?"-":Number(n).toLocaleString("es-ES")+" EUR";}
function esc(s){s=String(s==null?"":s);return s.split("&").join("&amp;").split("<").join("&lt;").split(">").join("&gt;").split(String.fromCharCode(34)).join("&quot;");}
function kpiCard(l,v,col){return "<div class='card kpi'><div class='l'>"+l+"</div><div class='v' style='"+(col?("color:"+col):"")+"'>"+v+"</div></div>";}
function kpiBox(l,v){return "<div class='kpi'><div class='l'>"+l+"</div><div class='v'>"+v+"</div></div>";}
function ocultarTodo(){["vistaLista","vistaDetalle","vistaContactos","vistaFacturas"].forEach(function(v){$(v).classList.add("hidden");});}
async function init(){var r=await sb.auth.getSession();if(r.data.session){showApp(r.data.session.user);}else{showLogin();}}
function showLogin(){$("login").classList.remove("hidden");$("app").classList.add("hidden");}
async function showApp(user){$("login").classList.add("hidden");$("app").classList.remove("hidden");
var rol="sin rol";var p=await sb.from("perfil").select("rol").eq("id",user.id).maybeSingle();if(p.data&&p.data.rol){rol=p.data.rol;}ROL=rol;
$("who").innerHTML="<b>"+esc(user.email)+"</b><span class='rolepill'>"+esc(rol)+"</span>";if(ROL=="direccion"){var b=$("btnNuevaPromo");b.classList.remove("hidden");b.onclick=nuevaPromocion;}mostrarLista();cargar();}
function mostrarLista(){ocultarTodo();$("vistaLista").classList.remove("hidden");}
async function mostrarContactos(){ocultarTodo();var v=$("vistaContactos");v.classList.remove("hidden");v.innerHTML="<h1>Contactos</h1><div class='sub' id='contSub'>Cargando...</div><div class='section'><h2>Clientes</h2><div id='tblClientes'></div></div><div class='section'><h2>Proveedores</h2><div id='tblProveedores'></div></div>";
var cli=await sb.from("cliente").select("nombre,nif,email,telefono").order("nombre");var prov=await sb.from("proveedor").select("nombre,cif,oficio").order("nombre");
var cd=cli.data||[],pd=prov.data||[];$("contSub").textContent=cd.length+" clientes - "+pd.length+" proveedores";
if(!cd.length){$("tblClientes").innerHTML="<div class='sub'>Sin clientes.</div>";}else{var hc="<table><tr><th>Nombre</th><th>NIF</th><th>Email</th><th>Telefono</th></tr>";cd.forEach(function(c){hc+="<tr><td>"+(esc(c.nombre)||"-")+"</td><td>"+(esc(c.nif)||"-")+"</td><td>"+(esc(c.email)||"-")+"</td><td>"+(esc(c.telefono)||"-")+"</td></tr>";});$("tblClientes").innerHTML=hc+"</table>";}
if(!pd.length){$("tblProveedores").innerHTML="<div class='sub'>Sin proveedores.</div>";}else{var hp="<table><tr><th>Nombre</th><th>CIF</th><th>Oficio</th></tr>";pd.forEach(function(p){hp+="<tr><td>"+(esc(p.nombre)||"-")+"</td><td>"+(esc(p.cif)||"-")+"</td><td>"+(esc(p.oficio)||"-")+"</td></tr>";});$("tblProveedores").innerHTML=hp+"</table>";}}
async function mostrarFacturas(){ocultarTodo();var v=$("vistaFacturas");v.classList.remove("hidden");v.innerHTML="<h1>Facturas (Holded)</h1><div id='facKpis' class='resumen'></div><div class='section'><h2>Detalle</h2><div class='sub' id='facSub'>Cargando...</div><div id='tblFacturas'></div></div>";
var res=await sb.from("factura_holded").select("tipo,doc_numero,fecha,contacto_nombre,total,pagada").order("fecha",{ascending:false});
if(res.error){$("facSub").textContent="Error: "+res.error.message;return;}var f=res.data||[];
if(!f.length){$("facSub").textContent="";$("tblFacturas").innerHTML="<div class='empty'>No hay facturas visibles (solo Direccion).</div>";return;}
function sum(t,pag){return f.filter(function(x){return x.tipo==t&&(pag===undefined||x.pagada===pag);}).reduce(function(s,x){return s+(Number(x.total)||0);},0);}
$("facKpis").innerHTML=kpiCard("Ventas",eur(sum("invoice")),"#16a34a")+kpiCard("Compras",eur(sum("purchase")),"#dc2626")+kpiCard("Pendiente cobro",eur(sum("invoice",false)),"#d97706")+kpiCard("Pendiente pago",eur(sum("purchase",false)),"#d97706");
$("facSub").textContent=f.length+" facturas";
var h="<table><tr><th>Tipo</th><th>Num</th><th>Fecha</th><th>Contacto</th><th class='rt'>Total</th><th class='rt'>Estado</th></tr>";f.forEach(function(x){h+="<tr><td>"+(x.tipo=="invoice"?"Venta":"Compra")+"</td><td>"+(esc(x.doc_numero)||"-")+"</td><td>"+(esc(x.fecha)||"-")+"</td><td>"+(esc(x.contacto_nombre)||"-")+"</td><td class='rt'>"+eur(x.total)+"</td><td class='rt' style='color:"+(x.pagada?"#16a34a":"#d97706")+"'>"+(x.pagada?"Pagada":"Pendiente")+"</td></tr>";});$("tblFacturas").innerHTML=h+"</table>";}
async function cargar(){var grid=$("grid");grid.innerHTML="";
var g=await sb.from("v_resumen_grupo").select("*").maybeSingle();if(g.data){var G=g.data;$("resumenGrupo").innerHTML=kpiCard("Promociones",G.num_promociones)+kpiCard("Valor cartera",eur(G.valor_cartera))+kpiCard("Total vendido",eur(G.total_vendido),"#0f766e")+kpiCard("Margen total",eur(G.margen_total),"#16a34a")+kpiCard("Caja cobrada",eur(G.caja_cobrada),"#16a34a")+kpiCard("Caja pendiente",eur(G.caja_pendiente),"#d97706");}
var com=await sb.from("v_comercializacion_promocion").select("*");var rent=await sb.from("v_rentabilidad_promocion").select("*");
if(com.error||rent.error){$("appSub").textContent="Error: "+((com.error&&com.error.message)||(rent.error&&rent.error.message));return;}
var rById={};(rent.data||[]).forEach(function(r){rById[r.id]=r;});var promos=com.data||[];
if(!promos.length){$("appSub").textContent="";grid.innerHTML="<div class='empty'>No tienes promociones visibles todavia.</div>";return;}
$("appSub").textContent=promos.length+" promocion(es) - haz clic en una para ver el detalle.";
promos.sort(function(a,b){return ((rById[b.id]&&rById[b.id].margen)||0)-((rById[a.id]&&rById[a.id].margen)||0);});
promos.forEach(function(p){PROMOS[p.id]=p;var r=rById[p.id]||{};var estado=p.estado||"-";var eClass=estado.toLowerCase().indexOf("obra")>=0?"eobra":"";var pct=p.pct_colocado!=null?Number(p.pct_colocado):0;
var c=document.createElement("div");c.className="promo";
c.innerHTML="<h3>"+esc(p.nombre)+"</h3><div class='loc'>"+(esc(p.municipio)||"")+" <span class='estado "+eClass+"'>"+esc(estado)+"</span></div><div class='kpis'><div class='kpi'><div class='l'>% Colocado</div><div class='v'>"+pct+"%</div></div><div class='kpi'><div class='l'>Vendidas</div><div class='v'>"+p.vendidas+"/"+p.total_unidades+"</div></div><div class='kpi'><div class='l'>Ingresos</div><div class='v'>"+eur(r.ingresos_contratados)+"</div></div><div class='kpi'><div class='l'>Margen</div><div class='v' style='color:#16a34a'>"+eur(r.margen)+"</div></div></div><div class='track'><div class='fill' style='width:"+Math.min(100,pct)+"%'></div></div>";
c.addEventListener("click",function(){abrirDetalle(p.id);});grid.appendChild(c);});}
function abrirDetalle(id){var p=PROMOS[id];if(!p)return;DETALLE_ID=id;ocultarTodo();var d=$("vistaDetalle");d.classList.remove("hidden");
d.innerHTML="<button class='back' id='volver'>&larr; Volver</button><h1>"+esc(p.nombre)+"</h1><div class='sub'>"+(esc(p.municipio)||"")+" - "+(esc(p.estado)||"")+"</div><div id='detKpis' class='kpis' style='grid-template-columns:repeat(4,1fr)'></div><div class='section'><h2>Unidades</h2><div id='detUnidades'>Cargando...</div></div><div class='section'><h2>Tesoreria</h2><div id='detTeso'>Cargando...</div></div><div class='section'><h2>Avance de obra</h2><div id='detObra'>Cargando...</div></div><div class='section'><h2>Documentos</h2><div id='detDocs'>Cargando...</div></div>";
$("volver").addEventListener("click",mostrarLista);cargarDetalle(id);}
async function cargarDetalle(id){
var rent=await sb.from("v_rentabilidad_promocion").select("*").eq("id",id).maybeSingle();var com=await sb.from("v_comercializacion_promocion").select("*").eq("id",id).maybeSingle();var teso=await sb.from("v_tesoreria_promocion").select("*").eq("id",id).maybeSingle();
var r=rent.data||{},cm=com.data||{},t=teso.data||{};
$("detKpis").innerHTML=kpiBox("% Colocado",(cm.pct_colocado!=null?cm.pct_colocado:0)+"%")+kpiBox("Ingresos",eur(r.ingresos_contratados))+kpiBox("Coste real",eur(r.coste_real))+kpiBox("Margen",eur(r.margen));
var fases=await sb.from("fase").select("id").eq("promocion_id",id);var faseIds=(fases.data||[]).map(function(f){return f.id;});
var ud=[];if(faseIds.length){var uni=await sb.from("unidad").select("id,referencia,tipo,estado,precio_venta,contrato_venta(cliente(nombre))").in("fase_id",faseIds).order("referencia");ud=uni.data||[];}
var puedeVender=(ROL=="direccion"||ROL=="comercial");
if(!ud.length){$("detUnidades").innerHTML="<div class='sub'>Sin unidades.</div>";}else{var hu="<table><tr><th>Ref.</th><th>Tipo</th><th>Estado</th><th class='rt'>Precio</th><th>Comprador</th><th></th></tr>";ud.forEach(function(u){var comp="-";if(u.contrato_venta&&u.contrato_venta.length&&u.contrato_venta[0].cliente){comp=esc(u.contrato_venta[0].cliente.nombre);}var accion="";if(puedeVender&&u.estado!="vendida"){accion="<button class='mini' data-id='"+u.id+"' data-precio='"+(u.precio_venta||0)+"' data-ref='"+esc(u.referencia)+"'>Vender</button>";}hu+="<tr><td>"+esc(u.referencia)+"</td><td>"+esc(u.tipo)+"</td><td>"+u.estado+"</td><td class='rt'>"+eur(u.precio_venta)+"</td><td>"+comp+"</td><td class='rt'>"+accion+"</td></tr>";});$("detUnidades").innerHTML=hu+"</table>";var bs=$("detUnidades").querySelectorAll("button.mini");bs.forEach(function(b){b.addEventListener("click",function(){registrarVenta(b.getAttribute("data-id"),Number(b.getAttribute("data-precio")),b.getAttribute("data-ref"));});});}
if(t&&t.total_hitos!=null){$("detTeso").innerHTML="<table><tr><th>Contratado en hitos</th><th>Cobrado</th><th>Pendiente</th><th class='rt'>% cobrado</th></tr><tr><td>"+eur(t.total_hitos)+"</td><td style='color:#16a34a'>"+eur(t.cobrado)+"</td><td style='color:#d97706'>"+eur(t.pendiente_cobro)+"</td><td class='rt'>"+(t.pct_cobrado!=null?t.pct_cobrado:0)+"%</td></tr></table>";}else{$("detTeso").innerHTML="<div class='sub'>Sin hitos de pago registrados.</div>";}
var puedeCert=(ROL=="direccion"||ROL=="obra");
var obra=await sb.from("contrato_obra").select("id,descripcion,importe_adjudicado,estado,certificacion(importe)").eq("promocion_id",id);var od=obra.data||[];
if(!od.length){$("detObra").innerHTML="<div class='sub'>Sin contratos de obra.</div>";}else{var ho="<table><tr><th>Contrato</th><th class='rt'>Adjudicado</th><th class='rt'>Certificado</th><th class='rt'>%</th><th></th></tr>";od.forEach(function(o){var cert=0;(o.certificacion||[]).forEach(function(c){cert+=Number(c.importe)||0;});var pct=o.importe_adjudicado?Math.round(1000*cert/o.importe_adjudicado)/10:0;var ac=puedeCert?"<button class='mini' data-co='"+o.id+"'>+ Cert.</button>":"";ho+="<tr><td>"+esc(o.descripcion)+"</td><td class='rt'>"+eur(o.importe_adjudicado)+"</td><td class='rt'>"+eur(cert)+"</td><td class='rt'>"+pct+"%</td><td class='rt'>"+ac+"</td></tr>";});$("detObra").innerHTML=ho+"</table>";var cb=$("detObra").querySelectorAll("button.mini");cb.forEach(function(b){b.addEventListener("click",function(){registrarCertificacion(b.getAttribute("data-co"));});});}
var docs=await sb.from("documento").select("tipo,nombre").eq("promocion_id",id);var dd=docs.data||[];
if(!dd.length){$("detDocs").innerHTML="<div class='sub'>Sin documentos.</div>";}else{var hx="<table><tr><th>Tipo</th><th>Nombre</th></tr>";dd.forEach(function(x){hx+="<tr><td>"+esc(x.tipo)+"</td><td>"+esc(x.nombre)+"</td></tr>";});$("detDocs").innerHTML=hx+"</table>";}}
function openModal(h){$("modalBody").innerHTML=h;$("modal").classList.remove("hidden");}
function closeModal(){$("modal").classList.add("hidden");$("modalBody").innerHTML="";}
async function nuevaPromocion(){var s=await sb.from("sociedad").select("id,nombre").order("nombre");var so="";(s.data||[]).forEach(function(x){so+="<option value='"+x.id+"'>"+esc(x.nombre)+"</option>";});var to="";TIPOS.forEach(function(t){to+="<option>"+t+"</option>";});var eo="";ESTADOS.forEach(function(e){eo+="<option>"+e+"</option>";});
openModal("<h2>Nueva promocion</h2><div class='field'><label>Nombre</label><input id='pNombre'></div><div class='field'><label>Municipio</label><input id='pMun'></div><div class='field'><label>Tipo</label><select id='pTipo'>"+to+"</select></div><div class='field'><label>Estado</label><select id='pEstado'>"+eo+"</select></div><div class='field'><label>Presupuesto (EUR)</label><input id='pPres' type='number'></div><div class='field'><label>Sociedad</label><select id='pSoc'><option value=''>- crear nueva -</option>"+so+"</select></div><div class='field'><label>Nueva sociedad (si no eliges una)</label><input id='pSocNueva'></div><div id='pMsg' class='msg'></div><div class='modal-actions'><button class='linkbtn' id='pCancel'>Cancelar</button><button class='btn btn-sm' id='pOk'>Crear</button></div>");
$("pCancel").addEventListener("click",closeModal);$("pOk").addEventListener("click",confirmarPromocion);}
async function confirmarPromocion(){var nombre=$("pNombre").value.trim();var m=$("pMsg");m.className="msg";if(!nombre){m.className="msg err";m.textContent="Indica el nombre.";return;}m.textContent="Guardando...";
var socId=$("pSoc").value;var nueva=$("pSocNueva").value.trim();
if(nueva){var r=await sb.from("sociedad").upsert({nombre:nueva},{onConflict:"nombre"}).select("id").single();if(r.error){m.className="msg err";m.textContent=r.error.message;return;}socId=r.data.id;}
if(!socId){m.className="msg err";m.textContent="Elige o crea una sociedad.";return;}
var ins=await sb.from("promocion").insert({sociedad_id:socId,nombre:nombre,municipio:$("pMun").value.trim()||null,tipo:$("pTipo").value,estado:$("pEstado").value,presupuesto_total:Number($("pPres").value)||null});
if(ins.error){m.className="msg err";m.textContent=ins.error.message;return;}closeModal();mostrarLista();cargar();}
async function registrarVenta(unidadId,precio,ref){VU=unidadId;var cl=await sb.from("cliente").select("id,nombre").order("nombre");var opts="";(cl.data||[]).forEach(function(c){opts+="<option value='"+c.id+"'>"+esc(c.nombre)+"</option>";});
openModal("<h2>Registrar venta - "+esc(ref)+"</h2><div class='field'><label>Cliente</label><select id='mCliente'>"+opts+"</select></div><div class='field'><label>Precio (EUR)</label><input id='mPrecio' type='number' value='"+precio+"'></div><div id='mMsg' class='msg'></div><div class='modal-actions'><button class='linkbtn' id='mCancel'>Cancelar</button><button class='btn btn-sm' id='mOk'>Confirmar venta</button></div>");
$("mCancel").addEventListener("click",closeModal);$("mOk").addEventListener("click",confirmarVenta);}
async function confirmarVenta(){var clienteId=$("mCliente").value;var precio=Number($("mPrecio").value);var m=$("mMsg");m.className="msg";m.textContent="Guardando...";if(!clienteId){m.className="msg err";m.textContent="Selecciona un cliente.";return;}
var ins=await sb.from("contrato_venta").insert({unidad_id:VU,cliente_id:clienteId,precio_total:precio,fecha_firma:new Date().toISOString().slice(0,10),estado:"escriturado"});
if(ins.error){m.className="msg err";m.textContent=ins.error.message;return;}closeModal();abrirDetalle(DETALLE_ID);}
async function registrarCertificacion(coId){CO=coId;openModal("<h2>Nueva certificacion de obra</h2><div class='field'><label>Numero</label><input id='cNum' type='number' value='1'></div><div class='field'><label>Importe (EUR)</label><input id='cImp' type='number' placeholder='0'></div><div class='field'><label>Fecha</label><input id='cFecha' type='date' value='"+new Date().toISOString().slice(0,10)+"'></div><div id='cMsg' class='msg'></div><div class='modal-actions'><button class='linkbtn' id='cCancel'>Cancelar</button><button class='btn btn-sm' id='cOk'>Guardar</button></div>");$("cCancel").addEventListener("click",closeModal);$("cOk").addEventListener("click",confirmarCert);}
async function confirmarCert(){var importe=Number($("cImp").value);var m=$("cMsg");m.className="msg";if(!importe){m.className="msg err";m.textContent="Indica un importe.";return;}m.textContent="Guardando...";var ins=await sb.from("certificacion").insert({contrato_obra_id:CO,numero:Number($("cNum").value)||null,importe:importe,fecha:$("cFecha").value});if(ins.error){m.className="msg err";m.textContent=ins.error.message;return;}closeModal();abrirDetalle(DETALLE_ID);}
$("loginForm").addEventListener("submit",async function(e){e.preventDefault();var m=$("loginMsg");m.className="msg";m.textContent="Entrando...";var res=await sb.auth.signInWithPassword({email:$("email").value.trim(),password:$("password").value});if(res.error){m.className="msg err";m.textContent=res.error.message;return;}m.textContent="";showApp(res.data.user);});
$("logout").addEventListener("click",async function(){await sb.auth.signOut();showLogin();});
$("navPromos").addEventListener("click",mostrarLista);$("navContactos").addEventListener("click",mostrarContactos);$("navFacturas").addEventListener("click",mostrarFacturas);
init();
</script></body></html>`;
Deno.serve(() => new Response(HTML, { headers: { "content-type": "text/html; charset=utf-8" } }));
