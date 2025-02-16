package main

import (
	"context"
	"log"
	"scripts-proto/src/k8s"
)

func main() {
	ctx := context.Background()

	client, err := k8s.NewK8sClient()
	if err != nil {
		log.Fatalf("Failed to create K8s client: %v", err)
	}

	config := k8s.DeploymentConfig{
		ProjectName:   "haylinmoore",
		Username:      "haylin",
		DomainName:    "haylin.script.mkr.cx",
		DockerImage:   "nginx:latest",
		ContainerPort: 80,
		Namespace:     "default",
		TLSSecretName: "wildcard-mkr-certs", // Specify the TLS secret name
	}

	err = client.CreateFullStack(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create resources: %v", err)
	}
}
