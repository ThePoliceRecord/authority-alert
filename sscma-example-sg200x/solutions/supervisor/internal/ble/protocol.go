// Package ble provides BLE WiFi provisioning via BlueZ D-Bus.
package ble

import "encoding/json"

// GATT UUIDs for the provisioning service and characteristics.
const (
	ServiceUUID    = "AA01FC00-B5A3-F393-E0A9-E50E24DCCA9E"
	SessionUUID    = "AA01FC01-B5A3-F393-E0A9-E50E24DCCA9E"
	WiFiScanUUID   = "AA01FC02-B5A3-F393-E0A9-E50E24DCCA9E"
	WiFiConfigUUID = "AA01FC03-B5A3-F393-E0A9-E50E24DCCA9E"
	DeviceInfoUUID = "AA01FC04-B5A3-F393-E0A9-E50E24DCCA9E"
	SetupUUID      = "AA01FC05-B5A3-F393-E0A9-E50E24DCCA9E"
)

// Request is the JSON envelope for all BLE commands (iOS -> camera).
type Request struct {
	Cmd   string          `json:"cmd"`
	Token string          `json:"token,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Response is the JSON envelope for all BLE responses (camera -> iOS).
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Fragment splits data into MTU-sized BLE packets with a 1-byte framing header.
//
// Header format (spec Section 4):
//
//	Bit 7: 1 = first fragment
//	Bit 6: 1 = last fragment
//	Bits 5-0: sequence number (0-63)
func Fragment(data []byte, mtu int) [][]byte {
	if mtu < 2 {
		mtu = 2
	}
	payload := mtu - 1 // 1 byte for header

	if len(data) <= payload {
		// Single packet: both first and last bits set.
		pkt := make([]byte, 1+len(data))
		pkt[0] = 0xC0 // first | last, seq=0
		copy(pkt[1:], data)
		return [][]byte{pkt}
	}

	var packets [][]byte
	seq := 0
	for off := 0; off < len(data); off += payload {
		end := off + payload
		if end > len(data) {
			end = len(data)
		}
		chunk := data[off:end]

		var hdr byte
		if off == 0 {
			hdr = 0x80 // first
		}
		if end == len(data) {
			hdr |= 0x40 // last
		}
		hdr |= byte(seq & 0x3F)

		pkt := make([]byte, 1+len(chunk))
		pkt[0] = hdr
		copy(pkt[1:], chunk)
		packets = append(packets, pkt)
		seq++
	}
	return packets
}

// Reassembler accumulates BLE fragments and returns the complete payload
// when the last fragment is received.
type Reassembler struct {
	buf []byte
}

// Write processes a single BLE packet. It returns the complete reassembled
// payload when the last fragment arrives, or nil if more fragments are expected.
func (r *Reassembler) Write(pkt []byte) []byte {
	if len(pkt) < 1 {
		return nil
	}
	hdr := pkt[0]
	payload := pkt[1:]

	first := hdr&0x80 != 0
	last := hdr&0x40 != 0

	if first {
		r.buf = r.buf[:0] // reset
	}
	r.buf = append(r.buf, payload...)

	if last {
		out := make([]byte, len(r.buf))
		copy(out, r.buf)
		r.buf = r.buf[:0]
		return out
	}
	return nil
}
