package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtract0085Requests(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "trace.log")
	line := "1 event=TX_IDL_ENCODE service=3 kind=REQ msg=0x0085 caller=qcril raw=100100101d0500019eb40700\n"
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	requests, err := extract0085Requests(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || !requests[0].HasChannel || requests[0].TLVs["0x1d"] != "019eb40700" {
		t.Fatalf("requests=%+v", requests)
	}
}
