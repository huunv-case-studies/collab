package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	readLimit = 512 // Maximum message length
	pongTimeout = 60 * time.Second
	pingInterval = (pongTimeout * 9) / 10
	writeTimeout = 10 * time.Second
)

var (
	fileName string
	addr = flag.String("addr", ":8081", "http service address")
	homeTemplate = template.Must(template.New("").Parse(homeHtml))
	upgrader = websocket.Upgrader{
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
	}
)

func readFileIfModified(file string, lastMod time.Time) ([]byte, time.Time, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return nil, lastMod, err
	}
	if !fi.ModTime().After(lastMod) {
		return nil, lastMod, nil 
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, lastMod, err
	}
	return data, fi.ModTime(), nil
}

func reader(ws *websocket.Conn) {
	ws.SetReadLimit(readLimit)
	ws.SetReadDeadline(time.Now().Add(pongTimeout))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongTimeout)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func writer(ws *websocket.Conn) {
	fileTick := time.NewTicker(time.Second)
	pingTick := time.NewTicker(pingInterval)
	lastMod := time.Time{}
	defer func() {
		fileTick.Stop()
		pingTick.Stop()
	}()

	for {
		select {
		case <-pingTick.C:
			ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			err := ws.WriteMessage(websocket.BinaryMessage, []byte{})
			if err != nil {
				return
			}
		case <-fileTick.C:
			data, newMod, _ := readFileIfModified(fileName, lastMod)
			if data != nil {
				ws.SetWriteDeadline(time.Now().Add(writeTimeout))
				err := ws.WriteMessage(websocket.BinaryMessage, data)
				if err != nil {
					return
				}
				lastMod = newMod
			}
		}
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	data, _, err := readFileIfModified(fileName, time.Time{})
	if err != nil {
		data = []byte(err.Error())
	}
	var v = struct {
		Host string
		Data string
	} {
		r.Host,
		string(data),
	}
	homeTemplate.Execute(w, &v)
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to create connection", http.StatusInternalServerError)
	}

	go writer(ws)
	reader(ws)
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("File name is not provided")
	}
	fileName = flag.Arg(0)
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.ListenAndServe(*addr, nil)
}

const homeHtml = `<!DOCTYPE html>
<html lang="en">
    <head>
        <title>WebSocket Example</title>
    </head>
    <body>
        <pre id="fileData">{{.Data}}</pre>
        <script type="text/javascript">
            (function() {
                var data = document.getElementById("fileData");
                var conn = new WebSocket("ws://{{.Host}}/ws");
                conn.onclose = function(evt) {
                    data.textContent = 'Connection closed';
                }
                conn.onmessage = async function(evt) {
                    console.log('file updated');
                    data.textContent = await evt.data.text();
                }
            })();
        </script>
    </body>
</html>
`