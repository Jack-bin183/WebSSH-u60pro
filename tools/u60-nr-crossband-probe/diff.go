package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var traceFieldPattern = regexp.MustCompile(`(?:^|\s)([A-Za-z0-9_]+)=([^\s]+)`)

type traceRequest struct {
	Line       int               `json:"line"`
	Event      string            `json:"event"`
	Service    string            `json:"service"`
	Message    string            `json:"message"`
	Caller     string            `json:"caller,omitempty"`
	Raw        string            `json:"raw"`
	TLVs       map[string]string `json:"tlvs,omitempty"`
	HasChannel bool              `json:"has_nr_arfcn_tlv_0x1d"`
}

type qmiDiffReport struct {
	ManualPath     string         `json:"manual_path"`
	QCRILPath      string         `json:"qcril_path"`
	ManualRequests []traceRequest `json:"manual_requests"`
	QCRILRequests  []traceRequest `json:"qcril_requests"`
	FirstRawEqual  bool           `json:"first_raw_equal"`
	ManualOnlyTLVs []string       `json:"manual_only_tlvs,omitempty"`
	QCRILOnlyTLVs  []string       `json:"qcril_only_tlvs,omitempty"`
	DifferentTLVs  []string       `json:"different_tlvs,omitempty"`
	Interpretation []string       `json:"interpretation"`
}

func runDiffQMI(args []string) error {
	flags := flag.NewFlagSet("diff-qmi", flag.ContinueOnError)
	manualPath := flags.String("manual", "", "QMI CCI trace from manual experiment A")
	qcrilPath := flags.String("qcril", "", "QMI CCI trace from complete QCRIL experiment B")
	outputPath := flags.String("out", "", "optional JSON output path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *manualPath == "" || *qcrilPath == "" {
		return errors.New("diff-qmi requires -manual and -qcril")
	}
	manual, err := extract0085Requests(*manualPath)
	if err != nil {
		return err
	}
	qcril, err := extract0085Requests(*qcrilPath)
	if err != nil {
		return err
	}
	if len(manual) == 0 || len(qcril) == 0 {
		return fmt.Errorf("0x0085 TX_IDL_ENCODE request missing: manual=%d qcril=%d", len(manual), len(qcril))
	}
	report := qmiDiffReport{ManualPath: *manualPath, QCRILPath: *qcrilPath, ManualRequests: manual, QCRILRequests: qcril}
	report.FirstRawEqual = manual[0].Raw == qcril[0].Raw
	report.ManualOnlyTLVs, report.QCRILOnlyTLVs, report.DifferentTLVs = diffTLVs(manual[0].TLVs, qcril[0].TLVs)
	report.Interpretation = []string{
		"TLV 0x1d is the validated NR-ARFCN list; its presence proves only that the list reached NAS encoding.",
		"A COMPLETE indication does not prove ML1 measured every listed ARFCN; correlate with QSH hash 0xd8f582a8.",
		"If QCRIL ONE_SHOT lacks TLV 0x1d, that matches the U60 qcrilNr reverse-engineered path which drops RadioAccessSpecifier channels.",
	}
	if *outputPath != "" {
		if err := writeJSONAtomic(*outputPath, report, 0600); err != nil {
			return err
		}
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
	return nil
}

func extract0085Requests(path string) ([]traceRequest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var requests []traceRequest
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		fields := map[string]string{}
		for _, match := range traceFieldPattern.FindAllStringSubmatch(line, -1) {
			fields[match[1]] = strings.Trim(match[2], `"`)
		}
		if fields["service"] != "3" || !strings.EqualFold(fields["kind"], "REQ") || normalizeMessage(fields["msg"]) != "0x0085" || fields["event"] != "TX_IDL_ENCODE" {
			continue
		}
		raw := fields["raw"]
		payload, decodeErr := hex.DecodeString(raw)
		if decodeErr != nil {
			return nil, fmt.Errorf("%s:%d invalid raw hex: %w", path, lineNumber, decodeErr)
		}
		tlvs, parseErr := parseTLVPayload(payload)
		if parseErr != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, parseErr)
		}
		request := traceRequest{Line: lineNumber, Event: fields["event"], Service: fields["service"], Message: "0x0085", Caller: fields["caller"], Raw: raw, TLVs: map[string]string{}}
		for _, tlv := range tlvs {
			key := fmt.Sprintf("0x%02x", tlv.Type)
			request.TLVs[key] = hex.EncodeToString(tlv.Value)
			if tlv.Type == 0x1d {
				request.HasChannel = true
			}
		}
		requests = append(requests, request)
	}
	return requests, scanner.Err()
}

func parseTLVPayload(payload []byte) ([]qmiTLV, error) {
	var tlvs []qmiTLV
	for offset := 0; offset < len(payload); {
		if len(payload)-offset < 3 {
			return nil, fmt.Errorf("truncated TLV header")
		}
		typeID := payload[offset]
		length := int(binary.LittleEndian.Uint16(payload[offset+1 : offset+3]))
		offset += 3
		if length > len(payload)-offset {
			return nil, fmt.Errorf("truncated TLV 0x%02x", typeID)
		}
		tlvs = append(tlvs, qmiTLV{Type: typeID, Value: append([]byte(nil), payload[offset:offset+length]...)})
		offset += length
	}
	return tlvs, nil
}

func diffTLVs(left, right map[string]string) (leftOnly, rightOnly, different []string) {
	for key, value := range left {
		other, ok := right[key]
		if !ok {
			leftOnly = append(leftOnly, key)
		} else if !bytes.Equal([]byte(value), []byte(other)) {
			different = append(different, key)
		}
	}
	for key := range right {
		if _, ok := left[key]; !ok {
			rightOnly = append(rightOnly, key)
		}
	}
	sort.Strings(leftOnly)
	sort.Strings(rightOnly)
	sort.Strings(different)
	return
}

func normalizeMessage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "0x")
	for len(value) < 4 {
		value = "0" + value
	}
	return "0x" + value
}
