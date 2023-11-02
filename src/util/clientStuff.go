package util

import (
	"fmt"
	"log"
	"os"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

func notworkinmain() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file")
	}

	username := ""
	client := Client{
		UrlAddr:  "ws://localhost:" + os.Getenv("PORT") + "/chat",
		Username: username,
		Message: Message{
			Sender:    username,
			Direction: C2S,
			Config:    "",
			Content:   "",
			Password:  os.Getenv("CONN_PASS"),
		},
	}
	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}
	if err := client.Run(); err != nil {
		log.Fatal(err)
	}
}

// ClientError.Simple => Simple description of the error
type ClientError struct {
	Err    error
	Code   int
	Simple string
}

func (ce *ClientError) Error() string {
	return fmt.Sprintf("%v", ce.Simple)
}

type Message struct {
	Sender    string
	Direction string
	Config    string
	Content   string
	Password  string
}

type Client struct {
	UrlAddr    string
	ConnStatus bool
	Conn       *websocket.Conn
	Username   string
	Message    Message
}

const (
	C2A = "client-to-all"
	C2S = "client-to-server"
	S2C = "server-to-client" // Server to single client broadcast
	S2A = "server-to-all"    // Global client broadcast
)

// The first interchange of messages between the client and the server
// Or the handshake
// Used to setup the username and other config info
func (client *Client) Handshake() error {
	client.Message.Direction = C2S
	client.Message.Config = "config-username"

	if err := client.SendMessage(); err != nil {
		return err
	}

	return nil
}

// Lists the clients online
// Only server-side for now
func (client *Client) ListClients() error {
	client.Message.Direction = C2S
	client.Message.Config = "config-list"

	if err := client.SendMessage(); err != nil {
		return err
	}

	return nil
}

// Send method to make the code a bit cleaner
func (client *Client) SendMessage() error {
	if err := client.Conn.WriteJSON(client.Message); err != nil {
		return &ClientError{
			Code:   0,
			Err:    err,
			Simple: "Error sending data to the server...",
		}
	}

	return nil
}

// Send a normal chat message to all the clients
func (client *Client) Send2All(content string) error {
	if len(content) == 0 {
		return nil
	}
	client.Message.Direction = C2A
	client.Message.Config = ""
	client.Message.Content = content

	if err := client.SendMessage(); err != nil {
		return err
	}

	return nil
}

// Close the connection to the server
// Graceful disconnection
func (client *Client) CloseConn() error {
	client.Message.Config = "config-close"
	client.Message.Content = ""
	client.Message.Direction = C2S
	if err := client.Conn.WriteJSON(client.Message); err != nil {
		return &ClientError{
			Code:   0,
			Err:    err,
			Simple: "Error sending data to the server...",
		}
	}
	client.Conn.Close()

	return nil
}

func (client *Client) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(client.UrlAddr, nil)
	if err != nil {
		return err
	}
	client.Conn = conn
	client.ConnStatus = true
	return nil
}

func (client *Client) CheckServerMessage() (Message, error) {
	var receivedMessage Message
	if err := client.Conn.ReadJSON(&receivedMessage); err != nil {
		return Message{}, err
	}
	return receivedMessage, nil
}

// Message transfer loop
func (client *Client) Run() error {
	defer client.Conn.Close()

	client.Handshake()
	for {
		var receivedMessage Message
		if err := client.Conn.ReadJSON(&receivedMessage); err != nil {
			return &ClientError{
				Code:   0,
				Err:    err,
				Simple: "Error reading JSON response",
			}
		}

		fmt.Printf("%s: %s\n", receivedMessage.Sender, receivedMessage.Content)
	}
}

func ClientToAll(clientMessage Message, conn *websocket.Conn) {
	clientMessage.Direction = C2A
	if err := conn.WriteJSON(clientMessage); err != nil {
		log.Fatal("error sending message:", err)
	}
}
