package main

import (
//    "database/sql"
//    "fmt"
    "io/ioutil"
    "log"
    "net/http"
//    "os"
    "time"

    _ "github.com/mattn/go-sqlite3"
    "gopkg.in/yaml.v2"
)

type Config struct {
    RemoteEndpoint      string      `yaml:"item"`
    CheckPeriod         int         `yaml:"check"`
    DatabasePath        string      `yaml:"database"`
}

var defaultConfig = Config {
    RemoteEndpoint: "https://meet.example.com/jvb",
    CheckPeriod: 5,
    DatabasePath: "./meet.example.com.db",
}

func readConfig(filePath string) Config {
    config := defaultConfig

    file, err := ioutil.ReadFile(filePath)
    if err != nil {
        log.Println("Can't read config file, using defaults.")
        return config
    }

    err = yaml.Unmarshal(file, &config)
    if err != nil {
        log.Println("Can't parse the config file, using defaults.")
        return config
    }

    return config
}

func checkEndpoint(endpoint string) (int, int64) {
    start := time.Now()
    resp, err := http.Get(endpoint)
    if err != nil {
        log.Println("Failed to check the endpoint: ", err)
        return 0, 0
    }
    defer resp.Body.Close()

    elapsed := time.Since(start).Milliseconds()
    return resp.StatusCode, elapsed
}

func main() {
    config := readConfig("jilo-server.conf")

    ticker := time.NewTicker(time.Duration(config.CheckPeriod) * time.Minute)
    defer ticker.Stop()

    log.Println("Starting endpoint checker...")

    for {
        statusCode, responseTime := checkEndpoint(config.RemoteEndpoint)
        log.Printf("Endpoint check: Status code: %d, Response time: %d ms", statusCode, responseTime)

        <-ticker.C
    }
}
