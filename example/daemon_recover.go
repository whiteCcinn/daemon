package main

import (
	"context"
	"github.com/whiteCcinn/daemon"
	"log"
	"os"
	"syscall"
	"time"
)

func main() {
	logName := "daemon.log"
	panicLogName := "panic-daemon.log"
	stdout, err := os.OpenFile(logName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	panicStdout, err := os.OpenFile(panicLogName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	dctx := daemon.Context{
		ProcAttr: syscall.SysProcAttr{},
		//Logger:   os.Stdout,
		Logger:      stdout,
		PanicLogger: panicStdout,
		MaxCount:    2,
		RestartCallback: func(ctx context.Context) {
			log.Println("restart...")
		},
	}

	parent, err := dctx.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if parent {
		return
	}

	// belong func main()
	dctx.WithRecovery(func() {
		log.Println(os.Getpid(), "start...")
		time.Sleep(time.Second * 3)
		panic("This trigger panic")
		log.Println(os.Getpid(), "end")
	}, nil)
}
