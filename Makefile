all: kubezonnet-agent-container kubezonnet-server-container

kubezonnet-agent:
	cd agent && go generate
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-installsuffix cgo" -o kubezonnet-agent cmd/agent/main.go

.PHONY: kubezonnet-agent
kubezonnet-agent-container: kubezonnet-agent
	docker build --platform=linux/amd64 -f Dockerfile.agent -t ghcr.io/polarsignals/kubezonnet-agent .

kubezonnet-server:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-installsuffix cgo" -o kubezonnet-server cmd/server/main.go

.PHONY: kubezonnet-agent
kubezonnet-server-container: kubezonnet-server
	docker build --platform=linux/amd64 -f Dockerfile.server -t ghcr.io/polarsignals/kubezonnet-server .

.PHONY: clean
clean:
	rm kubezonnet-server kubezonnet-agent agent/kubezonnet_bpfeb.o agent/kubezonnet_bpfel.o

.PHONY: build
build: kubezonnet-agent kubezonnet-server
