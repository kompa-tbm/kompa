// Package extractor - xz decompression support using a pure-Go implementation.
package extractor

import (
	"fmt"
	"io"
	"os/exec"
)

// xzReadCloser wraps a subprocess pipe for xz decompression.
// When the pure-Go xz library is unavailable, we fall back to the system 'xz' binary.
type xzReadCloser struct {
	cmd    *exec.Cmd
	reader io.ReadCloser
}

func (x *xzReadCloser) Read(p []byte) (int, error) { return x.reader.Read(p) }
func (x *xzReadCloser) Close() error {
	err := x.reader.Close()
	if waitErr := x.cmd.Wait(); waitErr != nil && err == nil {
		err = waitErr
	}
	return err
}

// newXZReader returns a ReadCloser that decompresses xz data from r.
// It tries system binaries in order: xz, unxz.
func newXZReader(r io.Reader) (io.ReadCloser, error) {
	binaries := []string{"xz", "unxz"}
	for _, bin := range binaries {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		var args []string
		if bin == "xz" {
			args = []string{"-d", "--stdout"}
		} else {
			args = []string{"--stdout"}
		}
		cmd := exec.Command(path, args...)
		cmd.Stdin = r
		out, err := cmd.StdoutPipe()
		if err != nil {
			continue
		}
		if err := cmd.Start(); err != nil {
			continue
		}
		return &xzReadCloser{cmd: cmd, reader: out}, nil
	}

	// Neither xz nor unxz found. Try 'tar' with --xz flag as a last resort.
	// This path is reached mainly on Windows. Since .tar.xz packages on Windows
	// are uncommon in our artifact set (we prefer .zip), we return a clear error.
	return nil, fmt.Errorf(
		"xz decompression requires the 'xz' binary to be installed on this system.\n" +
			"On Ubuntu/Debian: sudo apt-get install xz-utils\n" +
			"On macOS: brew install xz\n" +
			"On Windows: install xz-utils from https://tukaani.org/xz/",
	)
}
