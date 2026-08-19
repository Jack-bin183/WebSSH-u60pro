//go:build linux

package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const (
	afQIPCRTR       = 42
	qrtrPortCtrl    = 0xfffffffe
	qrtrTypeNewServ = 4
	qrtrTypeNewLook = 10
	nasService      = 3
)

type sockaddrQRTR struct {
	Family uint16
	Pad    uint16
	Node   uint32
	Port   uint32
}

type qrtrServer struct {
	Version  uint32
	Instance uint32
	Node     uint32
	Port     uint32
}

func runRawNRScan(ctx context.Context, arfcns []uint32, scanType uint32, emit func(qmiEvent)) (returnErr error) {
	request, err := makeNRIncrementalScanRequest(1, arfcns, scanType)
	if err != nil {
		return err
	}
	fd, err := syscall.Socket(afQIPCRTR, syscall.SOCK_DGRAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open AF_QIPCRTR socket: %w", err)
	}
	defer syscall.Close(fd)
	if err := setQRTRTimeout(fd, 250*time.Millisecond); err != nil {
		return err
	}
	local, err := getQRTRSockName(fd)
	if err != nil {
		return err
	}
	server, err := lookupNAS(fd, local)
	if err != nil {
		return err
	}
	destination := sockaddrQRTR{Family: afQIPCRTR, Node: server.Node, Port: server.Port}
	emit(qmiEvent{At: time.Now().UTC(), Service: "3", Event: "RAW_QRTR", Direction: "tx", Kind: "request", MessageID: "0x0085", Transaction: 1, Raw: hex.EncodeToString(request), Description: fmt.Sprintf("NAS node=%d port=%d", server.Node, server.Port)})
	if err := sendQRTR(fd, request, &destination); err != nil {
		return fmt.Errorf("send NAS 0x0085: %w", err)
	}
	scanMayBeActive := true
	defer func() {
		if !scanMayBeActive {
			return
		}
		abortErr := abortNRScan(fd, server, 2, emit)
		if abortErr != nil && returnErr == nil && !errors.Is(abortErr, context.Canceled) {
			returnErr = abortErr
		}
	}()
	responseSeen := false
	for {
		select {
		case <-ctx.Done():
			if responseSeen {
				// Cancellation after an accepted request is the probe's normal
				// bounded-stop path. The deferred NAS abort remains authoritative.
				return nil
			}
			return fmt.Errorf("NAS 0x0085 response not received: %w", ctx.Err())
		default:
		}
		packet := make([]byte, 65535)
		n, from, recvErr := recvQRTR(fd, packet)
		if recvErr != nil {
			if errors.Is(recvErr, syscall.EINTR) || errors.Is(recvErr, syscall.EAGAIN) || errors.Is(recvErr, syscall.EWOULDBLOCK) {
				continue
			}
			return fmt.Errorf("receive NAS 0x0085: %w", recvErr)
		}
		if from.Node != server.Node || from.Port != server.Port || n < 7 {
			continue
		}
		packet = packet[:n]
		flags, transaction, message, _, parseErr := parseQMIPacket(packet)
		if parseErr != nil || message != qmiIncrementalScan {
			continue
		}
		event := qmiEvent{At: time.Now().UTC(), Service: "3", Event: "RAW_QRTR", Direction: "rx", MessageID: "0x0085", Transaction: transaction, Raw: hex.EncodeToString(packet)}
		switch flags {
		case qmiFlagResponse:
			event.Kind = "response"
			result, qmiError, resultErr := qmiResult(packet)
			if resultErr != nil {
				return resultErr
			}
			event.Result, event.Error = &result, &qmiError
			emit(event)
			if result != 0 {
				scanMayBeActive = false
				return fmt.Errorf("NAS 0x0085 rejected: QMI error %d", qmiError)
			}
			responseSeen = true
		case qmiFlagIndication:
			event.Kind = "indication"
			status, ok, statusErr := indicationStatus(packet)
			if statusErr != nil {
				return statusErr
			}
			if ok {
				event.ScanStatus = &status
			}
			emit(event)
			if ok && (status == 0 || status == 2) {
				scanMayBeActive = false
				return nil
			}
		}
	}
}

func lookupNAS(fd int, local sockaddrQRTR) (qrtrServer, error) {
	request := make([]byte, 20)
	binary.LittleEndian.PutUint32(request[0:4], qrtrTypeNewLook)
	binary.LittleEndian.PutUint32(request[4:8], nasService)
	destination := sockaddrQRTR{Family: afQIPCRTR, Node: local.Node, Port: qrtrPortCtrl}
	if err := sendQRTR(fd, request, &destination); err != nil {
		return qrtrServer{}, err
	}
	for attempts := 0; attempts < 64; attempts++ {
		packet := make([]byte, 256)
		n, _, err := recvQRTR(fd, packet)
		if err != nil {
			if errors.Is(err, syscall.EINTR) || errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				continue
			}
			return qrtrServer{}, fmt.Errorf("receive QRTR lookup: %w", err)
		}
		if n < 20 || binary.LittleEndian.Uint32(packet[0:4]) != qrtrTypeNewServ {
			continue
		}
		serviceID := binary.LittleEndian.Uint32(packet[4:8])
		instanceField := binary.LittleEndian.Uint32(packet[8:12])
		node := binary.LittleEndian.Uint32(packet[12:16])
		port := binary.LittleEndian.Uint32(packet[16:20])
		if serviceID == nasService && node != 0 && port != 0 {
			return qrtrServer{Version: instanceField & 0xff, Instance: instanceField >> 8, Node: node, Port: port}, nil
		}
	}
	return qrtrServer{}, fmt.Errorf("NAS QRTR service not found")
}

func abortNRScan(fd int, server qrtrServer, transaction uint16, emit func(qmiEvent)) error {
	if err := setQRTRTimeout(fd, 500*time.Millisecond); err != nil {
		return err
	}
	request, err := makeAbortRequest(transaction)
	if err != nil {
		return err
	}
	destination := sockaddrQRTR{Family: afQIPCRTR, Node: server.Node, Port: server.Port}
	emit(qmiEvent{At: time.Now().UTC(), Service: "3", Event: "RAW_QRTR", Direction: "tx", Kind: "request", MessageID: "0x00c2", Transaction: transaction, Raw: hex.EncodeToString(request), Description: "bounded scan cleanup"})
	if err := sendQRTR(fd, request, &destination); err != nil {
		return err
	}
	for attempts := 0; attempts < 12; attempts++ {
		packet := make([]byte, 65535)
		n, from, recvErr := recvQRTR(fd, packet)
		if recvErr != nil {
			if errors.Is(recvErr, syscall.EINTR) || errors.Is(recvErr, syscall.EAGAIN) || errors.Is(recvErr, syscall.EWOULDBLOCK) {
				continue
			}
			return recvErr
		}
		if from.Node != server.Node || from.Port != server.Port || n < 7 {
			continue
		}
		packet = packet[:n]
		flags, txn, message, _, parseErr := parseQMIPacket(packet)
		if parseErr != nil || flags != qmiFlagResponse || txn != transaction || message != qmiAbortNetworkScan {
			continue
		}
		result, qmiError, resultErr := qmiResult(packet)
		if resultErr != nil {
			return resultErr
		}
		event := qmiEvent{At: time.Now().UTC(), Service: "3", Event: "RAW_QRTR", Direction: "rx", Kind: "response", MessageID: "0x00c2", Transaction: transaction, Raw: hex.EncodeToString(packet), Result: &result, Error: &qmiError}
		emit(event)
		if result != 0 {
			return fmt.Errorf("NAS abort rejected: QMI error %d", qmiError)
		}
		return nil
	}
	return fmt.Errorf("NAS abort response not received")
}

func getQRTRSockName(fd int) (sockaddrQRTR, error) {
	address := sockaddrQRTR{}
	addressLength := uint32(unsafe.Sizeof(address))
	_, _, errno := syscall.RawSyscall(syscall.SYS_GETSOCKNAME,
		uintptr(fd), uintptr(unsafe.Pointer(&address)), uintptr(unsafe.Pointer(&addressLength)))
	if errno != 0 {
		return sockaddrQRTR{}, errno
	}
	return address, nil
}

func sendQRTR(fd int, payload []byte, address *sockaddrQRTR) error {
	if len(payload) == 0 {
		return fmt.Errorf("refusing empty QRTR payload")
	}
	_, _, errno := syscall.RawSyscall6(syscall.SYS_SENDTO,
		uintptr(fd), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0,
		uintptr(unsafe.Pointer(address)), unsafe.Sizeof(*address))
	if errno != 0 {
		return errno
	}
	return nil
}

func recvQRTR(fd int, payload []byte) (int, sockaddrQRTR, error) {
	address := sockaddrQRTR{}
	addressLength := uint32(unsafe.Sizeof(address))
	n, _, errno := syscall.RawSyscall6(syscall.SYS_RECVFROM,
		uintptr(fd), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0,
		uintptr(unsafe.Pointer(&address)), uintptr(unsafe.Pointer(&addressLength)))
	if errno != 0 {
		return 0, sockaddrQRTR{}, errno
	}
	return int(n), address, nil
}

func setQRTRTimeout(fd int, timeout time.Duration) error {
	timeval := syscall.NsecToTimeval(timeout.Nanoseconds())
	return syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeval)
}
