package daemon

import "fmt"

func (dctx *Context) Information() string{
	return fmt.Sprintf("count:%d/%d; errNum:%d/%d", dctx.Count, dctx.MaxCount, dctx.ErrNum, dctx.MaxError)
}
