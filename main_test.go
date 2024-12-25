package main

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestWait_ProcessCompletesSuccessfully(t *testing.T) {
	process := &Process{
		Name:    "testProcess",
		Command: "echo",
		Args:    []string{"hello"},
	}

	process.startProcess()
	process.CmdObject.Wait() // Ensure the process has finished

	if process.CmdObject.ProcessState == nil {
		t.Fatal("Expected process to have a ProcessState, but it was nil")
	}

	if !process.CmdObject.ProcessState.Exited() {
		t.Fatalf("Expected process to have exited, but it did not")
	}

	if process.CmdObject.ProcessState.ExitCode() != 0 {
		t.Fatalf("Expected process to exit with code 0, but got %d", process.CmdObject.ProcessState.ExitCode())
	}
}

func TestWait_ProcessExitsWithError(t *testing.T) {
	process := &Process{
		Name:    "testProcess",
		Command: "sh",
		Args:    []string{"-c", "exit 1"},
	}

	process.startProcess()
	err := process.CmdObject.Wait() // Ensure the process has finished

	if err == nil {
		t.Fatal("Expected process to exit with an error, but it did not")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected exec.ExitError, but got %T", err)
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatalf("Expected syscall.WaitStatus, but got %T", exitErr.Sys())
	}

	if status.ExitStatus() != 1 {
		t.Fatalf("Expected process to exit with code 1, but got %d", status.ExitStatus())
	}
}

func TestWait_ProcessKilled(t *testing.T) {
	process := &Process{
		Name:    "testProcess",
		Command: "sleep",
		Args:    []string{"10"},
	}

	process.startProcess()
	err := process.CmdObject.Process.Kill()
	if err != nil {
		t.Fatalf("Failed to kill process: %v", err)
	}

	err = process.CmdObject.Wait() // Ensure the process has finished

	if err == nil {
		t.Fatal("Expected process to exit with an error, but it did not")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected exec.ExitError, but got %T", err)
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatalf("Expected syscall.WaitStatus, but got %T", exitErr.Sys())
	}

	if !status.Signaled() {
		t.Fatal("Expected process to be signaled, but it was not")
	}

	if status.Signal() != syscall.SIGKILL {
		t.Fatalf("Expected process to be killed with SIGKILL, but got %v", status.Signal())
	}
}
