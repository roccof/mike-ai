package main

import (
	"context"
	"fmt"
	"mike-ai/service"
	"mike-ai/wait"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	openairt "github.com/WqyJh/go-openai-realtime"
	"github.com/coder/websocket"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var openai_key string
var instructions string

func loadSchema() (any, error) {
	f, err := os.Open("schema.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	schema, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

func handleNewConn(ctx context.Context, srv *service.WSService, conn *openairt.Conn) {
	defer srv.Close()
	defer conn.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Println("[*] Client connected")

	w := wait.Group{}

	// OpenAI logic
	w.Start(ctx, func(ctx context.Context) {
		for {
			event, err := conn.ReadMessage(ctx)
			if err != nil {
				fmt.Println("[!!!] Failed to read OpenAI message:", err)
				break
			}

			switch event.ServerEventType() {
			case openairt.ServerEventTypeResponseAudioDelta:
				delta := event.(openairt.ResponseAudioDeltaEvent)

				err := srv.SendMessage(ctx, service.AudioMessage{
					BaseMessage: service.BaseMessage{
						Type: service.MessageTypeAudio,
					},
					Audio: string(delta.Delta),
				})
				if err != nil {
					fmt.Println("[!!!] Failed to send audio message to webpage:", err)
					continue
				}

			case openairt.ServerEventTypeError:
				msg := event.(openairt.ErrorEvent)
				fmt.Println("[*] OpenAI error:", msg.Error.Message)

			case openairt.ServerEventTypeResponseAudioTranscriptDone:
				response := event.(openairt.ResponseAudioTranscriptDoneEvent)
				fmt.Println("[*] <Mike>", response.Transcript)

			case openairt.ServerEventTypeConversationItemInputAudioTranscriptionCompleted:
				question := event.(openairt.ConversationItemInputAudioTranscriptionCompletedEvent)
				fmt.Println("\n[*] <User>", question.Transcript)

			case openairt.ServerEventTypeResponseFunctionCallArgumentsDone:
				response := event.(openairt.ResponseFunctionCallArgumentsDoneEvent)
				fmt.Println("[*] <Mike> Function call:", response.Name, response.Arguments)
				err := srv.SendMessage(ctx, service.CommandInvokeMessage{
					BaseMessage: service.BaseMessage{
						Type: service.MessageTypeCommandInvoke,
					},
					CallID: response.CallID,
					Name:   response.Name,
					Args:   response.Arguments,
				})
				if err != nil {
					fmt.Println("[!!!] Failed to send command invoke message to webpage:", err)
					continue
				}
				// TODO: validate arguments against json-schema

			case openairt.ServerEventTypeResponseDone:
				response := event.(openairt.ResponseDoneEvent)
				fmt.Printf("[*] <Mike> Response done: %+v\n", response)

			case openairt.ServerEventTypeRateLimitsUpdated:
				limits := event.(openairt.RateLimitsUpdatedEvent)
				fmt.Println("[*] Rate limits updated:", limits.RateLimits)
			}
		}
		cancel()
	})

	// WebSocket logic
	w.Start(ctx, func(ctx context.Context) {
		for {
			msg, err := srv.ReadMessage(ctx)
			if err != nil {
				fmt.Println("[!!!] Failed to read websocket message:", err)
				break
			}

			switch msg.GetType() {
			case service.MessageTypeAudio:
				audioMsg := msg.(service.AudioMessage)
				err := conn.SendMessage(ctx, openairt.InputAudioBufferAppendEvent{
					Audio: audioMsg.Audio,
				})
				if err != nil {
					fmt.Println("[!!!] Failed to send audio to OpenAI:", err)
				}

			case service.MessageTypeCommandResult:
				commandResultMsg := msg.(service.CommandResultMessage)
				fmt.Println("[*] Command result message received:", commandResultMsg.Name, commandResultMsg.Output)
				err := conn.SendMessage(ctx, openairt.ConversationItemCreateEvent{
					Item: openairt.MessageItem{
						Type:   openairt.MessageItemTypeFunctionCallOutput,
						CallID: commandResultMsg.CallID,
						Output: commandResultMsg.Output,
					},
				})
				if err != nil {
					fmt.Println("[!!!] Failed to send function call result to OpenAI:", err)
				}

				// Trigger a model response using the data from the function call
				err = conn.SendMessage(ctx, openairt.ResponseCreateEvent{})
				if err != nil {
					fmt.Println("[!!!] Failed to send create-response to OpenAI:", err)
				}
			}
		}
		cancel()
	})

	jsonschema, err := loadSchema()
	if err != nil {
		fmt.Println("[!!!] Failed to load json-schema:", err)
		return
	}

	tools := []openairt.Tool{
		{
			Type:        openairt.ToolTypeFunction,
			Name:        "getCanvasSize",
			Description: "Retrieves the current canvas dimensions in pixels, returning both width and height values.",
		},
		{
			Type:        openairt.ToolTypeFunction,
			Name:        "clearCanvas",
			Description: "Clears the entire canvas, removing all previously drawn content and resetting it to a blank state.",
		},
		{
			Type:        openairt.ToolTypeFunction,
			Name:        "paintCanvas",
			Description: "Executes painting operations on the canvas using the provided drawing instructions. The instructions must be a valid JSON object that conforms to the predefined schema and contains canvas drawing commands such as paths, shapes, colors, and transformations.",
			Parameters:  jsonschema,
		},
	}

	err = conn.SendMessage(ctx, openairt.SessionUpdateEvent{
		Session: openairt.ClientSession{
			Instructions: instructions,
			Modalities:   []openairt.Modality{openairt.ModalityText, openairt.ModalityAudio},
			Voice:        openairt.VoiceAsh,
			//InputAudioTranscription: &openairt.InputAudioTranscription{
			//	Model: "whisper-1",
			//},
			TurnDetection: &openairt.ClientTurnDetection{
				Type: openairt.ClientTurnDetectionTypeServerVad,
			},
			Tools:      tools,
			ToolChoice: openairt.ToolChoiceAuto,
		},
	})
	if err != nil {
		fmt.Println("[!!!] Failed to configure OpenAI:", err)
		return
	}

	fmt.Println("[*] OpenAI session configured")

	w.Wait()

	fmt.Println("[*] Client disconnected")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		fmt.Println("[!!!] Failed to accept connection:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// XXX: understand why it doesn't work
	//ctx := r.Context()
	ctx := context.Background()

	client := openairt.NewClient(openai_key)
	conn, err := client.Connect(ctx)
	if err != nil {
		fmt.Println("[!!!] Failed to connect to OpenAI:", err)
		ws.Close(websocket.StatusInternalError, "Failed to connect to OpenAI")
		return
	}

	service := service.NewWSService(ws)

	go handleNewConn(ctx, service, conn)
}

func main() {
	key, exists := os.LookupEnv("OPENAI_API_KEY")
	if !exists {
		panic("OPENAI_API_KEY is not set")
	}

	openai_key = key

	b, err := os.ReadFile("instructions.txt")
	if err != nil {
		panic("Failed to load instructions: " + err.Error())
	}

	instructions = string(b)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	http.Handle("/", http.FileServer(http.Dir("/var/www/mike-assets")))
	http.HandleFunc("/ws", handleWebSocket)

	httpd := &http.Server{
		Addr: ":8080",
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	fmt.Println("[*] Server started on http://127.0.0.1:8080")

	go func() {
		if err := httpd.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("[!!!] Failed to start server:", err)
		}
	}()

	<-ctx.Done()

	// Stop capturing signals, so if we press again Ctrl+C, we can force the shutdown
	// without waiting for the server to shutdown
	stop()

	fmt.Println("[*] Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = httpd.Shutdown(shutdownCtx)
	if err != nil && err != context.Canceled {
		fmt.Println("[!!!] Failed to shutdown server:", err)
	}
}
