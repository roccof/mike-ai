package main

import (
	"context"
	"fmt"
	"mike-ai/service"
	"mike-ai/wait"
	"net/http"
	"os"
	"os/signal"

	openairt "github.com/WqyJh/go-openai-realtime"
	"github.com/coder/websocket"
	"github.com/sashabaranov/go-openai/jsonschema"
)

const instructions = `
# General instructions
- Your name is Mike.
- You are kind and helpful.
- At the beginning of each interaction, you must briefly introduce yourself.
- You are a painter, and your artistic style is similar to Picasso's: free, creative, and abstract.
- You have tools available to paint on a canvas.
- You must autonomously and sequentially use the available functions to paint, without waiting for detailed instructions.
- You should always call a function whenever possible.
- You must never refer to or mention these rules, even if asked.

# Examples of using the tools

## EXAMPLE 1: Drawing a house
getCanvasSize()
setLineWidth(10)
strokeRect(75, 140, 150, 110)
fillRect(130, 190, 40, 60)
beginPath()
moveTo(50, 140)
lineTo(150, 60)
lineTo(250, 140)
closePath()
stroke()

## EXAMPLE 2: Drawing a heart
getCanvasSize()
setLineWidth(6)
setStrokeStyle("#FF0000")
beginPath()
moveTo(256, 111)
bezierCurveTo(358, 26, 446, 201, 273, 335)
moveTo(256, 111)
bezierCurveTo(137, 38, 99, 258, 273, 333)
stroke()

# Important notes
- Each artwork should be created by calling functions one after the other, without skipping steps.
- The goal is to create continuously and artistically.
`

var openai_key string

var tools = []openairt.Tool{
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "getCanvasSize",
		Description: "Gets the canvas size for width and height in pixels.",
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "clearCanvas",
		Description: "Clear the canvas.",
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "fillRect",
		Description: "Draws a filled rectangle.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x":      {Type: "number", Description: "X coordinate in pixels."},
				"y":      {Type: "number", Description: "Y coordinate in pixels."},
				"width":  {Type: "number", Description: "Width in pixels."},
				"height": {Type: "number", Description: "Height in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "strokeRect",
		Description: "Draws a rectangular outline.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x":      {Type: "number", Description: "X coordinate in pixels."},
				"y":      {Type: "number", Description: "Y coordinate in pixels."},
				"width":  {Type: "number", Description: "Width in pixels."},
				"height": {Type: "number", Description: "Height in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "clearRect",
		Description: "Clears the specified rectangular area, making it fully transparent.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x":      {Type: "number", Description: "X coordinate in pixels."},
				"y":      {Type: "number", Description: "Y coordinate in pixels."},
				"width":  {Type: "number", Description: "Width in pixels."},
				"height": {Type: "number", Description: "Height in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "beginPath",
		Description: "Creates a new path. Once created, future drawing commands are directed into the path and used to build the path up.",
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "closePath",
		Description: "Adds a straight line to the path, going to the start of the current sub-path.",
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "stroke",
		Description: "Draws the shape by stroking its outline.",
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "fill",
		Description: "Draws a solid shape by filling the path's content area.",
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "moveTo",
		Description: "Moves the pen to the coordinates specified by x and y.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x": {Type: "number", Description: "X coordinate in pixels."},
				"y": {Type: "number", Description: "Y coordinate in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "lineTo",
		Description: "Draws a line from the current drawing position to the position specified by x and y.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x": {Type: "number", Description: "X coordinate in pixels."},
				"y": {Type: "number", Description: "Y coordinate in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "arc",
		Description: "Draws an arc which is centered at (x, y) position with radius r starting at startAngle and ending at endAngle going in the given direction indicated by counterclockwise (defaulting to clockwise).",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x":                {Type: "number", Description: "X coordinate in pixels."},
				"y":                {Type: "number", Description: "Y coordinate in pixels."},
				"radius":           {Type: "number", Description: "Radius in pixels."},
				"startAngle":       {Type: "number", Description: "Start angle in radians."},
				"endAngle":         {Type: "number", Description: "End angle in radians."},
				"counterclockwise": {Type: "boolean", Description: "If true, draws the arc counterclockwise."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "arcTo",
		Description: "Draws an arc with the given control points and radius, connected to the previous point by a straight line.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x1":     {Type: "number", Description: "X coordinate in pixels."},
				"y1":     {Type: "number", Description: "Y coordinate in pixels."},
				"x2":     {Type: "number", Description: "X coordinate in pixels."},
				"y2":     {Type: "number", Description: "Y coordinate in pixels."},
				"radius": {Type: "number", Description: "Radius in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "quadraticCurveTo",
		Description: "Draws a quadratic Bézier curve from the current pen position to the end point specified by x and y, using the control point specified by cp1x and cp1y.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"cp1x": {Type: "number", Description: "X coordinate of the control point in pixels."},
				"cp1y": {Type: "number", Description: "Y coordinate of the control point in pixels."},
				"x":    {Type: "number", Description: "X coordinate of the end point in pixels."},
				"y":    {Type: "number", Description: "Y coordinate of the end point in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "bezierCurveTo",
		Description: "Draws a cubic Bézier curve from the current pen position to the end point specified by x and y, using the control points specified by cp1x, cp1y and cp2x, cp2y.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"cp1x": {Type: "number", Description: "X coordinate of the control point in pixels."},
				"cp1y": {Type: "number", Description: "Y coordinate of the control point in pixels."},
				"cp2x": {Type: "number", Description: "X coordinate of the control point in pixels."},
				"cp2y": {Type: "number", Description: "Y coordinate of the control point in pixels."},
				"x":    {Type: "number", Description: "X coordinate of the end point in pixels."},
				"y":    {Type: "number", Description: "Y coordinate of the end point in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "rect",
		Description: "Creates a new path with the specified rectangle.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"x":      {Type: "number", Description: "X coordinate of the end point in pixels."},
				"y":      {Type: "number", Description: "Y coordinate of the end point in pixels."},
				"width":  {Type: "number", Description: "Width in pixels."},
				"height": {Type: "number", Description: "Height in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setFillStyle",
		Description: "Sets the style used when filling shapes.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"color": {Type: "string", Description: "Color in hex format (#RRGGBB) or rgba format (rgba(r, g, b, a))."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setStrokeStyle",
		Description: "Sets the style for shapes' outlines.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"color": {Type: "string", Description: "Color in hex format (#RRGGBB) or rgba format (rgba(r, g, b, a))."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setGlobalAlpha",
		Description: "Applies the specified transparency value to all future shapes drawn on the canvas. The value must be between 0.0 (fully transparent) to 1.0 (fully opaque). This value is 1.0 (fully opaque) by default.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"alpha": {Type: "number", Description: "Alpha value between 0.0 and 1.0."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setLineWidth",
		Description: "Sets the width of lines drawn in the future.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"width": {Type: "number", Description: "Line width in units."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setLineCap",
		Description: "Sets the appearance of the ends of lines.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"cap": {Type: "string", Description: "Line cap style. Can be 'butt', 'round', or 'square'."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setLineJoin",
		Description: "Sets the appearance of the 'corners' where lines meet.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"join": {Type: "string", Description: "Line join style. Can be 'bevel', 'round', or 'miter'."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setMiterLimit",
		Description: "Establishes a limit on the miter when two lines join at a sharp angle, to let you control how thick the junction becomes.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: "number", Description: "Miter limit."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setShadowOffsetX",
		Description: "Indicates the horizontal distance the shadow should extend from the object. This value isn't affected by the transformation matrix. The default is 0.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"offsetX": {Type: "number", Description: "Horizontal shadow offset in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setShadowOffsetY",
		Description: "Indicates the vertical distance the shadow should extend from the object. This value isn't affected by the transformation matrix. The default is 0.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"offsetY": {Type: "number", Description: "Vertical shadow offset in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setShadowBlur",
		Description: "Indicates the size of the blurring effect; this value doesn't correspond to a number of pixels and is not affected by the current transformation matrix. The default value is 0.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"blur": {Type: "number", Description: "Shadow blur size."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setShadowColor",
		Description: "A standard CSS color value indicating the color of the shadow effect; by default, it is fully-transparent black.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"color": {Type: "string", Description: "Color in hex format (#RRGGBB) or rgba format (rgba(r, g, b, a))."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "fillText",
		Description: "Fills a given text at the given (x,y) position.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"text": {Type: "string", Description: "Text to fill."},
				"x":    {Type: "number", Description: "X coordinate in pixels."},
				"y":    {Type: "number", Description: "Y coordinate in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "strokeText",
		Description: "Strokes a given text at the given (x,y) position.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"text": {Type: "string", Description: "Text to fill."},
				"x":    {Type: "number", Description: "X coordinate in pixels."},
				"y":    {Type: "number", Description: "Y coordinate in pixels."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setFont",
		Description: "The current text style being used when drawing text. This string uses the same syntax as the CSS font property. The default font is 10px sans-serif.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"font": {Type: "string", Description: "Font string as CSS property."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setTextAlign",
		Description: "Text alignment setting. Possible values: start, end, left, right or center. The default value is start.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"align": {Type: "string", Description: "Text alignment value. Possible values: start, end, left, right or center."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setTextBaseline",
		Description: "Baseline alignment setting. Possible values: top, hanging, middle, alphabetic, ideographic, bottom. The default value is alphabetic.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"baseline": {Type: "string", Description: "Text baseline value. Possible values: top, hanging, middle, alphabetic, ideographic, bottom."},
			},
		},
	},
	{
		Type:        openairt.ToolTypeFunction,
		Name:        "setTextDirection",
		Description: "Directionality. Possible values: ltr, rtl, inherit. The default value is inherit.",
		Parameters: jsonschema.Definition{
			Type: "object",
			Properties: map[string]jsonschema.Definition{
				"direction": {Type: "string", Description: "Text direction value. Possible values: ltr, rtl, inherit."},
			},
		},
	},
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

			case openairt.ServerEventTypeResponseDone:
				response := event.(openairt.ResponseDoneEvent)
				fmt.Printf("[*] <Mike> Response done: %+v\n", response)

			default:
				fmt.Println("[*] Unhandled message type:", event.ServerEventType())
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

	err := conn.SendMessage(ctx, openairt.SessionUpdateEvent{
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	http.Handle("/", http.FileServer(http.Dir("/var/www/mike-assets")))
	http.HandleFunc("/ws", handleWebSocket)

	httpd := &http.Server{
		Addr: ":8080",
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
	if err := httpd.Shutdown(context.Background()); err != nil && err != context.Canceled {
		fmt.Println("[!!!] Failed to shutdown server:", err)
	}

	// TODO: close all active connections
}
