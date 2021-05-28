package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/erikdubbelboer/gspt"
	"github.com/whiteCcinn/named-pipe-ipc"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultChroot = "./"
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
	Chroot   string
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

	// supervisor pid file
	PidFile string
	// main pid file
	CPidFile string
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
	if len(dctx.Chroot) == 0 {
		dctx.Chroot = defaultChroot
	}

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
	// console show process information just use Args[0]
	// so we should wrapper the args for show process full command
	execs := strings.Split(dctx.Args[0], " ")
	cmd := &exec.Cmd{
		Path:        execs[0],
		Args:        []string{strings.Join(dctx.Args, " ")},
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
	err = dctx.syncPidIntoFile()
	if err != nil {
		dctx.log("[supervisor(%d)] [syncPidIntoFile] [%v]\n", dctx.Pid, err)
		os.Exit(1)
	}
	for {
		//daemon information
		if dctx.ErrNum > dctx.MaxError {
			dctx.log("[supervisor(%d)] [child process fails too many times]\n", dctx.Pid)
			dctx.cleanAll()
			os.Exit(1)
		}
		if dctx.MaxCount > 0 && dctx.Count > dctx.MaxCount {
			dctx.log("[supervisor(%d)] [reboot too many times quit]\n", dctx.Pid)
			dctx.cleanAll()
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
			go func() {
				ch := make(chan os.Signal, 2)
				exitFunc := func(sig os.Signal) {
					// this is fd(3)
					pipe := os.NewFile(uintptr(3), "pipe")
					message := PipeMessage{
						Type:     ProcessToSupervisor,
						Behavior: WantSafetyClose,
					}
					err = json.NewEncoder(pipe).Encode(message)
					if err != nil && !strings.Contains(err.Error(), "broken pipe") {
						panic(err)
					}
					return
				}
				signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

				for sig := range ch {
					exitFunc(sig)
				}
			}()
			break
		}

		err = dctx.syncCPidIntoFile()
		if err != nil {
			dctx.log("[supervisor(%d)] [syncCPidIntoFile] [%v]\n", dctx.Pid, err)
			os.Exit(1)
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
						dctx.cleanCPidEtc()
						dctx.log("[supervisor(%d)] [stop heart -pid %d] [safety exit]\n", dctx.Pid, dctx.CPid)
						os.Exit(0)
					}
				}
			}()
		})

		// named-pipe-ipc
		dctx.namedPipeOnce.Do(func() {
			dctx.namedPipeCtx, err = named_pipe_ipc.NewContext(context.Background(), dctx.Chroot, named_pipe_ipc.S)
			if err != nil {
				log.Fatal(err)
			}
			dctx.log("[supervisor(%d)] [named-pipe-ipc] [listen]\n", dctx.Pid)
			SetSigHandler(func(sig os.Signal) (err error) {
				dctx.cleanNamedPipeEtc().cleanPidEtc()
				os.Exit(0)
				return
			}, syscall.SIGINT, syscall.SIGTERM)
			go ServeSignals()

			go func() {
				go func() {
					for {
						msg, err := dctx.namedPipeCtx.Recv(false)
						if err != nil && (err.Error() != named_pipe_ipc.NoMessageMessage && err.Error() != named_pipe_ipc.PipeClosedMessage) {
							dctx.log("[supervisor(%d)] [named-pipe-ipc] [err:%v]\n", dctx.Pid, err)
							os.Exit(4)
						}

						if msg == nil {
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
							_, err = dctx.namedPipeCtx.Send(named_pipe_ipc.Message(ret))
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

func (dctx *Context) cleanNamedPipeEtc() *Context {
	if dctx.namedPipeCtx != nil {
		if err := dctx.namedPipeCtx.Close(); err != nil {
			dctx.log("[supervisor(%d)] [clean named-pipe-ipc] [close failed] [%v]\n", dctx.Pid, err)
		}
	}

	return dctx
}

func (dctx *Context) cleanPidEtc() *Context {
	if len(dctx.PidFile) > 0 {
		if err := os.Remove(dctx.PidFile); err != nil {
			dctx.log("[supervisor(%d)] [clean pic-file] [move failed] [%v]\n", dctx.Pid, err)
		}
	}

	return dctx
}

func (dctx *Context) cleanCPidEtc() *Context {
	if len(dctx.CPidFile) > 0 {
		if err := os.Remove(dctx.CPidFile); err != nil {
			dctx.log("[supervisor(%d)] [clean cpic-file] [move failed] [%v]\n", dctx.Pid, err)
		}
	}

	return dctx
}

func (dctx *Context) cleanPidFiles() {
	dctx.cleanPidEtc().cleanCPidEtc()
}

func (dctx *Context) cleanAll() {
	dctx.cleanNamedPipeEtc().cleanPidEtc().cleanCPidEtc()
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

// syncPidIntoFile is sync pid into file
func (dctx *Context) syncPidIntoFile() (err error) {
	if len(dctx.PidFile) > 0 {
		file, err := os.OpenFile(dctx.PidFile, os.O_WRONLY|os.O_CREATE, 0644)
		defer func() {
			err = file.Close()
			if err != nil {
				dctx.log("[supervisor(%d)] [sync pid into file] [close failed] [%v]\n", dctx.Pid, err)
			}
		}()
		if err != nil {
			return err
		}
		pid := strconv.Itoa(dctx.Pid)
		_, err = file.WriteString(pid)
		if err != nil {
			return err
		}
	}

	return
}

// syncCPidIntoFile is sync cpid into file
func (dctx *Context) syncCPidIntoFile() (err error) {
	if len(dctx.CPidFile) > 0 {
		file, err := os.OpenFile(dctx.CPidFile, os.O_WRONLY|os.O_CREATE, 0644)
		defer func() {
			err = file.Close()
			if err != nil {
				dctx.log("[supervisor(%d)] [sync cpid into file] [close failed] [%v]\n", dctx.Pid, err)
			}
		}()
		if err != nil {
			return err
		}
		pid := strconv.Itoa(dctx.CPid)
		_, err = file.WriteString(pid)
		if err != nil {
			return err
		}
	}

	return
}

// ExistByPidFile exists by pid file read pid
func ExistByPidFile(pidFile string) (err error, exist bool) {
	file, err := os.OpenFile(pidFile, os.O_RDONLY, 0644)
	defer file.Close()
	if err != nil {
		return err, false
	}
	pidStr, err := ioutil.ReadAll(file)
	if err != nil {
		return err, false
	}
	pid, err := strconv.Atoi(string(pidStr))
	if err != nil {
		return err, false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err, false
	}
	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return err, false
	} else {
		return nil, true
	}
}
