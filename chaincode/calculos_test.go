package main

import "testing"

// Pruebas de la aritmética del proyecto (sin Fabric). Se ejecutan con:  go test ./...

func TestHuellaG(t *testing.T) {
	// 190 m2 de cuero × 6 000 g CO2e/m2 = 1 140 000 g (1 140 kg)
	if got := huellaG(190, 6000); got != 1140000 {
		t.Fatalf("huellaG(190, 6000) = %d, esperado 1140000", got)
	}
	// 500 kWh × 220 g/kWh (factor real UPME SIN 2024) = 110 000 g
	if got := huellaG(500, 220); got != 110000 {
		t.Fatalf("huellaG(500, 220) = %d, esperado 110000", got)
	}
}

func TestHCPar(t *testing.T) {
	// Lote Oxford de la Semana 3: A=1 660 000, B=110 000, C=74 750, D=27 000, N=192 -> 9 749 g/par
	got, err := hcParG(1660000, 110000, 74750, 27000, 192)
	if err != nil || got != 9749 {
		t.Fatalf("hcParG = %d (%v), esperado 9749", got, err)
	}
	if kgStr(got) != "9.749" {
		t.Fatalf("kgStr(%d) = %s, esperado 9.749", got, kgStr(got))
	}
	if _, err := hcParG(1, 1, 1, 1, 0); err == nil {
		t.Fatal("N = 0 debía dar error")
	}
}

func TestValidaciones(t *testing.T) {
	if idValido("id", "LOTE-CUE-001") != nil {
		t.Fatal("LOTE-CUE-001 es un id válido")
	}
	for _, malo := range []string{"", "ab", "LOTE_CUE", "lote cue", "LOTE/CUE"} {
		if idValido("id", malo) == nil {
			t.Fatalf("%q debía ser inválido", malo)
		}
	}
	if hashValido(sha256Hex("hola")) != nil {
		t.Fatal("un SHA-256 real debe ser válido")
	}
	if hashValido("ABC") == nil {
		t.Fatal("un hash corto debía ser inválido")
	}
	if cantidadValida(0) == nil || cantidadValida(-1) == nil || cantidadValida(cantidadMax+1) == nil || cantidadValida(10) != nil {
		t.Fatal("validación de cantidad incorrecta")
	}
	if factorValido(0) == nil || factorValido(feMaxG+1) == nil || factorValido(6000) != nil {
		t.Fatal("validación de factor incorrecta")
	}
}
