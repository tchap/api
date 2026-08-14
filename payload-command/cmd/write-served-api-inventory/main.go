package main

import (
	"flag"
	"fmt"
	"os"

	gservedapis "github.com/openshift/api/payload-command/servedapis"
)

func main() {
	o := &gservedapis.WriteServedAPIInventory{}
	o.AddFlags(flag.CommandLine)
	flag.Parse()

	if err := o.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
