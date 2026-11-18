# Laboratorio Capstone · Semanas 4 + 5 — Guía paso a paso
**Proyecto:** Huella de carbono verificable en una fábrica de calzado (Sincelejo, Sucre) · Hyperledger Fabric 2.5
**Equipo:** Jesús Villalba · Jesús Madera · **Terminal:** Git Bash (los comandos de Fabric se ejecutan dentro de WSL/Ubuntu)

> Los comandos y las capturas deben salir de **su** equipo (la hoja lo exige en la declaración de entrega). Esta guía les dice qué ejecutar y qué mirar; los outputs, TxIDs y alturas serán los suyos.

---

## 0. Qué cambió respecto al laboratorio del profesor

El profesor usa un ejemplo genérico de frutas (PROD-001, Mango, SuperAlimentos). Ustedes lo trasladan a su problema: el "producto" es un **lote de material con su huella de carbono declarada**, que va del **proveedor (Org1)** a la **fábrica (Org2)** y termina entregado a una **orden de producción**.

| En la hoja del profesor | En su proyecto |
|---|---|
| PROD-001 / PROD-002 / PROD-003 (InitLedger) | `LOTE-CUE-001` (cuero), `LOTE-SUE-001` (suelas), `LOTE-ADH-001` (adhesivo) |
| PROD-010 (mango, 800 kg) | `LOTE-CUE-<hora>`: 200 m² de cuero (el script le pone un sufijo con la hora para que no choque) |
| Cliente final "SuperAlimentos S.A." | Orden de producción `OP-<hora>` |
| `temperatura` | Se reemplaza por **factor de emisión** y **hash de la evidencia** |
| Org1 = productor, Org2 = distribuidor | Org1 = **proveedores**, Org2 = **fábrica de calzado** |
| Cantidad × nada | **Emisiones = DA × FE** (huella en gramos de CO2e, enteros) |

Cambios que se hicieron sobre los archivos del profesor:

* `chaincode/supplycc.go` + `calculos.go`: mismas 10 funciones y mismos 4 estados, más `VerificarEvidencia` y `CalcularHuellaPar`. Usa la hora de la transacción (`GetTxTimestamp`) en vez de `time.Now()`, valida roles por MSP (Org1 registra/transfiere, Org2 confirma/entrega) y recalcula la huella cuando cambia la cantidad.
* `api/app.js`: las mismas rutas, más `POST /productos/:id/verificar` y `GET /huella/:orden`, validación de entradas, errores claros (400/403/404/409), `API_KEY` opcional.
* `deploy-supplycc.sh`: usa **CCaaS** (chaincode como servicio) porque el build clásico del peer falla con Docker Desktop (el "broken pipe" que ya vieron en la Semana 3), calcula solo la secuencia y aplica la política `AND('Org1MSP.peer','Org2MSP.peer')`.
* `flujo-completo.sh`: las 8 etapas del profesor + 2 etapas propias del proyecto (completar la orden con suelas, adhesivo y cajas, y calcular **HC por par**).
* Nuevos: `lib.sh`, `verificar-laboratorio.sh`, `pruebas-api.sh`.

Resultado que debe darles el cálculo final (igual que en la Semana 3): **9,749 kg CO2e por par** = (A + B + C + D) ÷ N = (suma de los 4 lotes + 110 000 + 74 750 + 27 000) ÷ 192.

---

## 1. Preparación (una sola vez)

### 1.1 Entrar a Ubuntu desde Git Bash
Fabric solo corre en Linux. Desde Git Bash:

```bash
wsl
```
El prompt cambia a `usuario@PC:~$`. **Todo lo que sigue se escribe ahí.** Si escriben `peer` en Git Bash puro, no lo va a encontrar.

### 1.2 Copiar el paquete a Ubuntu
Descarguen `semana4_huella.zip` en Windows (por ejemplo en Descargas) y, dentro de WSL:

```bash
sudo apt-get install -y unzip        # solo si unzip no existe
mkdir -p ~/fabric/semana4
cd ~/fabric/semana4
unzip -o /mnt/c/Users/TU_USUARIO/Downloads/semana4_huella.zip
# si queda dentro de una subcarpeta, entren a ella:  cd semana4_huella  (ls para ver)
ls
```
Cambien `TU_USUARIO` por su usuario de Windows (`ls /mnt/c/Users` lo muestra).

### 1.3 Quitar saltos de línea de Windows y dar permisos
Si algún archivo pasó por Windows puede traer `\r` y los scripts fallan con `$'\r': command not found`:

```bash
cd ~/fabric/semana4
find . -name "*.sh" -exec sed -i 's/\r$//' {} \;
chmod +x *.sh
```

### 1.4 Herramientas
```bash
go version      # debe ser 1.21 o superior
node -v         # 18 o superior
docker ps       # no debe dar error (Docker Desktop abierto)
ls ~/fabric/fabric-samples/bin/peer
```
Todo eso ya lo usaron en la Semana 3. Si algo falta, repitan el `setup-network.sh` de esa semana.

---

## 2. Red de la Semana 3 arriba

```bash
docker ps --format 'table {{.Names}}\t{{.Status}}'
```
Deben verse al menos `peer0.org1.example.com`, `peer0.org2.example.com`, `orderer.example.com`, `couchdb0`, `couchdb1` y las tres CAs. Si **no** hay contenedores, levanten la red como en la Semana 3:

```bash
cd ~/fabric/semana3/oficial
./setup-network.sh
```
(Debe quedar creado `~/fabric/fabric-samples/test-network/set-env-org1.sh` y `set-env-org2.sh`; los scripts de esta semana los usan.)

Revisen el estado:
```bash
cd ~/fabric/semana4
./verificar-laboratorio.sh
```
Todavía fallarán las dos líneas de las APIs (3000 y 3001): es normal.

> **Importante (Semana 3 → Semana 4):** en la Semana 3 ya desplegaron un `supplycc` con otro código. El script lo detecta y hace una **actualización** (secuencia siguiente, versión 2.0). Ese chaincode tiene otro esquema, así que los scripts de la Semana 3 (registrar factores, KPI...) dejan de funcionar con la versión nueva. Los datos viejos siguen en el ledger, pero esta función de consulta no los lista (filtra `docType = producto`). Si prefieren empezar desde cero, bajen y levanten la red con el `setup-network.sh`; el despliegue funciona igual.

---

## 3. PARTE 1 — Despliegue del chaincode

### 3.1 Antes de ejecutar: respondan las preguntas 1, 2 y 3
Abran `chaincode/supplycc.go` y léanlo (struct `Producto`, constantes de estado, funciones). Las respuestas borrador están en el documento `laboratorio_capstone_BORRADOR.docx`; **reescríbanlas con sus palabras**.

### 3.2 Desplegar
```bash
cd ~/fabric/semana4
./deploy-supplycc.sh
```
Qué hace y qué buscar en la pantalla (equivalencia con los 10 pasos de la hoja):

| Paso de la hoja | Dónde se ve |
|---|---|
| 0 Red y canal | "✓ Red Fabric activa" y "✓ Canal supplychannel disponible" |
| 1 package · 2 install Org1 · 3 install Org2 · 4 Package ID | Salida de `network.sh deployCCAAS` (también en `evidencias/deploy.log`) y el bloque "Package ID instalado" |
| 5 approve Org1 · 6 approve Org2 | Líneas "approved" de cada organización en el log |
| 7 checkcommitreadiness | Bloque "Aprobaciones": `Org1MSP: true`, `Org2MSP: true` |
| 8 commit | "Definición confirmada en el canal" (`querycommitted`) |
| 9 InitLedger | "PASO 9" con ✓ |
| 10 ConsultarTodos | "PASO 10": los 3 lotes en JSON |

Marquen en la tabla de la hoja solo los pasos que **vean** completados.

**Captura 1:** el PASO 10 (ConsultarTodos con los 3 lotes). Incluyan el nombre de la pareja visible (el script lo imprime en la primera línea; si la captura no lo muestra, tomen la pantalla completa).

Opcional pero recomendado, **Captura extra:** el bloque `checkcommitreadiness` con `true/true`. La hoja trae 7 recuadros de captura y la rúbrica habla de 8; esta sería la octava.

### 3.3 Pregunta 5 y CouchDB
```bash
source ~/fabric/fabric-samples/test-network/set-env-org1.sh
peer chaincode query -C supplychannel -n supplycc -c '{"function":"ConsultarTodos","Args":[]}'
```
En el navegador de Windows abran **http://localhost:5984/_utils** (usuario `admin`, contraseña `adminpw`). La base de datos del chaincode se llama **`supplychannel_supplycc`** (la hoja dice `supplychannel_`). Entren y abran los documentos `LOTE-CUE-001`, etc.

**Captura 2:** CouchDB con la base `supplychannel_supplycc` y los lotes del InitLedger.

---

## 4. PARTE 2 — ActualizarCantidad en vivo

Respondan 6, 7 y 8. Luego, con las variables de Org1 activas (siguen del paso anterior; si abrieron otra terminal repitan `source ...set-env-org1.sh`):

**Paso 1 — cantidad actual** (usen `LOTE-CUE-001` en lugar de `PROD-001`)
```bash
peer chaincode query -C supplychannel -n supplycc \
  -c '{"function":"LeerProducto","Args":["LOTE-CUE-001"]}'
```
Anoten `cantidad` (190) y `huellaG` (1 140 000).

**Paso 2 — actualizar (190 → 180 m²)**
```bash
peer chaincode invoke -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile $ORDERER_CA \
  -C supplychannel -n supplycc \
  --peerAddresses localhost:7051 --tlsRootCertFiles $PEER1_TLS \
  --peerAddresses localhost:9051 --tlsRootCertFiles $PEER2_TLS \
  -c '{"function":"ActualizarCantidad","Args":["LOTE-CUE-001","180","Medicion real del area cortable: 180 m2"]}'
```
Debe terminar con `status:200`.

**Paso 3 — verificar** (esperen 2 segundos)
```bash
peer chaincode query -C supplychannel -n supplycc \
  -c '{"function":"LeerProducto","Args":["LOTE-CUE-001"]}'
```
Anoten la nueva `cantidad` (180), el `huellaG` (1 080 000: se recalculó) y el `txId`.

**Paso 4 — historial**
```bash
peer chaincode query -C supplychannel -n supplycc \
  -c '{"function":"ObtenerHistorial","Args":["LOTE-CUE-001"]}' | python3 -m json.tool
```
**Captura 3:** el historial con la entrada de actualización de cantidad.

### Pregunta 9 — la validación de propietario (Org2 intenta cambiar un lote de Org1)
```bash
source ~/fabric/fabric-samples/test-network/set-env-org2.sh
peer chaincode invoke -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile $ORDERER_CA \
  -C supplychannel -n supplycc \
  --peerAddresses localhost:7051 --tlsRootCertFiles $PEER1_TLS \
  --peerAddresses localhost:9051 --tlsRootCertFiles $PEER2_TLS \
  -c '{"function":"ActualizarCantidad","Args":["LOTE-CUE-001","999","Intento de Org2"]}'
```
Debe **fallar** con un mensaje parecido a `solo el propietario (Org1MSP) puede actualizar la cantidad — llamante: Org2MSP`. Copien el error real en la hoja. Comprueben también que el lote sigue en 180 (no hubo cambio).

Vuelvan a Org1 para lo que sigue:
```bash
source ~/fabric/fabric-samples/test-network/set-env-org1.sh
```

---

## 5. PARTE 3 — Flujo completo

Respondan 10, 11 y 12 antes de ejecutar. Después:

```bash
cd ~/fabric/semana4
./flujo-completo.sh
```
Pausa en cada etapa (ENTER para seguir). Equivalencia con la hoja:

| Etapa hoja | Lo que pasa en su proyecto |
|---|---|
| 1 | Estado inicial: lotes REGISTRADOS |
| 2 | Org1 registra `LOTE-CUE-<hora>` (200 m² × 6 000 g/m²) con el hash de su remisión |
| 3 | Org1 actualiza 200 → 190 m² (recalcula la huella) |
| 4 | Org1 transfiere a `Org2MSP` → EN_TRANSITO |
| 5 | Org2 confirma recepción con su báscula → RECIBIDO |
| 6 | Org2 entrega a la orden `OP-<hora>` → ENTREGADO |
| 7 | **Historial: 5 entradas** + prueba de evidencia (original `true`, alterada `false`) |
| 8 | Consultas por estado (REGISTRADO vs ENTREGADO) |
| 9 y 10 (proyecto) | Completa la orden con suelas, adhesivo y cajas y calcula **HC por par ≈ 9,749 kg CO2e** |

**Captura 4:** la ETAPA 7 con las 5 entradas del historial y sus TxIDs. Si pasa muy rápido, vuelvan a consultar:
```bash
peer chaincode query -C supplychannel -n supplycc \
  -c '{"function":"ObtenerHistorial","Args":["ESCRIBAN-AQUI-SU-ID-LOTE-CUE-xxxxxx"]}' | python3 -m json.tool
```
**Captura 5:** CouchDB (`http://localhost:5984/_utils`, base `supplychannel_supplycc`) con el documento de ese lote y `"estado": "ENTREGADO"`. En CouchDB de Org2 (puerto 7984) también está.

Si quieren ejecutarlo sin pausas: `AUTO_CONTINUE=true ./flujo-completo.sh`.
Si una etapa falla (aparece un ✗ rojo), el script se detiene y muestra el motivo.

### Preguntas 13 y 14
* **13:** no modifiquen CouchDB de verdad; respondan con el razonamiento (el ledger son los bloques, CouchDB es el World State).
* **14:** altura del ledger:
  ```bash
  peer channel getinfo -c supplychannel
  ```
  Anoten `height`. Para responder bien necesitan el valor **antes**: lo ideal es haberlo anotado al empezar. Cuenta esperada: el despliegue suma 4 bloques (approve Org1, approve Org2, commit, InitLedger) y **cada ejecución de `flujo-completo.sh` suma 17** (5 del lote principal + 4 por cada uno de los otros 3 lotes). Las consultas (`query`) no generan bloques.

---

## 6. PARTE 4 — API REST

### 6.1 Instalar y arrancar (dos terminales)
Abran **dos ventanas de Git Bash**, y en cada una escriban `wsl`.

Terminal A (Org1, proveedores, puerto 3000):
```bash
cd ~/fabric/semana4/api
npm install
npm run org1
```
Terminal B (Org2, fábrica, puerto 3001):
```bash
cd ~/fabric/semana4/api
npm run org2
```
Deben verse mensajes `🚀 API Huella de carbono (Org1MSP) en http://localhost:3000` y el equivalente de Org2.

Una terminal C (también `wsl`) para los `curl`:
```bash
curl http://localhost:3000/health | python3 -m json.tool
```
**Captura 6:** `/health` con la conexión a Fabric y `Org1MSP`.

### 6.2 Probar los endpoints
Pueden usar el script que ejecuta todo en orden (con pausas), o los comandos a mano:

```bash
cd ~/fabric/semana4
./pruebas-api.sh
```
Recorre 16 pruebas: los 9 endpoints de la hoja (+ `verificar` y `huella` propios) y 3 pruebas de error (403, 409, 404).

Comandos a mano, adaptados al proyecto (reemplazan los de la hoja que usan Banano):
```bash
curl http://localhost:3000/productos | python3 -m json.tool
curl http://localhost:3000/productos/LOTE-CUE-001 | python3 -m json.tool
curl http://localhost:3000/productos/LOTE-CUE-001/historial | python3 -m json.tool

# POST: registrar un lote nuevo
curl -X POST http://localhost:3000/productos -H "Content-Type: application/json" -d '{
  "id":"LOTE-CAJ-020","nombre":"Cajas de empaque","cantidad":200,"unidad":"und",
  "origen":"Fabrica de empaques (proveedor ficticio), Barranquilla",
  "destino":"Fabrica de calzado, Sincelejo",
  "factorId":"FE-CAJA-UND","factorG":450,
  "evidenciaTexto":"Remision R-0200 | cajas | 200 und"}'

# PUT: transferir a la fábrica
curl -X PUT http://localhost:3000/productos/LOTE-CAJ-020/transferir -H "Content-Type: application/json" \
  -d '{"nuevoPropietario":"Org2MSP","detalle":"Camion 0.2 t x 700 km"}'

curl http://localhost:3000/productos/estado/EN_TRANSITO | python3 -m json.tool

# La fábrica (API Org2, puerto 3001) confirma y entrega
curl -X PUT http://localhost:3001/productos/LOTE-CAJ-020/confirmar -H "Content-Type: application/json" \
  -d '{"cantidadRecibida":200,"detalle":"Bascula de recepcion OK"}'
curl -X PUT http://localhost:3001/productos/LOTE-CAJ-020/entregar -H "Content-Type: application/json" \
  -d '{"clienteFinal":"OP-API-1","evidencia":"Vale de consumo VC-1"}'

# Verificar evidencia y huella por par
curl -X POST http://localhost:3000/productos/LOTE-CAJ-020/verificar -H "Content-Type: application/json" \
  -d '{"evidenciaTexto":"Remision R-0200 | cajas | 200 und"}'
curl "http://localhost:3000/huella/OP-API-1?b=110000&c=74750&d=27000&n=192"
```
Si repiten un POST con el mismo `id` la API responde 409 (el lote ya existe): cambien el id.

**Captura 7:** la secuencia `POST /productos` → `PUT /transferir` → `GET /historial` en la misma pantalla.

### 6.3 Preguntas 15, 16 y 17
* **16:** pruébenlo: `curl -X PUT http://localhost:3001/productos/LOTE-SUE-001/confirmar -H "Content-Type: application/json" -d '{"cantidadRecibida":200,"detalle":"x"}'` (nadie lo transfirió: error 409). Si ya lo transfirieron, usen otro lote en REGISTRADO.
* **17:** para probar la API Key: `API_KEY=mi-clave npm run org1` y luego `curl -H "x-api-key: mi-clave" http://localhost:3000/productos` (sin la cabecera devuelve 401).

---

## 7. PARTE 5 — Reflexión
Preguntas 18, 19 y 20 y la autoevaluación (5.2). Usen el borrador como guía y **escriban con sus palabras**. Para la 20 el borrador propone el ñame; pueden elegir otro producto, lo importante es justificar y decir quiénes serían Org1 y Org2.

---

## 8. Lista de capturas
| # | Qué | Dónde |
|---|---|---|
| 1 | Paso 10: ConsultarTodos con los 3 lotes | Parte 1 |
| 2 | CouchDB `supplychannel_supplycc` con los lotes iniciales | Parte 1 |
| 3 | Historial de `LOTE-CUE-001` con la actualización de cantidad | Parte 2 |
| 4 | ETAPA 7: historial de 5 entradas con TxIDs | Parte 3 |
| 5 | CouchDB: documento del lote en ENTREGADO | Parte 3 |
| 6 | `GET /health` (Org1MSP) | Parte 4 |
| 7 | `POST` → `PUT transferir` → `GET historial` | Parte 4 |
| (8, recomendada) | `checkcommitreadiness` con `true/true` | Parte 1 |

Que se vea el equipo, la fecha/hora y el nombre de la pareja (los scripts imprimen `S4 - Jesus Villalba y Jesus Madera - fecha`).

---

## 9. Errores comunes

| Síntoma | Causa y solución |
|---|---|
| `peer: command not found` / "No se encuentra el comando peer" | Están en Git Bash de Windows. Escriban `wsl`. |
| `$'\r': command not found` | Saltos de línea de Windows: `find . -name "*.sh" -exec sed -i 's/\r$//' {} \;` |
| `Cannot connect to the Docker daemon` | Abran Docker Desktop y esperen a que arranque; en Docker Desktop → Settings → Resources → WSL integration, active su distro. |
| `go mod tidy` falla (`dial tcp ... proxy.golang.org`) | Sin internet o proxy. Pruebe `curl -I https://proxy.golang.org`; si la red de la institución lo bloquea, usen datos del celular. |
| `broken pipe` / error de build al instalar | Es el problema del build clásico; este script ya usa CCaaS (`deployCCAAS`). No lo cambien a `deployCC`. |
| "chaincode definition ... sequence" | Ya existe una secuencia más alta. Ejecuten `CC_SEQUENCE=N ./deploy-supplycc.sh` con el número que muestra el error. |
| `Error: chaincode ... already exists` o un contenedor duplicado | `docker rm -f peer0org1_supplycc_ccaas peer0org2_supplycc_ccaas` y repetir. |
| `ENDORSEMENT_POLICY_FAILURE` / `MVCC_READ_CONFLICT` | Dos invocaciones casi simultáneas sobre el mismo lote. Esperen 2 segundos entre `invoke`. |
| `no existe el producto ...` / `ya existe en el ledger` | ID mal escrito o repetido. El script usa sufijo con la hora para evitar choques. |
| API: `ECONNREFUSED` o `UNAVAILABLE` | La red no está arriba o `FABRIC_DIR` es otro: `FABRIC_DIR=/ruta/fabric-samples npm run org1`. |
| API: `ENOENT ... signcerts` | La red se recreó y las identidades cambiaron de sitio; reinicien la API. |
| `EADDRINUSE: 3000` | Ya hay una API corriendo: `lsof -ti:3000 \| xargs kill`. |
| CouchDB no abre | Revisen `docker ps` (couchdb0 en 5984, couchdb1 en 7984). Pregunta de credenciales: `admin` / `adminpw`. |
| Rutas con `/` raras al usar `docker` desde Git Bash | Antepongan `MSYS_NO_PATHCONV=1` al comando. Dentro de WSL no hace falta. |

---

## 10. Antes de entregar
1. Los dos escribieron o revisaron las respuestas con sus palabras (el borrador está pensado como punto de partida, no como texto final).
2. Todas las capturas son de su pantalla, con el nombre del equipo y la fecha visibles.
3. Rellenaron `Programa`, `Fecha`, la autoevaluación 5.2 y las firmas.
4. Pegaron los outputs reales donde el borrador tiene líneas en rojo que empiezan con ▶.
5. Guarden `evidencias/deploy.log` y entreguen junto con el código (`chaincode/`, `api/`, scripts).
6. La rúbrica pide el código modificado: `chaincode/supplycc.go`, `chaincode/calculos.go` y `api/app.js` son los archivos que cambiaron.
