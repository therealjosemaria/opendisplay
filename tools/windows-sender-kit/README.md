# OpenDisplay Windows setup kit

**[Download the Windows second-display setup ZIP](https://github.com/therealjosemaria/opendisplay/raw/refs/heads/codex/windows-discovery-probe/tools/windows-sender-kit/downloads/OpenDisplay-Windows-Setup-0.1.0-review.zip)**

This kit contains the real **martinhoess/opendisplay-win v0.1.0** sender,
unchanged, plus a setup helper. It is separate from the unsuccessful discovery
probe. It uses a virtual-display driver and tray app, not Win+K.

## Start

1. Extract the ZIP. Open **OpenDisplay Windows** and double-click **Setup.exe**.
2. If the Parsec display driver is missing, setup downloads it from Parsec,
   checks its pinned checksum and Windows signing status, then opens its
   installer. Approve Windows administrator prompts and complete the vendor's
   installer. Setup then activates the driver through its `vddinstall.bat` if
   necessary. If you cannot approve the administrator prompt, stop: this driver
   cannot be installed without authorised administrator access.
3. Open OpenDisplay on your unlocked iPhone. Enter its **IPv4 address** when
   setup asks: iPhone **Settings > Wi-Fi > (i) > IP Address**, or its address in
   Tailscale. The laptop must be able to reach that address; on isolated campus
   Wi-Fi use Tailscale on both if it is available and permitted.
4. Keep the iPhone app visible. The sender runs beside the Windows clock. Move
   a window onto the phone. Use **Settings > System > Display** to arrange it,
   or **Win+P > Extend** if Windows is duplicating displays.

No Go, Node, Python, Visual Studio or build tools are required on the laptop.
The sender and helper are unsigned. Report any SmartScreen, organisation-policy
or security block; do not disable Windows protections or change university
policies. The driver installer itself must have a valid Parsec/Unity signature.

## Daily use and disconnect

After first setup, open **opendisplay-win.exe**, with OpenDisplay open on the
iPhone. It remembers the address. Right-click its tray icon to connect,
disconnect, change Settings, or Exit. Audio stays on the laptop; this sender
does not stream audio. Do not start a second sender for the same phone.

## Remove

Exit the sender through its tray icon. In **Windows Settings > Apps > Installed
apps**, uninstall **Parsec Virtual Display Driver** (administrator permission
may be required), then delete the extracted kit folder. The driver is a system
installation: do not assume logging off removes it. Saved settings and logs are
in `%APPDATA%\opendisplay-win`; this kit leaves them available for diagnostics.
Existing driver/settings installations are detected and preserved; setup backs
up an existing config before updating its phone address.

## Attribution and validation limits

- Sender: [martinhoess/opendisplay-win](https://github.com/martinhoess/opendisplay-win),
  GPL-3.0-or-later, v0.1.0, unchanged official release executable. Corresponding
  source and LICENSE are included in the ZIP. Sender SHA-256:
  `6598cba78a56a2c81108530f7b3c94ae30d4b1d4f4b144886505fe2618891244`.
- iPhone receiver: [peetzweg/OpenDisplay](https://github.com/peetzweg/opendisplay).
- Driver: Parsec Virtual Display Driver 0.45.0.0. Downloaded directly from
  `https://builds.parsec.app/vdd/parsec-vdd-0.45.0.0.exe`; not redistributed here.
  Driver SHA-256:
  `e23332448fdaf5aa017cb308db5ef6855fac526a7ded05d80c039404126d5362`.
- Helper: therealjosemaria, GPL-3.0-or-later; setup, download integrity and failure
  handling only. It does not implement or claim authorship of the sender.
- Local automated tests cover setup decisions, preserving existing settings,
  stopping after errors, IPv4 validation and checksum enforcement. Windows
  compilation is checked. Driver installation, UAC/PowerShell integration,
  actual capture/encoding, iPhone playback and rotation need the user's real
  Windows/iPhone test. **No claim of local end-to-end Windows validation.**
- The upstream sender is a proof of concept. Its documented rough edges include
  phantom monitor entries after runs and fixed stream settings. We have not
  changed its video, rotation or reconnect implementation. The Mac installation,
  Mac PR and published Mac installer are unchanged by this kit.
