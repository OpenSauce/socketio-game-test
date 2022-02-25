package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/tomknightdev/socketio-game-test/client/entities"
	"github.com/tomknightdev/socketio-game-test/client/gui"
	"github.com/tomknightdev/socketio-game-test/messages"
)

var (
	//go:embed resources/characters.png
	characters      []byte
	CharactersImage *ebiten.Image
	//go:embed resources/environments.png
	environments      []byte
	EnvironmentsImage *ebiten.Image
)

var addr string //= flag.String("addr", "localhost:8000", "http service address")

type Client struct {
	SendChan       chan *messages.Message
	RecvChan       chan *messages.Message
	NetworkPlayers []*entities.NetworkPlayer
	Player         *entities.Player
}

var ChatWindow = &gui.Chat{}
var client = Client{}

func init() {
	client.SendChan = make(chan *messages.Message)
	client.RecvChan = make(chan *messages.Message)

	img, err := png.Decode(bytes.NewReader(characters))
	if err != nil {
		log.Fatal(err)
	}
	CharactersImage = ebiten.NewImageFromImage(img)

	img, err = png.Decode(bytes.NewReader(environments))
	if err != nil {
		log.Fatal(err)
	}
	EnvironmentsImage = ebiten.NewImageFromImage(img)
}

func connectToServer(g *Game) error {
	fmt.Println("Client starting...")

	addr = g.serverAddr

	u := url.URL{Scheme: "ws", Host: addr, Path: "/connect"}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send connection request
	connectRequest := messages.NewConnectRequestMessage(messages.ConnectRequestContents{
		Username: g.username,
		Password: g.password,
	})

	if err = conn.WriteJSON(connectRequest); err != nil {
		return err
	}

	// Receive
	go func() {
		for {
			_, readMessage, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Failed to read message: %v", err)
				continue
			}

			message := &messages.Message{}
			if err = json.Unmarshal(readMessage, message); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
			}

			switch message.MessageType {
			case messages.ConnectResponseMessage:
				handleConnectResponse(message, g)
			case messages.FailedToConnectMessage:
				err := message.Contents.(error)
				log.Printf("Failed to connect to server: %v", err)
			case messages.ChatMessage:
				receiveChatMessage(message)
			case messages.UpdateMessage:
			case messages.ServerEntityUpdateMessage:
			}
		}
	}()

	// Send
	for {
		message := <-client.SendChan
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Failed to send message: %v - %v", message, err)
		}
	}
}

func handleConnectResponse(message *messages.Message, g *Game) {
	// If successful, the we receive our server client id
	messageContents := message.Contents.(uint16)

	client.Player = entities.NewPlayer(CharactersImage)
	client.Player.Username = g.username
	client.Player.Id = messageContents

	go func(client Client) {
		message := <-client.Player.SendChan
		client.SendChan <- messages.NewUpdateMessage(client.Player.Id, message)
	}(client)

	g.Player = client.Player

	// Now logged in, build world
	world := entities.NewWorld(EnvironmentsImage)
	g.Environment = append(g.Entities, world)

	ChatWindow = gui.NewChat(g.screenWidth, g.screenHeight)
	g.Gui = append(g.Gui, ChatWindow)

	// Messages from chat send channel will be forwarded to the client send channel
	go func(client Client, chat *gui.Chat) {
		message := <-chat.SendChan
		client.SendChan <- messages.NewChatMessage(client.Player.Id, message)
	}(client, ChatWindow)
}

func receiveChatMessage(message *messages.Message) {
	messageContents := message.Contents.(string)

	ChatWindow.RecvMessages = append(ChatWindow.RecvMessages, messageContents)
}

// func gameLoop(g *Game) error {

// 	u := url.URL{Scheme: "ws", Host: addr, Path: "/game"}

// 	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	if err != nil {
// 		return err
// 	}
// 	defer c.Close()

// 	// Announce connection
// 	em := &messages.EntityMessage{
// 		EntityId:  client.Player.Id,
// 		EntityPos: f64.Vec2{-1, 0},
// 	}
// 	glm := &messages.GameLoopMessage{
// 		EntityMessages: []messages.EntityMessage{
// 			*em,
// 		},
// 	}
// 	c.WriteJSON(glm)

// 	// Receive
// 	go func() {
// 		for {
// 			_, message, err := c.ReadMessage()
// 			if err != nil {
// 				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
// 					log.Printf("error: %v", err)
// 				} else {
// 					fmt.Printf("error in reading message: %v %s", err, message)
// 				}
// 				break
// 			}

// 			var glm = &messages.GameLoopMessage{}
// 			if err = json.Unmarshal([]byte(message), glm); err != nil {
// 				fmt.Printf("unmarshal error:", err, glm, message)
// 			}

// 			for _, em := range glm.EntityMessages {
// 				// Update information about other players
// 				e := func(m messages.EntityMessage) *entities.NetworkPlayer {
// 					for _, e := range client.NetworkPlayers {
// 						if e.Id == m.EntityId {
// 							return e
// 						}
// 					}
// 					return nil
// 				}(em)

// 				// If doesn't exist, create it
// 				if e == nil {
// 					e = entities.NewNetworkPlayer(CharactersImage, em.EntityTile)
// 					e.Id = em.EntityId
// 					client.NetworkPlayers = append(client.NetworkPlayers, e)
// 					g.Entities = append(g.Entities, e)
// 				}

// 				e.Position = em.EntityPos
// 			}
// 		}
// 	}()

// 	// Send
// 	for {
// 		pos := <-client.Player.SendChan
// 		em := &messages.EntityMessage{
// 			EntityId:   client.Player.Id,
// 			EntityPos:  pos,
// 			EntityTile: f64.Vec2{0, 0},
// 		}
// 		glm := &messages.GameLoopMessage{
// 			EntityMessages: []messages.EntityMessage{
// 				*em,
// 			},
// 		}

// 		c.WriteJSON(glm)
// 	}
// }

// func chatLoop(g *Game) error {
// 	chat := gui.NewChat(screenWidth, screenHeight)
// 	g.Entities = append(g.Entities, chat)

// 	u := url.URL{Scheme: "ws", Host: addr, Path: "/chat"}

// 	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	if err != nil {
// 		return err
// 	}
// 	defer c.Close()

// 	// Announce connection
// 	glm := &messages.ChatLoopMessage{
// 		ClientId: client.Player.Id,
// 		Message:  "connected",
// 	}
// 	c.WriteJSON(glm)

// 	// Receive
// 	go func() {
// 		for {
// 			_, message, err := c.ReadMessage()
// 			if err != nil {
// 				fmt.Printf("error in reading message: %s", err)
// 			}

// 			var chatMessage = &messages.ChatLoopMessage{}
// 			if err = json.Unmarshal([]byte(message), chatMessage); err != nil {
// 				fmt.Printf("unmarshal error:", err, chatMessage, message)
// 			}

// 			chat.RecvMessages = append(chat.RecvMessages, fmt.Sprint(chatMessage.ClientId, chatMessage.Message))
// 			fmt.Println(chatMessage.ClientId, chatMessage.Message)
// 		}
// 	}()

// 	// Send
// 	for {
// 		msg := <-chat.SendChan
// 		glm := &messages.ChatLoopMessage{
// 			ClientId: client.Player.Id,
// 			Message:  msg,
// 		}
// 		c.WriteJSON(glm)
// 	}
// }
