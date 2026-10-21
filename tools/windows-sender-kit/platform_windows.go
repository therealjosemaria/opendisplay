package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsPlatform struct{}

func newPlatform() platform { return windowsPlatform{} }
func ps(script string) ([]byte, error) {
	return exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
}

func (w windowsPlatform) DriverReady() (bool, error) {
	b, err := ps(`$ErrorActionPreference='Stop'; $d=Get-CimInstance Win32_PnPEntity -Filter "Name='Parsec Virtual Display Adapter'"; if($d | Where-Object {$_.ConfigManagerErrorCode -eq 0}){exit 0}else{exit 3}`)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3 {
		return false, nil
	}
	return false, fmt.Errorf("cannot check the display driver: %s", b)
}
func (w windowsPlatform) SenderRunning() (bool, error) {
	b, err := ps(`if(Get-Process -Name 'opendisplay-win' -ErrorAction SilentlyContinue){'running'}else{'stopped'}`)
	if err != nil {
		return false, fmt.Errorf("cannot check sender: %s", b)
	}
	return strings.TrimSpace(string(b)) == "running", nil
}
func (w windowsPlatform) VerifyDriver(path string) error {
	b, err := ps(`$s=Get-AuthenticodeSignature -LiteralPath ` + psQuote(path) + `; @{status=[string]$s.Status;subject=[string]$s.SignerCertificate.Subject}|ConvertTo-Json -Compress`)
	if err != nil {
		return fmt.Errorf("cannot verify driver signature: %s", b)
	}
	var sig struct {
		Status  string `json:"status"`
		Subject string `json:"subject"`
	}
	if json.Unmarshal(b, &sig) != nil || sig.Status != "Valid" || !(strings.Contains(strings.ToLower(sig.Subject), "parsec") || strings.Contains(strings.ToLower(sig.Subject), "unity")) {
		return errors.New("driver signature is not a valid Parsec/Unity signature; installer will not run")
	}
	return nil
}
func (w windowsPlatform) Elevated(path string) error {
	b, err := ps(`$ErrorActionPreference='Stop'; $p=Start-Process -FilePath ` + psQuote(path) + ` -Verb RunAs -Wait -PassThru; exit $p.ExitCode`)
	if err != nil {
		return fmt.Errorf("installer canceled, blocked or failed: %s", b)
	}
	return nil
}
func (w windowsPlatform) ActivateDriver() error {
	ready, err := w.DriverReady()
	if err != nil {
		return err
	}
	if ready {
		return nil
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if base == "" {
			continue
		}
		file := filepath.Join(base, "Parsec Virtual Display Driver", "vddinstall.bat")
		if _, err := os.Stat(file); err == nil {
			return w.Elevated(file)
		}
	}
	return errors.New("Parsec's vddinstall.bat was not found. The driver installer must finish before continuing")
}
func (w windowsPlatform) ConfigPath() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		return "", errors.New("APPDATA is unavailable")
	}
	return filepath.Join(base, "opendisplay-win", "config.json"), nil
}
func (w windowsPlatform) StartSender(path string) error {
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
