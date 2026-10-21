# Windows native discovery experiment

**[Download the Windows x64 test ZIP](https://github.com/therealjosemaria/opendisplay/raw/refs/heads/codex/windows-discovery-probe/tools/windows-discovery-probe/downloads/OpenDisplay-Windows-Discovery-Test.zip)** · [ZIP checksum](downloads/SHA256SUMS.txt)

No Go, Node or developer tools are needed to run the executable.

This is **not an installer or a working second display**. It tests whether the
native Windows Win+K picker discovers a local `_display._tcp` advertisement.
It does not implement Wi-Fi Direct information elements, Miracast RTSP/RTP,
video capture, an extended monitor, or the OpenDisplay phone protocol.

## Run on the Dell

1. Extract the ZIP into Downloads.
2. Open PowerShell in that folder and run `./OpenDisplay-Discovery-Test.exe`.
3. Press Win+K and look for **OpenDisplay Discovery Test**. Do not select it.
4. Report whether the name appears and the final query/contact counts.

It exits after 90 seconds, or Ctrl+C. There is no installation, startup item,
driver, saved configuration, registry edit or firewall change. A short-lived
mDNS advertisement is sent on the selected network; other local machines may
see the test name. A TCP listener on that interface's port 7250 immediately
closes incoming connections and receives no video. No phone is contacted.

If the wrong interface is selected, stop and report it. `-ip` selects an address
already belonging to this computer. Do not disable firewall, antivirus or
organisation policies for this experiment. Report any Windows block or prompt.

## Interpretation

- Appearing in Win+K is evidence of picker discovery only, not playable video.
- Query counts alone do not prove picker discovery; another local process or
  computer may query the service.
- No appearance is inconclusive: Wi-Fi Direct discovery, filtering of a local
  receiver, firewall and network configuration remain separate possibilities.
- Tailscale reachability alone does not establish Miracast discovery.

This self-contained executable is unsigned and has not been executed on Windows
yet. Protocol/unit tests and cross-compilation are separate from a live Windows
test. The existing Mac service and PR are unchanged by this experiment.

Reference: Microsoft's implementation requirements:
https://learn.microsoft.com/en-us/windows-hardware/design/device-experiences/wireless-projection-implementing-over-existing-network
