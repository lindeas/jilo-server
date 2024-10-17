package main

import (
    "database/sql"
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
    _ "github.com/mattn/go-sqlite3"
    "gopkg.in/yaml.v2"
)

// Structures
type Agent struct {
    ID              int
    URL             string
    Secret          string
    CheckPeriod     int
}
type Config struct {
    DatabasePath            string      `yaml:"database_path"`
    HealthCheckEnabled      bool        `yaml:"health_check_enabled"`
    HealthCheckPort         int         `yaml:"health_check_port"`
    HealthCheckEndpoint     string      `yaml:"health_check_endpoint"`
}

var defaultConfig = Config {
    DatabasePath: "./jilo-server.db",
    HealthCheckEnabled: false,
    HealthCheckPort: 8080,
    HealthCheckEndpoint: "/health",
}

// Loading the config file
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

// Start the health check
func startHealthCheckServer(port int, endpoint string) {
    http.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
        // If the server is healthy, the response if 200 OK json, no content
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
    })

    address := fmt.Sprintf(":%d", port)

    log.Printf("Starting health check server on %s%s", address, endpoint)
    go http.ListenAndServe(address, nil)
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
CREATE TABLE IF NOT EXISTS jilo_agent_checks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        agent_id INTEGER,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
        status_code INTEGER,
        response_time_ms INTEGER,
        response_content TEXT,
        FOREIGN KEY(agent_id) REFERENCES jilo_agents(id)
);`
    _, err = db.Exec(createTable)
    if err != nil {
        return nil, err
    }

    return db, nil
}

// Check for the table
func checkTableExists(db *sql.DB) bool {
    sql := `SELECT
                name
            FROM
                sqlite_master
            WHERE
                type='table'
            AND
                name='jilo_agent_checks';`
    row := db.QueryRow(sql)

    var name string
    err := row.Scan(&name)

    return err == nil && name == "jilo_agent_checks"
}

// Get Jilo agents details
func getAgents(db *sql.DB) ([]Agent, error) {
    sql := `SELECT
                ja.id,
                ja.url,
                ja.secret_key,
                ja.check_period,
                jat.endpoint
            FROM
                jilo_agents ja
            JOIN
                jilo_agent_types jat ON ja.agent_type_id = jat.id`
    rows, err := db.Query(sql)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var agents []Agent
    for rows.Next() {
        var agent Agent
        var endpoint string
        var checkPeriod int

        // Get the agent details
        if err := rows.Scan(&agent.ID, &agent.URL, &agent.Secret, &checkPeriod, &endpoint); err != nil {
            return nil, err
        }
        // Form the whole enpoint
        agent.URL += endpoint
        agent.CheckPeriod = checkPeriod

        agents = append(agents, agent)
    }

    // We return the endpoints, not all the details
    return agents, nil
}

// Format the enpoint URL to use in logs
func getDomainAndPath(fullURL string) string {
    parsedURL, err := url.Parse(fullURL)
    if err != nil {
        // Fallback to the original URL on error
        return fullURL
    }

    return parsedURL.Host + parsedURL.Path
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
func checkEndpoint(agent Agent) (int, int64, string, bool) {
    log.Println("Sending HTTP get request to Jilo agent:", agent.URL)

    // Create the http request
    req, err := http.NewRequest("GET", agent.URL, nil)
    if err != nil {
        log.Println("Failed to create the HTTP request:", err)
        return 0, 0, "", false
    }

    // Generate the JWT token
    if agent.Secret != "" {
        token, err := generateJWT(agent.Secret)
        if err != nil {
            log.Println("Failed to generate JWT token:", err)
            return 0, 0, "", false
        }
        // Set Authorization header
        req.Header.Set("Authorization", "Bearer "+token)
    }

    start := time.Now()
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Println("Failed to check the endpoint:", err)
        return 0, 0, "", false
    }
    defer resp.Body.Close()

    elapsed := time.Since(start).Milliseconds()

    // Read the response body
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Println("Failed to read the response body:", err)
        return resp.StatusCode, elapsed, "", false
    }

    log.Printf("Received response: %d, Time taken: %d ms", resp.StatusCode, elapsed)

    return resp.StatusCode, elapsed, string(body), true
}

// Insert the checks into the database
func saveData(db *sql.DB, agentID int, statusCode int, responseTime int64, responseContent string) {
    sql := `INSERT INTO
                jilo_agent_checks
                    (agent_id, status_code, response_time_ms, response_content)
                VALUES
                    (?, ?, ?, ?)`
    _, err := db.Exec(sql, agentID, statusCode, responseTime, responseContent)
    if err != nil {
        log.Println("Failed to insert data into the database:", err)
    }
}

// Main routine
func main() {
    // First flush all the logs
    log.SetFlags(log.LstdFlags | log.Lshortfile)

    // Command-line options
    // "--init-db" creates the table
    initDB := flag.Bool("init-db", false, "Create database table if not present without prompting")
    // Config file
    configPath := flag.String("config", "", "Path to the configuration file (use -c or --config)")
    flag.StringVar(configPath, "c", "", "Path to the configuration file")

    flag.Parse()

    // Choosing the config file
    finalConfigPath := "./jilo-server.conf" // this is the default we fall to
    if *configPath != "" {
        if _, err := os.Stat(*configPath); err == nil {
            finalConfigPath = *configPath
        } else {
            log.Printf("Specified file \"%s\" doesn't exist. Falling back to the default \"%s\".", *configPath, finalConfigPath)
        }
    }

    // Config file
    log.Printf("Using config file %s", finalConfigPath)
    config := readConfig(finalConfigPath)

    // Start the health check, if it's enabled in the config file
    if config.HealthCheckEnabled {
        startHealthCheckServer(config.HealthCheckPort, config.HealthCheckEndpoint)
    }

    // Connect to or setup the database
    log.Println("Initializing the database...")
    db, err := setupDatabase(config.DatabasePath, *initDB)
    if err != nil {
        log.Fatal("Failed to initialize the database:", err)
    }
    defer db.Close()

    // Prepare the Agents
    agents, err := getAgents(db)
    if err != nil {
        log.Fatal("Failed to fetch the agents:", err)
    }

    log.Println("Starting endpoint checker...")

    // Iterate over the servers and agents
    for _, agent := range agents {
        if agent.CheckPeriod > 0 {
            go func(agent Agent) {
                // Ticker for the periodic checks
                ticker := time.NewTicker(time.Duration(agent.CheckPeriod) * time.Minute)
                defer ticker.Stop()

                for {
                    log.Printf("Checking agent [%d]: %s", agent.ID, agent.URL)
                    statusCode, responseTime, responseContent, success := checkEndpoint(agent)
                    if success {
                        log.Printf("Agent [%d]: Status code: %d, Response time: %d ms", agent.ID, statusCode, responseTime)
                        saveData(db, agent.ID, statusCode, responseTime, responseContent)
                    } else {
                        log.Printf("Check for agent %s (%d) failed, skipping database insert", getDomainAndPath(agent.URL), agent.ID)
                    }

                    // Sleep until the next tick
                    <-ticker.C
                }
            }(agent)
        } else {
            log.Printf("Agent %s (%d) has an invalid CheckPeriod (%d), skipping it.", getDomainAndPath(agent.URL), agent.ID, agent.CheckPeriod)
        }
    }

    // Prevent the main from exiting
    select {}
}
