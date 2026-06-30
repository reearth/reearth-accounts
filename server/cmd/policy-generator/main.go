package main

import (
	"log"

	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/internal/rbac"
	"github.com/reearth/reearthx/cerbos/generator"
)

type policySet struct {
	serviceName     string
	defineResources generator.DefineResourcesFunc
	outputDir       string
}

func main() {
	sets := []policySet{
		{rbac.ServiceName, rbac.DefineResources, rbac.PolicyFileDir},
		{adminrbac.ServiceName, adminrbac.DefineResources, adminrbac.PolicyFileDir},
	}

	for _, s := range sets {
		if err := generator.GeneratePolicies(s.serviceName, s.defineResources, s.outputDir); err != nil {
			log.Fatalf("Failed to generate policies for %s: %v", s.serviceName, err)
		}
	}
}
