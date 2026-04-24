//go:build ignore

// Example: AWS Secrets Manager integration
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/core/circuit"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

// AWSSecretsLoader fetches secrets from AWS Secrets Manager.
type AWSSecretsLoader struct {
	secretIDs []string
	region    string
	priority  int
}

func NewAWSSecretsLoader(secretIDs []string, region string) *AWSSecretsLoader {
	return &AWSSecretsLoader{
		secretIDs: secretIDs,
		region:    region,
		priority:  60,
	}
}

func (a *AWSSecretsLoader) Load(ctx context.Context) (map[string]value.Value, error) {
	data := make(map[string]value.Value)
	// In production, use AWS SDK:
	// svc := secretsmanager.NewFromConfig(cfg)
	// for _, id := range a.secretIDs {
	//     result, err := svc.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &id})
	//     ...
	// }
	// For demo, simulate
	for _, id := range a.secretIDs {
		if v := os.Getenv("DEMO_" + id); v != "" {
			data[id] = value.New(v, value.TypeString, value.SourceKMS, a.priority)
		}
	}
	return data, nil
}

func (a *AWSSecretsLoader) Close(_ context.Context) error { return nil }
func (a *AWSSecretsLoader) Priority() int                 { return a.priority }
func (a *AWSSecretsLoader) String() string                { return "aws-secrets" }
func (a *AWSSecretsLoader) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

func main() {
	ctx := context.Background()

	cfg, err := config.New(ctx,
		config.WithLoader(loader.NewFileLoader("config.yaml")),
		config.WithLoader(loader.NewEnvLoader(loader.WithEnvPrefix("APP"))),
		config.WithCircuitBreakerLayer("aws-secrets",
			NewAWSSecretsLoader([]string{"db/password", "api/key"}, "us-east-1"),
			60, circuit.BreakerConfig{
				Threshold:        3,
				Timeout:          30 * time.Second,
				SuccessThreshold: 2,
			},
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer cfg.Close(ctx)

	// Get a secret value
	if v, ok := cfg.Get("db.password"); ok {
		fmt.Printf("DB password: [REDACTED]\n") // Never print secrets!
		_ = v
	}

	// Use Explain to get safe key info (auto-redacted)
	fmt.Println(cfg.Explain("db.password"))

	// Snapshot everything with redaction
	snapshot := cfg.Snapshot()
	fmt.Printf("Config has %d keys (secrets redacted)\n", len(snapshot))
}
