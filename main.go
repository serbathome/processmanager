package main

import (
	"encoding/json"
	"fmt"
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
}

func (logger *Logger) SetLevel(level string) {
	logger.Level = level
}

func (logger *Logger) Debug(message string) {
	if logger.Level == "DEBUG" {
		log.Default().Println("|Debug|", message)
	}
}

func (logger *Logger) Error(message string) {
	if logger.Level == "INFO" || logger.Level == "ERROR" {
		log.Default().Println("|Error|", message)
	}
}

func (logger *Logger) Info(message string) {
	if logger.Level == "DEBUG" || logger.Level == "ERROR" || logger.Level == "INFO" {
		log.Default().Println("|Info|", message)
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
	Processes []Process
	LogLevel  string
}

var (
	restartMutex sync.Mutex
	config       Config
	logger       Logger
)

func (config *Config) loadConfig() {
	logger.Debug("Loading process manager config...")
	file, err := os.Open("config.json")
	if err != nil {
		logger.Error("Error opening config file: " + err.Error())
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		logger.Error("Error decoding config file: " + err.Error())
		return
	}
	logger.Debug("Config loaded successfully")
}

func (config *Config) dumpConfig() {
	logger.Debug("Current config:")
	for _, process := range config.Processes {
		logger.Debug("Name: " + process.Name)
		logger.Debug("Command: " + process.Command)
		logger.Debug("Args: " + fmt.Sprintf("%v", process.Args))
		logger.Debug("PauseMs: " + fmt.Sprintf("%d", process.PauseMs))
		logger.Debug("Port: " + fmt.Sprintf("%d", process.Port))
	}
}

func (process *Process) PidExists() (bool, error) {
	pid := int32(process.CmdObject.Process.Pid)
	if pid <= 0 {
		return false, fmt.Errorf("invalid pid %v", pid)
	}
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false, err
	}
	switch errno {
	case syscall.ESRCH:
		return false, nil
	case syscall.EPERM:
		return true, nil
	}
	return false, err
}

func (process *Process) stopProcess() {
	logger.Debug(fmt.Sprintf("Stopping %s with pid %d", process.Name, process.CmdObject.Process.Pid))
	if process.CmdObject != nil {
		process.CmdObject.Process.Kill()
		process.CmdObject.Process.Wait()
		logger.Debug(fmt.Sprintf("%s stopped", process.Name))
	} else {
		logger.Debug(fmt.Sprintf("%s is not running", process.Name))
	}
}

func (process *Process) startProcess() {
	logger.Debug(fmt.Sprintf("Starting %s with command %s and args %v", process.Name, process.Command, process.Args))
	process.CmdObject = exec.Command(process.Command, process.Args...)
	if err := process.CmdObject.Start(); err != nil {
		logger.Error(fmt.Sprintf("Failed to start %s: %v", process.Name, err))
	} else {
		time.Sleep(time.Duration(process.PauseMs) * time.Millisecond)
		logger.Debug(fmt.Sprintf("%s started with pid %d", process.Name, process.CmdObject.Process.Pid))
	}
}

func (process *Process) isProcessHealthy() bool {
	logger.Debug(fmt.Sprintf("Checking health of %s...", process.Name))
	if ok, err := process.PidExists(); ok && err == nil {
		logger.Debug(fmt.Sprintf("%s is healthy", process.Name))
		return true
	} else if err != nil {
		logger.Error(fmt.Sprintf("Error checking health of %s: %v", process.Name, err))
	} else {
		logger.Error(fmt.Sprintf("%s is not healthy", process.Name))
	}
	return false
}

func (process *Process) canConnectToProcess() bool {
	logger.Debug(fmt.Sprintf("Checking network health of %s...", process.Name))
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", process.Port), 2*time.Second)
	if err != nil {
		logger.Error(fmt.Sprintf("Error connecting to %s: %v", process.Name, err))
		return false
	} else {
		logger.Debug(fmt.Sprintf("Connected to %s", process.Name))
		conn.Close()
		return true
	}
}

func healthCheck() bool {
	for i := 0; i < len(config.Processes); i++ {
		if !config.Processes[i].isProcessHealthy() || !config.Processes[i].canConnectToProcess() {
			return false
		}
	}
	return true
}

func healthCheckLoop() {
	for {
		time.Sleep(60 * time.Second)
		if healthCheck() {
			logger.Info("All processes are healthy")
		} else {
			logger.Error("One or more processes are not healthy")
		}
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
	restartMutex.Lock()
	defer restartMutex.Unlock()

	if healthCheck() {
		logger.Info("All processes are healthy, no need to restart")
		w.WriteHeader(http.StatusOK)
		return
	} else {
		logger.Error("One or more processes are not healthy. Restarting processes...")
		for i := 0; i < len(config.Processes); i++ {
			config.Processes[i].stopProcess()
			config.Processes[i].startProcess()
		}
		logger.Info("Processes restarted")
		w.WriteHeader(http.StatusOK)
	}
}

func main() {

	logger.SetLevel("DEBUG")

	logger.Info("Starting process manager...")

	// load config
	config.loadConfig()
	config.dumpConfig()

	// start processes
	for i := 0; i < len(config.Processes); i++ {
		config.Processes[i].startProcess()
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
