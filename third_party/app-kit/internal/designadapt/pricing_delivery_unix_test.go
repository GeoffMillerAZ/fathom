//go:build darwin || linux

package designadapt

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPricingDeliveryRefusesFIFO(t *testing.T) {
	files := syntheticDelivery(t)
	dir := filepath.Join(t.TempDir(), "delivery")
	if err := writePricingDelivery(dir, files); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pricing.css")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkPricingDelivery(dir, files); !errors.Is(err, ErrPricingDeliveryDrift) {
		t.Fatal("FIFO passed inspection")
	}
}
