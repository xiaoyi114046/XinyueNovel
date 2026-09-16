//go:build !windows

package main

func protectKey(s string) (string, error)   { return encodePlainKey(s), nil }
func unprotectKey(s string) (string, error) { return decodePlainKey(s), nil }
