# daemon
Go daemon mode for the process

## Features

- supervisor
- restart count
- restart callback
- custom logger
- worked time

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