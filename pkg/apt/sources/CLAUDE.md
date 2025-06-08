# APT Sources List Format

This package handles parsing of APT sources list files.

## Sources List Format

APT sources list entries use this format:
```
deb|deb-src URL distribution component1 component2 ...
```

### Entry Types
- `deb`: Binary package archives
- `deb-src`: Source package archives

### Components
- **URL**: Repository base URL
- **Distribution**: Release codename (e.g., "bookworm", "jammy") or class ("stable", "testing")
- **Components**: Space-separated list of archive areas:
  - `main`: DFSG-compliant packages
  - `contrib`: Packages with dependencies outside main
  - `non-free`: Software not complying with DFSG
  - `non-free-firmware`: Firmware not meeting DFSG standards

### Example Entries
```
deb https://deb.debian.org/debian bookworm main non-free-firmware
deb-src https://deb.debian.org/debian bookworm main
deb https://archive.ubuntu.com/ubuntu jammy main restricted universe multiverse
```

### File Locations
- `/etc/apt/sources.list`: Main configuration file
- `/etc/apt/sources.list.d/`: Directory for additional source files

Comments start with `#`. Empty lines are ignored.