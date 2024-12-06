package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/polarsignals/kubezonnet/agent"
)

func main() {
	subnetCidr := flag.String("subnet-cidr", "10.0.0.0/24", "Specify the subnet in CIDR notation (default: 10.0.0.0/24)")
	flushInterval := flag.Duration("flush-interval", 10*time.Second, "The interval at which data is sent to the server")
	server := flag.String("server", "", "The server to send statistics to")
	send := flag.Bool("send-data", true, "Whether to enable sending data to the server, only really useful when used with debugging")
	debug := flag.Bool("debug", false, "Turns on extra debugging features, not recommended for production")
	node := flag.String("node", "", "The Kubernetes node name of the node the agent is running on")
	flag.Parse()

	if *subnetCidr == "" {
		fmt.Println("Error: subnet CIDR cannot be empty")
		flag.Usage()
		os.Exit(1)
	}

	if *flushInterval <= 0 {
		fmt.Println("Error: flush interval must be greater than zero")
		flag.Usage()
		os.Exit(1)
	}

	if *server == "" {
		fmt.Println("Error: server must not be empty")
		flag.Usage()
		os.Exit(1)
	}

	if *node == "" {
		fmt.Println("Error: node name must not be empty")
		flag.Usage()
		os.Exit(1)
	}

	if err := agent.Run(*node, *subnetCidr, *server, *flushInterval, *debug, *send); err != nil {
		log.Fatal("error: ", err)
	}
}
