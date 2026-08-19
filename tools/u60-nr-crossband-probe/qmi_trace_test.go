package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCCITraceEventsFromOffset(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "trace.log")
	old := "1786582158.000000 pid=1 event=TX_IDL_ENCODE service=3 kind=REQ msg=0x0043 raw=\n"
	current := "1786582158.814164 pid=469 event=TX_IDL_ENCODE service=3 kind=REQ msg=0x0085 rc=0 caller=libqmi.so raw=10010010\n" +
		"1786582158.900000 pid=469 event=RX_IDL_DECODE service=3 kind=RESP msg=0x0085 qmi_result=0 qmi_error=0 raw=02040000000000\n"
	if err := os.WriteFile(path, []byte(old+current), 0600); err != nil {
		t.Fatal(err)
	}
	events, err := readCCITraceEvents(path, int64(len(old)))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || !hasQMIRequest(events, "3", "0x0085") {
		t.Fatalf("events=%+v", events)
	}
	if events[1].Result == nil || *events[1].Result != 0 || events[1].Error == nil || *events[1].Error != 0 {
		t.Fatalf("response=%+v", events[1])
	}
}
