package sqlite

import "testing"

func TestDecimalExponentBounds(t *testing.T) {
	for _, value := range []string{"1e-1000000", "1e1000000", "10e4096", "0.1e-4096"} {
		if boundedDecimalExponent(value) {
			t.Fatalf("out-of-range exponent %q accepted", value)
		}
	}
	for _, value := range []string{"1e4096", "1e-4096", "10e4095", "0.1e-4095"} {
		if !boundedDecimalExponent(value) {
			t.Fatalf("boundary exponent %q rejected", value)
		}
	}
}
