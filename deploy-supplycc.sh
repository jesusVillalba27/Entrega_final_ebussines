#!/bin/bash
# =============================================================================
#  deploy-supplycc.sh  (Semanas 4 + 5)  —  Proyecto: huella de carbono, fábrica de calzado
#  Despliega supplycc en supplychannel. Es el deploy-supplycc.sh del profesor adaptado:
#   · usa CHAINCODE COMO SERVICIO (CCaaS) porque el build clásico del peer falla con Docker Desktop 29
#     (error "broken pipe", ya visto en la Semana 3);
#   · calcula solo la versión/secuencia (si ya hay un supplycc confirmado, sube la secuencia en 1);
#   · aplica la política de endorsement AND('Org1MSP.peer','Org2MSP.peer') que pide la guía.
#  Uso:   ./deploy-supplycc.sh            (o  CC_VERSION=2.1 ./deploy-supplycc.sh)
#  Ejecutar en Ubuntu (WSL2) con la red de la Semana 3 arriba.
# =============================================================================
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh" || exit 1
CC_VERSION="${CC_VERSION:-2.0}"
CC_SRC="$BASE/chaincode"
mkdir -p "$EVID"; LOG="$EVID/deploy.log"

echo -e "\n${NEG}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${NEG}║  Laboratorio Capstone — Semanas 4 + 5                ║${NC}"
echo -e "${NEG}║  Despliegue de supplycc (huella de carbono) v$CC_VERSION      ║${NC}"
echo -e "${NEG}╚══════════════════════════════════════════════════════╝${NC}"
echo "S4 - $INTEGRANTES - $(date)"

# ── PASO 0: red activa y canal ────────────────────────────────────────────────
banner "PASO 0 · Verificando red activa y canal $CHANNEL"
docker ps --filter "name=peer0.org1" --format "{{.Names}}" | grep -q "peer0.org1" || { err "La red no está arriba. Levántala con la Semana 3: cd ~/fabric/semana3/oficial && ./setup-network.sh"; exit 1; }
ok "Red Fabric activa"
org 1
peer channel getinfo -c "$CHANNEL" >/dev/null 2>&1 || { err "El canal $CHANNEL no existe o peer0.org1 no está unido"; exit 1; }
ok "Canal $CHANNEL disponible"
ACTUAL=$(peer lifecycle chaincode querycommitted -C "$CHANNEL" -n "$CC" 2>/dev/null | grep -o "Sequence: [0-9]*" | grep -o "[0-9]*" | head -1)
CC_SEQUENCE="${CC_SEQUENCE:-$(( ${ACTUAL:-0} + 1 ))}"
if [ -n "$ACTUAL" ]; then warn "Ya hay un $CC confirmado (secuencia $ACTUAL): se actualiza a secuencia $CC_SEQUENCE, versión $CC_VERSION"
else ok "Primer despliegue de $CC: secuencia $CC_SEQUENCE, versión $CC_VERSION"; fi

# ── PASO 1: dependencias y pruebas del chaincode ─────────────────────────────
banner "PASO 1 · Dependencias, vendor y pruebas del chaincode (go mod tidy / vendor / vet / test)"
command -v go >/dev/null || { err "No se encuentra Go en Ubuntu (se necesita Go 1.21)"; exit 1; }
( cd "$CC_SRC" && go mod tidy && go mod vendor && go vet ./... && go test ./... ) || { err "Fallaron las pruebas o la compilación: revisa el mensaje de arriba y no despliegues."; exit 1; }
ok "Código compilado y pruebas aprobadas"

# ── PASOS 2 a 8: package → install → approve → commit (CCaaS) ─────────────────
banner "PASOS 2-8 · package, install Org1 y Org2, Package ID, approve Org1 y Org2, checkcommitreadiness y commit"
# Los contenedores del chaincode anterior (misma nombre) deben salir antes de arrancar los nuevos.
docker rm -f peer0org1_${CC}_ccaas peer0org2_${CC}_ccaas >/dev/null 2>&1 && ok "Contenedores del chaincode anterior retirados"
cd "$NETWORK_DIR" || exit 1
./network.sh deployCCAAS -c "$CHANNEL" -ccn "$CC" -ccp "$CC_SRC" -ccv "$CC_VERSION" -ccs "$CC_SEQUENCE" \
    -ccep "AND('Org1MSP.peer','Org2MSP.peer')" 2>&1 | tee "$LOG"
[ "${PIPESTATUS[0]}" -eq 0 ] || { err "network.sh deployCCAAS falló. Mira $LOG"; exit 1; }
ok "Ciclo de vida completado (log en $LOG)"

echo -e "\n${CYAN}— Aprobaciones (checkcommitreadiness, tomado del log) —${NC}"
grep -n -i -A6 "commit readiness" "$LOG" | grep -E "Org1MSP|Org2MSP|readiness" | head -8
echo -e "\n${CYAN}— Package ID instalado —${NC}"
org 1; peer lifecycle chaincode queryinstalled 2>/dev/null | grep "${CC}_${CC_VERSION}"
echo -e "\n${CYAN}— Definición confirmada en el canal —${NC}"
peer lifecycle chaincode querycommitted -C "$CHANNEL" -n "$CC"
echo -e "\n${CYAN}— Contenedores del chaincode —${NC}"
docker ps --filter "name=${CC}_ccaas" --format 'table {{.Names}}\t{{.Status}}'

# ── PASO 9: InitLedger ───────────────────────────────────────────────────────
banner "PASO 9 · InitLedger (3 lotes iniciales: cuero, suelas, adhesivo) — firma Org1"
sleep 3
org 1
inv InitLedger || { err "InitLedger falló"; exit 1; }
sleep 2

# ── PASO 10: verificación ────────────────────────────────────────────────────
banner "PASO 10 · ConsultarTodos"
org 1
pq ConsultarTodos
echo -e "\n${CYAN}— Altura del ledger —${NC}"
peer channel getinfo -c "$CHANNEL"

echo -e "\n${VERDE}${NEG}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${VERDE}${NEG}║  ✓ supplycc desplegado (huella de carbono)           ║${NC}"
echo -e "${VERDE}${NEG}╚══════════════════════════════════════════════════════╝${NC}"
echo "  Chaincode: $CC v$CC_VERSION · secuencia $CC_SEQUENCE · canal $CHANNEL"
echo "  Funciones: InitLedger, RegistrarProducto, TransferirProducto, ConfirmarRecepcion,"
echo "             ActualizarCantidad, MarcarEntregado, LeerProducto, ObtenerHistorial,"
echo "             ConsultarTodos, ConsultarPorEstado, VerificarEvidencia, CalcularHuellaPar"
echo -e "\n  ${AMAR}Siguiente paso: ./flujo-completo.sh${NC}\n"
