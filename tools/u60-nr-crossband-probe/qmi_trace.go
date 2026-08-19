package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func traceFileOffset(path string) int64 {
	if strings.TrimSpace(path) == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func readCCITraceEvents(path string, offset int64) ([]qmiEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	// A hooked process may truncate the log during a controlled restart. In
	// that case the whole current file belongs to the new capture generation.
	if offset < 0 || offset > info.Size() {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var events []qmiEvent
	for scanner.Scan() {
		if event, ok := parseCCITraceEvent(scanner.Text()); ok {
			events = append(events, event)
		}
	}
	return events, scanner.Err()
}

func parseCCITraceEvent(line string) (qmiEvent, bool) {
	fields := map[string]string{}
	for _, match := range traceFieldPattern.FindAllStringSubmatch(line, -1) {
		fields[match[1]] = strings.Trim(match[2], `"`)
	}
	kind := strings.ToUpper(fields["kind"])
	if fields["service"] == "" || fields["msg"] == "" || (kind != "REQ" && kind != "RESP" && kind != "IND") {
		return qmiEvent{}, false
	}
	event := qmiEvent{
		Service: fields["service"], Event: fields["event"], MessageID: normalizeMessage(fields["msg"]),
		Caller: fields["caller"], Raw: fields["raw"],
	}
	if kind == "REQ" {
		event.Direction = "tx"
		event.Kind = "request"
	} else {
		event.Direction = "rx"
		if kind == "RESP" {
			event.Kind = "response"
		} else {
			event.Kind = "indication"
		}
	}
	if event.Raw == "" {
		if kind == "REQ" {
			event.Raw = fields["req_c"]
		} else {
			event.Raw = fields["resp_c"]
		}
	}
	if first := strings.Fields(line); len(first) != 0 {
		if seconds, err := strconv.ParseFloat(first[0], 64); err == nil {
			whole := int64(seconds)
			nanos := int64((seconds - float64(whole)) * 1e9)
			event.At = time.Unix(whole, nanos).UTC()
		}
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	if transaction, ok := firstUint16(fields, "txn", "transaction"); ok {
		event.Transaction = transaction
	}
	if result, ok := firstUint16(fields, "qmi_result", "result"); ok {
		event.Result = &result
	}
	if qmiError, ok := firstUint16(fields, "qmi_error", "error"); ok {
		event.Error = &qmiError
	}
	if rc := fields["rc"]; rc != "" {
		event.Description = "rc=" + rc
	}
	return event, true
}

func firstUint16(fields map[string]string, keys ...string) (uint16, bool) {
	for _, key := range keys {
		value := strings.TrimSpace(fields[key])
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 0, 16)
		if err == nil {
			return uint16(parsed), true
		}
	}
	return 0, false
}

func hasQMIRequest(events []qmiEvent, service, message string) bool {
	message = normalizeMessage(message)
	for _, event := range events {
		if event.Service == service && event.Direction == "tx" && event.Kind == "request" && event.MessageID == message {
			return true
		}
	}
	return false
}

func validateCCITraceWindow(path string, events []qmiEvent) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("CCI trace path is empty")
	}
	if len(events) == 0 {
		return fmt.Errorf("CCI trace %s contains no QMI request/response/indication in this run window", path)
	}
	return nil
}
