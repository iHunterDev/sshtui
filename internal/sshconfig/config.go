package sshconfig

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Entry struct {
	Alias        string
	HostName     string
	User         string
	Port         string
	IdentityFile string
}

type block struct {
	lines    []string
	isHost   bool
	editable bool
	alias    string
}

type Config struct {
	path   string
	blocks []block
}

var knownKeys = map[string]func(*Entry, string){
	"hostname":     func(e *Entry, v string) { e.HostName = v },
	"user":         func(e *Entry, v string) { e.User = v },
	"port":         func(e *Entry, v string) { e.Port = v },
	"identityfile": func(e *Entry, v string) { e.IdentityFile = v },
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

func Parse(path string, data []byte) (*Config, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var blocks []block
	var cur block

	flush := func() {
		if cur.lines != nil {
			cur.classify()
			blocks = append(blocks, cur)
		}
		cur = block{}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if isHostLine(line) {
			flush()
		}
		cur.lines = append(cur.lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()

	return &Config{path: path, blocks: blocks}, nil
}

func (c *Config) Entries() []Entry {
	var entries []Entry
	for _, b := range c.blocks {
		if !b.editable {
			continue
		}
		entry := Entry{Alias: b.alias}
		for _, line := range b.lines[1:] {
			key, value, ok := directive(line)
			if !ok {
				continue
			}
			if set, ok := knownKeys[strings.ToLower(key)]; ok {
				set(&entry, value)
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func (c *Config) Find(alias string) (Entry, bool) {
	for _, entry := range c.Entries() {
		if entry.Alias == alias {
			return entry, true
		}
	}
	return Entry{}, false
}

func (c *Config) Upsert(originalAlias string, entry Entry) error {
	entry = normalize(entry)
	if err := Validate(entry); err != nil {
		return err
	}
	if c.aliasExists(entry.Alias, originalAlias) {
		return fmt.Errorf("alias %q already exists", entry.Alias)
	}

	if originalAlias == "" {
		c.blocks = appendHostBlock(c.blocks, renderNewBlock(entry))
		return nil
	}

	for i := range c.blocks {
		if c.blocks[i].editable && c.blocks[i].alias == originalAlias {
			c.blocks[i] = updateBlock(c.blocks[i], entry)
			return nil
		}
	}
	return fmt.Errorf("alias %q not found", originalAlias)
}

func (c *Config) Delete(alias string) error {
	for i := range c.blocks {
		if c.blocks[i].editable && c.blocks[i].alias == alias {
			c.blocks = append(c.blocks[:i], c.blocks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("alias %q not found", alias)
}

func (c *Config) Save() error {
	if c.path == "" {
		return errors.New("config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}

	data := c.Bytes()
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".sshtui-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, c.path)
}

func (c *Config) Bytes() []byte {
	var lines []string
	for _, b := range c.blocks {
		lines = append(lines, b.lines...)
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func Validate(entry Entry) error {
	if strings.TrimSpace(entry.Alias) == "" {
		return errors.New("alias is required")
	}
	if strings.TrimSpace(entry.HostName) == "" {
		return errors.New("hostname is required")
	}
	if strings.TrimSpace(entry.Port) != "" {
		port, err := strconv.Atoi(strings.TrimSpace(entry.Port))
		if err != nil || port < 1 || port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
	}
	return nil
}

func (c *Config) aliasExists(alias, except string) bool {
	for _, entry := range c.Entries() {
		if entry.Alias == alias && entry.Alias != except {
			return true
		}
	}
	return false
}

func (b *block) classify() {
	if len(b.lines) == 0 {
		return
	}
	fields := strings.Fields(strings.TrimSpace(b.lines[0]))
	if len(fields) < 2 || !strings.EqualFold(fields[0], "Host") {
		return
	}
	b.isHost = true
	if len(fields) == 2 && !strings.ContainsAny(fields[1], "*?!") {
		b.editable = true
		b.alias = fields[1]
	}
}

func updateBlock(b block, entry Entry) block {
	seen := map[string]bool{}
	out := []string{"Host " + entry.Alias}
	for _, line := range b.lines[1:] {
		key, _, ok := directive(line)
		if !ok {
			out = append(out, line)
			continue
		}
		lower := strings.ToLower(key)
		if value, ok := entryValue(entry, lower); ok {
			if value != "" {
				out = append(out, "  "+canonicalKey(lower)+" "+value)
			}
			seen[lower] = true
			continue
		}
		out = append(out, line)
	}
	for _, key := range []string{"hostname", "user", "port", "identityfile"} {
		if seen[key] {
			continue
		}
		if value, ok := entryValue(entry, key); ok && value != "" {
			out = append(out, "  "+canonicalKey(key)+" "+value)
		}
	}
	return block{lines: out, isHost: true, editable: true, alias: entry.Alias}
}

func renderNewBlock(entry Entry) block {
	lines := []string{"Host " + entry.Alias, "  HostName " + entry.HostName}
	if entry.User != "" {
		lines = append(lines, "  User "+entry.User)
	}
	if entry.Port != "" {
		lines = append(lines, "  Port "+entry.Port)
	}
	if entry.IdentityFile != "" {
		lines = append(lines, "  IdentityFile "+entry.IdentityFile)
	}
	return block{lines: lines, isHost: true, editable: true, alias: entry.Alias}
}

func appendHostBlock(blocks []block, b block) []block {
	if len(blocks) > 0 && len(blocks[len(blocks)-1].lines) > 0 {
		last := blocks[len(blocks)-1].lines
		if strings.TrimSpace(last[len(last)-1]) != "" {
			blocks = append(blocks, block{lines: []string{""}})
		}
	}
	return append(blocks, b)
}

func normalize(entry Entry) Entry {
	entry.Alias = strings.TrimSpace(entry.Alias)
	entry.HostName = strings.TrimSpace(entry.HostName)
	entry.User = strings.TrimSpace(entry.User)
	entry.Port = strings.TrimSpace(entry.Port)
	entry.IdentityFile = strings.TrimSpace(entry.IdentityFile)
	return entry
}

func directive(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], strings.TrimSpace(trimmed[len(fields[0]):]), true
}

func isHostLine(line string) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	return len(fields) >= 2 && strings.EqualFold(fields[0], "Host")
}

func entryValue(entry Entry, key string) (string, bool) {
	switch key {
	case "hostname":
		return entry.HostName, true
	case "user":
		return entry.User, true
	case "port":
		return entry.Port, true
	case "identityfile":
		return entry.IdentityFile, true
	default:
		return "", false
	}
}

func canonicalKey(key string) string {
	switch key {
	case "hostname":
		return "HostName"
	case "user":
		return "User"
	case "port":
		return "Port"
	case "identityfile":
		return "IdentityFile"
	default:
		return key
	}
}
