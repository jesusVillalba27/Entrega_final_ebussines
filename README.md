# Semana 4 + 5 · Capstone · Huella de carbono verificable (calzado, Sincelejo)

Contenido:
- `GUIA_PASO_A_PASO.md` — comandos y orden de trabajo (Git Bash → `wsl`).
- `chaincode/` — `supplycc.go` (contrato), `calculos.go` + `calculos_test.go` (fórmulas en enteros), `Dockerfile` (CCaaS), `go.mod`, `go.sum`.
- `api/` — `app.js` y `package.json` (Express + fabric-gateway).
- `deploy-supplycc.sh` · `flujo-completo.sh` · `pruebas-api.sh` · `verificar-laboratorio.sh` · `lib.sh`.
- `laboratorio_capstone_BORRADOR.docx` — hoja del profesor con respuestas de borrador (azul) y datos que deben pegar ustedes (rojo, con ▶).

Orden: `./verificar-laboratorio.sh` → `./deploy-supplycc.sh` → `./flujo-completo.sh` → API (`npm run org1` / `npm run org2`) → `./pruebas-api.sh`.
Todo se ejecuta dentro de WSL (Ubuntu) con la red de la Semana 3 arriba.
