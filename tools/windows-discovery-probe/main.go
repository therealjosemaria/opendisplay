// Discovery experiment only. No phone connection, capture, driver, or service.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"
)

const service = "_display._tcp.local."
const instance = "OpenDisplay Discovery Test._display._tcp.local."
const hostname = "opendisplay-discovery-test.local."

func name(s string) []byte {
	var b []byte
	for _, label := range strings.Split(strings.TrimSuffix(s, "."), ".") {
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	return append(b, 0)
}

func record(owner string, kind uint16, data []byte, ttl uint32) []byte {
	b := name(owner)
	h := make([]byte, 10)
	binary.BigEndian.PutUint16(h, kind)
	binary.BigEndian.PutUint16(h[2:], 1)
	binary.BigEndian.PutUint32(h[4:], ttl)
	binary.BigEndian.PutUint16(h[8:], uint16(len(data)))
	return append(append(b, h...), data...)
}

func advertisement(ip net.IP, ttl uint32) []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint16(b[2:], 0x8400)
	binary.BigEndian.PutUint16(b[6:], 4)
	b = append(b, record(service, 12, name(instance), ttl)...)
	srv := make([]byte, 6)
	binary.BigEndian.PutUint16(srv[4:], 7250)
	b = append(b, record(instance, 33, append(srv, name(hostname)...), ttl)...)
	txt := "container_id=757cd548-8215-4da0-909b-20e0609cfc1a"
	b = append(b, record(instance, 16, append([]byte{byte(len(txt))}, txt...), ttl)...)
	return append(b, record(hostname, 1, ip.To4(), ttl)...)
}

// Decode DNS labels with bounded compression traversal, including malformed input.
func readName(b []byte, at int) (string, int, bool) {
	var labels []string
	next := -1
	for hops := 0; hops < 128; hops++ {
		if at < 0 || at >= len(b) {
			return "", 0, false
		}
		n := int(b[at])
		at++
		if n == 0 {
			if next < 0 {
				next = at
			}
			return strings.ToLower(strings.Join(labels, ".") + "."), next, true
		}
		if n&0xc0 == 0xc0 {
			if at >= len(b) {
				return "", 0, false
			}
			if next < 0 {
				next = at + 1
			}
			at = ((n & 63) << 8) | int(b[at])
			continue
		}
		if n > 63 || at+n > len(b) {
			return "", 0, false
		}
		labels = append(labels, string(b[at:at+n]))
		at += n
	}
	return "", 0, false
}

func relevantQuery(b []byte) bool {
	if len(b) < 12 || b[2]&0x80 != 0 {
		return false
	}
	at := 12
	for i := 0; i < int(binary.BigEndian.Uint16(b[4:])); i++ {
		s, next, ok := readName(b, at)
		if !ok || next+4 > len(b) {
			return false
		}
		if s == service || s == strings.ToLower(instance) || s == hostname {
			return true
		}
		at = next + 4
	}
	return false
}

func localInterface(value string) (net.IP, *net.Interface, error) {
	ip := net.ParseIP(value).To4()
	if value == "" {
		c, err := net.Dial("udp4", "224.0.0.251:5353")
		if err != nil {
			return nil, nil, err
		}
		ip = c.LocalAddr().(*net.UDPAddr).IP.To4()
		c.Close()
	}
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return nil, nil, fmt.Errorf("choose your physical Wi-Fi IPv4 address with -ip")
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		for _, a := range addresses {
			p, _, _ := net.ParseCIDR(a.String())
			if p.Equal(ip) {
				return ip, &iface, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("address does not belong to this laptop")
}

func run() error {
	value := flag.String("ip", "", "physical Wi-Fi IPv4 address (normally automatic)")
	seconds := flag.Int("seconds", 90, "experiment duration, 10–300 seconds")
	flag.Parse()
	if *seconds < 10 || *seconds > 300 {
		return fmt.Errorf("duration must be 10–300 seconds")
	}
	ip, iface, err := localInterface(*value)
	if err != nil {
		return err
	}
	group := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	u, err := net.ListenMulticastUDP("udp4", iface, group)
	if err != nil {
		return fmt.Errorf("mDNS unavailable: %w; do not disable security settings", err)
	}
	defer u.Close()
	if err := configureMulticast(u); err != nil {
		return fmt.Errorf("multicast setup failed: %w", err)
	}
	// Bind only the selected local address. No port scan and no phone connection.
	tcp, err := net.Listen("tcp4", net.JoinHostPort(ip.String(), "7250"))
	if err != nil {
		return fmt.Errorf("test listener unavailable: %w", err)
	}
	defer tcp.Close()
	var queries, connections atomic.Uint32
	go func() {
		for {
			c, err := tcp.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			fmt.Println("A source contacted the test listener. Video is not implemented; closing it.")
			c.Close()
		}
	}()
	go func() {
		b := make([]byte, 9000)
		for {
			n, peer, err := u.ReadFromUDP(b)
			if err != nil {
				return
			}
			if relevantQuery(b[:n]) {
				queries.Add(1)
				fmt.Println("Received a display-discovery query (this alone does not prove Win+K visibility).")
				reply := group
				if peer.Port != 5353 {
					reply = peer
				}
				u.WriteToUDP(advertisement(ip, 120), reply)
			}
		}
	}()
	fmt.Printf("OpenDisplay DISCOVERY experiment — no screen sharing yet\nInterface: %s (%s)\n", iface.Name, ip)
	fmt.Println("Press Win+K. Look for 'OpenDisplay Discovery Test'. Do not select it yet.")
	fmt.Println("No drivers, services, firewall or registry changes. Closes automatically; Ctrl+C also stops.")
	if _, err := u.WriteToUDP(advertisement(ip, 120), group); err != nil {
		return err
	}
	defer u.WriteToUDP(advertisement(ip, 0), group)
	timer := time.NewTimer(time.Duration(*seconds) * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)
	for {
		select {
		case <-tick.C:
			u.WriteToUDP(advertisement(ip, 120), group)
		case <-timer.C:
			fmt.Printf("Finished. Display queries: %d. Source contacts: %d. Tell us whether the name appeared in Win+K.\n", queries.Load(), connections.Load())
			return nil
		case <-stop:
			return nil
		}
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
