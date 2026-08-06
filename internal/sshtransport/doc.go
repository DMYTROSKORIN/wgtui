// Package sshtransport wraps the system OpenSSH client (ssh, ssh -G, an
// optional ControlMaster socket) to run remote commands and PTY sessions.
// It owns host-key verification, password/PTY handling and session
// lifecycle. It must never accept a private key or password as a process
// argument, and must never write secrets to logs.
package sshtransport
