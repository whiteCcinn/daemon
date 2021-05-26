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
	stdout, err := os.OpenFile(logName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	dctx := daemon.Context{
		//Chroot:   "./pipe",
		//PidFile: "./daemon.pid",
		//CPidFile: "./main.pid",
		ProcAttr: syscall.SysProcAttr{},
		//Logger:   os.Stdout,
		Logger:   stdout,
		MaxCount: 2,
		RestartCallback: func(ctx context.Context) {
			log.Println("restart...")
		},
	}

	err = dctx.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// belong func main()
	log.Println(os.Getpid(), "start...")
	time.Sleep(time.Second * 10)
	//panic("This trigger panic")
	log.Println(os.Getpid(), "end")
}
