package main

import (
    "bufio"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "strings"
    "time"
)

type LogEntry struct {
    Timestamp string          `json:"timestamp"`
    Direction string          `json:"direction"`
    Type      string          `json:"type"`
    RawJSON   interface{}     `json:"raw_json"`
}

type OutputWriter struct {
    writer io.Writer
}

func NewOutputWriter(outputFile string) (*OutputWriter, error) {
    var w io.Writer = os.Stdout

    if outputFile != "" {
        file, err := os.Create(outputFile)
        if err != nil {
            return nil, fmt.Errorf("error creating output file: %w", err)
        }
        w = file
    }

    return &OutputWriter{writer: w}, nil
}

func (w *OutputWriter) Write(format string, a ...interface{}) {
    fmt.Fprintf(w.writer, format, a...)
}

func (w *OutputWriter) WriteString(s string) {
    fmt.Fprint(w.writer, s)
}

func formatJSON(data interface{}) string {
    b, err := json.MarshalIndent(data, "", "    ")
    if err != nil {
        return fmt.Sprintf("%v", data)
    }
    return string(b)
}

func formatTimestamp(timestamp string) string {
    t, err := time.Parse(time.RFC3339Nano, timestamp)
    if err != nil {
        return timestamp
    }
    return t.Format("2006-01-02 15:04:05.000")
}

func printLogEntry(writer *OutputWriter, entry LogEntry) {
    // Create direction indicator
    directionArrow := "→"
    if entry.Direction == "received" {
        directionArrow = "←"
    }

    // Format timestamp
    timestamp := formatTimestamp(entry.Timestamp)

    // Print header
    writer.Write("\n%s %s [%s] %s\n", 
        timestamp,
        directionArrow,
        strings.ToUpper(entry.Direction),
        strings.ToUpper(entry.Type))

    // Print separator
    writer.WriteString(strings.Repeat("-", 80) + "\n")

    // Print formatted JSON data
    writer.WriteString(formatJSON(entry.RawJSON) + "\n")
}

func processLogFile(inputFile string, writer *OutputWriter) error {
    file, err := os.Open(inputFile)
    if err != nil {
        return fmt.Errorf("error opening input file: %w", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    // Increase scanner buffer size for large JSON lines
    const maxCapacity = 1024 * 1024 // 1MB
    buf := make([]byte, maxCapacity)
    scanner.Buffer(buf, maxCapacity)

    for scanner.Scan() {
        var entry LogEntry
        if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
            log.Printf("Error parsing log entry: %v", err)
            continue
        }
        printLogEntry(writer, entry)
    }

    if err := scanner.Err(); err != nil {
        return fmt.Errorf("error reading file: %w", err)
    }

    return nil
}

func main() {
    // Parse command line flags
    inputFile := flag.String("f", "", "Input log file to process")
    outputFile := flag.String("o", "", "Output file (optional, defaults to terminal)")
    flag.Parse()

    if *inputFile == "" {
        log.Fatal("Please provide an input log file using the -f flag")
    }

    // Create output writer
    writer, err := NewOutputWriter(*outputFile)
    if err != nil {
        log.Fatal(err)
    }

    // If we're writing to a file, make sure it's closed properly
    if *outputFile != "" {
        if closer, ok := writer.writer.(io.Closer); ok {
            defer closer.Close()
        }
    }

    // Process the log file
    if err := processLogFile(*inputFile, writer); err != nil {
        log.Fatal(err)
    }
}
