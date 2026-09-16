//go:build windows

package main

import (
	"encoding/base64"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var pCryptProtectData = crypt32.NewProc("CryptProtectData")
var pCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
var pLocalFree = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")

func protectKey(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	b := []byte(s)
	in := dataBlob{uint32(len(b)), &b[0]}
	var out dataBlob
	r, _, e := pCryptProtectData.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return "", e
	}
	defer pLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	raw := unsafe.Slice(out.pbData, out.cbData)
	cpy := append([]byte(nil), raw...)
	return base64.StdEncoding.EncodeToString(cpy), nil
}
func unprotectKey(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	raw, e := base64.StdEncoding.DecodeString(s)
	if e != nil {
		return "", e
	}
	if len(raw) == 0 {
		return "", nil
	}
	in := dataBlob{uint32(len(raw)), &raw[0]}
	var out dataBlob
	r, _, er := pCryptUnprotectData.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return "", er
	}
	defer pLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	b := unsafe.Slice(out.pbData, out.cbData)
	return string(append([]byte(nil), b...)), nil
}
