# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
This is a Go project called "executr" that uses Nix flakes for development environment management with direnv integration.

## Development Environment Setup
The project uses Nix flakes (nixos-25.05) with direnv. To activate the development environment:
```bash
direnv allow
```

## Common Commands

### Go Development
- `go build` - Build the project
- `go test` - Run all tests  
- `go test ./... -v` - Run all tests with verbose output
- `go test -run TestName` - Run a specific test
- `go run .` - Run the application
- `golangci-lint run` - Run the linter

### Debugging
- `dlv debug` - Start debugger for the main package
- `dlv test` - Debug tests

## Project Initialization Status
**Note**: This is a new Go project without a go.mod file yet. When initializing:
1. Run `go mod init github.com/draganm/executr` (or appropriate module path)
2. Update the `vendorHash` in flake.nix after adding dependencies

## Development Tools Available
The Nix flake provides:
- Go 1.23
- gopls (language server)
- golangci-lint (linter)
- delve (debugger)
- go-tools
- make and pkg-config for build automation