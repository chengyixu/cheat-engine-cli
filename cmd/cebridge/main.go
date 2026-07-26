package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/chengyixu/cheat-engine-cli/internal/localbridge"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:52736", "TCP listen address")
	flag.Parse()
	backend, err := localbridge.NewSystemBackend()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "cebridge (%s) listening on %s\n", runtime.GOOS, *listenAddress)
	if err := localbridge.ListenAndServe(*listenAddress, backend, "cebridge-"+runtime.GOOS); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
