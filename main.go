package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Process struct {
	Name      string
	Command   string
	Args      []string
	PauseMs   int
	Port      int
	CmdObject *exec.Cmd
}

type Config struct {
	Processes []*Process
	LogLevel  string
}

var (
	restartMutex sync.Mutex
	config       Config
)

func loadConfig() {
	fmt.Println("Loading process manager config...")
	file, err := os.Open("config.json")
	if err != nil {
		fmt.Println("Error opening config file:", err)
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Println("Error decoding config file:", err)
		return
	}
	fmt.Println("Config loaded successfully")
}

func dumpConfig() {
	fmt.Println("Dumping config...")
	for _, process := range config.Processes {
		fmt.Printf("Name: %s\n", process.Name)
		fmt.Printf("Command: %s\n", process.Command)
		fmt.Printf("Args: %v\n", process.Args)
		fmt.Printf("PauseMs: %d\n", process.PauseMs)
		fmt.Printf("Port: %d\n", process.Port)
	}
	fmt.Printf("LogLevel: %s\n", config.LogLevel)
}

func stopProcesses(process *Process) {
	fmt.Printf("Stopping %s...\n", process.Name)
	if process.CmdObject != nil {
		process.CmdObject.Process.Kill()
		fmt.Printf("%s stopped\n", process.Name)
	} else {
		fmt.Printf("%s is not running\n", process.Name)
	}
}

func startProcesses(process *Process) {
	fmt.Printf("Starting %s...\n", process.Name)
	process.CmdObject = exec.Command(process.Command, process.Args...)
	if err := process.CmdObject.Start(); err != nil {
		fmt.Printf("Failed to start %s: %v\n", process.Name, err)
	} else {
		fmt.Printf("%s started\n", process.Name)
		time.Sleep(time.Duration(process.PauseMs) * time.Millisecond)
	}
}

func isProcessHealthy(process Process) bool {
	fmt.Printf("Checking health of %s...\n", process.Name)
	if process.CmdObject == nil {
		fmt.Printf("%s has not been started or CmdObject is nil\n", process.Name)
		return false
	}
	if process.CmdObject.ProcessState != nil &&
		!process.CmdObject.ProcessState.Exited() {
		fmt.Printf("%s is healthy\n", process.Name)
		return true
	}
	fmt.Printf("%s is not healthy\n", process.Name)
	return false
}

func canConnectToProcess(process Process) bool {
	fmt.Printf("Checking network health of %s...\n", process.Name)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", process.Port), 2*time.Second)
	if err != nil {
		fmt.Printf("%s is not running\n", process.Name)
		return false
	} else {
		fmt.Printf("%s is running\n", process.Name)
		conn.Close()
		return true
	}
}

func healthCheck() bool {
	fmt.Println("Checking processes health...")
	for _, process := range config.Processes {
		if !isProcessHealthy(*process) || !canConnectToProcess(*process) {
			fmt.Printf("Process %s is not healthy\n", process.Name)
			return false
		}
	}
	return true
}

func healthCheckLoop() {
	for {
		time.Sleep(5 * time.Second)
		if healthCheck() {
			fmt.Println("All processes are healthy")
		} else {
			fmt.Println("One or more processes are not healthy")
		}
	}
}

// handlers
// healthcheck handler
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("External health check request received")
	if healthCheck() {
		fmt.Println("All processes are healthy")
		w.WriteHeader(http.StatusOK)
	} else {
		fmt.Println("One or more processes are not healthy")
		http.Error(w, "One or more processes are not healthy", http.StatusServiceUnavailable)
	}
}

// restart handler
func restartHandler(w http.ResponseWriter) {
	fmt.Println("External restart request received")
	restartMutex.Lock()
	defer restartMutex.Unlock()

	if healthCheck() {
		fmt.Println("All processes are healthy, no need to restart")
		w.WriteHeader(http.StatusOK)
		return
	} else {
		fmt.Println("One or more processes are not healthy, restarting...")
		for _, process := range config.Processes {
			stopProcesses(process)
			startProcesses(process)
		}
		fmt.Println("Processes restarted")
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	fmt.Println("Starting process manager...")

	// load config
	loadConfig()
	// dumpConfig()

	// start processes
	for _, process := range config.Processes {
		startProcesses(process)
	}

	// start health check process
	fmt.Println("Staring health check process...")
	//go healthCheckLoop()

	// start HTTP server
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		restartHandler(w)
	})
	fmt.Println("Starting HTTP server on port 8080...")
	http.ListenAndServe(":8080", nil)
	fmt.Println("HTTP server stopped")
}
