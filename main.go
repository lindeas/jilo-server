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

    "github.com/golang-jwt/jwt/v5"
    _ "github.com/mattn/go-sqlite3"
    "gopkg.in/yaml.v2"
)

// Structures
type Agent struct {
    Endpoint            string      `yaml:"endpoint"`
    Secret              string      `yaml:"secret"`
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

// Database initialization
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
        response_time_ms INTEGER,
        response_content TEXT
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

// JWT token generation
func generateJWT(secret string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "iat": time.Now().Unix(),
    })

    tokenString, err := token.SignedString([]byte(secret))
    if err != nil {
        return "", err
    }

    return tokenString, nil
}

// Check agent endpoint
func checkEndpoint(agent Agent) (int, int64, string) {
    log.Println("Sending HTTP get request to Jilo agent:", agent.Endpoint)

    // Generate the JWT token
    token, err := generateJWT(agent.Secret)
    if err != nil {
        log.Println("Failed to generate JWT token:", err)
        return 0, 0, ""
    }

    // Create the http request
    req, err := http.NewRequest("GET", agent.Endpoint, nil)
    if err != nil {
        log.Println("Failed to create the HTTP request:", err)
        return 0, 0, ""
    }

    // Set Authorization header
    req.Header.Set("Authorization", "Bearer "+token)

    start := time.Now()
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Println("Failed to check the endpoint:", err)
        return 0, 0, ""
    }
    defer resp.Body.Close()

    elapsed := time.Since(start).Milliseconds()

    // Read the response body
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Println("Failed to read the response body:", err)
        return resp.StatusCode, elapsed, ""
    }

    log.Printf("Received response: %d, Time taken: %d ms", resp.StatusCode, elapsed)

    return resp.StatusCode, elapsed, string(body)
}

// Insert the checks into the database
func saveData(db *sql.DB, statusCode int, responseTime int64, responseContent string) {
    _, err := db.Exec("INSERT INTO endpoint_data (status_code, response_time_ms, response_content) VALUES (?, ?, ?)", statusCode, responseTime, responseContent)
    if err != nil {
        log.Println("Failed to insert data into the database:", err)
    }
}

// Main routine
func main() {
    // First flush all the logs
    log.SetFlags(log.LstdFlags | log.Lshortfile)

    // Command-line option "--init-db" creates the table
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
                    statusCode, responseTime, responseContent := checkEndpoint(agent)
                    log.Printf("Agent [%s - %s]: Status code: %d, Response time: %d ms", serverName, agentName, statusCode, responseTime)

                    saveData(db, statusCode, responseTime, responseContent)

                    // Sleep until the next tick
                    <-ticker.C
                }
            }(serverName, agentName, agent)
        }
    }

    // Prevent the main from exiting
    select {}
}
