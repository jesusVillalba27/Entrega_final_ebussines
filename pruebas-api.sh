#!/bin/bash
# =============================================================================
#  pruebas-api.sh — recorre los endpoints de la API (Parte 4 del laboratorio)
#  Requiere las DOS APIs arriba:  Org1 en el puerto 3000 y Org2 en el 3001
#  (terminal A: npm run org1   |   terminal B: npm run org2)
#  Ejecutar en Ubuntu (WSL2) — usa python3 para mostrar el JSON ordenado.
#  Uso: ./pruebas-api.sh
# =============================================================================
A1="http://localhost:3000"   # API de proveedores (Org1MSP)
A2="http://localhost:3001"   # API de la fábrica (Org2MSP)
SUF="${SUF:-$(date +%H%M%S)}"
LOTE="LOTE-EMP-$SUF"
ORDEN="OP-API-$SUF"
VERDE='\033[0;32m'; CYAN='\033[0;36m'; NEG='\033[1m'; NC='\033[0m'
INTEGRANTES="${INTEGRANTES:-Jesus Villalba y Jesus Madera}"
AUTO="${AUTO_CONTINUE:-false}"

t() { echo -e "\n${NEG}${CYAN}▶ $1${NC}"; echo -e "  \$ $2"; }
j() { python3 -m json.tool 2>/dev/null || cat; }
pausa() { [ "$AUTO" = "true" ] || { echo -e "\n  (ENTER para continuar)"; read -r; }; }
curl -s -o /dev/null -f "$A1/health" || { echo "La API Org1 no responde en :3000. Arráncala: cd api && npm run org1"; exit 1; }
curl -s -o /dev/null -f "$A2/health" || { echo "La API Org2 no responde en :3001. Arráncala: cd api && npm run org2"; exit 1; }
echo "S4 - $INTEGRANTES - $(date)"

t "1. GET /health (Org1)" "curl $A1/health";                              curl -s $A1/health | j
t "2. GET /productos — todos los lotes" "curl $A1/productos";             curl -s $A1/productos | j; pausa
t "3. GET /productos/:id" "curl $A1/productos/LOTE-CUE-001";              curl -s $A1/productos/LOTE-CUE-001 | j
t "4. GET /productos/:id/historial" "curl $A1/productos/LOTE-CUE-001/historial"; curl -s $A1/productos/LOTE-CUE-001/historial | j; pausa

t "5. POST /productos — Org1 declara $LOTE (200 cajas × 450 g CO2e)" "curl -X POST $A1/productos -d '{...}'"
curl -s -X POST $A1/productos -H "Content-Type: application/json" -d "{
  \"id\":\"$LOTE\",\"nombre\":\"Cajas de empaque\",\"cantidad\":200,\"unidad\":\"und\",
  \"origen\":\"Fabrica de empaques (proveedor ficticio), Barranquilla\",\"destino\":\"Fabrica de calzado, Sincelejo\",
  \"factorId\":\"FE-CAJA-UND\",\"factorG\":450,\"evidenciaTexto\":\"Remision R-$SUF | cajas | 200 und\"}" | j

t "6. PUT /productos/:id/transferir — Org1 despacha a Org2MSP" "curl -X PUT $A1/productos/$LOTE/transferir"
curl -s -X PUT $A1/productos/$LOTE/transferir -H "Content-Type: application/json" \
  -d '{"nuevoPropietario":"Org2MSP","detalle":"Camion 0.2 t x 700 km - guia API"}' | j
t "7. GET /productos/estado/EN_TRANSITO" "curl $A1/productos/estado/EN_TRANSITO"; curl -s $A1/productos/estado/EN_TRANSITO | j; pausa

t "8. PUT /productos/:id/confirmar — la fábrica (API Org2, puerto 3001) confirma con la báscula" "curl -X PUT $A2/productos/$LOTE/confirmar"
curl -s -X PUT $A2/productos/$LOTE/confirmar -H "Content-Type: application/json" \
  -d '{"cantidadRecibida":200,"detalle":"Bascula de recepcion OK"}' | j
t "9. PUT /productos/:id/cantidad — la fábrica ajusta por cajas dañadas (200 → 198)" "curl -X PUT $A2/productos/$LOTE/cantidad"
curl -s -X PUT $A2/productos/$LOTE/cantidad -H "Content-Type: application/json" \
  -d '{"cantidad":198,"motivo":"2 cajas danadas en descarga"}' | j
t "10. PUT /productos/:id/entregar — entrega a la orden $ORDEN" "curl -X PUT $A2/productos/$LOTE/entregar"
curl -s -X PUT $A2/productos/$LOTE/entregar -H "Content-Type: application/json" \
  -d "{\"clienteFinal\":\"$ORDEN\",\"evidencia\":\"Vale de consumo VC-API-$SUF\"}" | j; pausa

t "11. GET /productos/:id/historial — 5 entradas" "curl $A1/productos/$LOTE/historial"; curl -s $A1/productos/$LOTE/historial | j; pausa
t "12. POST /productos/:id/verificar — evidencia original (true) y alterada (false)" "curl -X POST $A1/productos/$LOTE/verificar"
curl -s -X POST $A1/productos/$LOTE/verificar -H "Content-Type: application/json" -d "{\"evidenciaTexto\":\"Remision R-$SUF | cajas | 200 und\"}" | j
curl -s -X POST $A1/productos/$LOTE/verificar -H "Content-Type: application/json" -d "{\"evidenciaTexto\":\"Remision R-$SUF | cajas | 900 und\"}" | j
t "13. GET /huella/:orden — HC por par (B, C, D de ejemplo; N = 192 pares)" "curl '$A1/huella/$ORDEN?b=110000&c=74750&d=27000&n=192'"
curl -s "$A1/huella/$ORDEN?b=110000&c=74750&d=27000&n=192" | j; pausa

echo -e "\n${NEG}${CYAN}── Pruebas de rechazo (deben fallar con un error claro) ──${NC}"
t "14. Org2 intenta actualizar un lote que no es suyo → 403" "curl -X PUT $A2/productos/LOTE-CUE-001/cantidad"
curl -s -w "\n  HTTP %{http_code}\n" -X PUT $A2/productos/LOTE-CUE-001/cantidad -H "Content-Type: application/json" -d '{"cantidad":999,"motivo":"Intento de Org2"}'
t "15. Org2 confirma un lote que nadie transfirió (sigue REGISTRADO) → 409" "curl -X PUT $A2/productos/LOTE-SUE-001/confirmar"
curl -s -w "\n  HTTP %{http_code}\n" -X PUT $A2/productos/LOTE-SUE-001/confirmar -H "Content-Type: application/json" -d '{"cantidadRecibida":200,"detalle":"x"}'
t "16. Lote inexistente → 404" "curl $A1/productos/NO-EXISTE"
curl -s -w "\n  HTTP %{http_code}\n" $A1/productos/NO-EXISTE
echo -e "\n${VERDE}${NEG}✓ Recorrido de la API terminado${NC}  (lote $LOTE · orden $ORDEN)"
