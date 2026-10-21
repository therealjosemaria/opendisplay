package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakePlatform struct {
	calls          []string
	ready, running bool
	fail           string
	config         string
}

func (f *fakePlatform) call(name string) error {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return errors.New("simulated " + name + " failure")
	}
	return nil
}
func (f *fakePlatform) DriverReady() (bool, error)   { return f.ready, f.call("driver-check") }
func (f *fakePlatform) SenderRunning() (bool, error) { return f.running, f.call("running-check") }
func (f *fakePlatform) VerifyDriver(string) error    { return f.call("signature") }
func (f *fakePlatform) Elevated(string) error        { return f.call("installer") }
func (f *fakePlatform) ActivateDriver() error {
	err := f.call("activate")
	if err == nil {
		f.ready = true
	}
	return err
}
func (f *fakePlatform) StartSender(string) error    { return f.call("start") }
func (f *fakePlatform) ConfigPath() (string, error) { return f.config, f.call("config") }
func hash(b []byte) string                          { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func options(t *testing.T, f *fakePlatform) setupOptions {
	t.Helper()
	root := t.TempDir()
	body := []byte("isolated sender fixture")
	if err := os.WriteFile(filepath.Join(root, "opendisplay-win.exe"), body, 0600); err != nil {
		t.Fatal(err)
	}
	f.config = filepath.Join(root, "appdata", "config.json")
	return setupOptions{root: root, hash: hash(body), p: f, input: strings.NewReader("100.100.1.2\n"), output: &bytes.Buffer{}, download: func(string) error { return f.call("download") }}
}

func TestFreshSetup(t *testing.T) {
	f := &fakePlatform{}
	o := options(t, f)
	if err := setup(o); err != nil {
		t.Fatal(err)
	}
	want := "running-check,driver-check,download,signature,installer,activate,driver-check,config,start"
	if strings.Join(f.calls, ",") != want {
		t.Fatal(f.calls)
	}
	b, _ := os.ReadFile(f.config)
	var cfg map[string]any
	if json.Unmarshal(b, &cfg) != nil || cfg["ip"] != "100.100.1.2" || cfg["autoReconnect"] != true {
		t.Fatal(string(b))
	}
	if strings.Contains(o.output.(*bytes.Buffer).String(), "Installation successful") {
		t.Fatal("must not claim playback proof")
	}
}

func TestExistingDriverAndSettings(t *testing.T) {
	f := &fakePlatform{ready: true}
	o := options(t, f)
	os.MkdirAll(filepath.Dir(f.config), 0700)
	before := []byte(`{"ip":"192.168.1.5","unknownSetting":"keep"}`)
	os.WriteFile(f.config, before, 0600)
	if err := setup(o); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.calls, ","), "installer") {
		t.Fatal("existing healthy driver must not be reinstalled")
	}
	b, _ := os.ReadFile(f.config)
	if !bytes.Contains(b, []byte("keep")) {
		t.Fatal("setting lost")
	}
	backups, _ := filepath.Glob(f.config + ".before-setup-*")
	if len(backups) != 1 {
		t.Fatal(backups)
	}
	b, _ = os.ReadFile(backups[0])
	if !bytes.Equal(b, before) {
		t.Fatal("backup changed")
	}
}

func TestFailuresStopBeforeLaunchingSender(t *testing.T) {
	for _, step := range []string{"running-check", "driver-check", "download", "signature", "installer", "activate", "config"} {
		t.Run(step, func(t *testing.T) {
			f := &fakePlatform{fail: step}
			o := options(t, f)
			if err := setup(o); err == nil {
				t.Fatal("failure accepted")
			}
			for _, call := range f.calls {
				if call == "start" {
					t.Fatal("sender launched after failure")
				}
			}
		})
	}
}

func TestRejectModifiedSenderAndRunningInstance(t *testing.T) {
	f := &fakePlatform{}
	o := options(t, f)
	o.hash = strings.Repeat("0", 64)
	if err := setup(o); err == nil || len(f.calls) != 0 {
		t.Fatal("modified payload reached backend")
	}
	f = &fakePlatform{running: true}
	o = options(t, f)
	if err := setup(o); err == nil || strings.Join(f.calls, ",") != "running-check" {
		t.Fatal("running sender disturbed")
	}
}

func TestBadAddressAndBadConfigPreserveSettings(t *testing.T) {
	for _, ip := range []string{"", "127.0.0.1", "::1", "2001:db8::1", "0.0.0.0", "224.0.0.251", "255.255.255.255", "1.2.3.4; whoami", "https://1.2.3.4"} {
		if _, err := phoneAddress(ip); err == nil {
			t.Fatal(ip)
		}
	}
	for _, ip := range []string{"192.168.1.2", "10.129.99.2", "100.106.150.88"} {
		if _, err := phoneAddress(ip); err != nil {
			t.Fatal(err)
		}
	}
	f := &fakePlatform{ready: true}
	o := options(t, f)
	os.MkdirAll(filepath.Dir(f.config), 0700)
	for _, bad := range []string{"broken", "null"} {
		os.WriteFile(f.config, []byte(bad), 0600)
		if err := setup(o); err == nil {
			t.Fatal("bad config accepted")
		}
		b, _ := os.ReadFile(f.config)
		if string(b) != bad {
			t.Fatal("bad config overwritten")
		}
	}
}

func TestPinnedDownloadAndCorruption(t *testing.T) {
	body := []byte("official download fixture")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "driver.exe")
	if err := fetch(server.Client(), server.URL, dest, hash(body)); err != nil {
		t.Fatal(err)
	}
	if err := verify(dest, hash(body)); err != nil {
		t.Fatal(err)
	}
	os.Remove(dest)
	if err := fetch(server.Client(), server.URL, dest, strings.Repeat("0", 64)); err == nil {
		t.Fatal("corrupt download accepted")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("corrupt executable written")
	}
}

func TestPowerShellPathQuoting(t *testing.T) {
	if got := psQuote(`C:\O'Brien\$(whoami);data.exe`); got != `'C:\O''Brien\$(whoami);data.exe'` {
		t.Fatal(got)
	}
}
