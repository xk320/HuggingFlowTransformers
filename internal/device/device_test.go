package device

import "testing"

func TestDetectUsesExplicitDevicesWithoutNvidiaSMI(t *testing.T) {
	devices, err := Detect([]int{2, 4})
	if err != nil {
		t.Fatalf("Detect returned error for explicit devices: %v", err)
	}
	if len(devices) != 2 || devices[0] != 2 || devices[1] != 4 {
		t.Fatalf("devices = %#v", devices)
	}
}
