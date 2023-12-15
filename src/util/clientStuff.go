package util

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Returns datetime in the format "2006-01-02 15:04:05"
func GetDateTime() string {
	currentTime := time.Now()
	return currentTime.Format("2006-01-02 15:04:05")
}

// func notworkinmain() {
// 	if err := godotenv.Load(); err != nil {
// 		log.Printf("Error loading .env file")
// 	}

// 	username := ""
// 	client := Client{
// 		UrlAddr:  "ws://localhost:" + os.Getenv("PORT") + "/chat",
// 		Username: username,
// 		Message: Message{
// 			Sender:    username,
// 			Direction: C2S,
// 			Config:    "",
// 			Content:   "",
// 			Password:  os.Getenv("CONN_PASS"),
// 		},
// 	}
// 	if err := client.Connect(); err != nil {
// 		log.Fatal(err)
// 	}
// 	if err := client.Run(); err != nil {
// 		log.Fatal(err)
// 	}
// }

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
	Timestamp string
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

	if err := client.SendMessageStruct(); err != nil {
		return err
	}

	return nil
}

// Lists the clients online
// Only server-side for now
func (client *Client) ListClients() error {
	client.Message.Direction = C2S
	client.Message.Config = "config-list"

	if err := client.SendMessageStruct(); err != nil {
		return err
	}

	return nil
}

// Send method to make the code a bit cleaner
func (client *Client) SendMessageStruct() error {
	if err := client.Conn.WriteJSON(client.Message); err != nil {
		return &ClientError{
			Code:   0,
			Err:    err,
			Simple: "Error sending to server...",
		}
	}
	return nil
}

// Sends the Message struct to the Server
// Checks if command or message and sends appropriate config setup
// If command, content begins with ':'
func (client *Client) SendMsgOrCmd(content string) error {
	commandChar := ":"
	// Normal Chat Message
	if string(content[0]) != commandChar {
		if err := client.Send2All(content); err != nil {
			return err
		}

		return nil
	}

	switch content {
	case ":list":
		client.Message.Direction = C2S
		client.Message.Config = "config-list"
		client.Message.Content = ""
		client.Message.Timestamp = GetDateTime()

	default:
		return &ClientError{
			Code:   0,
			Err:    nil,
			Simple: "Invalid Command:",
		}
	}

	if err := client.SendMessageStruct(); err != nil {
		return &ClientError{
			Code:   0,
			Err:    err,
			Simple: err.Error(),
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
	client.Message.Timestamp = GetDateTime()

	if err := client.SendMessageStruct(); err != nil {
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
