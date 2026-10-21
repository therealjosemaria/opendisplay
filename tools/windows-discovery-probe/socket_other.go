//go:build !windows

package main

import (
	"net"
	"syscall"
)

func configureMulticast(c *net.UDPConn) error {
	raw, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var optionError error
	err = raw.Control(func(fd uintptr) {
		optionError = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_MULTICAST_LOOP, 1)
		if optionError == nil {
			optionError = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_MULTICAST_TTL, 255)
		}
	})
	if err != nil {
		return err
	}
	return optionError
}
