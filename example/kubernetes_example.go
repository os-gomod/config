//go:build ignore

// Example: Kubernetes ConfigMap and Secret integration
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
	"github.com/os-gomod/config/observability"
)

// K8sConfig represents application configuration loaded from Kubernetes ConfigMaps and Secrets.
type K8sConfig struct {
	Server struct {
		Host    string        `config:"server.host" default:"0.0.0.0"`
		Port    int           `config:"server.port" default:"8080"`
		Timeout time.Duration `config:"server.timeout" default:"30s"`
	} `config:"server"`
	Database struct {
		Host     string `config:"database.host"`
		Port     int    `config:"database.port" default:"5432"`
		Name     string `config:"database.name"`
		Password string `config:"database.password"`
	} `config:"database"`
}

// k8sLoader implements core.Loadable to read from Kubernetes ConfigMaps and Secrets.
// In production, this would use the Kubernetes client-go library.
type k8sLoader struct {
	configMapName string
	secretName    string
	namespace     string
}

func (k *k8sLoader) Load(ctx context.Context) (map[string]value.Value, error) {
	data := make(map[string]value.Value)
	// In production, use client-go:
	// cm, err := clientset.CoreV1().ConfigMaps(k.namespace).Get(ctx, k.configMapName, metav1.GetOptions{})
	// secret, err := clientset.CoreV1().Secrets(k.namespace).Get(ctx, k.secretName, metav1.GetOptions{})
	// For demo, read from files mounted by Kubernetes
	if err := k.loadFromFS("/etc/config", data); err != nil {
		return data, err
	}
	if err := k.loadFromFS("/etc/secrets", data); err != nil {
		return data, err
	}
	return data, nil
}

func (k *k8sLoader) loadFromFS(dir string, data map[string]value.Value) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		data[e.Name()] = value.New(string(content), value.TypeString, value.SourceKubernetes, 50)
	}
	return nil
}

func (k *k8sLoader) Close(_ context.Context) error { return nil }
func (k *k8sLoader) Priority() int                 { return 50 }
func (k *k8sLoader) String() string                { return "kubernetes" }

func main() {
	ctx := context.Background()

	k8s := &k8sLoader{
		configMapName: "app-config",
		secretName:    "app-secrets",
		namespace:     "default",
	}

	cfg, err := config.New(ctx,
		config.WithLoader(loader.NewEnvLoader(loader.WithEnvPrefix("APP"))),
		config.WithCircuitBreakerLayer("kubernetes", k8s, 50, circuit.BreakerConfig{
			Threshold:        3,
			Timeout:          30 * time.Second,
			SuccessThreshold: 2,
		}),
		config.WithRecorder(&observability.AtomicMetrics{}),
		config.WithStrictReload(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer cfg.Close(ctx)

	var appCfg K8sConfig
	if err := cfg.Bind(ctx, &appCfg); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Server: %s:%d\n", appCfg.Server.Host, appCfg.Server.Port)
	fmt.Printf("Database: %s:%d/%s\n", appCfg.Database.Host, appCfg.Database.Port, appCfg.Database.Name)

	// Watch for Kubernetes config changes
	cancel := cfg.OnChange("*", func(ctx context.Context, evt event.Event) error {
		log.Printf("k8s config changed: %s %s", evt.Type, evt.Key)
		return cfg.Bind(ctx, &appCfg)
	})
	defer cancel()

	// Snapshot with secret redaction
	snapshot := cfg.Snapshot()
	fmt.Printf("Snapshot keys: %d (secrets redacted)\n", len(snapshot))

	<-ctx.Done()
}
