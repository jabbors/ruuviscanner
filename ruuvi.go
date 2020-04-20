package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
)

const (
	headerBytes = 13
	// CompanyIdentifier = "\x99\x04" // []byte{0x99, 0x04}
	DataFormatTypeRaw       = 255 // 0xFF
	DataFormatTypeEddystone = 22  // 0x16
	DataFormatRawV1ID       = 3   // 0x03
	DataFormatRawV1Length   = 14
	DataFormatRawV2ID       = 5 // 0x05
	DataFormatRawV2Length   = 24
)

var (
	// CompanyIdentifier is a unique byte sequence to identify Ruuvi Tags in advertisement payloads
	CompanyIdentifier = []byte{0x99, 0x04} // TODO: figure out how to define it as const
	// ErrUnknownDataFormat is returned when a Measurment couldn't be created due to unknown data format
	ErrUnknownDataFormat = errors.New("unknown data format")
)

type advertisement struct {
	Length uint
	Type   uint
	Data   []byte
}

// Measurement represents sensor readings transmitted by a RuuviTag
type Measurement struct {
	MAC            net.HardwareAddr
	RSSI           int
	DataFormat     int
	Humidity       float64
	Temperature    float64
	Pressure       int
	AccelerationX  float64
	AccelerationY  float64
	AccelerationZ  float64
	BatteryVoltage float64
	advertisements []advertisement
}

func reverse(b []byte) {
	for i := len(b)/2 - 1; i >= 0; i-- {
		opp := len(b) - 1 - i
		b[i], b[opp] = b[opp], b[i]
	}
}

// NewMeasurement returns a Measurment from a byte stream or an error if it could not be parsed.
func NewMeasurement(b []byte) (*Measurement, error) {
	if len(b) < headerBytes {
		return &Measurement{}, fmt.Errorf("not enough header data") // bytes 8-12 contains the reversed MAC
	}

	// bytes 0-12 containers header data
	// packetType := uint(b[0])
	// eventCode := uint(b[1])
	// packetLength := uint(b[2])
	// subEvent := uint(b[3])
	// numberOfReports := uint(b[4])
	// eventType := uint(b[5])
	// peerAddressType := uint(b[6])

	//							     mac          rl   al at ad   al at ad ad ad ad ad ad ad ad ad ad
	//  0  1  2  3  4  5  6 |  7  8  9 10 11 12 | 13 | 14 15 16 | 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 | 35
	// 04 3E 21 02 01 03 01 | B4 98 17 16 D8 F0 | 15 | 02 01 06 | 11 FF 99 04 03 42 17 19 C7 66 00 75 FF FC 04 17 0C 19 | DB
	// 04 3E 21 02 01 03 01 | 60 35 18 F1 C6 EC | 15 | 02 01 06 | 11 FF 99 04 03 48 14 63 C7 4C FF BD FF E1 FC 0A 0C 43 | B3
	// 04 3E 21 02 01 03 01 | 60 35 18 F1 C6 EC | 15 | 02 01 06 | 11 FF 99 04 03 48 14 63 C7 4C FF B9 FF E0 FC 05 0C 3D | B3

	mac := b[7:13]
	reverse(mac)
	rssi := int(b[len(b)-1])

	// bytes 13 and forward contains package payload data
	payloadIndexID := 13
	payloadLength := int(b[payloadIndexID])
	if len(b) < payloadIndexID+payloadLength {
		return &Measurement{}, fmt.Errorf("not enough payload data")
	}
	payloadIndexID++

	// extract advertisements from package payload data
	advertisements := []advertisement{}
	for {
		if payloadIndexID >= payloadLength+headerBytes {
			break
		}
		a := advertisement{}
		a.Length = uint(b[payloadIndexID]) // the length byte is not part of the advertisement payload length
		payloadIndexID++
		a.Type = uint(b[payloadIndexID])
		payloadIndexID++
		a.Data = b[payloadIndexID : payloadIndexID+int(a.Length)-1]
		payloadIndexID += len(a.Data)
		advertisements = append(advertisements, a)
	}

	m := Measurement{}
	m.MAC = mac
	m.RSSI = rssi
	m.advertisements = advertisements

	err := m.extractSensorReadings()

	return &m, err
}

// extracts sensor readings from parsed data
func (m *Measurement) extractSensorReadings() error {
	// Format: "RAW v1" BLE Manufacturer specific data, all current sensor readings
	// Format: "RAW v2" BLE Manufacturer specific data, all current sensor readings
	for _, a := range m.advertisements {
		if a.Type != DataFormatTypeRaw {
			continue
		}
		if len(a.Data) < 2 || bytes.HasPrefix(a.Data, CompanyIdentifier) != true {
			continue
		}
		if len(a.Data) < DataFormatRawV1Length+2 || uint(a.Data[2]) != DataFormatRawV1ID && uint(a.Data[2]) != DataFormatRawV2ID {
			continue
		}
		if uint(a.Data[2]) == DataFormatRawV2ID && len(a.Data) < DataFormatRawV2Length+2 {
			continue
		}
		if uint(a.Data[2]) == DataFormatRawV1ID {
			return m.extractReadingsFormatRaw1(a.Data[2:])
		}
		return m.extractReadingsFormatRaw2(a.Data[2:])

	}
	// Format: Eddystone-URL, URL-safe base64-encoded, kickstarter edition
	// Format: Eddystone-URL, URL-safe base64-encoded, with tag id
	for _, a := range m.advertisements {
		if a.Type != DataFormatTypeEddystone {
			continue
		}
	}

	return ErrUnknownDataFormat
}

func (m *Measurement) extractReadingsFormatRaw1(b []byte) error {
	if len(b) < DataFormatRawV1Length {
		return fmt.Errorf("not enough data for Raw v1 format")
	}
	if b[0] != DataFormatRawV1ID {
		return fmt.Errorf("format raw v1 mismatch")
	}
	// first byte is data format
	m.DataFormat = DataFormatRawV1ID

	// second byte is humidity
	m.Humidity = float64(b[1]) / 2

	// bytes 3-4 is temperature
	tempSign := b[2] >> 7
	tempBase := b[2] & 0x7F
	tempFraction := float64(b[3]) / 100
	m.Temperature = float64(tempBase) + tempFraction
	if tempSign == 1 {
		m.Temperature = m.Temperature * -1
	}

	// bytes 5-6 are pressure hi and lo
	pressHi := b[4] // 199
	pressLo := b[5] // 102
	m.Pressure = int(pressHi)*256 + 50000 + int(pressLo)

	// bytes 7-12 are acceleration

	// bytes 13-14 are battery hi and lo
	battHi := b[12]
	battLo := b[13]
	m.BatteryVoltage = float64(int(battHi)*256+int(battLo)) / 1000
	return nil
}

func (m *Measurement) extractReadingsFormatRaw2(b []byte) error {
	if len(b) < DataFormatRawV2Length {
		return fmt.Errorf("not enough data for Raw v2 format")
	}
	return nil
}

func (m *Measurement) extractReadingsFormatEddystoneURLKickstarter(b []byte) error {
	return nil
}

func (m *Measurement) extractReadingsFormatEddystoneURLTagID(b []byte) error {
	return nil
}

func (m *Measurement) InfluxLineProtocol() string {
	return fmt.Sprintf("reading,sensor=%s humidity=%f,temperature=%f,pressure=%d,battery=%f,rssi=%d",
		m.MAC.String(),
		m.Humidity,
		m.Temperature,
		m.Pressure,
		m.BatteryVoltage,
		m.RSSI)
}
