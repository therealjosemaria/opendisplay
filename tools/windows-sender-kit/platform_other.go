//go:build !windows

package main

func newPlatform() platform { panic("Windows only; tests inject an isolated backend") }
