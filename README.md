# edgespoof

Find CDN edge IPs. Pin domains in `/etc/hosts`. Switch to Akamai staging when needed.


## Overview

`edgespoof` automates resolution of CDN edge servers and Akamai staging networks. The tool traces CNAME chains via `dig`, extracts terminal edge nodes, concurrently queries `A` (IPv4) and `AAAA` (IPv6) records, updates `/etc/hosts` within an isolated block using atomic file swaps, and flushes local OS DNS caches.


## Need

- Go 1.22+
- `dig` in `$PATH` (`dnsutils` or `bind-tools`)
- `sudo` for changes to `/etc/hosts`

## Build & Installation

```zsh
go build -ldflags="-s -w" -o edgespoof ./cmd/edgespoof
sudo install -m 0755 edgespoof /usr/local/bin/edgespoof
```

## Use

```zsh
# Check IPs. Change nothing
edgespoof resolve example.com
edgespoof resolve example.com -s

# Pin edge IPs
sudo edgespoof set example.com

# Pin staging IPs.
sudo edgespoof set example.com -s

# Show pinned domains
edgespoof list

# Remove one domain
sudo edgespoof unset example.com

# Remove all edgespoof entries
sudo edgespoof clear

# Shell prompt token: [SPOOF:2]. Empty when none
edgespoof status
```

Replace `example.com` with the target domain.

## Flags

Use with `resolve` or `set`, before or after the domain.

- `-s`, `--staging`: use Akamai staging for supported CDN CNAMEs.
- `--v4-only`: use IPv4 only. Default: IPv4 + IPv6.

## Hosts file notes

- Entries live between `# --- EDGESPOOF START ---` and `# --- EDGESPOOF END ---`.
- Edge target, mode, and timestamp live in inline comments.
- Writes use a temporary file and atomic rename. Bind mounts may need a direct write.
- Changes attempt a DNS cache flush on macOS and Linux. Flush errors are ignored.

## Use another hosts file

Set `EDGESPOOF_HOSTS_PATH` to an existing writable file. No `sudo` needed.

```zsh
EDGESPOOF_HOSTS_PATH=/tmp/hosts edgespoof list
```
