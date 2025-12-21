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

## Test

```zsh
go test -race ./...
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

# Show pinned domains. UPDATED in local time
edgespoof list

# Remove one domain
sudo edgespoof unset example.com

# Remove all edgespoof entries
sudo edgespoof clear

# Shell prompt token: [SPOOF:2]. Empty when none
edgespoof status

# Show help
edgespoof help

# Show build version
edgespoof version
```

Replace `example.com` with the target domain.

## Domains

- One domain for `resolve`, `set`, or `unset`.
- Names become lowercase. One trailing dot gets removed.
- Labels allow letters, digits, and `-`. No `-` at label start or end.
- Labels up to 63 characters. Names up to 253.
- Empty names and empty labels fail.
- Unicode names need punycode (`xn--...`).
- Extra arguments and unknown flags fail.
- Aliases: `rm` for `unset`, `ls` for `list`, `--version` for `version`.

## Flags

Use with `resolve` or `set`, before or after the domain.

- `-s`, `--staging`: use Akamai staging for supported CDN CNAMEs.
- `--v4-only`: use IPv4 only. Default: IPv4 + IPv6.

## DNS lookups

- `dig` gets 2 tries, 2s each. Hard stop at 5s per record type.
- One failed address family prints a warning. The other family still shows and pins.

## Output

- Colors only on terminals. Checked per stream (stdout, stderr).
- `NO_COLOR=1` turns colors off.

## Hosts file notes

- Entries live between `# --- EDGESPOOF START ---` and `# --- EDGESPOOF END ---`.
- Edge target, mode, and timestamp live in inline comments.
- Broken block markers stop reads and writes. Unmanaged text stays intact.
- A missing final newline gets added before the block. It stays after the block goes.
- Writes use a synced temporary file and atomic rename. Bind mounts may need a direct write.
- Writes keep the hosts file mode.
- Concurrent writes use `<hosts path>.edgespoof.lock`. Keep this lock file.
- System hosts changes attempt a DNS cache flush on macOS and Linux. Failures print a warning.
- Custom hosts files skip OS cache flushing.
- Flush warnings leave hosts changes saved. Exit status stays `0`.

## Use another hosts file

Set `EDGESPOOF_HOSTS_PATH` to an existing writable file. No `sudo` needed.

```zsh
EDGESPOOF_HOSTS_PATH=/tmp/hosts edgespoof list
```
