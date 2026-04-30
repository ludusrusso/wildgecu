// Package bounded wraps os/exec with two responsibilities: enforce a per-call
// timeout (returning a typed TimedOut bool rather than just an error), and
// capture stdout/stderr through line-aligned head+tail buffers with
// total-byte counters.
//
// The package is dependency-free of the tool/agent layers so it can be tested
// directly with synthetic processes.
package bounded

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// Defaults are applied when the corresponding Opts field is zero.
const (
	DefaultTimeout    = 30 * time.Second
	DefaultMaxTimeout = 10 * time.Minute
	DefaultHeadBytes  = 24576
	DefaultTailBytes  = 6144
)

// truncationMarkerFormat is the marker injected between the head and tail
// when output is truncated. The %d is the total number of bytes dropped from
// the middle (including bytes trimmed off head/tail to reach a newline).
const truncationMarkerFormat = "[... truncated %d bytes from middle ...]\n"

// Opts configures a Run call. Zero values pick defaults from this package's
// Default* constants.
type Opts struct {
	// Timeout is the per-call timeout. Defaults to DefaultTimeout.
	Timeout time.Duration
	// MaxTimeout is the hard ceiling enforced at the call boundary. A
	// Timeout greater than MaxTimeout returns an error rather than being
	// silently clamped. Defaults to DefaultMaxTimeout.
	MaxTimeout time.Duration
	// HeadBytes is the size of the captured head window for stdout/stderr.
	HeadBytes int
	// TailBytes is the size of the captured tail window for stdout/stderr.
	TailBytes int
	// WorkDir, if non-empty, is the working directory for the child
	// process.
	WorkDir string
}

// Result carries the captured output and exit status of a Run call.
type Result struct {
	Stdout           string
	Stderr           string
	ExitCode         int
	TimedOut         bool
	StdoutTotalBytes int
	StderrTotalBytes int
}

// Run executes name with args, enforcing the configured timeout and capturing
// stdout/stderr through head+tail buffers. The returned error is non-nil only
// for unexpected failures (e.g. command not found). A non-zero exit code, a
// timeout, or large output do NOT produce an error: they surface through
// Result fields so the caller can react distinctly.
func Run(ctx context.Context, name string, args []string, opts Opts) (Result, error) {
	maxT := opts.MaxTimeout
	if maxT <= 0 {
		maxT = DefaultMaxTimeout
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if timeout > maxT {
		return Result{}, fmt.Errorf("timeout %s exceeds maximum %s", timeout, maxT)
	}

	headBytes := opts.HeadBytes
	if headBytes <= 0 {
		headBytes = DefaultHeadBytes
	}
	tailBytes := opts.TailBytes
	if tailBytes <= 0 {
		tailBytes = DefaultTailBytes
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	stdout := newHeadTailBuf(headBytes, tailBytes)
	stderr := newHeadTailBuf(headBytes, tailBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	timedOut := errors.Is(cmdCtx.Err(), context.DeadlineExceeded)

	res := Result{
		Stdout:           stdout.String(),
		Stderr:           stderr.String(),
		StdoutTotalBytes: stdout.Total(),
		StderrTotalBytes: stderr.Total(),
		TimedOut:         timedOut,
	}

	switch {
	case timedOut:
		res.ExitCode = -1
		return res, nil
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return res, runErr
	default:
		res.ExitCode = 0
		return res, nil
	}
}

// headTailBuf is an io.Writer that retains the first HeadBytes and the last
// TailBytes of all bytes written to it. When the total written exceeds
// head+tail, String() returns head, a truncation marker, then tail —
// line-aligned (head trimmed back to the last newline; tail advanced past
// the first newline).
type headTailBuf struct {
	head      []byte
	headCap   int
	tail      []byte // ring buffer of size tailCap
	tailCap   int
	tailStart int // index in tail of the oldest retained byte
	tailLen   int // current count of bytes retained in tail (≤ tailCap)
	total     int
}

func newHeadTailBuf(headCap, tailCap int) *headTailBuf {
	return &headTailBuf{
		headCap: headCap,
		tailCap: tailCap,
		head:    make([]byte, 0, headCap),
		tail:    make([]byte, tailCap),
	}
}

func (b *headTailBuf) Write(p []byte) (int, error) {
	n := len(p)
	b.total += n
	if len(b.head) < b.headCap {
		need := b.headCap - len(b.head)
		if need >= len(p) {
			b.head = append(b.head, p...)
			return n, nil
		}
		b.head = append(b.head, p[:need]...)
		p = p[need:]
	}
	for _, c := range p {
		if b.tailLen < b.tailCap {
			b.tail[(b.tailStart+b.tailLen)%b.tailCap] = c
			b.tailLen++
			continue
		}
		b.tail[b.tailStart] = c
		b.tailStart = (b.tailStart + 1) % b.tailCap
	}
	return n, nil
}

// Total returns the total number of bytes written, including any that were
// dropped from the middle when the buffer overflowed.
func (b *headTailBuf) Total() int { return b.total }

// String renders the captured head+tail. When no truncation occurred, it
// returns simply head followed by tail (concatenated). When truncation
// occurred (total > head+tail), the result is line-aligned and includes a
// "[... truncated N bytes from middle ...]" marker.
func (b *headTailBuf) String() string {
	if b.tailLen == 0 {
		return string(b.head)
	}

	tail := b.linearTail()
	bytesDropped := b.total - len(b.head) - b.tailLen
	headFull := len(b.head) == b.headCap
	if !headFull || bytesDropped <= 0 {
		return string(b.head) + string(tail)
	}

	headPart := b.head
	if i := bytes.LastIndexByte(headPart, '\n'); i >= 0 {
		headPart = headPart[:i+1]
	}
	tailPart := tail
	if i := bytes.IndexByte(tailPart, '\n'); i >= 0 {
		tailPart = tailPart[i+1:]
	}
	totalDropped := bytesDropped + (len(b.head) - len(headPart)) + (len(tail) - len(tailPart))
	marker := fmt.Sprintf(truncationMarkerFormat, totalDropped)
	return string(headPart) + marker + string(tailPart)
}

func (b *headTailBuf) linearTail() []byte {
	out := make([]byte, b.tailLen)
	for i := 0; i < b.tailLen; i++ {
		out[i] = b.tail[(b.tailStart+i)%b.tailCap]
	}
	return out
}
