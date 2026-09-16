package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	clix "github.com/gloo-foo/cli"
	"github.com/spf13/afero"
	urf "github.com/urfave/cli/v3"
)

// parse runs args through a bare command carrying the wrapper's flags and
// returns the parsed accessor.
func parse(t *testing.T, args ...string) *urf.Command {
	t.Helper()
	var got *urf.Command
	app := &urf.Command{
		Name:   name,
		Flags:  spec.Flags,
		Action: func(_ context.Context, c *urf.Command) error { got = c; return nil },
	}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return got
}

func invocation(t *testing.T, fs afero.Fs, args ...string) clix.Invocation {
	t.Helper()
	return clix.Invocation{Args: parse(t, args...), Stdin: strings.NewReader(""), Fs: fs}
}

func TestOpenFiles_OpensOperands(t *testing.T) {
	fs := afero.NewMemMapFs()
	writers, err := openFiles(parse(t, name, "a.txt", "b.txt"), fs)
	if err != nil {
		t.Fatalf("openFiles: %v", err)
	}
	if len(writers) != 2 {
		t.Fatalf("writers len=%d, want 2", len(writers))
	}
}

func TestOpenFiles_ErrorIsWrappedSentinel(t *testing.T) {
	fs := afero.NewReadOnlyFs(afero.NewMemMapFs())
	writers, err := openFiles(parse(t, name, "a.txt"), fs)
	if !errors.Is(err, ErrOpenFile) {
		t.Fatalf("err=%v, want ErrOpenFile", err)
	}
	if writers != nil {
		t.Fatalf("writers=%v, want nil on error", writers)
	}
	if !strings.HasPrefix(err.Error(), string(ErrOpenFile)) {
		t.Fatalf("message=%q, want prefix %q", err.Error(), string(ErrOpenFile))
	}
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("err=%v, want wrapped syscall.EPERM cause", err)
	}
}

func TestFileFlag(t *testing.T) {
	if got := fileFlag(true); got != os.O_WRONLY|os.O_CREATE|os.O_APPEND {
		t.Fatalf("append flag=%d", got)
	}
	if got := fileFlag(false); got != os.O_WRONLY|os.O_CREATE|os.O_TRUNC {
		t.Fatalf("truncate flag=%d", got)
	}
}

func TestCloseAll(t *testing.T) {
	fs := afero.NewMemMapFs()
	f, err := fs.OpenFile("x.txt", os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	closeAll([]io.Closer{f})
}

func TestBuild_Filter(t *testing.T) {
	src, filter, err := build(invocation(t, afero.NewMemMapFs(), name, "out.txt"))
	if err != nil || src == nil || filter == nil {
		t.Fatalf("build: src=%v filter=%v err=%v", src, filter, err)
	}
}

func TestBuild_OpenError(t *testing.T) {
	src, filter, err := build(invocation(t, afero.NewReadOnlyFs(afero.NewMemMapFs()), name, "out.txt"))
	if !errors.Is(err, ErrOpenFile) {
		t.Fatalf("err=%v, want ErrOpenFile", err)
	}
	if src != nil || filter != nil {
		t.Fatalf("src=%v filter=%v, want both nil on error", src, filter)
	}
}

func Test_main(t *testing.T) {
	orig := runMain
	t.Cleanup(func() { runMain = orig })
	var gotName clix.Name
	runMain = func(s clix.Spec, _ clix.Version) { gotName = s.Name }
	main()
	if gotName != name {
		t.Fatalf("main used spec %q, want %s", gotName, name)
	}
}
