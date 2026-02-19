package main

import (
	"context"
	"flag"
	"log"

	"github.com/JonasKop/terraform-provider-seaweedfs/seaweedfs"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name seaweedfs

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/jonaskop/seaweedfs",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), seaweedfs.NewProvider, opts); err != nil {
		log.Fatal(err.Error())
	}
}
