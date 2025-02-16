package k8s

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"time"
)

type K8sClient struct {
	clientset *kubernetes.Clientset
}

type DeploymentConfig struct {
	ProjectName   string
	Username      string
	DomainName    string
	DockerImage   string
	ContainerPort int32
	Namespace     string
	TLSSecretName string
}

// Common labels that will be applied to all resources
func getCommonLabels(config DeploymentConfig) map[string]string {
	return map[string]string{
		"app":        config.ProjectName,
		"leashUser":  config.Username,
		"managedBy":  config.Username,
		"created-by": config.Username,
	}
}

func NewK8sClient() (*K8sClient, error) {
	var config *rest.Config
	var err error

	if os.Getenv("DEV") == "true" {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load local kubeconfig: %v", err)
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load in-cluster config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	return &K8sClient{clientset: clientset}, nil
}

func (k *K8sClient) CreateFullStack(ctx context.Context, config DeploymentConfig) error {
	commonLabels := getCommonLabels(config)

	// Create Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.ProjectName,
			Labels: commonLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": config.ProjectName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: commonLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            config.ProjectName,
							Image:           config.DockerImage,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: config.ContainerPort,
								},
							},
						},
					},
				},
			},
		},
	}

	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.ProjectName,
			Labels: commonLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(int(config.ContainerPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": config.ProjectName,
			},
		},
	}

	// Create Ingress
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.ProjectName,
			Labels: commonLabels,
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer": "letsencrypt-prod",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: stringPtr("nginx"),
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{config.DomainName},
					SecretName: config.TLSSecretName,
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: config.DomainName,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: config.ProjectName,
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the resources
	_, err := k.clientset.AppsV1().Deployments(config.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %v", err)
	}

	_, err = k.clientset.CoreV1().Services(config.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %v", err)
	}

	_, err = k.clientset.NetworkingV1().Ingresses(config.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ingress: %v", err)
	}

	return nil
}

func stringPtr(s string) *string {
	return &s
}

// ListDeploymentsByUser lists all deployments with the specified leashUser label
func (k *K8sClient) ListDeploymentsByUser(ctx context.Context, username string) (*appsv1.DeploymentList, error) {
	labelSelector := fmt.Sprintf("leashUser=%s", username)
	return k.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// RestartDeployment performs a rollout restart on a deployment
func (k *K8sClient) RestartDeployment(ctx context.Context, namespace, deploymentName string) error {
	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}
