// supplycc.go: chaincode del laboratorio Capstone (Semanas 4 + 5) adaptado al proyecto
// "Huella de carbono verificable en una fábrica de calzado" (Sincelejo, Sucre).
//
// Base: el supplycc.go del profesor (mismas funciones y mismo ciclo de vida de 4 estados).
// Adaptación: el "producto" es un LOTE DE MATERIAL (cuero, suelas, adhesivo, empaques) que
// el proveedor declara con su huella de carbono (Emisiones = DA × FE) y entrega a la fábrica:
//
//	REGISTRADO  --Transferir-->  EN_TRANSITO  --ConfirmarRecepcion-->  RECIBIDO  --MarcarEntregado-->  ENTREGADO
//	  (Org1)                        (Org1)                                (Org2)                          (Org2)
//
// ENTREGADO significa "entregado a una orden de producción": ese lote ya cuenta en la parte A
// (materiales y empaques) de la fórmula  HC por par = (A + B + C + D) ÷ N.
//
// Principios (los mismos de las diapositivas):
//   - La cadena guarda el HASH de la evidencia (remisión, certificado), no el archivo.
//   - Cálculo determinista con enteros (gramos de CO2e), sin decimales flotantes.
//   - Hora de la transacción = hora que firma el orderer (GetTxTimestamp), NO time.Now():
//     así los dos peers que simulan la transacción obtienen exactamente el mismo resultado.
//   - Cada cambio guarda quién lo hizo (MSP) y queda en el historial inmutable.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

const (
	tipoProducto  = "producto"
	tipoHistorial = "historial"
	prefHistorial = "historial"
)

// Producto es un lote de material con su huella de carbono declarada.
type Producto struct {
	DocType       string `json:"docType"`
	ID            string `json:"id"`            // LOTE-CUE-001
	Nombre        string `json:"nombre"`        // Cuero vacuno negro
	Cantidad      int    `json:"cantidad"`      // DA: dato de actividad (m2, pares, kg, und)
	Unidad        string `json:"unidad"`        // m2, par, kg, und
	Propietario   string `json:"propietario"`   // Org1MSP -> Org2MSP -> orden de producción
	Estado        string `json:"estado"`        // REGISTRADO, EN_TRANSITO, RECIBIDO, ENTREGADO
	Origen        string `json:"origen"`        // dónde lo produce el proveedor
	Destino       string `json:"destino"`       // a dónde va (la fábrica)
	FactorID      string `json:"factorId"`      // FE-CUERO-M2: factor que declaró el proveedor
	FactorG       int64  `json:"factorG"`       // FE: g CO2e por unidad
	HuellaG       int64  `json:"huellaG"`       // Emisiones = DA × FE (g CO2e)
	HashEvidencia string `json:"hashEvidencia"` // SHA-256 del soporte del proveedor
	Timestamp     string `json:"timestamp"`
	TxID          string `json:"txId"`
}

// EventoHistorial registra cada cambio de un lote (quién, cuándo y por qué).
type EventoHistorial struct {
	DocType   string `json:"docType"`
	TxID      string `json:"txId"`
	Timestamp string `json:"timestamp"`
	Estado    string `json:"estado"`
	Actor     string `json:"actor"`
	Cantidad  int    `json:"cantidad"`
	HuellaG   int64  `json:"huellaG"`
	Detalle   string `json:"detalle"`
}

// HuellaPar es el resultado de HC por par = (A + B + C + D) ÷ N para una orden de producción.
type HuellaPar struct {
	Orden       string   `json:"orden"`
	Lotes       []string `json:"lotes"`
	AG          int64    `json:"aG"`
	BG          int64    `json:"bG"`
	CG          int64    `json:"cG"`
	DG          int64    `json:"dG"`
	TotalG      int64    `json:"totalG"`
	ParesBuenos int      `json:"paresBuenos"`
	HCParG      int64    `json:"hcParG"`
	HCParKg     string   `json:"hcParKg"`
}

// SupplyContract implementa el contrato.
type SupplyContract struct {
	contractapi.Contract
}

// ---------------------------------------------------------------------------------------------
// utilidades internas (en minúscula: no son funciones invocables de la red)
// ---------------------------------------------------------------------------------------------

func horaTx(ctx contractapi.TransactionContextInterface) (string, error) {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("no se pudo leer la hora de la transacción: %v", err)
	}
	return time.Unix(ts.Seconds, 0).UTC().Format(time.RFC3339), nil
}

func mspLlamante(ctx contractapi.TransactionContextInterface) (string, error) {
	msp, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", fmt.Errorf("no se pudo leer la identidad del llamante: %v", err)
	}
	return msp, nil
}

// exigirMSP falla si el llamante no pertenece a una de las organizaciones permitidas.
func exigirMSP(ctx contractapi.TransactionContextInterface, permitidos ...string) (string, error) {
	msp, err := mspLlamante(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range permitidos {
		if msp == p {
			return msp, nil
		}
	}
	return "", fmt.Errorf("el MSP %s no tiene permiso para esta operación (permitidos: %s)",
		msp, strings.Join(permitidos, ", "))
}

func (sc *SupplyContract) existeProducto(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	data, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, err
	}
	return data != nil, nil
}

func (sc *SupplyContract) guardarProducto(ctx contractapi.TransactionContextInterface, p *Producto) error {
	pJSON, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(p.ID, pJSON)
}

// guardarHistorial escribe el evento con una clave compuesta (historial, id, txId).
// Los errores NO se ignoran: si no se puede escribir, la transacción falla.
func (sc *SupplyContract) guardarHistorial(ctx contractapi.TransactionContextInterface, id string, e EventoHistorial) error {
	e.DocType = tipoHistorial
	e.TxID = ctx.GetStub().GetTxID()
	key, err := ctx.GetStub().CreateCompositeKey(prefHistorial, []string{id, e.TxID})
	if err != nil {
		return err
	}
	eJSON, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(key, eJSON)
}

// cerrarCambio guarda historial, evento de la red y estado en el World State.
func (sc *SupplyContract) cerrarCambio(ctx contractapi.TransactionContextInterface,
	p *Producto, actor, detalle, nombreEvento string, payloadEvento []byte) error {
	hora, err := horaTx(ctx)
	if err != nil {
		return err
	}
	p.Timestamp = hora
	p.TxID = ctx.GetStub().GetTxID()
	if err := sc.guardarHistorial(ctx, p.ID, EventoHistorial{
		Timestamp: hora, Estado: p.Estado, Actor: actor,
		Cantidad: p.Cantidad, HuellaG: p.HuellaG, Detalle: detalle,
	}); err != nil {
		return err
	}
	if payloadEvento == nil {
		payloadEvento, _ = json.Marshal(p)
	}
	if err := ctx.GetStub().SetEvent(nombreEvento, payloadEvento); err != nil {
		return err
	}
	return sc.guardarProducto(ctx, p)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// =============================================================================
//
//	FUNCIÓN 1 — InitLedger
//	Carga 3 lotes de ejemplo del proyecto (valores SUPUESTOS, salvo lo que se indique).
//	Solo Org1 (proveedores) puede inicializar. Es idempotente: no pisa lotes existentes.
//
// =============================================================================
func (sc *SupplyContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	msp, err := exigirMSP(ctx, MSPProveedores)
	if err != nil {
		return err
	}
	destino := "Fábrica de calzado, Sincelejo (Sucre)"
	lotes := []Producto{
		{ID: "LOTE-CUE-001", Nombre: "Cuero vacuno negro", Cantidad: 190, Unidad: "m2",
			Origen: "Curtiembre (proveedor ficticio), Cartago", FactorID: "FE-CUERO-M2", FactorG: 6000,
			HashEvidencia: sha256Hex("Remision R-0101 | proveedor ficticio de cuero | 190 m2")},
		{ID: "LOTE-SUE-001", Nombre: "Suelas oxford", Cantidad: 200, Unidad: "par",
			Origen: "Fábrica de suelas (proveedor ficticio), Medellín", FactorID: "FE-SUELA-OXF", FactorG: 1800,
			HashEvidencia: sha256Hex("Remision R-0102 | proveedor ficticio de suelas | 200 pares")},
		{ID: "LOTE-ADH-001", Nombre: "Adhesivo de montaje", Cantidad: 20, Unidad: "kg",
			Origen: "Fabricante de adhesivos (proveedor ficticio), Barranquilla", FactorID: "FE-ADHESIVO-KG", FactorG: 3500,
			HashEvidencia: sha256Hex("Remision R-0103 | adhesivo | 20 kg")},
	}
	for i := range lotes {
		p := lotes[i]
		existe, err := sc.existeProducto(ctx, p.ID)
		if err != nil {
			return err
		}
		if existe {
			continue
		}
		p.DocType = tipoProducto
		p.Propietario = msp
		p.Estado = EstadoRegistrado
		p.Destino = destino
		p.HuellaG = huellaG(p.Cantidad, p.FactorG)
		hora, err := horaTx(ctx)
		if err != nil {
			return err
		}
		p.Timestamp = hora
		p.TxID = ctx.GetStub().GetTxID()
		if err := sc.guardarProducto(ctx, &p); err != nil {
			return err
		}
		if err := sc.guardarHistorial(ctx, p.ID, EventoHistorial{
			Timestamp: hora, Estado: p.Estado, Actor: msp, Cantidad: p.Cantidad, HuellaG: p.HuellaG,
			Detalle: fmt.Sprintf("Lote inicial (InitLedger): %d %s × %d g CO2e/%s = %d g CO2e",
				p.Cantidad, p.Unidad, p.FactorG, p.Unidad, p.HuellaG),
		}); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
//
//	FUNCIÓN 2 — RegistrarProducto
//	El proveedor (Org1) declara un lote de material con su huella: Emisiones = DA × FE.
//	Guarda el factor que usó (copia) y el hash del soporte (remisión, ACV, certificado).
//
// =============================================================================
func (sc *SupplyContract) RegistrarProducto(
	ctx contractapi.TransactionContextInterface,
	id, nombre string, cantidad int,
	unidad, origen, destino, factorID string, factorG int64, hashEvidencia string,
) error {
	msp, err := exigirMSP(ctx, MSPProveedores)
	if err != nil {
		return err
	}
	if err := idValido("id", id); err != nil {
		return err
	}
	if err := idValido("factorId", factorID); err != nil {
		return err
	}
	for campo, v := range map[string]string{"nombre": nombre, "unidad": unidad, "origen": origen, "destino": destino} {
		if err := textoValido(campo, v); err != nil {
			return err
		}
	}
	if err := cantidadValida(cantidad); err != nil {
		return err
	}
	if err := factorValido(factorG); err != nil {
		return err
	}
	if err := hashValido(hashEvidencia); err != nil {
		return err
	}
	existe, err := sc.existeProducto(ctx, id)
	if err != nil {
		return err
	}
	if existe {
		return fmt.Errorf("el producto %s ya existe en el ledger (no se sobrescribe)", id)
	}

	p := &Producto{
		DocType: tipoProducto, ID: id, Nombre: nombre, Cantidad: cantidad, Unidad: unidad,
		Propietario: msp, Estado: EstadoRegistrado, Origen: origen, Destino: destino,
		FactorID: factorID, FactorG: factorG, HuellaG: huellaG(cantidad, factorG),
		HashEvidencia: hashEvidencia,
	}
	detalle := fmt.Sprintf("Registrado: %d %s × %d g CO2e/%s (%s) = %d g CO2e",
		cantidad, unidad, factorG, unidad, factorID, p.HuellaG)
	return sc.cerrarCambio(ctx, p, msp, detalle, "ProductoRegistrado", nil)
}

// =============================================================================
//
//	FUNCIÓN 3 — TransferirProducto
//	El proveedor (dueño actual) despacha el lote a la fábrica. REGISTRADO -> EN_TRANSITO.
//
// =============================================================================
func (sc *SupplyContract) TransferirProducto(
	ctx contractapi.TransactionContextInterface,
	id, nuevoPropietario, detalle string,
) error {
	msp, err := exigirMSP(ctx, MSPProveedores)
	if err != nil {
		return err
	}
	producto, err := sc.LeerProducto(ctx, id)
	if err != nil {
		return err
	}
	if producto.Propietario != msp {
		return fmt.Errorf("solo el propietario (%s) puede transferir el producto %s — llamante: %s",
			producto.Propietario, id, msp)
	}
	if producto.Estado != EstadoRegistrado {
		return fmt.Errorf("el producto %s no puede transferirse (estado: %s; se requiere REGISTRADO)", id, producto.Estado)
	}
	if nuevoPropietario != MSPFabrica {
		return fmt.Errorf("los lotes de proveedores solo se transfieren a la fábrica (%s), recibido: %q", MSPFabrica, nuevoPropietario)
	}
	if err := textoValido("detalle", detalle); err != nil {
		return err
	}

	producto.Propietario = nuevoPropietario
	producto.Estado = EstadoEnTransito
	return sc.cerrarCambio(ctx, producto, msp,
		fmt.Sprintf("Despachado a %s — %s", nuevoPropietario, detalle), "ProductoTransferido", nil)
}

// =============================================================================
//
//	FUNCIÓN 4 — ConfirmarRecepcion
//	La fábrica (Org2) confirma con su báscula de recepción la cantidad realmente recibida.
//	Si difiere de lo declarado, la cantidad y la huella se RECALCULAN (DA × FE) y queda
//	registrada la diferencia. EN_TRANSITO -> RECIBIDO.
//
// =============================================================================
func (sc *SupplyContract) ConfirmarRecepcion(
	ctx contractapi.TransactionContextInterface,
	id string, cantidadRecibida int, detalle string,
) error {
	msp, err := exigirMSP(ctx, MSPFabrica)
	if err != nil {
		return err
	}
	producto, err := sc.LeerProducto(ctx, id)
	if err != nil {
		return err
	}
	if producto.Estado != EstadoEnTransito {
		return fmt.Errorf("el producto %s no está EN_TRANSITO (estado: %s)", id, producto.Estado)
	}
	if producto.Propietario != msp {
		return fmt.Errorf("solo el destinatario (%s) puede confirmar la recepción — llamante: %s", producto.Propietario, msp)
	}
	if err := cantidadValida(cantidadRecibida); err != nil {
		return err
	}
	if err := textoValido("detalle", detalle); err != nil {
		return err
	}

	nota := fmt.Sprintf("%s — báscula: %d %s (coincide con lo declarado)", detalle, cantidadRecibida, producto.Unidad)
	if cantidadRecibida != producto.Cantidad {
		nota = fmt.Sprintf("%s — DIFERENCIA de báscula: declarado %d %s, recibido %d %s; huella recalculada de %d a %d g CO2e",
			detalle, producto.Cantidad, producto.Unidad, cantidadRecibida, producto.Unidad,
			producto.HuellaG, huellaG(cantidadRecibida, producto.FactorG))
		producto.Cantidad = cantidadRecibida
		producto.HuellaG = huellaG(cantidadRecibida, producto.FactorG)
	}
	producto.Estado = EstadoRecibido
	return sc.cerrarCambio(ctx, producto, msp, nota, "RecepcionConfirmada", nil)
}

// =============================================================================
//
//	FUNCIÓN 5 — ActualizarCantidad  (con las 3 mejoras del laboratorio)
//	  1) valida que quien actualiza es el PROPIETARIO actual
//	  2) valida que la cantidad sea positiva
//	  3) registra el cambio en el historial y emite el evento CantidadActualizada
//	Carbono: al cambiar la cantidad (DA) se recalcula la huella del lote (DA × FE).
//
// =============================================================================
func (sc *SupplyContract) ActualizarCantidad(
	ctx contractapi.TransactionContextInterface,
	id string, nuevaCantidad int, motivo string,
) error {
	producto, err := sc.LeerProducto(ctx, id)
	if err != nil {
		return err
	}

	clientID, err := mspLlamante(ctx)
	if err != nil {
		return err
	}
	if clientID != producto.Propietario {
		return fmt.Errorf("solo el propietario (%s) puede actualizar la cantidad — llamante: %s",
			producto.Propietario, clientID)
	}
	if producto.Estado != EstadoRegistrado && producto.Estado != EstadoRecibido {
		return fmt.Errorf("no se puede actualizar la cantidad en estado %s (solo REGISTRADO o RECIBIDO; en tránsito manda la báscula de recepción)", producto.Estado)
	}
	if err := cantidadValida(nuevaCantidad); err != nil {
		return err
	}
	if err := textoValido("motivo", motivo); err != nil {
		return err
	}
	if nuevaCantidad == producto.Cantidad {
		return fmt.Errorf("la cantidad nueva (%d) es igual a la actual: no hay nada que actualizar", nuevaCantidad)
	}

	cantidadAnterior, huellaAnterior := producto.Cantidad, producto.HuellaG
	producto.Cantidad = nuevaCantidad
	producto.HuellaG = huellaG(nuevaCantidad, producto.FactorG)

	detalle := fmt.Sprintf("Cantidad actualizada de %d a %d %s — Motivo: %s — Huella %d -> %d g CO2e",
		cantidadAnterior, nuevaCantidad, producto.Unidad, motivo, huellaAnterior, producto.HuellaG)
	payload, _ := json.Marshal(map[string]interface{}{
		"id":               id,
		"cantidadAnterior": cantidadAnterior,
		"cantidadNueva":    nuevaCantidad,
		"unidad":           producto.Unidad,
		"huellaAnteriorG":  huellaAnterior,
		"huellaNuevaG":     producto.HuellaG,
		"motivo":           motivo,
		"actor":            clientID,
	})
	return sc.cerrarCambio(ctx, producto, clientID, detalle, "CantidadActualizada", payload)
}

// =============================================================================
//
//	FUNCIÓN 6 — MarcarEntregado
//	La fábrica entrega el lote a una orden de producción (OP-...). RECIBIDO -> ENTREGADO.
//	El destino final es un TEXTO (no un MSP): la orden de producción no es una organización
//	de la red. Desde aquí el lote cuenta en la parte A de esa orden.
//
// =============================================================================
func (sc *SupplyContract) MarcarEntregado(
	ctx contractapi.TransactionContextInterface,
	id, clienteFinal, evidencia string,
) error {
	msp, err := exigirMSP(ctx, MSPFabrica)
	if err != nil {
		return err
	}
	producto, err := sc.LeerProducto(ctx, id)
	if err != nil {
		return err
	}
	if producto.Estado != EstadoRecibido {
		return fmt.Errorf("solo productos RECIBIDOS pueden marcarse como entregados (estado actual: %s)", producto.Estado)
	}
	if producto.Propietario != msp {
		return fmt.Errorf("solo el propietario (%s) puede entregar el producto — llamante: %s", producto.Propietario, msp)
	}
	if err := idValido("clienteFinal (orden de producción)", clienteFinal); err != nil {
		return err
	}
	if err := textoValido("evidencia", evidencia); err != nil {
		return err
	}

	producto.Estado = EstadoEntregado
	producto.Propietario = clienteFinal
	return sc.cerrarCambio(ctx, producto, msp,
		fmt.Sprintf("Entregado a: %s — Evidencia: %s", clienteFinal, evidencia), "ProductoEntregado", nil)
}

// =============================================================================
//
//	FUNCIÓN 7 — LeerProducto (World State)
//
// =============================================================================
func (sc *SupplyContract) LeerProducto(
	ctx contractapi.TransactionContextInterface, id string,
) (*Producto, error) {
	pJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("error leyendo %s: %v", id, err)
	}
	if pJSON == nil {
		return nil, fmt.Errorf("producto %s no encontrado en el ledger", id)
	}
	var producto Producto
	if err := json.Unmarshal(pJSON, &producto); err != nil {
		return nil, err
	}
	if producto.DocType != tipoProducto {
		return nil, fmt.Errorf("producto %s no encontrado en el ledger", id)
	}
	return &producto, nil
}

// =============================================================================
//
//	FUNCIÓN 8 — ObtenerHistorial
//	Recorre el historial INMUTABLE del ledger (GetHistoryForKey): una entrada por transacción.
//	Une cada una con el evento guardado (quién y por qué) usando el TxID.
//
// =============================================================================
func (sc *SupplyContract) ObtenerHistorial(
	ctx contractapi.TransactionContextInterface, id string,
) ([]*EventoHistorial, error) {
	iterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	historial := []*EventoHistorial{}
	for iterator.HasNext() {
		mod, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		if mod.IsDelete {
			continue
		}
		var p Producto
		if err := json.Unmarshal(mod.Value, &p); err != nil {
			continue
		}
		e := &EventoHistorial{
			DocType: tipoHistorial, TxID: mod.TxId,
			Timestamp: time.Unix(mod.Timestamp.Seconds, 0).UTC().Format(time.RFC3339),
			Estado:    p.Estado, Actor: p.Propietario, Cantidad: p.Cantidad, HuellaG: p.HuellaG,
			Detalle: fmt.Sprintf("Cantidad: %d %s | Huella: %d g CO2e", p.Cantidad, p.Unidad, p.HuellaG),
		}
		// Completa quién actuó y por qué, si existe el evento de esa transacción.
		key, err := ctx.GetStub().CreateCompositeKey(prefHistorial, []string{id, mod.TxId})
		if err == nil {
			if raw, err := ctx.GetStub().GetState(key); err == nil && raw != nil {
				var guardado EventoHistorial
				if json.Unmarshal(raw, &guardado) == nil {
					e.Actor, e.Detalle = guardado.Actor, guardado.Detalle
				}
			}
		}
		historial = append(historial, e)
	}
	return historial, nil
}

// =============================================================================
//
//	FUNCIÓN 9 — ConsultarTodos
//
// =============================================================================
func (sc *SupplyContract) ConsultarTodos(
	ctx contractapi.TransactionContextInterface,
) ([]*Producto, error) {
	iterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	productos := []*Producto{}
	for iterator.HasNext() {
		resp, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		var p Producto
		if err := json.Unmarshal(resp.Value, &p); err != nil || p.DocType != tipoProducto {
			continue // omite eventos de historial u otros documentos
		}
		productos = append(productos, &p)
	}
	return productos, nil
}

// =============================================================================
//
//	FUNCIÓN 10 — ConsultarPorEstado
//
// =============================================================================
func (sc *SupplyContract) ConsultarPorEstado(
	ctx contractapi.TransactionContextInterface, estado string,
) ([]*Producto, error) {
	validos := map[string]bool{
		EstadoRegistrado: true, EstadoEnTransito: true, EstadoRecibido: true, EstadoEntregado: true,
	}
	if !validos[estado] {
		return nil, fmt.Errorf("estado inválido: %s. Válidos: REGISTRADO, EN_TRANSITO, RECIBIDO, ENTREGADO", estado)
	}
	todos, err := sc.ConsultarTodos(ctx)
	if err != nil {
		return nil, err
	}
	filtrados := []*Producto{}
	for _, p := range todos {
		if p.Estado == estado {
			filtrados = append(filtrados, p)
		}
	}
	return filtrados, nil
}

// =============================================================================
//
//	FUNCIÓN 11 (proyecto) — VerificarEvidencia
//	Compara el SHA-256 de un archivo con el hash guardado en la cadena.
//	Si alguien altera la remisión o el certificado, el hash deja de coincidir (false).
//
// =============================================================================
func (sc *SupplyContract) VerificarEvidencia(
	ctx contractapi.TransactionContextInterface, id, hash string,
) (bool, error) {
	p, err := sc.LeerProducto(ctx, id)
	if err != nil {
		return false, err
	}
	if err := hashValido(strings.ToLower(strings.TrimSpace(hash))); err != nil {
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(hash)) == p.HashEvidencia, nil
}

// =============================================================================
//
//	FUNCIÓN 12 (proyecto) — CalcularHuellaPar     HC por par = (A + B + C + D) ÷ N
//	A sale del ledger: suma de la huella de los lotes ENTREGADOS a la orden de producción.
//	B (electricidad), C (gas y combustibles) y D (transporte) se reciben ya calculados en g CO2e
//	(en el contrato de la Semana 3 salen de DA × FE). Es solo lectura: no escribe en el ledger.
//
// =============================================================================
func (sc *SupplyContract) CalcularHuellaPar(
	ctx contractapi.TransactionContextInterface,
	orden string, bG, cG, dG int64, paresBuenos int,
) (*HuellaPar, error) {
	if err := idValido("orden", orden); err != nil {
		return nil, err
	}
	if bG < 0 || cG < 0 || dG < 0 {
		return nil, fmt.Errorf("B, C y D no pueden ser negativos")
	}
	if paresBuenos <= 0 {
		return nil, fmt.Errorf("N (pares buenos) debe ser mayor que cero")
	}
	todos, err := sc.ConsultarTodos(ctx)
	if err != nil {
		return nil, err
	}
	r := &HuellaPar{Orden: orden, Lotes: []string{}, BG: bG, CG: cG, DG: dG, ParesBuenos: paresBuenos}
	for _, p := range todos {
		if p.Estado == EstadoEntregado && p.Propietario == orden {
			r.AG += p.HuellaG
			r.Lotes = append(r.Lotes, p.ID)
		}
	}
	if len(r.Lotes) == 0 {
		return nil, fmt.Errorf("la orden %s no tiene lotes ENTREGADOS: no hay parte A que calcular", orden)
	}
	r.TotalG = r.AG + bG + cG + dG
	if r.HCParG, err = hcParG(r.AG, bG, cG, dG, paresBuenos); err != nil {
		return nil, err
	}
	r.HCParKg = kgStr(r.HCParG)
	return r, nil
}

// =============================================================================
//
//	MAIN — arranca como chaincode clásico o como servicio (CCaaS)
//
// =============================================================================
func main() {
	cc, err := contractapi.NewChaincode(&SupplyContract{})
	if err != nil {
		log.Panicf("error creando el chaincode supplycc: %v", err)
	}
	// Modo CCaaS (chaincode como servicio): el contenedor escucha y el peer se conecta a él.
	// Las variables las pone ./network.sh deployCCAAS. Sin ellas, arranca en modo clásico.
	ccid := os.Getenv("CHAINCODE_ID")
	addr := os.Getenv("CHAINCODE_SERVER_ADDRESS")
	if ccid != "" && addr != "" {
		server := &shim.ChaincodeServer{
			CCID:     ccid,
			Address:  addr,
			CC:       cc,
			TLSProps: shim.TLSProperties{Disabled: true}, // la test-network usa tls_required=false
		}
		if err := server.Start(); err != nil {
			log.Panicf("error iniciando supplycc como servicio: %v", err)
		}
		return
	}
	if err := cc.Start(); err != nil {
		log.Panicf("error iniciando el chaincode supplycc: %v", err)
	}
}
