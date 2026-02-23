# NAS Sync Guide

Sync files from Linux workstation to Synology NAS.

## Build & Install

```bash
cd /home/[...]/nas-sync
go build -o nas-sync .
```

Add to `~/.zshrc`:
```bash
alias nas-sync="/home/[...]/nas-sync/nas-sync"
```

## Setup

```bash
cp config.toml.example config.toml
# Edit config.toml with your NAS IP, username, and mount paths
```

## Usage

```
nas-sync up                            Connect wifi + mount shares
nas-sync down                          Unmount shares
nas-sync sync                          Dry-run: show what would be copied (local -> NAS)
nas-sync sync --execute                Copy files from local machine to NAS
nas-sync sync ~/Documents ~/repos      Dry-run specific dirs -> NAS
nas-sync sync --execute ~/Documents    Copy ~/Documents -> NAS for real
```

Each directory is mapped to the NAS by its path relative to `$HOME`, under the `docs_subfolder` set in `config.toml`. For example, with `docs_subfolder = "MyName"`:

```
~/Documents                -> /mnt/nas-docs/MyName/Documents
~/repos/personal/nas-sync  -> /mnt/nas-docs/MyName/repos/personal/nas-sync
nas-sync status                        Show connection and mount status
nas-sync help                          Show help
```

Sync always copies **from local to NAS**, never the other direction.

## Network Setup

The tool assumes your workstation can reach the NAS over a local network. If the NAS is on a different subnet (e.g. reachable only via wifi), `nas-sync up` will prompt for an SSID and password to connect.

## Prerequisites

- `cifs-utils` must be installed
- Go 1.23+ for building
- Know your Synology credentials

## Prevent suspend (if SSH'd in remotely)

```bash
sudo systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
```

Undo after you're done:

```bash
sudo systemctl unmask sleep.target suspend.target hibernate.target hybrid-sleep.target
```

## NAS share reference

| Share     | Type         | Mount path    | Notes                          |
|-----------|--------------|---------------|--------------------------------|
| home      | Personal     | /mnt/nas      | Your Synology Drive "My Drive" |
| Documents | Team Folder  | /mnt/nas-docs | Needs `sec=ntlmssp`            |
| Photos    | Shared       | -             | Not typically synced            |
| Pictures  | Shared       | -             | Raw and edited pictures         |
| video     | Shared       | -             | System default                  |

## Troubleshooting

| Problem | Solution |
|---------|----------|
| NAS not reachable via ping | Connect wifi to .1 subnet |
| `mount error(95): Operation not supported` | Use `vers=2.0` |
| `mount error(13): Permission denied` | Add `sec=ntlmssp` for Team Folders |
| Files read-only after mount | Add `uid=$(id -u),gid=$(id -g)` to mount options |
| Workstation suspends during SSH | `sudo systemctl mask suspend.target` |
