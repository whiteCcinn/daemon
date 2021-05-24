package main

import (
	"context"
	"encoding/json"
	"github.com/whiteCcinn/daemon"
	named_pipe_ipc "github.com/whiteCcinn/named-pipe-ipc"
	"log"
)

func main() {
	nctx, err := named_pipe_ipc.NewContext(context.Background(), "./", named_pipe_ipc.C)
	if err != nil {
		log.Fatal(err)
	}

	message := daemon.NamedPipeMessage{
		Api: daemon.PrintInformation,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = nctx.Send(named_pipe_ipc.Message(string(data)))
	msg, err := nctx.Recv(true)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(msg)
}
