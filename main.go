package main

import (
    "database/sql"
    "io/ioutil"
    "log"
    "net/http"
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

func setupDatabase(dbPath string) (*sql.DB, error) {

    // Open the database
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }

    // If the table is not there, initialize it
    createTable := `
    CREATE TABLE IF NOT EXISTS endpoint_data (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
        status_code INTEGER,
        response_time_ms INTEGER
    );`
    _, err = db.Exec(createTable)
    if err != nil {
        return nil, err
    }

    return db, nil
}

func checkEndpoint(endpoint string) (int, int64) {
    start := time.Now()
    resp, err := http.Get(endpoint)
    if err != nil {
        log.Println("Failed to check the endpoint:", err)
        return 0, 0
    }
    defer resp.Body.Close()

    elapsed := time.Since(start).Milliseconds()
    return resp.StatusCode, elapsed
}

func saveData(db *sql.DB, statusCode int, responseTime int64) {
    _, err := db.Exec("INSERT INTO endpoint_data (status_code, response_time_ms) VALUES (?, ?)", statusCode, responseTime)
    if err != nil {
        log.Println("Failed to insert data into the database:", err)
    }
}

func main() {
    // config file
    config := readConfig("jilo-server.conf")

    // Connect to or setup the database
    db, err := setupDatabase(config.DatabasePath)
    if err != nil {
        log.Fatal("Failed to initialize the database:", err)
    }
    defer db.Close()

    ticker := time.NewTicker(time.Duration(config.CheckPeriod) * time.Minute)
    defer ticker.Stop()

    log.Println("Starting endpoint checker...")

    for {
        statusCode, responseTime := checkEndpoint(config.RemoteEndpoint)
        log.Printf("Endpoint check: Status code: %d, Response time: %d ms", statusCode, responseTime)

        <-ticker.C
    }
}
