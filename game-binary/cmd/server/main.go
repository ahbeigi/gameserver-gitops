package main

import (
  "fmt"
  "log"
  "net/http"
  "os"
)

func status(w http.ResponseWriter, r *http.Request) {
  _, _ = w.Write([]byte(`{"ok":true}`))
}

func main() {
  port := os.Getenv("GAME_PORT")
  if port == "" {
    port = "30000"
  }
  mux := http.NewServeMux()
  mux.HandleFunc("/status", status)
  addr := fmt.Sprintf(":%s", port)
  log.Printf("Game server listening on %s", addr)
  if err := http.ListenAndServe(addr, mux); err != nil {
    log.Fatal(err)
  }
}
