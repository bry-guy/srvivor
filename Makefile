.PHONY: help checkhealth bootstrap download test build run install uninstall clean

# Default target executed when no arguments are given to make.
help:
	@echo "Available commands:"
	@echo "  checkhealth  	- Verify development dependencies are installed"
	@echo "  bootstrap  	- Install developer dependencies"
	@echo "  download   	- Download dependencies"
	@echo "  test       	- Run tests"
	@echo "  build      	- Build the application"
	@echo "  run        	- Run the application (requires 'filepath' argument)"
	@echo "  install    	- Install the binary to OPATH/bin"
	@echo "  uninstall  	- Uninstall the binary from OPATH/bin"
	@echo "  clean      	- Remove built application and any generated files"

# Verify development tools are installed
checkhealth:
	@./checkhealth.sh

# Install tools from tools.go
bootstrap:
	@cat tools.go | grep _ | awk '{ print $$2 }' | xargs -L1 go install

# Download necessary dependencies
download:
	go mod tidy

# Run tests
test:
	gotest -v ./...

# Build the app
build:
	go build -o srvivor

# Run the app
run:
	./srvivor score --file $(FILEPATH)

# Install the binary to $GOPATH/bin
install:
	go build -o $(GOPATH)/bin/srvivor

# Uninstall the binary from $GOPATH/bin
uninstall:
	rm -f $(GOPATH)/bin/srvivor

# Clean up
clean:
	rm -f srvivor

# Set the default goal to 'help' when no targets are given on the command line
.DEFAULT_GOAL := help

