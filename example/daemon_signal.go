package main

import (
	"context"
	"github.com/whiteCcinn/daemon"
	"log"
	"os"
	"sync"
	"syscall"
)

func main() {
	logName := "daemon.log"
	stdout, err := os.OpenFile(logName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	dctx := daemon.Context{
		ProcAttr: syscall.SysProcAttr{},
		//Logger:   os.Stdout,
		Logger:   stdout,
		MaxCount: 2,
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
		wg := new(sync.WaitGroup)
		wg.Add(1)
		go func() {
			defer wg.Done()
			daemon.SetSigHandler(func(sig os.Signal) (err error) {
				log.Println("sigint")
				return daemon.ErrStop
			}, syscall.SIGINT)

			daemon.SetSigHandler(func(sig os.Signal) (err error) {
				log.Println("sigterm")
				return daemon.ErrStop
			}, syscall.SIGTERM)

			err := daemon.ServeSignals()
			if err != nil {
				log.Println(err)
			}
		}()

		log.Println(os.Getpid(), "start...")
		//time.Sleep(time.Second * 10)
		log.Println(os.Getpid(), "end...")
		wg.Wait()
	}, nil)
}
