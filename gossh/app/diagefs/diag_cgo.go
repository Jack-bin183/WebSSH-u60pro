//go:build linux && cgo

package diagefs

/*
#cgo LDFLAGS: -ldl

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <dlfcn.h>

#define DIAG_DCI_NO_ERROR 1001
#define DIAG_RSP_MAX 8192

typedef int (*Diag_LSM_Init_t)(void *);
typedef int (*Diag_LSM_DeInit_t)(void);
typedef int (*diag_register_dci_client_t)(int *, void *, int, void *, void *);
typedef int (*diag_get_dci_support_list_t)(void *);
typedef int (*diag_send_dci_async_req_t)(int, void *, int, void *, int, void *, void *);
typedef int (*diag_release_dci_client_t)(int *);

static void *diag_handle;
static Diag_LSM_Init_t p_Diag_LSM_Init;
static Diag_LSM_DeInit_t p_Diag_LSM_DeInit;
static diag_register_dci_client_t p_diag_register_dci_client;
static diag_get_dci_support_list_t p_diag_get_dci_support_list;
static diag_send_dci_async_req_t p_diag_send_dci_async_req;
static diag_release_dci_client_t p_diag_release_dci_client;

static int dci_ready;
static int dci_client_id;
static unsigned char dci_reg_buf[256];
static unsigned char dci_signal_data[128];

static volatile int rsp_ready;
static int rsp_len;
static unsigned char rsp_buf[DIAG_RSP_MAX];

static void noop_cb(void *buf, int len, void *data) {
	(void)buf;
	(void)len;
	(void)data;
}

static void dci_rsp_cb(void *buf, int len, void *data) {
	(void)data;
	if (buf == NULL || len <= 0) {
		rsp_len = 0;
		rsp_ready = 1;
		return;
	}
	if (len > DIAG_RSP_MAX) {
		len = DIAG_RSP_MAX;
	}
	memcpy(rsp_buf, buf, (size_t)len);
	rsp_len = len;
	rsp_ready = 1;
}

static const char *last_dl_error(void) {
	const char *err = dlerror();
	return err ? err : "unknown dlerror";
}

static int load_sym(void **dst, const char *name, char *err, int err_len) {
	*dst = dlsym(diag_handle, name);
	if (*dst == NULL) {
		snprintf(err, (size_t)err_len, "dlsym(%s): %s", name, last_dl_error());
		return -1;
	}
	return 0;
}

static int diag_open_c(const char *path, char *err, int err_len) {
	if (diag_handle != NULL) {
		return 0;
	}

	if (path != NULL && path[0] != '\0') {
		diag_handle = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
	} else {
		diag_handle = dlopen("libdiag.so.1", RTLD_NOW | RTLD_GLOBAL);
		if (diag_handle == NULL) {
			diag_handle = dlopen("libdiag.so", RTLD_NOW | RTLD_GLOBAL);
		}
	}
	if (diag_handle == NULL) {
		snprintf(err, (size_t)err_len, "dlopen libdiag failed: %s", last_dl_error());
		return -1;
	}

	if (load_sym((void **)&p_Diag_LSM_Init, "Diag_LSM_Init", err, err_len) != 0) return -1;
	if (load_sym((void **)&p_Diag_LSM_DeInit, "Diag_LSM_DeInit", err, err_len) != 0) return -1;
	if (load_sym((void **)&p_diag_register_dci_client, "diag_register_dci_client", err, err_len) != 0) return -1;
	if (load_sym((void **)&p_diag_get_dci_support_list, "diag_get_dci_support_list", err, err_len) != 0) return -1;
	if (load_sym((void **)&p_diag_send_dci_async_req, "diag_send_dci_async_req", err, err_len) != 0) return -1;
	if (load_sym((void **)&p_diag_release_dci_client, "diag_release_dci_client", err, err_len) != 0) return -1;
	return 0;
}

static int diag_init_dci_c(char *err, int err_len) {
	int rc;

	if (dci_ready) {
		return 0;
	}
	if (diag_handle == NULL) {
		snprintf(err, (size_t)err_len, "libdiag is not loaded");
		return -1;
	}

	memset(dci_reg_buf, 0, sizeof(dci_reg_buf));
	memset(dci_signal_data, 0, sizeof(dci_signal_data));

	rc = p_Diag_LSM_Init(NULL);
	if (rc == 0) {
		snprintf(err, (size_t)err_len, "Diag_LSM_Init failed");
		return -1;
	}

	rc = p_diag_register_dci_client(&dci_client_id, dci_reg_buf, 0, dci_signal_data, (void *)noop_cb);
	if (rc != DIAG_DCI_NO_ERROR) {
		snprintf(err, (size_t)err_len, "diag_register_dci_client=%d", rc);
		return -1;
	}

	rc = p_diag_get_dci_support_list(dci_reg_buf);
	if (rc != DIAG_DCI_NO_ERROR) {
		snprintf(err, (size_t)err_len, "diag_get_dci_support_list=%d", rc);
		return -1;
	}

	dci_ready = 1;
	return 0;
}

static void diag_close_c(void) {
	if (dci_ready && p_diag_release_dci_client != NULL) {
		p_diag_release_dci_client(&dci_client_id);
		dci_ready = 0;
	}
	if (p_Diag_LSM_DeInit != NULL) {
		p_Diag_LSM_DeInit();
	}
	if (diag_handle != NULL) {
		dlclose(diag_handle);
		diag_handle = NULL;
	}
}

static void put_le32(unsigned char *buf, int *off, uint32_t v) {
	buf[(*off)++] = (unsigned char)(v);
	buf[(*off)++] = (unsigned char)(v >> 8);
	buf[(*off)++] = (unsigned char)(v >> 16);
	buf[(*off)++] = (unsigned char)(v >> 24);
}

static uint32_t get_le32(const unsigned char *buf) {
	return ((uint32_t)buf[0]) |
		((uint32_t)buf[1] << 8) |
		((uint32_t)buf[2] << 16) |
		((uint32_t)buf[3] << 24);
}

static int efs_rsp_off(void) {
	uint32_t cmd;
	if (rsp_len < 4) {
		return 0;
	}
	cmd = get_le32(rsp_buf);
	if ((cmd & 0x0000ffffu) == 0x134bu) {
		return 4;
	}
	return 0;
}

static int diag_send_wait_c(const unsigned char *req, int req_len, char *err, int err_len) {
	int rc;
	time_t start;
	struct timespec ts;

	if (!dci_ready) {
		snprintf(err, (size_t)err_len, "DCI client is not initialized");
		return -1;
	}

	rsp_ready = 0;
	rsp_len = 0;
	memset(rsp_buf, 0, sizeof(rsp_buf));

	rc = p_diag_send_dci_async_req(dci_client_id, (void *)req, req_len, rsp_buf, DIAG_RSP_MAX, (void *)dci_rsp_cb, NULL);
	if (rc != DIAG_DCI_NO_ERROR) {
		snprintf(err, (size_t)err_len, "diag_send_dci_async_req=%d", rc);
		return -1;
	}

	start = time(NULL);
	ts.tv_sec = 0;
	ts.tv_nsec = 50 * 1000 * 1000;
	while (!rsp_ready) {
		nanosleep(&ts, NULL);
		if (time(NULL) - start > 5) {
			snprintf(err, (size_t)err_len, "timeout waiting diag reply");
			return -1;
		}
	}
	return rsp_len;
}

static int efs_open_c(const char *path, int flags, int mode, char *err, int err_len) {
	unsigned char req[8192];
	int off = 0;
	size_t path_len = strlen(path) + 1;
	int n;
	int fd;
	int efs_errno;
	int roff;

	if (path_len + 12 > sizeof(req)) {
		snprintf(err, (size_t)err_len, "path too long");
		return -1;
	}

	put_le32(req, &off, 0x0002134b);
	put_le32(req, &off, (uint32_t)flags);
	put_le32(req, &off, (uint32_t)mode);
	memcpy(req + off, path, path_len);
	off += (int)path_len;

	n = diag_send_wait_c(req, off, err, err_len);
	roff = efs_rsp_off();
	if (n - roff <= 7) {
		snprintf(err, (size_t)err_len, "efs open failed: short response len=%d", n);
		return -1;
	}

	fd = (int)get_le32(rsp_buf + roff + 0);
	efs_errno = (int)get_le32(rsp_buf + roff + 4);
	if (efs_errno != 0 || fd == -1) {
		snprintf(err, (size_t)err_len, "efs open failed: fd=%d errno=%d", fd, efs_errno);
		return -1;
	}
	return fd;
}

static int efs_close_c(int fd, char *err, int err_len) {
	unsigned char req[16];
	int off = 0;
	int n;

	put_le32(req, &off, 0x0003134b);
	put_le32(req, &off, (uint32_t)fd);
	n = diag_send_wait_c(req, off, err, err_len);
	if (n < 0) return -1;
	return 0;
}

static int efs_write_c(int fd, int offset, const unsigned char *data, int len, char *err, int err_len) {
	unsigned char req[8192];
	int off = 0;
	int n;
	int roff;
	int rsp_fd;
	int rsp_offset;
	int written;
	int efs_errno;

	if (len < 0 || len > 4096) {
		snprintf(err, (size_t)err_len, "efs write invalid chunk len=%d", len);
		return -1;
	}
	if (len + 12 > (int)sizeof(req)) {
		snprintf(err, (size_t)err_len, "efs write request too large");
		return -1;
	}

	put_le32(req, &off, 0x0005134b);
	put_le32(req, &off, (uint32_t)fd);
	put_le32(req, &off, (uint32_t)offset);
	memcpy(req + off, data, (size_t)len);
	off += len;

	n = diag_send_wait_c(req, off, err, err_len);
	roff = efs_rsp_off();
	if (n - roff < 16) {
		snprintf(err, (size_t)err_len, "efs write failed: short response len=%d", n);
		return -1;
	}

	rsp_fd = (int)get_le32(rsp_buf + roff + 0);
	rsp_offset = (int)get_le32(rsp_buf + roff + 4);
	written = (int)get_le32(rsp_buf + roff + 8);
	efs_errno = (int)get_le32(rsp_buf + roff + 12);
	if (efs_errno != 0 || written < 0) {
		snprintf(err, (size_t)err_len, "efs write failed: fd=%d offset=%d written=%d errno=%d",
			rsp_fd, rsp_offset, written, efs_errno);
		return -1;
	}
	return written;
}

static int efs_write_file_c(const char *path, const unsigned char *data, int len, char *err, int err_len) {
	int fd;
	int total = 0;
	int chunk = 4096;

	if (len < 0) {
		snprintf(err, (size_t)err_len, "invalid write length");
		return -1;
	}

	fd = efs_open_c(path, 0x241, 0100666, err, err_len);
	if (fd < 0) {
		return -1;
	}

	while (total < len) {
		int want = len - total;
		int written;
		if (want > chunk) {
			want = chunk;
		}
		written = efs_write_c(fd, total, data + total, want, err, err_len);
		if (written <= 0) {
			efs_close_c(fd, err, err_len);
			if (written == 0) {
				snprintf(err, (size_t)err_len, "efs write made no progress at offset=%d", total);
			}
			return -1;
		}
		total += written;
	}

	if (efs_close_c(fd, err, err_len) != 0) {
		return -1;
	}
	return total;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func diagOpen(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	errBuf := make([]byte, 512)
	rc := C.diag_open_c(cpath, (*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	if rc != 0 {
		return errors.New(cString(errBuf))
	}
	return nil
}

func diagInitDCI() error {
	errBuf := make([]byte, 512)
	rc := C.diag_init_dci_c((*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	if rc != 0 {
		return errors.New(cString(errBuf))
	}
	return nil
}

func diagClose() {
	C.diag_close_c()
}

func efsWriteFile(path string, data []byte) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	errBuf := make([]byte, 512)
	var ptr *C.uchar
	if len(data) > 0 {
		ptr = (*C.uchar)(unsafe.Pointer(&data[0]))
	}
	n := C.efs_write_file_c(cpath, ptr, C.int(len(data)), (*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	if n < 0 {
		return errors.New(cString(errBuf))
	}
	if int(n) != len(data) {
		return errors.New("short EFS write")
	}
	return nil
}

func cString(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}
