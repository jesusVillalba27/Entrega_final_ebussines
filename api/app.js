'use strict';
// =============================================================================
//  app.js — API REST | Laboratorio Capstone Semanas 4+5
//  Proyecto: huella de carbono verificable en una fábrica de calzado (Sincelejo)
//  Hyperledger Fabric 2.5 + Express + fabric-gateway
//  Base: app.js del profesor Jaider Reyes Herazo (mismas rutas) + 2 rutas del proyecto
//
//  Uso:   MSP_ID=Org1MSP node app.js          -> proveedores, puerto 3000
//         MSP_ID=Org2MSP PORT=3001 node app.js -> fábrica, puerto 3001
//  Opcional: API_KEY=clave  -> exige la cabecera  x-api-key: clave  (ver Pregunta 17)
// =============================================================================

const express = require('express');
const cors    = require('cors');
const morgan  = require('morgan');
const crypto  = require('crypto');
const fs      = require('fs');
const path    = require('path');
const grpc    = require('@grpc/grpc-js');
const { connect, signers } = require('@hyperledger/fabric-gateway');

const app  = express();
const PORT = process.env.PORT || (process.env.MSP_ID === 'Org2MSP' ? 3001 : 3000);

// ── CONFIGURACIÓN FABRIC ─────────────────────────────────────────────────────
const FABRIC_DIR  = process.env.FABRIC_DIR || path.join(process.env.HOME || '.', 'fabric', 'fabric-samples');
const NETWORK_DIR = path.join(FABRIC_DIR, 'test-network');
const CHANNEL     = 'supplychannel';
const CHAINCODE   = 'supplycc';
const MSP_ID      = process.env.MSP_ID || 'Org1MSP';
const ES_ORG2     = MSP_ID === 'Org2MSP';
const PEER_PORT   = ES_ORG2 ? '9051' : '7051';
const PEER_HOST   = ES_ORG2 ? 'peer0.org2.example.com' : 'peer0.org1.example.com';
const ORG_DOM     = ES_ORG2 ? 'org2.example.com' : 'org1.example.com';
const ORG_PATH    = path.join(NETWORK_DIR, 'organizations/peerOrganizations', ORG_DOM);
const USER        = `User1@${ORG_DOM}`;
const API_KEY     = process.env.API_KEY || '';

// ── MIDDLEWARE ────────────────────────────────────────────────────────────────
app.use(cors());
app.use(express.json());
if (process.env.NODE_ENV !== 'test') app.use(morgan('dev'));

// Seguridad básica opcional: si API_KEY está definida, todas las rutas (menos /health) la exigen.
app.use((req, res, next) => {
  if (!API_KEY || req.path === '/health') return next();
  if (req.get('x-api-key') === API_KEY) return next();
  return res.status(401).json({ error: 'Falta o es incorrecta la cabecera x-api-key' });
});

// ── CONTRATO (instancia global) ───────────────────────────────────────────────
let contract;
const setContract = (c) => { contract = c; };

function primerArchivo(dir) {
  const files = fs.readdirSync(dir).filter((f) => !f.startsWith('.'));
  if (files.length === 0) throw new Error(`No hay archivos en ${dir}`);
  return path.join(dir, files[0]);
}

async function conectarFabric() {
  const tlsCert = fs.readFileSync(path.join(ORG_PATH, `tlsca/tlsca.${ORG_DOM}-cert.pem`));
  const certPem = fs.readFileSync(primerArchivo(path.join(ORG_PATH, `users/${USER}/msp/signcerts`)));
  const keyPem  = fs.readFileSync(primerArchivo(path.join(ORG_PATH, `users/${USER}/msp/keystore`)));

  const client = new grpc.Client(
    `localhost:${PEER_PORT}`,
    grpc.credentials.createSsl(tlsCert),
    { 'grpc.ssl_target_name_override': PEER_HOST }
  );

  const gateway = connect({
    client,
    identity: { mspId: MSP_ID, credentials: certPem },
    signer: signers.newPrivateKeySigner(crypto.createPrivateKey(keyPem)),
    evaluateOptions:     () => ({ deadline: Date.now() + 5000 }),
    endorseOptions:      () => ({ deadline: Date.now() + 15000 }),
    submitOptions:       () => ({ deadline: Date.now() + 5000 }),
    commitStatusOptions: () => ({ deadline: Date.now() + 60000 }),
  });

  const network = await gateway.getNetwork(CHANNEL);
  contract = network.getContract(CHAINCODE);
  escucharEventos(network);
  console.log('\n✅ Conectado a Fabric');
  console.log(`   Org:       ${MSP_ID}`);
  console.log(`   Canal:     ${CHANNEL}`);
  console.log(`   Chaincode: ${CHAINCODE}\n`);
}

// Los eventos del chaincode (SetEvent) llegan aquí: ProductoRegistrado, CantidadActualizada...
// En producción alimentarían el sistema ESG / ERP de la fábrica (ver Pregunta 8).
async function escucharEventos(network) {
  try {
    const events = await network.getChaincodeEvents(CHAINCODE);
    for await (const ev of events) {
      let resumen = '';
      try { resumen = JSON.stringify(JSON.parse(Buffer.from(ev.payload).toString())).slice(0, 200); } catch (e) { /* payload no JSON */ }
      console.log(`📣 Evento ${ev.eventName} (tx ${ev.transactionId.slice(0, 10)}…) ${resumen}`);
    }
  } catch (err) {
    console.error('⚠️  Se detuvo la escucha de eventos:', err.message);
  }
}

// ── HELPERS ───────────────────────────────────────────────────────────────────
const decode = (result) => {
  const txt = Buffer.from(result).toString();
  return txt === '' ? null : JSON.parse(txt);
};
const wrap = (fn) => (req, res, next) => Promise.resolve(fn(req, res, next)).catch(next);
const esEntero = (v) => v !== undefined && v !== null && v !== '' && Number.isInteger(Number(v));
const sha256 = (txt) => crypto.createHash('sha256').update(String(txt), 'utf8').digest('hex');

// Extrae el mensaje legible del error de Fabric (el de la regla de negocio del chaincode).
function mensajeDe(err) {
  const det = Array.isArray(err.details) && err.details.length ? err.details[0].message : '';
  const bruto = det || err.message || 'Error interno';
  const m = bruto.match(/chaincode response \d+, (.*)$/s);
  return (m ? m[1] : bruto).trim();
}

// =============================================================================
//  RUTAS
// =============================================================================

app.get('/', (req, res) => {
  res.json({
    nombre: 'API Huella de carbono — Hyperledger Fabric',
    estado: 'activa',
    org: MSP_ID,
    canal: CHANNEL,
    chaincode: CHAINCODE,
    endpoints: {
      salud: 'GET /health',
      productos: 'GET /productos',
      producto: 'GET /productos/:id',
      historial: 'GET /productos/:id/historial',
      porEstado: 'GET /productos/estado/:estado',
      registrar: 'POST /productos',
      transferir: 'PUT /productos/:id/transferir',
      confirmar: 'PUT /productos/:id/confirmar',
      cantidad: 'PUT /productos/:id/cantidad',
      entregar: 'PUT /productos/:id/entregar',
      verificar: 'POST /productos/:id/verificar',
      huella: 'GET /huella/:orden?b=&c=&d=&n=',
    },
  });
});

// GET /health
app.get('/health', (req, res) => {
  res.json({
    status: 'ok',
    org: MSP_ID,
    canal: CHANNEL,
    chaincode: CHAINCODE,
    conectado: Boolean(contract),
    timestamp: new Date().toISOString(),
  });
});

// GET /productos — todos los lotes (evaluateTransaction: solo lectura, no genera bloque)
app.get('/productos', wrap(async (req, res) => {
  res.json(decode(await contract.evaluateTransaction('ConsultarTodos')) || []);
}));

// GET /productos/estado/:estado — filtra por REGISTRADO | EN_TRANSITO | RECIBIDO | ENTREGADO
app.get('/productos/estado/:estado', wrap(async (req, res) => {
  const estado = req.params.estado.toUpperCase();
  res.json(decode(await contract.evaluateTransaction('ConsultarPorEstado', estado)) || []);
}));

// GET /productos/:id
app.get('/productos/:id', wrap(async (req, res) => {
  res.json(decode(await contract.evaluateTransaction('LeerProducto', req.params.id)));
}));

// GET /productos/:id/historial — historial inmutable del ledger
app.get('/productos/:id/historial', wrap(async (req, res) => {
  res.json(decode(await contract.evaluateTransaction('ObtenerHistorial', req.params.id)) || []);
}));

// POST /productos — el proveedor declara un lote con su huella (Emisiones = DA × FE)
app.post('/productos', wrap(async (req, res) => {
  const { id, nombre, cantidad, unidad, origen, destino, factorId, factorG, hashEvidencia, evidenciaTexto } = req.body;
  if (!id || !nombre || !unidad || !origen || !destino || !factorId) {
    return res.status(400).json({ error: 'Campos requeridos: id, nombre, cantidad, unidad, origen, destino, factorId, factorG y hashEvidencia (o evidenciaTexto)' });
  }
  if (!esEntero(cantidad) || Number(cantidad) <= 0) {
    return res.status(400).json({ error: 'cantidad debe ser un entero mayor a 0' });
  }
  if (!esEntero(factorG) || Number(factorG) <= 0) {
    return res.status(400).json({ error: 'factorG debe ser un entero mayor a 0 (g CO2e por unidad)' });
  }
  const hash = hashEvidencia || (evidenciaTexto ? sha256(evidenciaTexto) : '');
  if (!hash) {
    return res.status(400).json({ error: 'Envíe hashEvidencia (SHA-256) o evidenciaTexto para calcularlo' });
  }
  await contract.submitTransaction('RegistrarProducto',
    id, nombre, String(cantidad), unidad, origen, destino, factorId, String(factorG), hash);
  res.status(201).json({
    message: `Producto ${id} registrado en el ledger`,
    huellaG: Number(cantidad) * Number(factorG),
    hashEvidencia: hash,
    timestamp: new Date().toISOString(),
  });
}));

// PUT /productos/:id/transferir — el proveedor despacha el lote a la fábrica
app.put('/productos/:id/transferir', wrap(async (req, res) => {
  const { nuevoPropietario, detalle } = req.body;
  if (!nuevoPropietario || !detalle) {
    return res.status(400).json({ error: 'Campos requeridos: nuevoPropietario, detalle' });
  }
  await contract.submitTransaction('TransferirProducto', req.params.id, nuevoPropietario, detalle);
  res.json({ message: `Producto ${req.params.id} transferido a ${nuevoPropietario}` });
}));

// PUT /productos/:id/confirmar — la fábrica confirma con su báscula la cantidad recibida
app.put('/productos/:id/confirmar', wrap(async (req, res) => {
  const { cantidadRecibida, detalle } = req.body;
  if (!esEntero(cantidadRecibida) || Number(cantidadRecibida) <= 0) {
    return res.status(400).json({ error: 'cantidadRecibida debe ser un entero mayor a 0' });
  }
  await contract.submitTransaction('ConfirmarRecepcion',
    req.params.id, String(cantidadRecibida), detalle || 'Recepción confirmada');
  res.json({ message: `Recepción del producto ${req.params.id} confirmada` });
}));

// PUT /productos/:id/cantidad — actualiza la cantidad (recalcula la huella DA × FE)
app.put('/productos/:id/cantidad', wrap(async (req, res) => {
  const { cantidad, motivo } = req.body;
  if (!esEntero(cantidad) || !motivo) {
    return res.status(400).json({ error: 'Campos requeridos: cantidad (entero) y motivo' });
  }
  await contract.submitTransaction('ActualizarCantidad', req.params.id, String(cantidad), motivo);
  res.json({ message: `Cantidad del producto ${req.params.id} actualizada a ${cantidad}` });
}));

// PUT /productos/:id/entregar — la fábrica entrega el lote a una orden de producción
app.put('/productos/:id/entregar', wrap(async (req, res) => {
  const { clienteFinal, evidencia } = req.body;
  if (!clienteFinal || !evidencia) {
    return res.status(400).json({ error: 'Campos requeridos: clienteFinal (orden de producción), evidencia' });
  }
  await contract.submitTransaction('MarcarEntregado', req.params.id, clienteFinal, evidencia);
  res.json({ message: `Producto ${req.params.id} marcado como ENTREGADO a ${clienteFinal}` });
}));

// POST /productos/:id/verificar — ¿el archivo de evidencia sigue siendo el original?
//   body: { "hash": "<sha256>" }  o  { "evidenciaTexto": "<contenido del documento>" }
app.post('/productos/:id/verificar', wrap(async (req, res) => {
  const hash = req.body.hash || (req.body.evidenciaTexto ? sha256(req.body.evidenciaTexto) : '');
  if (!hash) return res.status(400).json({ error: 'Envíe hash (SHA-256) o evidenciaTexto' });
  const result = await contract.evaluateTransaction('VerificarEvidencia', req.params.id, hash);
  res.json({ id: req.params.id, hashProbado: hash, coincide: decode(result) === true });
}));

// GET /huella/:orden?b=&c=&d=&n=  — HC por par = (A + B + C + D) ÷ N  (A sale del ledger)
app.get('/huella/:orden', wrap(async (req, res) => {
  const { b = '0', c = '0', d = '0', n } = req.query;
  if (![b, c, d].every(esEntero) || !esEntero(n) || Number(n) <= 0) {
    return res.status(400).json({ error: 'Parámetros: b, c, d (gramos CO2e, enteros) y n (pares buenos, entero > 0)' });
  }
  const result = await contract.evaluateTransaction('CalcularHuellaPar', req.params.orden, String(b), String(c), String(d), String(n));
  res.json(decode(result));
}));

// ── MANEJO DE ERRORES ─────────────────────────────────────────────────────────
app.use((err, req, res, next) => { // eslint-disable-line no-unused-vars
  const msg = mensajeDe(err);
  console.error('❌', msg);
  let status = 500;
  if (/no encontrado/i.test(msg)) status = 404;
  else if (/no tiene permiso|solo el propietario|solo el destinatario/i.test(msg)) status = 403;
  else if (/ya existe|no está|solo productos|no puede transferirse|no se puede actualizar|no tiene lotes/i.test(msg)) status = 409;
  else if (/inválid|debe ser|obligatorio|supera|solo se transfieren|igual a la actual/i.test(msg)) status = 400;
  res.status(status).json({ error: msg });
});

app.use((req, res) => res.status(404).json({ error: `Ruta no encontrada: ${req.method} ${req.url}` }));

// =============================================================================
//  ARRANQUE
// =============================================================================
async function main() {
  try {
    await conectarFabric();
    app.listen(PORT, () => {
      console.log(`🚀 API Huella de carbono (${MSP_ID}) en http://localhost:${PORT}`);
      console.log('\n📋 Endpoints disponibles:');
      console.log('   GET    /health');
      console.log('   GET    /productos');
      console.log('   GET    /productos/:id');
      console.log('   GET    /productos/:id/historial');
      console.log('   GET    /productos/estado/:estado');
      console.log('   POST   /productos');
      console.log('   PUT    /productos/:id/transferir');
      console.log('   PUT    /productos/:id/confirmar');
      console.log('   PUT    /productos/:id/cantidad');
      console.log('   PUT    /productos/:id/entregar');
      console.log('   POST   /productos/:id/verificar');
      console.log('   GET    /huella/:orden?b=&c=&d=&n=\n');
    });
  } catch (err) {
    console.error('❌ Error conectando a Fabric:', err);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { app, setContract, mensajeDe };
