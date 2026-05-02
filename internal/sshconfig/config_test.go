package sshconfig

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseEntriesPreservesSimpleHostFields(t *testing.T) {
	cfg, err := Parse("", []byte(`# top
Host prod-api
  HostName 10.0.1.12
  User ubuntu
  Port 22
  IdentityFile ~/.ssh/prod.pem

Host *.internal
  User ignored
`))
	if err != nil {
		t.Fatal(err)
	}

	entries := cfg.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Alias != "prod-api" || entries[0].HostName != "10.0.1.12" || entries[0].User != "ubuntu" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestUpsertUpdatesKnownFieldsAndKeepsUnknown(t *testing.T) {
	cfg, err := Parse("", []byte(`Host prod-api
  HostName old
  Compression yes
`))
	if err != nil {
		t.Fatal(err)
	}

	err = cfg.Upsert("prod-api", Entry{Alias: "prod-api", HostName: "new", User: "ubuntu", Port: "22"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg.Bytes())
	for _, want := range []string{"Host prod-api", "HostName new", "User ubuntu", "Port 22", "Compression yes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestAddAndDelete(t *testing.T) {
	cfg, err := Parse("", []byte("Host old\n  HostName old.example\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Upsert("", Entry{Alias: "new", HostName: "new.example", Port: "22"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Delete("old"); err != nil {
		t.Fatal(err)
	}
	got := string(cfg.Bytes())
	if strings.Contains(got, "Host old") || !strings.Contains(got, "Host new") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestValidate(t *testing.T) {
	tests := []Entry{
		{},
		{Alias: "x"},
		{Alias: "x", HostName: "example", Port: "0"},
		{Alias: "x", HostName: "example", Port: "bad"},
	}
	for _, test := range tests {
		if err := Validate(test); err == nil {
			t.Fatalf("expected validation error for %+v", test)
		}
	}
	if err := Validate(Entry{Alias: "x", HostName: "example", Port: "22"}); err != nil {
		t.Fatal(err)
	}
}

func TestParseReturnsScannerErrorForLongLines(t *testing.T) {
	tooLong := strings.Repeat("a", bufio.MaxScanTokenSize)
	_, err := Parse("", []byte("Host "+tooLong+"\n"))
	if err == nil {
		t.Fatal("expected scanner error")
	}
}
