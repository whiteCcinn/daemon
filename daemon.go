package daemon

import (
	"context"
	"fmt"
	"github.com/erikdubbelboer/gspt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

var defaultOption = &options{
	exit: true,
}

type options struct {
	exit bool
}

type Option interface {
	apply(*options)
}

type OptionsFunc func(o *options)

func (f OptionsFunc) apply(o *options) {
	f(o)
}

func WithNoExit() Option {
	return OptionsFunc(func(o *options) {
		o.exit = false
	})
}

const EnvName = "_DAEMON"

var runIdx int = 0

type Context struct {
	//PidFileName string
	//PidFilePerm os.FileMode

	ProcAttr syscall.SysProcAttr

	Logger io.Writer

	Env  []string
	Args []string

	// Maximum number of loop restarts, 0 for unlimited restarts
	MaxCount int
	// The maximum number of consecutive startup failures or abnormal exits beyond which the daemon exits without restarting the child process
	MaxError int
	// The minimum time (in seconds) for a child process to exit normally. Less than this time is considered an abnormal exit
	MinExitTime time.Duration
	// If the run time exceeds this time, reset the restart count
	RestoreTime time.Duration

	*exec.Cmd
	Pid  int // supervisor pid
	CPid int // main pid

	RestartCallback
}

type RestartCallback func(ctx context.Context)

func attachContext(dctx *Context) (isChild bool) {
	runIdx++
	envIdx, err := strconv.Atoi(os.Getenv(EnvName))
	if err != nil {
		envIdx = 0
	}
	dctx.Pid = os.Getpid()

	// This is child process
	if runIdx <= envIdx {
		return true
	}

	// set the environ var
	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=%d", EnvName, runIdx))
	dctx.Env = env
	dctx.Args = os.Args

	return false
}

func Background(ctx context.Context, dctx *Context, opts ...Option) (*exec.Cmd, error) {

	for _, o := range opts {
		o.apply(defaultOption)
	}

	if attachContext(dctx) {
		return nil, nil
	}

	// starting child process
	cmd, err := startProc(ctx, dctx)
	if err != nil {
		dctx.log("[start exec process failed, err: %s]\n", dctx.Pid, err)
		return nil, err
	} else {
		dctx.CPid = cmd.Process.Pid
		if !defaultOption.exit {
			dctx.log("[process(%d)] [started]\n", dctx.CPid)
			dctx.log("[supervisor(%d)] [watch --pid=%d]\n", dctx.Pid, dctx.CPid)
		}
	}

	if defaultOption.exit {
		os.Exit(0)
	}

	return cmd, nil
}

func startProc(ctx context.Context, dctx *Context) (*exec.Cmd, error) {
	cmd := &exec.Cmd{
		Path:        dctx.Args[0],
		Args:        dctx.Args,
		Env:         dctx.Env,
		SysProcAttr: &dctx.ProcAttr,
	}
	dctx.Cmd = cmd

	if dctx.Logger != nil {
		cmd.Stderr = dctx.Logger
		cmd.Stdout = dctx.Logger
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func (dctx *Context) Run(ctx context.Context) error {
	_, err := Background(ctx, dctx)
	if err != nil {
		log.Fatal(err)
	}

	count := 1
	isReset := false
	errNum := 0
	for {
		//daemon information
		dInfo := fmt.Sprintf("count:%d/%d; errNum:%d/%d", count, dctx.MaxCount, errNum, dctx.MaxError)
		if errNum > dctx.MaxError {
			dctx.log("[supervisor(%d)] [child process fails too many times]\n", dctx.Pid)
			os.Exit(1)
		}
		if dctx.MaxCount > 0 && count > dctx.MaxCount {
			dctx.log("[supervisor(%d)] [reboot too many times quit]\n", dctx.Pid)
			os.Exit(0)
		}
		count++
		begin := time.Now()
		cmd, err := Background(ctx, dctx, WithNoExit())
		if err != nil {
			dctx.log("[supervisor(%d)] [child process start failed, err: %s]\n", dctx.Pid, err)
			errNum++
			continue
		}

		// child process
		if cmd == nil {
			break
		}
		gspt.SetProcTitle(fmt.Sprintf("heart -pid %d", dctx.CPid))
		if count > 2 || isReset {
			if dctx.RestartCallback != nil {
				dctx.RestartCallback(ctx)
			}
		}

		// parent process wait child process exit
		err = cmd.Wait()
		end := time.Now()
		cost := end.Sub(begin)

		// start slow
		if cost < dctx.MinExitTime {
			errNum++
		} else {
			errNum = 0
		}

		if dctx.RestoreTime > 0 && cost > dctx.RestoreTime {
			isReset = true
			count = 1
		}

		if err != nil {
			dctx.log("[supervisor(%d)] [%s] [heart -pid=%d exit] [%d-worked %v] [err: %v]\n", dctx.Pid, dInfo, dctx.CPid, dctx.CPid, cost, err)
		} else {
			dctx.log("[supervisor(%d)] [%s] [heart -pid=%d exit] [%d-worked %v]\n", dctx.Pid, dInfo, dctx.CPid, dctx.CPid, cost)

		}
	}

	return nil
}

func (dctx *Context) log(format string, args ...interface{}) {
	_, fe := fmt.Fprintf(dctx.Logger, format, args...)
	if fe != nil {
		log.Fatal(fe)
	}
}
