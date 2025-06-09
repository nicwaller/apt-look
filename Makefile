SOURCES := $(shell find cmd pkg -name "*.go" 2>/dev/null)

apt-look: $(SOURCES) go.mod go.sum
	go build -o apt-look ./cmd/apt-look/
