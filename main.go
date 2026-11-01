// Terraform provider for Calendoro. The resource table in internal/crud is generated from OpenAPI.
package main

import (
	"context"
	"log"

	"github.com/Calendoro/terraform-provider-calendoro/internal/tfprovider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	err := providerserver.Serve(context.Background(), tfprovider.New, providerserver.ServeOpts{Address: "registry.terraform.io/calendoro/calendoro"})
	if err != nil {
		log.Fatal(err)
	}
}
