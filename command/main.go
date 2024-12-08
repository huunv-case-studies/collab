package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsMaxMsgSize = 8192
	wsPongWait = 60 * time.Second
	wsPingInterval = (9*wsPongWait)/10
	wsWriteWait = 5 * time.Second
)

var (
	command string
	upgrader = websocket.Upgrader {
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
	}
)

// Accept client input and send to the command stdin
func writeStdin(ws *websocket.Conn, w io.Writer) {
	for {
		_, data, err := ws.ReadMessage();
		if err != nil {
			log.Println("websocket:", err.Error())
			break
		}
		_, err = w.Write(append(data, '\n'))
		if err != nil {
			log.Println("io:", err.Error())
			break
		}
	}
}

// Send the command output to the client
func readStdOut(ws *websocket.Conn, r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
		if err := ws.WriteMessage(websocket.BinaryMessage, s.Bytes()); err != nil {
			log.Println("websocket:", err.Error())
			return
		}
	}
	if s.Err() != nil {
		log.Println("bufio:", s.Err().Error())
	}
}

func startCommand(ctx context.Context, command string, argv []string) (io.Writer, io.Reader, error) {
	inr, inw, err := os.Pipe()
	if err != nil {
		log.Println("os:", err.Error())	
		return nil, nil, err
	}
	outr, outw, err := os.Pipe()
	if err != nil {
		log.Println("os:", err.Error())	
		return nil, nil, err
	}
	pro, err := os.StartProcess(command, argv, &os.ProcAttr{
		Files: []*os.File { inr, outw, outw },
	})
	if err != nil {
		return nil, nil, err
	}

	go func() {
		defer inr.Close()
		defer inw.Close()
		defer outr.Close()
		defer outw.Close()
		defer pro.Signal(os.Interrupt)
		<-ctx.Done()	
	}()

	return inw, outr, nil
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r,"home.html")
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err.Error())
	}
	ws.SetReadLimit(wsMaxMsgSize)
	ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
	ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(wsPongWait)); return nil })

	defer func() {
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		time.Sleep(time.Second)
		ws.Close()
	}()
	
	ctx, cancel := context.WithCancel(context.Background())
	stdinWriter, stdoutReader, err := startCommand(ctx, command, []string{})
	if err != nil {
		log.Fatal(err.Error())
	}

	go func() {
		readStdOut(ws, stdoutReader)
		cancel()
	}()

	go func() {
		writeStdin(ws, stdinWriter)
		cancel()
	}()

	go func() {
		pingTicker := time.NewTicker(wsPingInterval)
		
		defer cancel()
		defer pingTicker.Stop()

		for range pingTicker.C {
			ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				break
			}
		}
	}()

	<-ctx.Done()
}

func main() {
	addr := flag.String("addr", ":8081", "host")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("Only one argument is expected.")
		return
	}
	command = flag.Arg(0)

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.ListenAndServe(*addr, nil)
}