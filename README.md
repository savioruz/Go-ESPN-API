# Go-ESPN-API

A **Go client for the ESPN public API** — live scores, standings, and game data for your favorite sports, without an API key.

## Features

- 🏀 Live scores across leagues (NBA, NFL, MLB, NCAA, ...)
- 📊 Standings and team data
- ⚡ Concurrent, typed, idiomatic Go
- 🧪 Tested against the live ESPN API

## Install

```bash
go get github.com/savioruz/Go-ESPN-API
```

## Usage

```go
package main

import (
    "fmt"
    espn "github.com/savioruz/Go-ESPN-API"
)

func main() {
    client := espn.NewClient()
    scores, err := client.Scores("nba") // or "nfl", "mlb", ...
    if err != nil {
        panic(err)
    }
    fmt.Printf("Got %d live games\n", len(scores))
}
```

## Why

ESPN's public endpoints are undocumented and change shape often. This library wraps them in a typed, stable API so your Go projects just work.

## License

MIT
