package main

import (
	"testing"
)

func TestNewMeasurement(t *testing.T) {
	b := []byte{}
	m, err := NewMeasurement(b)
	if err == nil {
		t.Errorf("expected error with empty input")
	}
	b = []byte{0x04, 0x3E, 0x21, 0x02, 0x01, 0x03, 0x01, 0xB4, 0x98, 0x17, 0x16, 0xD8, 0xF0, 0x15, 0x02, 0x01, 0x06, 0x11, 0xFF, 0x99, 0x04, 0x03, 0x42, 0x17, 0x19, 0xC7, 0x66, 0x00, 0x78, 0xFF, 0xFA, 0x04, 0x19, 0x0C, 0x19, 0xE1}
	m, err = NewMeasurement(b)
	if err != nil {
		t.Errorf("didn't expect an error with valid input, got error '%s'", err)
	}
	if m.MAC.String() != "f0:d8:16:17:98:b4" {
		t.Errorf("expected MAC f0:d8:16:17:98:b4, got %s", m.MAC.String())
	}
	if m.RSSI != 225 {
		t.Errorf("expected RSSI %d, got %d", 225, m.RSSI)
	}
	if len(m.advertisements) != 2 {
		t.Errorf("expected 2 advertisements, got %d", len(m.advertisements))
	}
	if m.advertisements[0].Length != 2 && m.advertisements[0].Type != 1 && len(m.advertisements[0].Data)-1 != 1 {
		t.Errorf("advertisement 0 not parsed correctly")
	}
	if m.advertisements[1].Length != 17 && m.advertisements[1].Type != 255 && len(m.advertisements[1].Data)-1 != 16 {
		t.Errorf("advertisement 1 not parsed correctly")
	}

	b = []byte{0x04, 0x3E, 0x21, 0x02, 0x01, 0x03, 0x01, 0x60, 0x35, 0x18, 0xF1, 0xC6, 0xEC, 0x15, 0x02, 0x01, 0x06, 0x11, 0xFF, 0x99, 0x04, 0x03, 0x48, 0x14, 0x63, 0xC7, 0x4C, 0xFF, 0xB9, 0xFF, 0xE0, 0xFC, 0x05, 0x0C, 0x3D, 0xB3}
	m, err = NewMeasurement(b)
	if err != nil {
		t.Errorf("didn't expect an error with valid input, got error '%s'", err)
	}
	if m.MAC.String() != "ec:c6:f1:18:35:60" {
		t.Errorf("expected MAC ec:c6:f1:18:35:60, got %s", m.MAC.String())
	}
	if m.RSSI != 179 {
		t.Errorf("expected RSSI %d, got %d", 179, m.RSSI)
	}
	if len(m.advertisements) != 2 {
		t.Errorf("expected 2 advertisements, got %d", len(m.advertisements))
	}
	if m.advertisements[0].Length != 2 && m.advertisements[0].Type != 1 && len(m.advertisements[0].Data)-1 != 1 {
		t.Errorf("advertisement 0 not parsed correctly")
	}
	if m.advertisements[1].Length != 17 && m.advertisements[1].Type != 255 && len(m.advertisements[1].Data)-1 != 16 {
		t.Errorf("advertisement 1 not parsed correctly")
	}
}

func TestExtractReadingsFormatRaw1(t *testing.T) {
	m := Measurement{}
	b := []byte{0x03, 0x42, 0x17, 0x19, 0xC7, 0x66, 0x00, 0x78, 0xFF, 0xFA, 0x04, 0x19, 0x0C, 0x19}
	err := m.extractReadingsFormatRaw1(b)
	if err != nil {
		t.Errorf("execpected error with valid input: %s", err)
	}

	if m.Humidity != 33.0 {
		t.Errorf("expected humidity %2f, got %2f", 33.0, m.Humidity)
	}
	if m.Temperature != 23.25 {
		t.Errorf("expected temperature %2f, got %2f", 23.25, m.Temperature)
	}
	if m.Pressure != 101046 {
		t.Errorf("expected pressure %d, got %d", 101046, m.Pressure)
	}
	if m.BatteryVoltage != 3.097 {
		t.Errorf("expected battery voltage %2f, got %2f", 3.097, m.BatteryVoltage)
	}
}
