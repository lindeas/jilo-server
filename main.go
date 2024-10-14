package main

import (
    "database/sql"
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "time"

    _ "github.com/mattn/go-sqlite3"
    "gopkg.in/yaml.v2"
)

// Structures
type Agent struct {
    Endpoint            string      `yaml:"endpoint"`
    CheckPeriod         int         `yaml:"check_period"`
}
type Server struct {
    Agents              map[string]Agent    `yaml:"agents"`
}
type Config struct {
    Servers             map[string]Server   `yaml:"servers"`
    DatabasePath        string              `yaml:"database_path"`
}

// Loading the config file
func readConfig(filePath string) Config {
    var config Config

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

func setupDatabase(dbPath string, initDB bool) (*sql.DB, error) {

    // Open the database
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }

    // Check if the table exists
    tableExists := checkTableExists(db)
    if !tableExists && !initDB {
        // Ask if we should create the table
        fmt.Print("Table not found. Do you want to create it? (y/n): ")
        var response string
        fmt.Scanln(&response)

        if response != "y" && response != "Y" {
            log.Println("Exiting because the table is missing, but mandatory.")
            os.Exit(1)
        }
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

// Check for the table
func checkTableExists(db *sql.DB) bool {
    sql := `
    SELECT name
        FROM sqlite_master
        WHERE type='table'
        AND name='endpoint_data';`
    row := db.QueryRow(sql)

    var name string
    err := row.Scan(&name)

    return err == nil && name == "endpoint_data"
}

func checkEndpoint(endpoint string) (int, int64) {
    log.Println("Sending HTTP get request to Jilo agent:", endpoint)
    start := time.Now()
    resp, err := http.Get(endpoint)
    if err != nil {
        log.Println("Failed to check the endpoint:", err)
        return 0, 0
    }
    defer resp.Body.Close()

    elapsed := time.Since(start).Milliseconds()
    log.Printf("Received response: %d, Time taken: %d ms", resp.StatusCode, elapsed)
    return resp.StatusCode, elapsed
}

func saveData(db *sql.DB, statusCode int, responseTime int64) {
    _, err := db.Exec("INSERT INTO endpoint_data (status_code, response_time_ms) VALUES (?, ?)", statusCode, responseTime)
    if err != nil {
        log.Println("Failed to insert data into the database:", err)
    }
}

// Main routine
func main() {
    // First flush all the logs
    log.SetFlags(log.LstdFlags | log.Lshortfile)

    // Init the DB, "--init-db" creates the table
    initDB := flag.Bool("init-db", false, "Create database table if not present without prompting")
    flag.Parse()

    // Config file
    log.Println("Reading the config file...")
    config := readConfig("jilo-server.conf")

    // Connect to or setup the database
    log.Println("Initializing the database...")
    db, err := setupDatabase(config.DatabasePath, *initDB)
    if err != nil {
        log.Fatal("Failed to initialize the database:", err)
    }
    defer db.Close()

    log.Println("Starting endpoint checker...")

    // Iterate over the servers and agents
    for serverName, server := range config.Servers {
        for agentName, agent := range server.Agents {
            go func(serverName, agentName string, agent Agent) {
                // Ticker for the periodic checks
                ticker := time.NewTicker(time.Duration(agent.CheckPeriod) * time.Minute)
                defer ticker.Stop()

                for {
                    log.Printf("Checking agent [%s - %s]: %s", serverName, agentName, agent.Endpoint)
                    statusCode, responseTime := checkEndpoint(agent.Endpoint)
                    log.Printf("Agent [%s - %s]: Status code: %d, Response time: %d ms", serverName, agentName, statusCode, responseTime)

                    saveData(db, statusCode, responseTime)

                    // Sleep until the next tick
                    <-ticker.C
                }
            }(serverName, agentName, agent)
        }
    }

    // Prevent the main from exiting
    select {}
}
