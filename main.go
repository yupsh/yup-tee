// Command yup-tee is the CLI wrapper around github.com/gloo-foo/cmd-tee.
package main

import (
	"io"
	"os"

	clix "github.com/gloo-foo/cli"
	command "github.com/gloo-foo/cmd-tee"
	errs "github.com/gomatic/go-error"
	"github.com/spf13/afero"
	urf "github.com/urfave/cli/v3"
)

// version is the build version. It defaults to "dev" for local builds and is
// overridden at release time via the linker: -ldflags "-X main.version=<v>".
var version = "dev"

const (
	name       = "tee"
	flagAppend = "append"
)

// ErrOpenFile is wrapped (via errs.Const.With) around the underlying afero
// failure when a FILE operand cannot be opened for writing, keeping both the
// sentinel and the cause matchable with errors.Is.
const ErrOpenFile errs.Const = "open file"

// synopsis is the multi-line --help usage block; urfave/cli indents it three
// spaces, so the lines stay flush-left.
const synopsis = `tee [OPTIONS] [FILE...]

Copy standard input to each FILE, and also to standard output.`

// spec declares the tee wrapper: a stdin filter that also writes each line to
// every FILE operand as a side effect.
var spec = clix.Spec{
	Name:     name,
	Summary:  "read from standard input and write to standard output and files",
	Synopsis: synopsis,
	Build:    build,
	Flags: []urf.Flag{
		&urf.BoolFlag{
			Name:    flagAppend,
			Aliases: []string{"a"},
			Usage:   "append to the given FILEs, do not overwrite",
		},
	},
}

// build maps the invocation to tee's pipeline: standard input feeds the tee
// command, which taps each line to the opened FILE writers and passes it
// through to standard output. A FILE that cannot be opened is a usage error.
func build(inv clix.Invocation) (clix.Source, clix.Command, error) {
	writers, err := openFiles(inv.Args, inv.Fs)
	if err != nil {
		return nil, nil, err
	}
	return clix.Stdin(inv.Stdin), command.Tee(writers...), nil
}

// openFiles opens each FILE operand on the injected filesystem for writing and
// returns the resulting io.Writers, which Tee taps to. Append mode is selected
// by the -a flag. On the first open failure the already-opened files are closed
// and the wrapped ErrOpenFile is returned. The successfully opened writers are
// left open: the process writes to them and exits, so the OS releases the
// descriptors after the pipeline has flushed each line.
func openFiles(c *urf.Command, fs afero.Fs) ([]any, error) {
	flag := fileFlag(appendMode(c.Bool(flagAppend)))
	writers := make([]any, 0, c.NArg())
	opened := make([]io.Closer, 0, c.NArg())
	for i := 0; i < c.NArg(); i++ {
		f, err := fs.OpenFile(c.Args().Get(i), flag, 0o644)
		if err != nil {
			closeAll(opened)
			return nil, ErrOpenFile.With(err)
		}
		writers = append(writers, f)
		opened = append(opened, f)
	}
	return writers, nil
}

// appendMode selects appending (-a) over truncation when opening the FILE
// operands for writing.
type appendMode bool

// fileFlag is the open flag set for the FILE operands: append or truncate.
func fileFlag(isAppendMode appendMode) int {
	if bool(isAppendMode) {
		return os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	return os.O_WRONLY | os.O_CREATE | os.O_TRUNC
}

// closeAll closes every opened file, used to unwind after a partial open
// failure.
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		_ = c.Close()
	}
}

// runMain is an indirection seam so main's wiring is testable without spawning
// the process; a test swaps it and restores it.
var runMain = clix.Main

func main() { runMain(spec, version) }
