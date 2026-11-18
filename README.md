#  Trazabilidad y Huella de Carbono Verificable - Fábrica de Calzado (Sincelejo)

**Laboratorio Capstone — Semanas 4 + 5**
*Blockchain Empresarial con Hyperledger Fabric*

Este repositorio contiene la solución completa para el sistema de trazabilidad y cálculo de huella de carbono (CO2e) de una fábrica de calzado en Sincelejo, utilizando una red permisionada de Hyperledger Fabric, un Smart Contract (Chaincode) en Go y una API REST en Node.js.

##  Video de Sustentación y Demostración
Mira la demostración en vivo del sistema de principio a fin, incluyendo la ejecución del flujo completo y la explicación técnica:
 **[Ver Video en YouTube](https://youtu.be/NPPASpuzCss)**

---

##  Integrantes
*   Jesús Villalba
*   Jesús Madera
*   *Programa:* Ingeniería de Sistemas

---

##  Arquitectura de la Solución
El sistema simula un consorcio entre dos organizaciones:
*   **Org1 (Proveedores):** Curtiembres y fábricas de suelas que declaran la huella de sus materiales.
*   **Org2 (Fábrica):** Recibe la mercancía, valida con su báscula y calcula la huella final por par de zapato.
*   **Red:** Hyperledger Fabric 2.5 (Canal: `supplychannel`, Ordenador Raft).
*   **Consenso:** Política de Endorsement `AND('Org1MSP.peer', 'Org2MSP.peer')`.

##  Estructura del Repositorio

```text
Laboratorio 4 y 5/
├── chaincode/
│   ├── supplycc.go        # Lógica de negocio y contrato inteligente
│   ├── calculos.go        # Fórmulas y validaciones (HC por par = A+B+C+D / N)
│   ├── calculos_test.go   # Pruebas unitarias del contrato
│   └── go.mod / go.sum    # Dependencias de Go
├── api/
│   ├── app.js             # API REST con fabric-gateway
│   ├── package.json       # Configuración de Node.js
│   └── package-lock.json  # Dependencias exactas
├── deploy-supplycc.sh     # Script para desplegar el chaincode (CCaaS)
├── flujo-completo.sh      # Script de demostración del flujo completo
└── README.md              # Este documento
