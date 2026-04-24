//go:build ignore

// Example: GitOps workflow with profile switching
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/profile"
)

func main() {
	ctx := context.Background()

	// Define profiles for different environments using LayerSpec
	stagingProfile := profile.New("staging",
		profile.LayerSpec{
			Name:     "staging-file",
			Priority: 30,
			Source:   loader.NewFileLoader("config/staging.yaml", loader.WithFilePriority(30)),
		},
		profile.LayerSpec{
			Name:     "staging-env",
			Priority: 40,
			Source:   loader.NewEnvLoader(loader.WithEnvPrefix("STAGING"), loader.WithEnvPriority(40)),
		},
	)

	productionProfile := profile.New("production",
		profile.LayerSpec{
			Name:     "production-file",
			Priority: 30,
			Source:   loader.NewFileLoader("config/production.yaml", loader.WithFilePriority(30)),
		},
		profile.LayerSpec{
			Name:     "production-env",
			Priority: 40,
			Source:   loader.NewEnvLoader(loader.WithEnvPrefix("PROD"), loader.WithEnvPriority(40)),
		},
	)

	// Start with base config + staging profile
	cfg, err := config.New(ctx,
		config.WithLoader(loader.NewFileLoader("config/base.yaml")),
		config.WithDeltaReload(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer cfg.Close(ctx)

	// Load staging profile
	_, err = cfg.LoadProfile(ctx, stagingProfile)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Running with profile: staging (v%d)\n", cfg.Version())

	// Simulate GitOps: switch to production profile at runtime
	fmt.Println("Switching to production profile...")
	result, err := cfg.LoadProfile(ctx, productionProfile)
	if err != nil {
		log.Printf("Profile switch failed: %v", err)
		return
	}
	fmt.Printf("Switched to production (v%d, %d events)\n", cfg.Version(), len(result.Events))

	// Key changes are published via event bus
	cancel := cfg.OnChange("*", func(ctx context.Context, evt event.Event) error {
		log.Printf("[%s] %s = %v", evt.Type, evt.Key, evt.NewValue.String())
		return nil
	})
	defer cancel()

	time.Sleep(1 * time.Second)
}
