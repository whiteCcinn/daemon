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

	_, err = daemon.Background(context.Background(), &daemon.Context{
		ProcAttr: syscall.SysProcAttr{},
		//Logger:   os.Stdout,
		Logger: stdout,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(os.Getpid(), "start...")
	time.Sleep(time.Second * 10)
	log.Println(os.Getpid(), "end")
}
