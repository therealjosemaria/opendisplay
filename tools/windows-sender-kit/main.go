// Setup wrapper for martinhoess/opendisplay-win v0.1.0, unchanged upstream binary.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const senderHash = "6598cba78a56a2c81108530f7b3c94ae30d4b1d4f4b144886505fe2618891244"
const driverHash = "e23332448fdaf5aa017cb308db5ef6855fac526a7ded05d80c039404126d5362"
const driverURL = "https://builds.parsec.app/vdd/parsec-vdd-0.45.0.0.exe"

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

type platform interface {
	DriverReady() (bool, error)
	SenderRunning() (bool, error)
	VerifyDriver(string) error
	Elevated(string) error
	ActivateDriver() error
	StartSender(string) error
	ConfigPath() (string, error)
}

func verify(path, expected string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("checksum mismatch: %s; file will not be run", filepath.Base(path))
	}
	return nil
}

func fetch(client *http.Client, url, dest, expected string) error {
	if verify(dest, expected) == nil {
		return nil
	}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, 20*1024*1024+1))
	if err != nil {
		return err
	}
	if len(b) > 20*1024*1024 {
		return errors.New("download exceeded size limit")
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != expected {
		return errors.New("download checksum mismatch; nothing was installed")
	}
	return os.WriteFile(dest, b, 0600)
}

func phoneAddress(value string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.Equal(net.IPv4bcast) {
		return "", errors.New("enter the iPhone's IPv4 address, such as 192.168.1.20 or its Tailscale address")
	}
	return ip.To4().String(), nil
}

func configure(path, ip string) error {
	values := map[string]any{}
	previous, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(previous, &values); err != nil || values == nil {
			return errors.New("existing sender settings are invalid; left unchanged")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	values["ip"] = ip
	values["port"] = 9000
	values["autoReconnect"] = true
	b, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if previous != nil {
		backup := path + ".before-setup-" + time.Now().UTC().Format("20060102T150405.000000000")
		if err = os.WriteFile(backup, previous, 0600); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

type setupOptions struct {
	root, hash string
	p          platform
	input      io.Reader
	output     io.Writer
	download   func(string) error
}

func setup(o setupOptions) error {
	sender := filepath.Join(o.root, "opendisplay-win.exe")
	if err := verify(sender, o.hash); err != nil {
		return err
	}
	running, err := o.p.SenderRunning()
	if err != nil {
		return err
	}
	if running {
		return errors.New("OpenDisplay Windows is already running. Use its tray icon to connect, or choose Exit before running setup again")
	}
	ready, err := o.p.DriverReady()
	if err != nil {
		return err
	}
	if !ready {
		fmt.Fprintln(o.output, "Installing the virtual display driver. Windows administrator approval is required.")
		fmt.Fprintln(o.output, "Complete the Parsec installer, then wait for setup to continue. No silent license acceptance.")
		driver := filepath.Join(o.root, "parsec-vdd-0.45.0.0.exe")
		if err = o.download(driver); err != nil {
			return err
		}
		if err = o.p.VerifyDriver(driver); err != nil {
			return err
		}
		if err = o.p.Elevated(driver); err != nil {
			return err
		}
		if err = o.p.ActivateDriver(); err != nil {
			return err
		}
		ready, err = o.p.DriverReady()
		if err != nil {
			return err
		}
		if !ready {
			return errors.New("Windows has not activated the display driver. If its installer requested a restart, save your work and restart before retrying")
		}
	}
	fmt.Fprintln(o.output, "Open OpenDisplay on the unlocked iPhone.")
	fmt.Fprintln(o.output, "Use the same reachable Wi-Fi, or enable Tailscale on both devices.")
	fmt.Fprintln(o.output, "Phone address: iPhone Settings > Wi-Fi > (i) > IP Address, or its address in Tailscale.")
	fmt.Fprint(o.output, "Enter iPhone IPv4 address: ")
	scanner := bufio.NewScanner(o.input)
	if !scanner.Scan() {
		return errors.New("no phone address entered; sender was not started")
	}
	ip, err := phoneAddress(scanner.Text())
	if err != nil {
		return err
	}
	config, err := o.p.ConfigPath()
	if err != nil {
		return err
	}
	if err = configure(config, ip); err != nil {
		return err
	}
	if err = o.p.StartSender(sender); err != nil {
		return err
	}
	fmt.Fprintln(o.output, "Sender started. The iPhone picture still needs your confirmation.")
	fmt.Fprintln(o.output, "Use Windows Settings > System > Display to arrange the second screen.")
	fmt.Fprintln(o.output, "To disconnect or quit, right-click the OpenDisplay icon beside the clock.")
	fmt.Fprintln(o.output, "This is not a Win+K connection. Audio stays on the laptop; this sender has no audio streaming.")
	return nil
}

func main() {
	if runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "Run this setup on Windows x64.")
		os.Exit(1)
	}
	fmt.Println("OpenDisplay Windows — real second-display setup")
	fmt.Println("Sender by martinhoess; iOS receiver by peetzweg. Setup helper only by therealjosemaria.")
	exe, err := os.Executable()
	if err == nil {
		client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) > 5 {
				return errors.New("unsafe download redirect")
			}
			return nil
		}}
		err = setup(setupOptions{root: filepath.Dir(exe), hash: senderHash, p: newPlatform(), input: os.Stdin, output: os.Stdout,
			download: func(dest string) error { return fetch(client, driverURL, dest, driverHash) }})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Setup stopped:", err)
		fmt.Fprintln(os.Stderr, "Do not disable security settings or university policies. Logs: %APPDATA%\\opendisplay-win\\log.txt")
	}
	fmt.Println("Press Enter to close setup.")
	bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		os.Exit(1)
	}
}
