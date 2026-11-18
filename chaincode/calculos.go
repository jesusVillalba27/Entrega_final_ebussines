package main

// calculos.go: aritmética y validaciones puras del proyecto (sin dependencias de Fabric).
// Todo se calcula con ENTEROS (gramos de CO2e) para que los dos peers que simulan la
// transacción obtengan exactamente el mismo resultado: nada de decimales flotantes.

import (
	"fmt"
	"regexp"
	"strings"
)

// Estados del ciclo de vida de un lote de material (los 4 del laboratorio).
const (
	EstadoRegistrado = "REGISTRADO"  // el proveedor declaró el lote y su huella
	EstadoEnTransito = "EN_TRANSITO" // el proveedor lo transfirió a la fábrica
	EstadoRecibido   = "RECIBIDO"    // la fábrica confirmó con su báscula
	EstadoEntregado  = "ENTREGADO"   // entregado a una orden de producción
)

// Organizaciones de la test-network en este proyecto.
const (
	MSPProveedores = "Org1MSP" // proveedores: cuero, suelas, adhesivos, empaques
	MSPFabrica     = "Org2MSP" // fábrica de calzado de Sincelejo
)

const (
	feMaxG      = 1000000   // un factor no puede superar 1 000 kg CO2e por unidad
	cantidadMax = 100000000 // 100 millones de unidades (evita desbordes de int64)
	textoMaxLen = 120       // largo máximo de textos libres
)

var (
	reHash = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reID   = regexp.MustCompile(`^[A-Za-z0-9-]{3,40}$`)
)

// huellaG implementa  Emisiones = DA × FE  para un lote:
// DA = cantidad (m2, pares, kg, und) y FE = factor declarado (g CO2e por unidad).
func huellaG(cantidad int, factorG int64) int64 {
	return int64(cantidad) * factorG
}

// hcParG implementa  HC por par = (A + B + C + D) ÷ N  en gramos por par (redondeo al gramo).
func hcParG(aG, bG, cG, dG int64, n int) (int64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("N (pares buenos) debe ser mayor que cero")
	}
	total := aG + bG + cG + dG
	return (total + int64(n)/2) / int64(n), nil
}

// kgStr muestra gramos como kg con 3 decimales (9749 -> "9.749"), solo con enteros.
func kgStr(g int64) string {
	neg := ""
	if g < 0 {
		neg, g = "-", -g
	}
	return fmt.Sprintf("%s%d.%03d", neg, g/1000, g%1000)
}

func idValido(campo, v string) error {
	if !reID.MatchString(v) {
		return fmt.Errorf("%s inválido %q: use 3 a 40 caracteres entre letras, números y guion (sin guion bajo ni espacios)", campo, v)
	}
	return nil
}

func hashValido(h string) error {
	if !reHash.MatchString(h) {
		return fmt.Errorf("hashEvidencia inválido: debe ser un SHA-256 en hexadecimal minúscula (64 caracteres)")
	}
	return nil
}

func textoValido(campo, v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("%s es obligatorio", campo)
	}
	if len(v) > textoMaxLen {
		return fmt.Errorf("%s supera %d caracteres", campo, textoMaxLen)
	}
	return nil
}

func cantidadValida(c int) error {
	if c <= 0 {
		return fmt.Errorf("la cantidad debe ser mayor a 0, recibido: %d", c)
	}
	if c > cantidadMax {
		return fmt.Errorf("la cantidad %d supera el máximo permitido (%d)", c, cantidadMax)
	}
	return nil
}

func factorValido(fe int64) error {
	if fe <= 0 || fe > feMaxG {
		return fmt.Errorf("el factor de emisión debe estar entre 1 y %d g CO2e por unidad, recibido: %d", feMaxG, fe)
	}
	return nil
}
