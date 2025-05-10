# edgespoof

Find CDN edge IPs. Pin domains in `/etc/hosts`. Switch to Akamai staging when needed.

## Need

- macOS or Linux
- Go 1.22+
- `dig` in `$PATH` (`dnsutils` or `bind-tools`)
- `sudo` for changes to `/etc/hosts`

## Build

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

# Pin staging IPs
sudo edgespoof set example.com -s

# Show pinned domains
edgespoof list

# Remove one domain
sudo edgespoof unset example.com

# Remove all edgespoof entries
sudo edgespoof clear

# Shell prompt token: [SPOOF:2]. Empty when none
edgespoof status

# Show help
edgespoof help
```

Replace `example.com` with the target domain.

## Domains

- One domain for `resolve`, `set`, or `unset`.
- Names become lowercase. One trailing dot gets removed.
- Empty names, whitespace, and control characters fail.
- Extra arguments and unknown flags fail.
- Aliases: `rm` for `unset`, `ls` for `list`.

## Flags

Use with `resolve` or `set`, before or after the domain.

- `-s`, `--staging`: use Akamai staging for supported CDN CNAMEs.
- `--v4-only`: use IPv4 only. Default: IPv4 + IPv6.

## Hosts file notes

- Entries live between `# --- EDGESPOOF START ---` and `# --- EDGESPOOF END ---`.
- Edge target, mode, and timestamp live in inline comments.
- Broken block markers stop reads and writes. Unmanaged text stays intact.
- Writes use a temporary file and atomic rename. Bind mounts may need a direct write.
- Concurrent writes use `<hosts path>.edgespoof.lock`. Keep this lock file.
- System hosts changes attempt a DNS cache flush on macOS and Linux. Failures print a warning.
- Custom hosts files skip OS cache flushing.
- Flush warnings leave hosts changes saved. Exit status stays `0`.

## Use another hosts file

Set `EDGESPOOF_HOSTS_PATH` to an existing writable file. No `sudo` needed.

```zsh
EDGESPOOF_HOSTS_PATH=/tmp/hosts edgespoof list
```
