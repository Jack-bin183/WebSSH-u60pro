package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	qmiFlagRequest    = 0x00
	qmiFlagResponse   = 0x02
	qmiFlagIndication = 0x04

	qmiIncrementalScan  = 0x0085
	qmiAbortNetworkScan = 0x00c2
)

type qmiTLV struct {
	Type  byte
	Value []byte
}

func makeQMIRequest(transactionID, messageID uint16, tlvs ...qmiTLV) ([]byte, error) {
	payloadLength := 0
	for _, tlv := range tlvs {
		if len(tlv.Value) > 0xffff {
			return nil, fmt.Errorf("TLV 0x%02x is too large: %d", tlv.Type, len(tlv.Value))
		}
		payloadLength += 3 + len(tlv.Value)
	}
	if payloadLength > 0xffff {
		return nil, fmt.Errorf("QMI payload is too large: %d", payloadLength)
	}
	request := make([]byte, 7, 7+payloadLength)
	request[0] = qmiFlagRequest
	binary.LittleEndian.PutUint16(request[1:3], transactionID)
	binary.LittleEndian.PutUint16(request[3:5], messageID)
	binary.LittleEndian.PutUint16(request[5:7], uint16(payloadLength))
	for _, tlv := range tlvs {
		request = append(request, tlv.Type, byte(len(tlv.Value)), byte(len(tlv.Value)>>8))
		request = append(request, tlv.Value...)
	}
	return request, nil
}

// makeNRIncrementalScanRequest models the already validated advanced NAS
// 0x0085 path. TLV 0x1d is count + little-endian uint32 NR-ARFCNs.
func makeNRIncrementalScanRequest(transactionID uint16, arfcns []uint32, scanType uint32) ([]byte, error) {
	if len(arfcns) < 1 || len(arfcns) > 10 {
		return nil, fmt.Errorf("NR-ARFCN count must be between 1 and 10: %d", len(arfcns))
	}
	if scanType != 0 && scanType != 2 {
		return nil, fmt.Errorf("validated scan type must be 0 or 2, got %d", scanType)
	}
	channelList := make([]byte, 1+4*len(arfcns))
	channelList[0] = byte(len(arfcns))
	for index, arfcn := range arfcns {
		if arfcn == 0 || arfcn > 3279165 {
			return nil, fmt.Errorf("invalid NR-ARFCN %d", arfcn)
		}
		binary.LittleEndian.PutUint32(channelList[1+index*4:], arfcn)
	}
	maxSearchTime := make([]byte, 2)
	binary.LittleEndian.PutUint16(maxSearchTime, 60)
	scanTypeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(scanTypeBytes, scanType)
	return makeQMIRequest(transactionID, qmiIncrementalScan,
		qmiTLV{Type: 0x10, Value: []byte{0x10}},
		qmiTLV{Type: 0x11, Value: scanTypeBytes},
		qmiTLV{Type: 0x18, Value: maxSearchTime},
		qmiTLV{Type: 0x19, Value: []byte{1}},
		qmiTLV{Type: 0x1a, Value: []byte{1}},
		qmiTLV{Type: 0x1d, Value: channelList},
	)
}

// makeQCRILOneShotExpectedRequest captures the U60 firmware's actual
// ONE_SHOT down-conversion: it preserves only the current-mode RAT bitmap and
// drops all RIL RadioAccessSpecifier bands/channels.
func makeQCRILOneShotExpectedRequest(transactionID uint16, currentModeRATMask byte) ([]byte, error) {
	if currentModeRATMask == 0 {
		return nil, fmt.Errorf("current-mode RAT mask cannot be zero")
	}
	return makeQMIRequest(transactionID, qmiIncrementalScan,
		qmiTLV{Type: 0x10, Value: []byte{currentModeRATMask}},
	)
}

func makeAbortRequest(transactionID uint16) ([]byte, error) {
	return makeQMIRequest(transactionID, qmiAbortNetworkScan)
}

func parseQMIPacket(packet []byte) (flags byte, transaction, message uint16, tlvs []qmiTLV, err error) {
	if len(packet) < 7 {
		err = fmt.Errorf("QMI packet too short: %d", len(packet))
		return
	}
	flags = packet[0]
	transaction = binary.LittleEndian.Uint16(packet[1:3])
	message = binary.LittleEndian.Uint16(packet[3:5])
	payloadLength := int(binary.LittleEndian.Uint16(packet[5:7]))
	if payloadLength != len(packet)-7 {
		err = fmt.Errorf("QMI payload length mismatch: declared=%d actual=%d", payloadLength, len(packet)-7)
		return
	}
	for offset := 7; offset < len(packet); {
		if len(packet)-offset < 3 {
			err = fmt.Errorf("truncated TLV header")
			return
		}
		typeID := packet[offset]
		length := int(binary.LittleEndian.Uint16(packet[offset+1 : offset+3]))
		offset += 3
		if length > len(packet)-offset {
			err = fmt.Errorf("truncated TLV 0x%02x", typeID)
			return
		}
		tlvs = append(tlvs, qmiTLV{Type: typeID, Value: append([]byte(nil), packet[offset:offset+length]...)})
		offset += length
	}
	return
}

func qmiResult(packet []byte) (uint16, uint16, error) {
	_, _, _, tlvs, err := parseQMIPacket(packet)
	if err != nil {
		return 0, 0, err
	}
	for _, tlv := range tlvs {
		if tlv.Type != 0x02 {
			continue
		}
		if len(tlv.Value) != 4 {
			return 0, 0, fmt.Errorf("invalid result TLV length %d", len(tlv.Value))
		}
		return binary.LittleEndian.Uint16(tlv.Value), binary.LittleEndian.Uint16(tlv.Value[2:]), nil
	}
	return 0, 0, fmt.Errorf("QMI result TLV not found")
}

func indicationStatus(packet []byte) (uint32, bool, error) {
	_, _, _, tlvs, err := parseQMIPacket(packet)
	if err != nil {
		return 0, false, err
	}
	for _, tlv := range tlvs {
		if tlv.Type != 0x01 {
			continue
		}
		if len(tlv.Value) != 4 {
			return 0, false, fmt.Errorf("invalid status TLV length %d", len(tlv.Value))
		}
		return binary.LittleEndian.Uint32(tlv.Value), true, nil
	}
	return 0, false, nil
}

func parseHexPacket(value string) ([]byte, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "0x"))
	if value == "" {
		return nil, fmt.Errorf("empty hex packet")
	}
	packet, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return packet, nil
}

func parseARFCNList(value string) ([]uint32, error) {
	parts := strings.Split(value, ",")
	if strings.TrimSpace(value) == "" || len(parts) > 10 {
		return nil, fmt.Errorf("provide 1 to 10 comma-separated NR-ARFCNs")
	}
	seen := map[uint32]bool{}
	result := make([]uint32, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.ParseUint(strings.TrimSpace(part), 0, 32)
		if err != nil || parsed == 0 || parsed > 3279165 {
			return nil, fmt.Errorf("invalid NR-ARFCN %q", part)
		}
		value := uint32(parsed)
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}
