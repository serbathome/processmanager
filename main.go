package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Logger struct {
	Level string
	mu    sync.Mutex
}

func (logger *Logger) SetLevel(level string) {
	logger.mu.Lock()
	logger.Level = level
	logger.mu.Unlock()
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	config.HealthCheckIntervalSeconds = 60
}

func (logger *Logger) Debug(message string) {
	if logger.Level == "DEBUG" {
		logger.mu.Lock()
		log.Default().Println("[Debug]", message)
		logger.mu.Unlock()
	}
}

func (logger *Logger) Info(message string) {
	if logger.Level == "DEBUG" || logger.Level == "INFO" {
		logger.mu.Lock()
		log.Default().Println("[Info]", message)
		logger.mu.Unlock()
	}
}

type Process struct {
	Name      string
	Command   string
	Args      []string
	PauseMs   int
	Port      int
	CmdObject *exec.Cmd
}

type Config struct {
	Processes                  []Process
	LogLevel                   string
	HealthCheckIntervalSeconds int
}

var (
	restartMutex sync.Mutex
	config       Config
	logger       Logger = Logger{Level: "INFO"}
)

func (config *Config) loadConfig() {
	logger.Debug("Loading process manager config...")
	file, err := os.Open("config.json")
	if err != nil {
		logger.Debug("Error opening config file: " + err.Error())
		panic(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		logger.Debug("Error decoding config file: " + err.Error())
		panic(err)
	}
	logger.SetLevel(config.LogLevel)
	logger.Debug("Config loaded successfully")
}

func (config *Config) dumpConfig() {
	logger.Debug("--- Configuration ---")
	for _, process := range config.Processes {
		logger.Debug("Process Name: " + process.Name)
		logger.Debug("  Command: " + process.Command)
		logger.Debug("  Args: " + fmt.Sprintf("%v", process.Args))
		logger.Debug("  PauseMs: " + fmt.Sprintf("%d", process.PauseMs))
		logger.Debug("  Port: " + fmt.Sprintf("%d", process.Port))
	}
	logger.Debug("Log level: " + config.LogLevel)
	logger.Debug("HealthCheckIntervalSeconds: " + fmt.Sprintf("%d", config.HealthCheckIntervalSeconds))
	logger.Debug("--- End of Configuration ---")
}

func (p *Process) Wait() {
	logger.Debug(fmt.Sprintf("[Watchdog] Start monitoring process: %s", p.Name))
	err := p.CmdObject.Wait() // Wait for the process to finish
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				logger.Debug(fmt.Sprintf("[Watchdog] Process %s exited with code: %d", p.Name, status.ExitStatus()))
				if status.Signaled() {
					logger.Debug(fmt.Sprintf("[Watchdog] Process %s was terminated by signal: %s", p.Name, status.Signal()))
				}
			}
		} else {
			logger.Debug(fmt.Sprintf("[Watchdog] Process %s exited with error: %v", p.Name, err))
		}
	} else {
		logger.Debug(fmt.Sprintf("[Watchdog] Process %s completed successfully", p.Name))
	}
}

func (p *Process) watchProcess() {
	for {
		p.startProcess()
		p.Wait()
		logger.Info(fmt.Sprintf("[Watchdog] Process %s exited, restarting...", p.Name))
	}
}

func (process *Process) stopProcess() {
	logger.Debug(fmt.Sprintf("Stopping %s with pid %d", process.Name, process.CmdObject.Process.Pid))
	if process.CmdObject != nil {
		process.CmdObject.Process.Kill()
		logger.Debug(fmt.Sprintf("%s stopped", process.Name))
	} else {
		logger.Debug(fmt.Sprintf("%s is not running", process.Name))
	}
}

func (process *Process) startProcess() {
	logger.Debug(fmt.Sprintf("Starting %s with command %s and args %v", process.Name, process.Command, process.Args))
	time.Sleep(time.Duration(process.PauseMs) * time.Millisecond)
	process.CmdObject = exec.Command(process.Command, process.Args...)

	stdoutPipe, err := process.CmdObject.StdoutPipe()
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to get stdout pipe for %s: %v", process.Name, err))
		return
	}

	stderrPipe, err := process.CmdObject.StderrPipe()
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to get stderr pipe for %s: %v", process.Name, err))
		return
	}

	if err := process.CmdObject.Start(); err != nil {
		logger.Debug(fmt.Sprintf("Failed to start %s: %v", process.Name, err))
		return
	}

	go process.captureOutput(stdoutPipe)
	go process.captureOutput(stderrPipe)

	logger.Debug(fmt.Sprintf("%s started with pid %d", process.Name, process.CmdObject.Process.Pid))
}

func (process *Process) captureOutput(pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		logger.Debug(fmt.Sprintf("[%s] %s", process.Name, scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		logger.Debug(fmt.Sprintf("Error reading from pipe for %s: %v", process.Name, err))
	}
}

func (process *Process) canConnectToProcess() bool {
	logger.Debug(fmt.Sprintf("Checking network health of %s...", process.Name))
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", process.Port), 2*time.Second)
	if err != nil {
		logger.Debug(fmt.Sprintf("Error connecting to %s: %v", process.Name, err))
		return false
	} else {
		logger.Debug(fmt.Sprintf("Connected to %s", process.Name))
		conn.Close()
		return true
	}
}

func healthCheck() bool {
	for i := 0; i < len(config.Processes); i++ {
		if !config.Processes[i].canConnectToProcess() {
			return false
		}
	}
	return true
}

func healthCheckLoop() {
	for {
		time.Sleep(time.Duration(config.HealthCheckIntervalSeconds) * time.Second)
		if healthCheck() {
			logger.Info("Network connection to all processes is healthy")
		} else {
			logger.Info("One or more processes are not accessible over network, restarting processes...")
			if !restartMutex.TryLock() {
				logger.Info("Restart already in progress, skipping health check")
				continue
			} else {
				restartProcesses()
			}
			restartMutex.Unlock()
		}
	}
}

func restartProcesses() {
	for i := 0; i < len(config.Processes); i++ {
		config.Processes[i].stopProcess()
		// we don't need to start the process here, the watchProcess function will do it
	}
}

// handlers
// healthcheck handler
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("External health check request received")
	if healthCheck() {
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "One or more processes are not healthy", http.StatusServiceUnavailable)
	}
}

// restart handler
func restartHandler(w http.ResponseWriter) {
	logger.Info("External restart request received")

	if !restartMutex.TryLock() {
		logger.Info("Restart already in progress, ignoring request")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	defer restartMutex.Unlock()

	if healthCheck() {
		logger.Info("All processes are healthy, no need to restart")
		w.WriteHeader(http.StatusOK)
		return
	} else {
		logger.Debug("One or more processes are not healthy. Restarting processes...")
		restartProcesses()
		logger.Info("Processes restarted")
		w.WriteHeader(http.StatusOK)
	}
}

func main() {

	logger.Info("Starting process manager...")

	// load config
	config.loadConfig()
	config.dumpConfig()

	// start processes
	logger.Info("Starting processes...")
	for i := range config.Processes {
		go config.Processes[i].watchProcess()
	}

	// start health check process
	logger.Debug("Starting health check loop...")
	go healthCheckLoop()

	// start HTTP server
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		restartHandler(w)
	})
	logger.Info("Starting HTTP server on port 8080...")
	http.ListenAndServe(":8080", nil)
	logger.Info("HTTP server stopped")
}
