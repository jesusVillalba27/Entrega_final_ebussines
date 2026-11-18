#!/bin/bash
# =============================================================================
#  flujo-completo.sh  (Semanas 4 + 5)  —  Proyecto: huella de carbono, fábrica de calzado
#  Registrar → Actualizar → Transferir → Confirmar → Entregar → Historial → Estados → HC por par
#  Es el flujo-completo.sh del profesor adaptado: el "producto" es un LOTE DE MATERIAL con su
#  huella declarada (Emisiones = DA × FE) y termina entregado a una ORDEN DE PRODUCCIÓN.
#  Uso:  ./flujo-completo.sh          (con pausas: ENTER para continuar)
#        AUTO_CONTINUE=true ./flujo-completo.sh    (sin pausas)
# =============================================================================
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh" || exit 1
AUTO_CONTINUE="${AUTO_CONTINUE:-false}"
SUF="${SUF:-$(date +%H%M%S)}"
LOTE="${LOTE:-LOTE-CUE-$SUF}"     # lote principal (cuero del zapato Oxford)
ORDEN="OP-$SUF"                    # orden de producción que consume los lotes

pause() { [ "$AUTO_CONTINUE" = "true" ] && return 0; echo -e "\n${AMAR}  Presiona ENTER para continuar...${NC}"; read -r; }
paso()  { "$@" || { err "Se detiene el flujo: falló $1 $2"; exit 1; }; sleep 1; }

clear 2>/dev/null || true
echo -e "${NEG}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${NEG}║  Laboratorio Capstone — Flujo completo (huella de carbono) ║${NC}"
echo -e "${NEG}║  Semanas 4 + 5 | Hyperledger Fabric 2.5                    ║${NC}"
echo -e "${NEG}╚══════════════════════════════════════════════════════════╝${NC}"
echo "S4 - $INTEGRANTES - $(date)"
echo "Lote principal: $LOTE   ·   Orden de producción: $ORDEN"

banner "ETAPA 1 — Estado inicial del ledger (lotes de InitLedger en REGISTRADO)"
org 1; pq ConsultarTodos
pause

banner "ETAPA 2 — Org1 (proveedor) registra $LOTE: 200 m² de cuero × 6 000 g CO2e/m²"
H=$(evid "$LOTE" "Remision R-$SUF | proveedor ficticio de cuero | 200 m2")
echo "  SHA-256 de la remisión: $H"
org 1; paso inv RegistrarProducto "$LOTE" "Cuero vacuno negro (Oxford)" 200 m2 "Curtiembre (proveedor ficticio), Cartago" "Fábrica de calzado, Sincelejo" FE-CUERO-M2 6000 "$H"
pq LeerProducto "$LOTE"
pause

banner "ETAPA 3 — Org1 actualiza la cantidad: 200 → 190 m² (medición real del área). La huella se recalcula"
org 1; paso inv ActualizarCantidad "$LOTE" 190 "Medicion real del area cortable: 190 m2"
pq LeerProducto "$LOTE"
pause

banner "ETAPA 4 — Org1 transfiere $LOTE a Org2MSP (la fábrica) → EN_TRANSITO"
org 1; paso inv TransferirProducto "$LOTE" Org2MSP "Camion 0.3 t x 900 km - guia T-$SUF"
pq LeerProducto "$LOTE"
pause

banner "ETAPA 5 — Org2 (fábrica) confirma la recepción con su báscula: 190 m² → RECIBIDO"
org 2; paso inv ConfirmarRecepcion "$LOTE" 190 "Lote recibido en bascula de recepcion, QR del proveedor leido"
pq LeerProducto "$LOTE"
pause

banner "ETAPA 6 — Org2 entrega el lote a la orden de producción $ORDEN → ENTREGADO"
org 2; paso inv MarcarEntregado "$LOTE" "$ORDEN" "Vale de consumo VC-$SUF - linea de corte"
pq LeerProducto "$LOTE"
pause

banner "ETAPA 7 — Historial inmutable de $LOTE (5 entradas) y prueba de evidencia"
org 1; pq ObtenerHistorial "$LOTE"
echo -e "\n  Verificación de la remisión original y de una remisión alterada:"
echo -e "  original → VerificarEvidencia = $(qry VerificarEvidencia "$LOTE" "$H")"
HA=$(printf '%s\n' "Remision R-$SUF | proveedor ficticio de cuero | 290 m2" | sha256sum | cut -d' ' -f1)
echo -e "  alterada → VerificarEvidencia = $(qry VerificarEvidencia "$LOTE" "$HA")"
pause

banner "ETAPA 8 — Estado final: lotes REGISTRADOS vs ENTREGADOS"
org 1
echo "  Productos REGISTRADOS:"; pq ConsultarPorEstado REGISTRADO
echo "  Productos ENTREGADOS:";  pq ConsultarPorEstado ENTREGADO
pause

banner "ETAPA 9 (proyecto) — Completar la orden $ORDEN con suelas, adhesivo y cajas (ciclo completo de cada lote)"
ciclo() { # id nombre cantidad unidad factorId factorG documento
  local id="$1" nombre="$2" cant="$3" uni="$4" fid="$5" fg="$6" doc="$7" h
  h=$(evid "$id" "$doc")
  org 1; paso inv RegistrarProducto "$id" "$nombre" "$cant" "$uni" "Proveedor ficticio" "Fábrica de calzado, Sincelejo" "$fid" "$fg" "$h"
  org 1; paso inv TransferirProducto "$id" Org2MSP "Despacho $id"
  org 2; paso inv ConfirmarRecepcion "$id" "$cant" "Bascula de recepcion OK"
  org 2; paso inv MarcarEntregado "$id" "$ORDEN" "Vale de consumo $id"
}
ciclo "LOTE-SUE-$SUF" "Suelas oxford"  200 par FE-SUELA-OXF   1800 "Remision R-$SUF-2 | suelas | 200 pares"
ciclo "LOTE-ADH-$SUF" "Adhesivo"        20 kg  FE-ADHESIVO-KG 3500 "Remision R-$SUF-3 | adhesivo | 20 kg"
ciclo "LOTE-CAJ-$SUF" "Cajas de empaque" 200 und FE-CAJA-UND    450 "Remision R-$SUF-4 | cajas | 200 und"
pause

banner "ETAPA 10 (proyecto) — HC por par = (A + B + C + D) ÷ N  para $ORDEN"
echo "  A = suma de la huella de los 4 lotes ENTREGADOS (sale del ledger)"
echo "  B = 500 kWh × 220 g/kWh = 110 000 g  (factor real UPME SIN 2024)"
echo "  C = 25 kg GLP × 2 990 g/kg = 74 750 g   ·   D = 270 t-km × 100 g = 27 000 g   ·   N = 192 pares buenos"
org 1; pq CalcularHuellaPar "$ORDEN" 110000 74750 27000 192
echo -e "\n  (Mismo resultado del lote Oxford de la Semana 3: 9,749 kg CO2e por par)"

echo -e "\n${VERDE}${NEG}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${VERDE}${NEG}║  ✓ Flujo completo ejecutado                               ║${NC}"
printf "${VERDE}${NEG}║  Lote %-20s Orden %-20s║${NC}\n" "$LOTE" "$ORDEN"
echo -e "${VERDE}${NEG}║  Estado final: ENTREGADO                                  ║${NC}"
echo -e "${VERDE}${NEG}╚══════════════════════════════════════════════════════════╝${NC}"
