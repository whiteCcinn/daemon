package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/erikdubbelboer/gspt"
	"github.com/whiteCcinn/named-pipe-ipc"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
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

	Logger      io.Writer
	PanicLogger io.Writer

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
	ExtraFiles []*os.File
	// supervisor pid
	Pid int
	// main pid
	CPid int
	// start count
	Count int
	// start error number
	ErrNum int

	// Restart after callback
	RestartCallback

	namedPipeCtx *named_pipe_ipc.Context

	noNamedPipeOnce sync.Once
	namedPipeOnce   sync.Once
}

type RestartCallback func(ctx context.Context)

// attachContext attach value to Context
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

// Background Converts the current process to a background process
// If `WithNoExit()` is called, it doesn't exit
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
			dctx.log("[supervisor(%d)] [heart --pid=%d]\n", dctx.Pid, dctx.CPid)
		}
	}

	if defaultOption.exit {
		os.Exit(0)
	}

	return cmd, nil
}

// startProc start am process
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

	if dctx.PanicLogger == nil {
		dctx.PanicLogger = dctx.Logger
	}

	if dctx.ExtraFiles != nil {
		cmd.ExtraFiles = dctx.ExtraFiles
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

// Run An supervisor daemon
func (dctx *Context) Run(ctx context.Context) error {
	_, err := Background(ctx, dctx)
	if err != nil {
		log.Fatal(err)
	}

	dctx.Count = 1
	isReset := false
	dctx.ErrNum = 0
	for {
		//daemon information
		if dctx.ErrNum > dctx.MaxError {
			dctx.log("[supervisor(%d)] [child process fails too many times]\n", dctx.Pid)
			os.Exit(1)
		}
		if dctx.MaxCount > 0 && dctx.Count > dctx.MaxCount {
			dctx.log("[supervisor(%d)] [reboot too many times quit]\n", dctx.Pid)
			os.Exit(0)
		}
		dctx.Count++

		r, w, err := os.Pipe()
		if err != nil {
			dctx.log("[supervisor(%d)] [create pipe failed] [err: %v]\n", dctx.Pid, err)
			os.Exit(2)
		}

		// Because there is no need to send from the parent to the child, w2 is not required
		// tips: It might be useful later
		//r2, w2, err := os.Pipe()
		//if err != nil {
		//	dctx.log("[supervisor(%d)] [create pipe failed] [err: %v]\n", dctx.Pid, err)
		//	os.Exit(2)
		//}

		extraFile := make([]*os.File, 0, 2)
		// so fd(3) = w
		extraFile = append(extraFile, w)
		if dctx.ExtraFiles != nil {
			extraFile = append(extraFile, dctx.ExtraFiles...)
		}
		dctx.ExtraFiles = extraFile

		begin := time.Now()
		cmd, err := Background(ctx, dctx, WithNoExit())
		if err != nil {
			dctx.log("[supervisor(%d)] [child process start failed, err: %s]\n", dctx.Pid, err)
			dctx.ErrNum++
			continue
		}

		// child process
		if cmd == nil {
			exitFunc := func(sig os.Signal) (err error) {
				// this is fd(3)
				pipe := os.NewFile(uintptr(3), "pipe")
				message := PipeMessage{
					Type:     ProcessToSupervisor,
					Behavior: WantSafetyClose,
				}
				err = json.NewEncoder(pipe).Encode(message)
				if err != nil {
					panic(err)
				}
				return
			}
			SetSigHandler(exitFunc, syscall.SIGINT, syscall.SIGTERM)
			go ServeSignals()
			break
		}

		// supervisor process
		gspt.SetProcTitle(fmt.Sprintf("heart -pid %d", dctx.CPid))
		if dctx.Count > 2 || isReset {
			if dctx.RestartCallback != nil {
				dctx.RestartCallback(ctx)
			}
		}

		// read from child process
		dctx.noNamedPipeOnce.Do(func() {
			dctx.log("[supervisor(%d)] [no-named-pipe-ipc] [listen]\n", dctx.Pid)
			go func() {
				for {
					var data PipeMessage
					decoder := json.NewDecoder(r)
					if err := decoder.Decode(&data); err != nil {
						log.Printf("decode r, err: %v", err)
						break
					}

					if data.Type != ProcessToSupervisor {
						continue
					}

					if data.Behavior == WantSafetyClose {
						dctx.log("[supervisor(%d)] [stop heart -pid %d] [safety exit]\n", dctx.Pid, dctx.CPid)
						os.Exit(0)
					}
				}
			}()
		})

		// named-pipe-ipc
		dctx.namedPipeOnce.Do(func() {
			dctx.namedPipeCtx, err = named_pipe_ipc.NewContext(context.Background(), "./", named_pipe_ipc.S)
			if err != nil {
				log.Fatal(err)
			}
			dctx.log("[supervisor(%d)] [named-pipe-ipc] [listen]\n", dctx.Pid)
			go func() {
				go func() {
					for {
						msg, err := dctx.namedPipeCtx.Recv(false)
						if err != nil && err.Error() != named_pipe_ipc.NoMessageMessage {
							dctx.log("[supervisor(%d)] [named-pipe-ipc] [err:%v]\n", dctx.Pid, err)
							os.Exit(4)
						}

						if msg == nil {
							if ctx.Err() != nil {
								return
							}
							time.Sleep(500 * time.Millisecond)
							continue
						}

						var epm NamedPipeMessage
						err = json.Unmarshal(msg, &epm)
						if err != nil {
							dctx.log("[supervisor(%d)] [named-pipe-ipc] [err:%v]\n", dctx.Pid, err)
						}

						if epm.Api == PrintInformation {
							ret := dctx.Information()
							_, err = dctx.namedPipeCtx.Send(named_pipe_ipc.Message(ret + "\n"))
							if err != nil {
								dctx.log("[supervisor(%d)] [named-pipe-ipc] [send-error:%v]\n", dctx.Pid, err)
							}
						}
					}
				}()

				err = dctx.namedPipeCtx.Listen()
				if err != nil {
					dctx.log("[supervisor(%d)] [named-pipe-ipc start failed] [err:%v]\n", dctx.Pid, err)
					os.Exit(3)
				}
			}()
		})

		// parent process wait child process exit
		err = cmd.Wait()
		end := time.Now()
		cost := end.Sub(begin)

		// start slow
		if cost < dctx.MinExitTime {
			dctx.ErrNum++
		} else {
			dctx.ErrNum = 0
		}

		if dctx.RestoreTime > 0 && cost > dctx.RestoreTime {
			isReset = true
			dctx.Count = 1
		}

		if err != nil {
			dctx.log("[supervisor(%d)] [%s] [heart -pid=%d exit] [%d-worked %v] [err: %v]\n", dctx.Pid, dctx.Information(), dctx.CPid, dctx.CPid, cost, err)
		} else {
			dctx.log("[supervisor(%d)] [%s] [heart -pid=%d exit] [%d-worked %v]\n", dctx.Pid, dctx.Information(), dctx.CPid, dctx.CPid, cost)
		}
	}

	return nil
}

// output log-message to Context.Logger
func (dctx *Context) log(format string, args ...interface{}) {
	_, fe := fmt.Fprintf(dctx.Logger, format, args...)
	if fe != nil {
		log.Fatal(fe)
	}
}

// output log-message to Context.PanicLogger
func (dctx *Context) logPanic(format string, args ...interface{}) {
	_, fe := fmt.Fprintf(dctx.PanicLogger, format, args...)
	if fe != nil {
		log.Fatal(fe)
	}
}

// WithRecovery wraps goroutine startup call with force recovery.
// it will dump current goroutine stack into log if catch any recover result.
//   exec:      execute logic function.
//   recoverFn: handler will be called after recover and before dump stack, passing `nil` means noop.
func (dctx *Context) WithRecovery(exec func(), recoverFn func(r interface{})) {
	defer func() {
		r := recover()
		if recoverFn != nil {
			recoverFn(r)
		}
		if r != nil {
			dctx.logPanic("panic in the recoverable goroutine, error: %v\n", r)
		}
	}()
	exec()
}
