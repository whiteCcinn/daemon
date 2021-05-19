# daemon
Go daemon mode for the process

## Features

- supervisor
- restart count
- restart callback
- custom logger
- worked time
- panic recover

## Installation

```shell
go get github.com/whiteCcinn/daemon
```

## Usage

```go
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

```

## Other Examples
- [backgroud](https://github.com/whiteCcinn/daemon/blob/main/example/backgroud.go)
- [daemon](https://github.com/whiteCcinn/daemon/blob/main/example/daemon.go)
- [daemon-recover](https://github.com/whiteCcinn/daemon/blob/main/example/daemon_recover.go)
- [daemon-signal](https://github.com/whiteCcinn/daemon/blob/main/example/daemon_signal.go)

## Log

```log
[process(pid=2719)] [started]
[supervisor(2713)] [heart --pid=2719]
2021/05/18 10:13:25 2719 start...
2021/05/18 10:13:35 2719 end
[supervisor(2713)] [count:1/2; errNum:0/0] [heart -pid=2719 exit] [2719-worked 10.0194103s]
[process(pid=2725)] [started]
[supervisor(2713)] [watch --pid=2725]
2021/05/18 10:13:35 restart...
2021/05/18 10:13:35 2725 start...
2021/05/18 10:13:45 2725 end
[supervisor(2713)] [count:2/2; errNum:0/0] [heart -pid=2725 exit] [2725-worked 10.0305976s]
[supervisor(2713)] [reboot too many times quit]
[process(pid=2930)] [started]
[supervisor(2924)] [watch --pid=2930]
2021/05/18 10:14:20 2930 start...
panic: This trigger panic

goroutine 1 [running]:
main.main()
	/www/example/daemon.go:38 +0x2c8
[supervisor(2924)] [count:1/2; errNum:0/0] [heart -pid=2930 exit] [2930-worked 10.0413272s] [err: exit status 2]
[process(pid=2936)] [started]
[supervisor(2924)] [heart --pid=2936]
2021/05/18 10:14:30 restart...
2021/05/18 10:14:30 2936 start...
panic: This trigger panic

goroutine 1 [running]:
main.main()
	/www/example/daemon.go:38 +0x2c8
[supervisor(2924)] [count:2/2; errNum:0/0] [heart -pid=2936 exit] [2936-worked 10.0428623s] [err: exit status 2]
[supervisor(2924)] [reboot too many times quit]


[process(1648)] [started]
[supervisor(1642)] [heart --pid=1648]
2021/05/19 07:46:12 1648 start...
2021/05/19 07:46:22 1648 end
[supervisor(1642)] [count:1/2; errNum:0/0] [heart -pid=1648 exit] [1648-worked 10.0249616s]
[process(1661)] [started]
[supervisor(1642)] [heart --pid=1661]
2021/05/19 07:46:22 restart...
2021/05/19 07:46:22 1661 start...
2021/05/19 07:46:32 1661 end
[supervisor(1642)] [count:2/2; errNum:0/0] [heart -pid=1661 exit] [1661-worked 10.0243316s]
[supervisor(1642)] [reboot too many times quit]
[process(1782)] [started]
[supervisor(1775)] [heart --pid=1782]
2021/05/19 07:50:59 1782 start...
2021/05/19 07:51:05 sigterm
[supervisor(1775)] [stop heart -pid 1782] [safety exit]
2021/05/19 07:51:09 1782 end...
```

## Terminal

```
root@87ced9181ef6:/www/example# ps -ef
UID        PID  PPID  C STIME TTY          TIME CMD
root         1     0  0 07:50 pts/0    00:00:00 bash
root      1930     1  0 09:27 pts/0    00:00:00 heart -pid 1936
root      1936  1930  0 09:27 pts/0    00:00:00 ./daemon

root@87ced9181ef6:/www/example# ps -ef
UID        PID  PPID  C STIME TTY          TIME CMD
root         1     0  0 07:50 pts/0    00:00:00 bash
root      1930     1  0 09:27 pts/0    00:00:00 heart -pid 1937
root      1937  1930  0 09:27 pts/0    00:00:00 ./daemon
```