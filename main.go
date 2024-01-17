package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
}

var clients = make(map[*websocket.Conn]bool) // Connected WebSocket clients

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	clients[conn] = true

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			delete(clients, conn)
			break
		}
	}
}

func modifyResponse(res *http.Response) error {
	if res.Header.Get("Content-Type") == "text/html" {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		err = res.Body.Close()
		if err != nil {
			return err
		}

		// Append WebSocket script to the end of the body
		script := `<script>(function() { var ws = new WebSocket('ws://localhost:8000/ws'); ws.onmessage = function() { window.location.reload(); }; })();</script>`
		body = append(body, script...)

		res.Body = ioutil.NopCloser(bytes.NewReader(body))
		res.ContentLength = int64(len(body))
		res.Header.Set("Content-Length", strconv.Itoa(len(body)))
	}
	return nil
}

// monitorServerStatus periodically checks if the server is online and notifies clients on status change
func monitorServerStatus() {
	var serverOnline bool = false

	for {
		// Attempt to connect to the server
		_, err := net.Dial("tcp", "localhost:8080")

		if err != nil {
			// If there's an error connecting, the server is offline
			if serverOnline {
				// If the server was previously online, change status and log
				serverOnline = false
				log.Println("Server went offline.")
			}
		} else {
			// No error means the server is online
			if !serverOnline {
				// If the server was previously offline, notify all clients
				serverOnline = true
				log.Println("Server is back online. Notifying clients.")
				notifyClients()
			}
		}

		time.Sleep(1 * time.Second) // Check every second
	}
}

// notifyClients sends a message to all connected WebSocket clients
func notifyClients() {
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, []byte("reload"))
		if err != nil {
			log.Println("Error sending message:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func main() {
	// URL of the actual Go server
	target := "http://localhost:8080"
	url, _ := url.Parse(target)

	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.ModifyResponse = modifyResponse

	// Set up WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	// Start the proxy server
	log.Println("Starting proxy server on :8000")
	http.ListenAndServe(":8000", nil)
}
