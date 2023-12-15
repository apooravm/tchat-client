package main

// //go:generate go run -tags embedenv gen_embedded_env.go
import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/apooravm/tchat-client/src/util"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

var (
	client      util.Client
	usernameSet bool
	devMode     bool = false
	// envFile     embed.FS

	urlAddr   string = "wss://multi-serve.onrender.com/api/chat"
	pass      string
	loadEnv   bool = false
	timestamp bool = false
)

type (
	errMsg error
)

const (
	SERVER_ERR = 500
	SELF_ERR   = 400
)

func helper() {
	fmt.Println("tchat Helper")
	fmt.Println("Format: 'tchat [FLAG]' OR 'tchat [FLAG]=[VALUE]'")
	fmt.Println("\nDefaults")
	fmt.Println("url=" + urlAddr)
	fmt.Println("loadEnv=" + "false")
	fmt.Println("timestamp=" + "false")
	fmt.Println("pass=" + pass)

	fmt.Println("\nIn Chat Commands")
	fmt.Println(":list => List Users Online")
}

func main() {
	cli_args := os.Args[1:]

	for _, val := range cli_args {
		flag_KV := strings.Split(val, "=")
		if len(flag_KV) == 2 {
			switch flag_KV[0] {
			case "url":
				urlAddr = flag_KV[1]

			case "pass":
				pass = flag_KV[1]

			case "loadEnv":
				if flag_KV[1] == "true" {
					loadEnv = true
				}

			case "timestamp":
				if flag_KV[1] == "true" {
					timestamp = true
				}

			default:
				fmt.Println("Invalid Command\nTry 'tchat help'")
				return
			}

		} else if len(flag_KV) == 1 {
			switch flag_KV[0] {
			case "help":
				helper()
				return

			default:
				fmt.Println("Invalid Command\nTry 'tchat help'")
				return
			}

		} else {
			fmt.Println("Invalid Command format\nTry 'tchat help'")
			return
		}
	}

	pass = "1234"

	if loadEnv {
		if err := godotenv.Load(); err != nil {
			log.Printf("Error loading .env file")

		} else {
			pass = os.Getenv("PASS")
		}
	}

	client = util.Client{
		UrlAddr:    urlAddr,
		Username:   "unset",
		ConnStatus: false,
		Message: util.Message{
			Sender:    "unset",
			Direction: "client-to-server",
			Config:    "",
			Content:   "",
			Password:  pass,
			Timestamp: util.GetDateTime(),
		},
	}
	p := tea.NewProgram(initialModel())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func LogData(data string) error {
	if devMode {
		return nil
	}
	logFilePath := os.Getenv("LOG_PATH")
	file, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return &util.ClientError{
			Err:    err,
			Code:   SELF_ERR,
			Simple: "Error opening the log file",
		}
	}
	defer file.Close()

	currentTime := time.Now()
	timeString := currentTime.Format("15:04:05")
	data = timeString + " " + data + "\n"

	_, err = file.WriteString(data)
	if err != nil {
		return &util.ClientError{
			Err:    err,
			Code:   SELF_ERR,
			Simple: "Error logging",
		}
	}

	return nil
}

type ChatModel struct {
	usernameInput        textinput.Model
	vpWidth              int
	vpHeight             int
	msgInputWidth        int
	viewport             viewport.Model
	messages             []string
	textarea             textarea.Model
	serverMessageStyle   lipgloss.Style
	incomingMessageStyle lipgloss.Style
	selfMessageStyle     lipgloss.Style
	errorMessageStyle    lipgloss.Style
	errToggle            bool
	err                  error
}

func initialModel() ChatModel {
	// Input in username
	usernameInput := textinput.New()
	usernameInput.Placeholder = "right here..."
	usernameInput.Focus()
	usernameInput.CharLimit = 156
	usernameInput.Width = 20

	// Chat UI
	vpWidth := 0
	vpHeight := 18
	msgInputWidth := 30

	messageInput := textarea.New()
	messageInput.Placeholder = "Send a message..."
	messageInput.Focus()

	messageInput.Prompt = "┃ "
	messageInput.CharLimit = 280

	messageInput.SetWidth(msgInputWidth)
	messageInput.SetHeight(2)

	messageInput.FocusedStyle.CursorLine = lipgloss.NewStyle()

	messageInput.ShowLineNumbers = false

	chatHistory := viewport.New(vpWidth, vpHeight)
	chatHistory.SetContent(`Welcome to the chat room!
Type a message and press Enter to send.`)

	messageInput.KeyMap.InsertNewline.SetEnabled(false)

	return ChatModel{
		usernameInput:        usernameInput,
		vpWidth:              vpWidth,
		vpHeight:             vpHeight,
		msgInputWidth:        msgInputWidth,
		textarea:             messageInput,
		messages:             []string{},
		viewport:             chatHistory,
		incomingMessageStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#c674d6")),
		serverMessageStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#1db1c2")),
		selfMessageStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#75c21d")),
		errorMessageStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#a11a29")),
		errToggle:            false,
		err:                  nil,
	}
}

func (m ChatModel) Init() tea.Cmd {
	if !usernameSet {
		return textinput.Blink
	}
	// Connect to server
	if err := client.Connect(); err != nil {
		LogData("Init Error: Trying to Connect to the Server")
		return ReturnClientError(err, SERVER_ERR, "Server Connection Failed: Try Again Later!")
	}

	client.Handshake()
	return tea.Batch(textarea.Blink, ListenServerMessages)
}

// The connection can fail midway through
// If this method is not "turned off", then a panic error will be thrown
// if the error occurs >1000 times
func ListenServerMessages() tea.Msg {
	receivedMessage, err := client.CheckServerMessage()
	if err != nil {
		client.ConnStatus = false
		return util.ClientError{
			Err:    err,
			Code:   SERVER_ERR,
			Simple: "Disconnected!",
		}
	}

	LogData("Message from Server: " + receivedMessage.Content)
	return receivedMessage
}

// If the message length is greater than the viewport width,
// it doesnt show the message
// This method wraps the message by adding a \n char
// Done by word or char??
func (m ChatModel) WrapMessageStrings(message string, vpWidth int) string {
	if vpWidth == 0 {
		return message
	}

	if len(message) <= vpWidth {
		return message
	}

	var messageStringArr []string
	lenCount := 0
	for _, word := range strings.Split(message, " ") {
		if len(word)+lenCount <= vpWidth {
			lenCount += len(word)
			messageStringArr = append(messageStringArr, word)
		} else {
			lenCount = len(word)
			messageStringArr = append(messageStringArr, "\n"+word)
		}
	}
	return strings.Join(messageStringArr, " ")
}

func (m ChatModel) FormatChatMessage(message util.Message, senderStyle lipgloss.Style) string {
	if message.Sender == "Server" {
		senderStyle = m.serverMessageStyle
	}
	extra := ""
	if timestamp {
		extra = "\n" + message.Timestamp
	}
	sender := senderStyle.Render(message.Sender + ": ")
	content := message.Content
	return m.WrapMessageStrings(sender+content+extra, m.vpWidth)
}

func (m ChatModel) FormatChatMessageString(message string, senderStyle lipgloss.Style) string {
	extra := ""
	if timestamp {
		extra = "\n" + util.GetDateTime()
	}
	sender := senderStyle.Render("You: ")
	content := message
	return m.WrapMessageStrings(sender+content+extra, m.vpWidth)
}

func (m ChatModel) FormatChatErrorString(message string) string {
	return m.errorMessageStyle.Render(message)
}

func ReturnClientError(err error, code int, simple string) tea.Cmd {
	return func() tea.Msg {
		return util.ClientError{
			Err:    err,
			Code:   code,
			Simple: simple,
		}
	}
}

// Message received from ListenServerMessages method
// Which returns a tea.Msg of type util.Message
// This is then passed through the formatter and then a string wrapper method
// If the message is from the input, they are passed to another formatter that
// takes in strings and a different lipgloss.Style type
//
// Every client request is first met with a connection checking condition
// Done so the user cannot crash the app in any way.
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd       tea.Cmd
		vpCmd       tea.Cmd
		usernameCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if client.ConnStatus {
				if err := client.CloseConn(); err != nil {
					m.messages = append(m.messages, m.FormatChatErrorString(err.Error()))
					m.viewport.SetContent(strings.Join(m.messages, "\n"))
					m.viewport.GotoBottom()

				} else {
					return m, tea.Quit
				}
			} else {
				return m, tea.Quit
			}

		case tea.KeyEnter:
			if !usernameSet {
				username := m.usernameInput.Value()
				if len(username) != 0 {
					client.Username = username
					client.Message.Sender = username
					usernameSet = true
					m.textarea.Reset()
					return m, m.Init()
				}
			}
			if len(m.textarea.Value()) != 0 {
				m.messages = append(m.messages, m.FormatChatMessageString(m.textarea.Value(), m.selfMessageStyle))
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				if client.ConnStatus {
					if err := client.SendMsgOrCmd(m.textarea.Value()); err != nil {
						m.messages = append(m.messages, m.FormatChatErrorString(err.Error()))
						m.viewport.SetContent(strings.Join(m.messages, "\n"))
						m.viewport.GotoBottom()
					}
				}
				m.textarea.Reset()
			}
		}

	case tea.WindowSizeMsg:
		m.vpWidth = msg.Width
		m.vpHeight = msg.Height

	case util.ClientError:
		if !m.errToggle {
			m.messages = append(m.messages, m.FormatChatErrorString(msg.Simple))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()

			m.errToggle = true
		}

	case util.Message:
		m.messages = append(m.messages, m.FormatChatMessage(msg, m.incomingMessageStyle))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case errMsg:
		m.err = msg
		return m, nil
	}

	if !usernameSet {
		m.usernameInput, usernameCmd = m.usernameInput.Update(msg)
		return m, usernameCmd
	}

	if client.ConnStatus {
		return m, tea.Batch(tiCmd, vpCmd, ListenServerMessages)
	} else {
		return m, tea.Batch(tiCmd, vpCmd)
	}
}

func (m ChatModel) View() string {
	if !usernameSet {
		return fmt.Sprintf(
			"Your Username\n\n%s\n\n%s",
			m.usernameInput.View(),
			"(esc or ctrl+c to quit)",
		) + "\n"

	}
	return fmt.Sprintf(
		"%s\n\n%s",
		m.viewport.View(),
		m.textarea.View(),
	) + "\n"
}
