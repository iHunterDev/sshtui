# sshtui

`sshtui` is a terminal UI and CLI for managing simple entries in your OpenSSH config.

## Features

- Open a searchable selector and connect with `ssh`.
- Add, edit, delete, list, and reload editable `Host` entries.
- Preserve unsupported or unknown directives inside existing editable blocks.
- Skip wildcard and multi-pattern `Host` blocks so broad SSH config rules are not rewritten accidentally.

## Install

```sh
go install ./cmd/sshtui
```

## Usage

```sh
sshtui
sshtui config
sshtui list
sshtui add prod --hostname 10.0.1.12 --user ubuntu --port 22 --identity-file ~/.ssh/prod.pem
sshtui edit prod --hostname prod.example.com
sshtui delete prod
```

By default, `sshtui` reads and writes `~/.ssh/config`. Use `--config PATH` to target another file.

```sh
sshtui --config ./test_config list
```

## TUI Controls

Connection selector:

- Type to filter by alias, user, or hostname.
- `Enter` connects to the selected alias.
- `r` reloads the config when the search query is empty.
- `q`, `Esc`, or `Ctrl+C` quits.

Config editor:

- `a` adds a host entry.
- `Enter` edits the selected entry.
- `d` deletes the selected entry.
- `/` searches entries.
- `r` reloads the config.
- `Ctrl+S` saves a form.
- `Esc` leaves the current view or opens the discard prompt when a form has changes.

## Editable Entries

`sshtui` only edits simple `Host` blocks with exactly one non-wildcard alias, for example:

```sshconfig
Host prod
  HostName prod.example.com
  User ubuntu
  Port 22
  IdentityFile ~/.ssh/prod.pem
```

Wildcard rules such as `Host *.internal` remain in the file, but are not shown as editable entries.
