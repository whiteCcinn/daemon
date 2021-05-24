package daemon

import "fmt"

func (dctx *Context) Information() string {
	return fmt.Sprintf("[supervisor-pid: %d] [pid: %d] [count: %d/%d] [errNum: %d/%d]", dctx.Pid, dctx.CPid, dctx.Count-1, dctx.MaxCount, dctx.ErrNum, dctx.MaxError)
}
