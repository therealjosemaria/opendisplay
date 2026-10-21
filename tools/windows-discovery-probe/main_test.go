package main

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func query(s string) []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint16(b[4:], 1)
	return append(append(b, name(s)...), 0, 12, 0, 1)
}

func TestQuery(t *testing.T) {
	for _, s := range []string{service, instance, hostname, "_DISPLAY._TCP.LOCAL."} {
		if !relevantQuery(query(s)) {
			t.Fatal(s)
		}
	}
	for _, b := range [][]byte{nil, {0}, query("_airplay._tcp.local."), advertisement(net.IPv4(192, 0, 2, 1), 120)} {
		if relevantQuery(b) {
			t.Fatal("unrelated or nonquery packet accepted")
		}
	}
}

func TestMalformedDNS(t *testing.T) {
	b := query(service)
	for n := 0; n < len(b); n++ {
		if relevantQuery(b[:n]) {
			t.Fatalf("truncated packet accepted: %d", n)
		}
	}
	for _, raw := range [][]byte{{0xc0, 0}, {64, 0}, {0xc0, 255}, {0xff}} {
		if _, _, ok := readName(raw, 0); ok {
			t.Fatal("malformed name accepted")
		}
	}
	compressed := append(name(service), 0xc0, 0)
	if s, _, ok := readName(compressed, len(name(service))); !ok || s != service {
		t.Fatal("valid compression rejected")
	}
}

func TestAdvertisement(t *testing.T) {
	for _, ttl := range []uint32{0, 120} {
		b := advertisement(net.IPv4(192, 0, 2, 42), ttl)
		if binary.BigEndian.Uint16(b[2:]) != 0x8400 || binary.BigEndian.Uint16(b[6:]) != 4 {
			t.Fatal("invalid header")
		}
		at := 12
		for _, kind := range []uint16{12, 33, 16, 1} {
			_, next, ok := readName(b, at)
			if !ok || next+10 > len(b) {
				t.Fatal("record truncated")
			}
			if binary.BigEndian.Uint16(b[next:]) != kind || binary.BigEndian.Uint32(b[next+4:]) != ttl {
				t.Fatal("record metadata")
			}
			size := int(binary.BigEndian.Uint16(b[next+8:]))
			data := b[next+10 : next+10+size]
			if kind == 33 && binary.BigEndian.Uint16(data[4:]) != 7250 {
				t.Fatal("wrong MICE port")
			}
			if kind == 16 && (int(data[0]) != len(data)-1 || string(data[1:14]) != "container_id=") {
				t.Fatal("invalid TXT")
			}
			if kind == 1 && !net.IP(data).Equal(net.IPv4(192, 0, 2, 42)) {
				t.Fatal("wrong local address")
			}
			at = next + 10 + size
		}
		if at != len(b) {
			t.Fatal("unexpected trailing bytes")
		}
	}
}

func FuzzQuery(f *testing.F) {
	f.Add(query(service))
	f.Add([]byte{0xc0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) { relevantQuery(b) })
}

func TestUDPQueryRoundTrip(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err = configureMulticast(server); err != nil {
		t.Fatal(err)
	}
	server.SetDeadline(time.Now().Add(2 * time.Second))
	client, err := net.DialUDP("udp4", nil, server.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err = client.Write(query(service)); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 9000)
	n, peer, err := server.ReadFromUDP(b)
	if err != nil || !relevantQuery(b[:n]) {
		t.Fatalf("query round trip: %v", err)
	}
	response := advertisement(net.IPv4(192, 0, 2, 42), 120)
	if _, err = server.WriteToUDP(response, peer); err != nil {
		t.Fatal(err)
	}
	n, err = client.Read(b)
	if err != nil || string(b[:n]) != string(response) {
		t.Fatalf("response round trip: %v", err)
	}
}
