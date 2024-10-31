package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "strings"
    "time"

    "github.com/gorilla/websocket"
)

type SessionUpdate struct {
    Type    string  `json:"type"`
    Session Session `json:"session"`
}

type Session struct {
    Modalities             []string    `json:"modalities"`
    Instructions           string      `json:"instructions"`
    Temperature            float64     `json:"temperature"`
    MaxResponseOutputTokens int        `json:"max_response_output_tokens"`
}

type ConversationItem struct {
    Type string `json:"type"`
    Item struct {
        Type    string `json:"type"`
        Role    string `json:"role"`
        Content []struct {
            Type string `json:"type"`
            Text string `json:"text"`
        } `json:"content"`
    } `json:"item"`
}

type ResponseCreate struct {
    Type string `json:"type"`
}

type ResponseMessage struct {
    Type     string   `json:"type"`
    Response Response `json:"response"`
}

type Response struct {
    Output []OutputItem `json:"output"`
    Status string       `json:"status"`
}

type OutputItem struct {
    Content []ContentItem `json:"content"`
    Role    string       `json:"role"`
    Status  string       `json:"status"`
}

type ContentItem struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

func sendUserMessage(c *websocket.Conn, text string) error {
    // 1. Send conversation item
    conversationItem := ConversationItem{
        Type: "conversation.item.create",
        Item: struct {
            Type    string `json:"type"`
            Role    string `json:"role"`
            Content []struct {
                Type string `json:"type"`
                Text string `json:"text"`
            } `json:"content"`
        }{
            Type: "message",
            Role: "user",
            Content: []struct {
                Type string `json:"type"`
                Text string `json:"text"`
            }{
                {
                    Type: "input_text",
                    Text: text,
                },
            },
        },
    }
    
    if err := c.WriteJSON(conversationItem); err != nil {
        return fmt.Errorf("write conversation item: %w", err)
    }
    time.Sleep(time.Second)

    // 2. Send response create
    responseCreate := ResponseCreate{
        Type: "response.create",
    }
    
    if err := c.WriteJSON(responseCreate); err != nil {
        return fmt.Errorf("write response create: %w", err)
    }

    return nil
}

func main() {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        log.Fatal("OPENAI_API_KEY environment variable is not set")
    }

    header := make(map[string][]string)
    header["Authorization"] = []string{"Bearer " + apiKey}
    header["OpenAI-Beta"] = []string{"realtime=v1"}

    url := "wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01"
    c, _, err := websocket.DefaultDialer.Dial(url, header)
    if err != nil {
        log.Fatal("dial:", err)
    }
    defer c.Close()

    // 1. Send session update
    sessionUpdate := SessionUpdate{
        Type: "session.update",
        Session: Session{
            Modalities:             []string{"text"},
            Temperature:            0.8,
            MaxResponseOutputTokens: 4096,
            Instructions:           "System settings:\nInstructions:\n- You are an artificial intelligence agent\n- Be kind, helpful, and curteous\n- It is okay to ask the user questions\n- Be open to exploration and conversation\n- Remember: this is just for fun and testing!\n\nPersonality:\n- Be upbeat and genuine\n",
        },
    }
    
    if err := c.WriteJSON(sessionUpdate); err != nil {
        log.Fatal("write session update:", err)
    }
    fmt.Println("Sent session update")
    time.Sleep(time.Second)

    // Create a channel to signal message completion
    done := make(chan struct{})

    // Start goroutine to read responses
    go func() {
        for {
            _, message, err := c.ReadMessage()
            if err != nil {
                log.Println("read:", err)
                close(done)
                return
            }

            var resp ResponseMessage
            if err := json.Unmarshal(message, &resp); err != nil {
                continue
            }

            if resp.Type == "response.done" && resp.Response.Status == "completed" {
                for _, output := range resp.Response.Output {
                    for _, content := range output.Content {
                        if content.Type == "text" {
                            fmt.Printf("Assistant: %s\n", content.Text)
                        }
                    }
                }
                // Signal that we can accept new input
                fmt.Print("\nYou: ")
            }
        }
    }()

    // Read user input
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Print("You: ")
    for scanner.Scan() {
        text := strings.TrimSpace(scanner.Text())
        if text == "" {
            fmt.Print("You: ")
            continue
        }
        if text == "quit" || text == "exit" {
            break
        }

        if err := sendUserMessage(c, text); err != nil {
            log.Printf("Error sending message: %v", err)
            break
        }
    }

    if err := scanner.Err(); err != nil {
        log.Printf("Error reading input: %v", err)
    }

    // Clean up
    close(done)
    c.Close()
}
