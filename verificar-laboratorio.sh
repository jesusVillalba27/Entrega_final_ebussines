#!/bin/bash
# =============================================================================
#  verificar-laboratorio.sh — comprueba todo ANTES de tomar las capturas de la Semana 4
#  Revisa: Docker, peers, orderer, CouchDB, canal, chaincode confirmado, ledger y las dos APIs.
#  Ejecutar en Ubuntu (WSL2).   Uso: ./verificar-laboratorio.sh
# =============================================================================
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh" || exit 1
OK=0; FAIL=0
chk() { # descripción, comando...
  local d="$1"; shift
  if "$@" >/dev/null 2>&1; then echo -e "  ${VERDE}✓${NC} $d"; OK=$((OK+1)); else echo -e "  ${ROJO}✗${NC} $d"; FAIL=$((FAIL+1)); fi
}
echo "S4 - $INTEGRANTES - $(date)"
echo "Equipo: $(whoami)@$(hostname)"

echo -e "\n${NEG}Docker y contenedores${NC}"
chk "Docker responde" docker info
for c in peer0.org1.example.com peer0.org2.example.com orderer.example.com couchdb0 couchdb1; do
  chk "contenedor $c en ejecución" bash -c "docker ps --format '{{.Names}}' | grep -qx '$c'"
done
chk "contenedores del chaincode (CCaaS) en ejecución" bash -c "docker ps --format '{{.Names}}' | grep -q '${CC}_ccaas'"

echo -e "\n${NEG}Canal y chaincode${NC}"
org 1
chk "canal $CHANNEL (peer0.org1 unido)" peer channel getinfo -c "$CHANNEL"
chk "chaincode $CC confirmado en el canal" peer lifecycle chaincode querycommitted -C "$CHANNEL" -n "$CC"
chk "política AND(Org1MSP, Org2MSP) en la definición" bash -c "peer lifecycle chaincode querycommitted -C $CHANNEL -n $CC --output json | grep -q 'Org2MSP.peer'"
N=$(qry ConsultarTodos 2>/dev/null | python3 -c 'import sys,json; print(len(json.load(sys.stdin)))' 2>/dev/null)
chk "ConsultarTodos responde (${N:-0} lotes en el World State)" test "${N:-0}" -ge 3
H=$(peer channel getinfo -c "$CHANNEL" 2>/dev/null | grep -o '"height":[0-9]*' | grep -o '[0-9]*')
echo -e "  ${CYAN}•${NC} Altura del ledger (height): ${H:-?}"

echo -e "\n${NEG}CouchDB y API${NC}"
chk "CouchDB Org1 http://localhost:5984/_utils" curl -s -o /dev/null -f http://localhost:5984/_utils/
chk "CouchDB Org2 http://localhost:7984/_utils" curl -s -o /dev/null -f http://localhost:7984/_utils/
chk "API Org1 (proveedores) http://localhost:3000/health" curl -s -f http://localhost:3000/health
chk "API Org2 (fábrica)     http://localhost:3001/health" curl -s -f http://localhost:3001/health

echo -e "\n${NEG}Resultado: ${VERDE}$OK correctos${NC}, ${ROJO}$FAIL con problema${NC}"
[ $FAIL -eq 0 ] && echo -e "${VERDE}${NEG}ENTORNO LISTO PARA LA SEMANA 4${NC}" || echo -e "${AMAR}Revisa los ✗ (si aún no arrancaste las APIs, es normal que fallen las dos últimas líneas).${NC}"
