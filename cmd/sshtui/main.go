package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"sshtui/internal/sshconfig"
	"sshtui/internal/tui"
)

func main() {
	defaultPath, err := defaultConfigPath()
	if err != nil {
		exitErr(err)
	}

	global := flag.NewFlagSet("sshtui", flag.ExitOnError)
	global.Usage = func() { usage(defaultPath) }
	configPath := global.String("config", defaultPath, "SSH config path")
	_ = global.Parse(os.Args[1:])
	args := global.Args()

	if len(args) == 0 {
		if err := runConnect(*configPath); err != nil {
			exitErr(err)
		}
		return
	}

	switch args[0] {
	case "config":
		if err := tui.RunConfig(*configPath); err != nil {
			exitErr(err)
		}
	case "list":
		if err := runList(*configPath); err != nil {
			exitErr(err)
		}
	case "add":
		if err := runAdd(*configPath, args[1:]); err != nil {
			exitErr(err)
		}
	case "edit":
		if err := runEdit(*configPath, args[1:]); err != nil {
			exitErr(err)
		}
	case "delete", "remove", "rm":
		if err := runDelete(*configPath, args[1:]); err != nil {
			exitErr(err)
		}
	case "help", "-h", "--help":
		if len(args) > 1 {
			commandUsage(args[1], defaultPath)
		} else {
			usage(defaultPath)
		}
	default:
		exitErr(fmt.Errorf("unknown command %q", args[0]))
	}
}

func runConnect(configPath string) error {
	alias, ok, err := tui.RunConnectSelector(configPath)
	if err != nil || !ok {
		return err
	}
	cmd := exec.Command("ssh", alias)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runList(configPath string) error {
	cfg, err := sshconfig.Load(configPath)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tUSER\tHOSTNAME\tPORT\tIDENTITYFILE")
	for _, entry := range cfg.Entries() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", entry.Alias, entry.User, entry.HostName, entry.Port, entry.IdentityFile)
	}
	return w.Flush()
}

func runAdd(configPath string, args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	fs.Usage = func() { commandUsage("add", "") }
	entry := &sshconfig.Entry{Port: "22"}
	bindEntryFlags(fs, entry)
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		entry.Alias = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := sshconfig.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.Upsert("", *entry); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("Added %s.\n", entry.Alias)
	return nil
}

func runEdit(configPath string, args []string) error {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	fs.Usage = func() { commandUsage("edit", "") }
	alias := fs.String("alias", "", "Alias to edit")
	hostName := fs.String("hostname", "", "HostName")
	user := fs.String("user", "", "User")
	port := fs.String("port", "", "Port")
	identityFile := fs.String("identity-file", "", "IdentityFile")
	newAlias := fs.String("new-alias", "", "New alias")
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		*alias = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *alias == "" {
		return fmt.Errorf("alias is required")
	}

	cfg, err := sshconfig.Load(configPath)
	if err != nil {
		return err
	}
	entry, ok := cfg.Find(*alias)
	if !ok {
		return fmt.Errorf("alias %q not found", *alias)
	}
	if *newAlias != "" {
		entry.Alias = *newAlias
	}
	applyStringFlag(fs, "hostname", hostName, &entry.HostName)
	applyStringFlag(fs, "user", user, &entry.User)
	applyStringFlag(fs, "port", port, &entry.Port)
	applyStringFlag(fs, "identity-file", identityFile, &entry.IdentityFile)

	if err := cfg.Upsert(*alias, entry); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("Saved %s.\n", entry.Alias)
	return nil
}

func runDelete(configPath string, args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	fs.Usage = func() { commandUsage("delete", "") }
	alias := fs.String("alias", "", "Alias to delete")
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		*alias = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *alias == "" {
		return fmt.Errorf("alias is required")
	}

	cfg, err := sshconfig.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.Delete(*alias); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("Deleted %s.\n", *alias)
	return nil
}

func bindEntryFlags(fs *flag.FlagSet, entry *sshconfig.Entry) {
	fs.StringVar(&entry.Alias, "alias", entry.Alias, "Alias")
	fs.StringVar(&entry.HostName, "hostname", entry.HostName, "HostName")
	fs.StringVar(&entry.User, "user", entry.User, "User")
	fs.StringVar(&entry.Port, "port", entry.Port, "Port")
	fs.StringVar(&entry.IdentityFile, "identity-file", entry.IdentityFile, "IdentityFile")
}

func applyStringFlag(fs *flag.FlagSet, name string, value *string, target *string) {
	if fs.Lookup(name) == nil {
		return
	}
	seen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			seen = true
		}
	})
	if seen {
		*target = *value
	}
}

func usage(defaultPath string) {
	fmt.Fprintf(os.Stderr, `Usage:
  sshtui [--config %s]
  sshtui [--config PATH] config
  sshtui [--config PATH] list
  sshtui [--config PATH] add [ALIAS] --hostname HOST [--alias NAME] [--user USER] [--port PORT] [--identity-file PATH]
  sshtui [--config PATH] edit [ALIAS] [--alias NAME] [--new-alias NAME] [--hostname HOST] [--user USER] [--port PORT] [--identity-file PATH]
  sshtui [--config PATH] delete [ALIAS] [--alias NAME]
  sshtui help [COMMAND]

Commands:
  config        Open the config editor TUI
  list          Print editable Host entries
  add           Add a Host entry
  edit          Edit a Host entry
  delete        Delete a Host entry
  remove, rm    Aliases for delete
  help          Show help, optionally for a command

Global options:
  --config PATH SSH config path (default %s)

Default command:
  Opens a selector and connects with ssh after Enter.

`, defaultPath, defaultPath)
}

func commandUsage(command, defaultPath string) {
	switch command {
	case "config":
		fmt.Fprintln(os.Stderr, `Usage:
  sshtui [--config PATH] config

Open the config editor TUI.`)
	case "list":
		fmt.Fprintln(os.Stderr, `Usage:
  sshtui [--config PATH] list

Print editable Host entries as columns: ALIAS, USER, HOSTNAME, PORT, IDENTITYFILE.`)
	case "add":
		fmt.Fprintln(os.Stderr, `Usage:
  sshtui [--config PATH] add [ALIAS] --hostname HOST [--alias NAME] [--user USER] [--port PORT] [--identity-file PATH]

Add a Host entry. ALIAS can be passed positionally or with --alias.

Options:
  --alias NAME          Alias for the Host entry
  --hostname HOST       HostName value (required)
  --user USER           User value
  --port PORT           Port value (default 22)
  --identity-file PATH  IdentityFile value`)
	case "edit":
		fmt.Fprintln(os.Stderr, `Usage:
  sshtui [--config PATH] edit [ALIAS] [--alias NAME] [--new-alias NAME] [--hostname HOST] [--user USER] [--port PORT] [--identity-file PATH]

Edit a Host entry. The existing alias can be passed positionally or with --alias.

Options:
  --alias NAME          Alias to edit
  --new-alias NAME      New alias
  --hostname HOST       HostName value
  --user USER           User value
  --port PORT           Port value
  --identity-file PATH  IdentityFile value`)
	case "delete", "remove", "rm":
		fmt.Fprintln(os.Stderr, `Usage:
  sshtui [--config PATH] delete [ALIAS] [--alias NAME]
  sshtui [--config PATH] remove [ALIAS] [--alias NAME]
  sshtui [--config PATH] rm [ALIAS] [--alias NAME]

Delete a Host entry. ALIAS can be passed positionally or with --alias.`)
	default:
		if defaultPath != "" {
			fmt.Fprintf(os.Stderr, "Unknown command %q.\n\n", command)
			usage(defaultPath)
			return
		}
		fmt.Fprintf(os.Stderr, "Unknown command %q.\n", command)
	}
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func exitErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
