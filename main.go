package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

var content embed.FS

const CACHE_SIZE = 1024

var syncronizedTimes = make(map[string]uint64)

var snapshot = "{}"

type Transaction struct {
	Source  string
	Id      uint64
	Payload string
}

var tList = make([]Transaction, 0)
var tq = make(chan Transaction)
var q = make(chan int)
var mutex sync.Mutex
var localTransactionCounter uint64 = 1

var (
	port string
	name string
)

func init() {
	flag.StringVar(&port, "port", "8080", "The server port to listen on.")
	flag.StringVar(&name, "name", "default", "The name identifier for the server.")
}

func main() {
	flag.Parse()
	log.Printf("Starting server on port %s with name %s...\n", port, name)

	go runHandlers(port)
	go runTransactionHandler()
	go runReplicationHandler()

	stop := make(chan struct{})
	<-stop
}

func getHandle(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(snapshot))
}

func replaceHandle(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	tq <- Transaction{Source: name, Id: localTransactionCounter, Payload: string(data)}
	result := <-q

	localTransactionCounter++
	w.WriteHeader(result)
}

func replicaReaderRoutine(c *websocket.Conn, ctx context.Context, peer string) {
	for {
		var transaction Transaction
		err := wsjson.Read(ctx, c, &transaction)

		if err != nil {
			break
		}

		tq <- transaction
		<-q
	}
}

func replicationDialRoutine(peer string, port string) {
	routineUrl := peer + ":" + port

	for {
		var ctx = context.Background()
		c, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://%s/ws", routineUrl), nil)

		if err != nil {
			return
		}

		defer c.Close(websocket.StatusInternalError, "Internal server error")

		replicaReaderRoutine(c, ctx, routineUrl)
		c.Close(websocket.StatusNormalClosure, "")

		time.Sleep(5 * time.Second)
	}
}

func runReplicationHandler() {
	f, _ := os.Open("peers.txt")
	defer f.Close()

	var peerList [CACHE_SIZE]byte
	n, _ := f.Read(peerList[:])

	for _, peer := range strings.Split(string(peerList[:n]), "\n") {
		go replicationDialRoutine(peer, "8080")
	}
}

func handleTransaction() {
	t := <-tq

	mutex.Lock()
	defer mutex.Unlock()

	if syncronizedTimes[t.Source] >= t.Id {
		q <- http.StatusOK
		return
	}

	syncronizedTimes[t.Source] = t.Id

	patch, err := jsonpatch.DecodePatch([]byte(t.Payload))
	if err != nil {
		q <- http.StatusBadRequest
		return
	}

	tList = append(tList, t)
	new, err := patch.Apply([]byte(snapshot))

	if err != nil {
		q <- http.StatusBadRequest
		return
	}
	snapshot = string(new)

	q <- http.StatusOK
}

func runTransactionHandler() {
	for {
		handleTransaction()
	}
}

func vclockHandle(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(syncronizedTimes)
	if err != nil {
		panic(err)
	}

	w.Write(data)
}

func replicaWriter(c *websocket.Conn, ctx context.Context) {
	var count = 0

	for {
		time.Sleep(100 * time.Millisecond)

		for ; count < len(tList); count++ {
			err := wsjson.Write(ctx, c, tList[count])
			if err != nil {
				c.Close(websocket.StatusInternalError, err.Error())
				return
			}
		}
	}
}

func wsHandle(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})

	if err != nil {
		panic(err)
	}

	defer c.Close(websocket.StatusInternalError, "Internal server error")

	ctx := r.Context()

	var replicationChannel = make(chan Transaction, 100)
	defer close(replicationChannel)

	replicaWriter(c, ctx)

	c.Close(websocket.StatusNormalClosure, "")
}

func runHandlers(port string) {
	mux := http.NewServeMux()

	mux.Handle("/test/", http.StripPrefix("/test/", http.FileServer(http.FS(content))))
	mux.HandleFunc("/vclock", vclockHandle)
	mux.HandleFunc("/replace", replaceHandle)
	mux.HandleFunc("/get", getHandle)
	mux.HandleFunc("/ws", wsHandle)

	log.Printf("Listening on port %s...", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
}
