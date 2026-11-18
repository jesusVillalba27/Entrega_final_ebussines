#!/bin/bash
# lib.sh: funciones comunes de la Semana 4. Se usa con:   source lib.sh
# Debe ejecutarse en Ubuntu (WSL2), donde están los binarios de Fabric (peer, configtxlator...).

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FABRIC_DIR="${FABRIC_DIR:-$HOME/fabric/fabric-samples}"
NETWORK_DIR="$FABRIC_DIR/test-network"
BASE="${BASE:-$LIB_DIR}"
CHANNEL="supplychannel"
CC="supplycc"
EVID="$BASE/evidencias"
INTEGRANTES="${INTEGRANTES:-Jesus Villalba y Jesus Madera}"
export PATH="$FABRIC_DIR/bin:$PATH"

VERDE='\033[0;32m'; ROJO='\033[0;31m'; AMAR='\033[1;33m'; CYAN='\033[0;36m'; NEG='\033[1m'; NC='\033[0m'

# El peer es un binario de Linux: si el prompt es de Git Bash (no de WSL), esto avisa.
if ! command -v peer >/dev/null 2>&1; then
  echo -e "${ROJO}✗ No se encuentra el comando 'peer'.${NC}"
  echo "  Estás en Git Bash de Windows. Escribe  wsl  para entrar a Ubuntu y vuelve a ejecutar."
  echo "  (Si ya estás en Ubuntu, revisa que exista $FABRIC_DIR/bin/peer)"
  return 1 2>/dev/null || exit 1
fi

# org 1 -> actúa como Org1MSP (proveedores) | org 2 -> actúa como Org2MSP (fábrica)
org() {
  [ -f "$NETWORK_DIR/set-env-org$1.sh" ] || { echo "Falta $NETWORK_DIR/set-env-org$1.sh: ejecuta primero setup-network.sh de la Semana 3"; return 1; }
  source "$NETWORK_DIR/set-env-org$1.sh" >/dev/null
}

# inv FUNCION arg1 arg2 ...  -> transacción firmada por los peers de Org1 y Org2
# Devuelve 0 si se confirmó; muestra el motivo si el chaincode la rechazó.
inv() {
  local fn="$1"; shift
  local args="" a out rc
  for a in "$@"; do args+="\"${a}\","; done
  out=$(peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
        --tls --cafile "$ORDERER_CA" -C "$CHANNEL" -n "$CC" \
        --peerAddresses localhost:7051 --tlsRootCertFiles "$PEER1_TLS" \
        --peerAddresses localhost:9051 --tlsRootCertFiles "$PEER2_TLS" \
        -c "{\"function\":\"$fn\",\"Args\":[${args%,}]}" 2>&1)
  rc=$?
  if [ $rc -eq 0 ]; then
    echo -e "  ${VERDE}✓ $fn $1${NC}"
  else
    echo -e "  ${ROJO}✗ $fn $*${NC}"
    echo "$out" | grep -E "Error|chaincode response" | tail -3 | sed 's/^/      /'
  fi
  return $rc
}

# qry FUNCION arg1 ...  -> consulta (no genera bloque)
qry() {
  local fn="$1"; shift
  local args="" a
  for a in "$@"; do args+="\"${a}\","; done
  peer chaincode query -C "$CHANNEL" -n "$CC" -c "{\"function\":\"$fn\",\"Args\":[${args%,}]}"
}

# pq FUNCION args... -> consulta con JSON bonito
pq() { qry "$@" | python3 -m json.tool; }

# evid NOMBRE "texto del documento" -> crea el archivo de evidencia y devuelve su SHA-256
evid() {
  mkdir -p "$EVID"
  printf '%s\n' "$2" > "$EVID/$1.txt"
  sha256sum "$EVID/$1.txt" | cut -d' ' -f1
}

banner() { echo -e "\n${NEG}${CYAN}▶ $1${NC}\n"; }
ok()     { echo -e "  ${VERDE}✓ $1${NC}"; }
warn()   { echo -e "  ${AMAR}⚠ $1${NC}"; }
err()    { echo -e "  ${ROJO}✗ $1${NC}"; }
