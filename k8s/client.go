package k8s

import (
	"context"
	"log"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/versioneer-tech/package-r/v2/k8s/api/alphav1"
)

type NamespacedClient struct {
	client    client.Client
	namespace string
}

func NewDefaultClient() *NamespacedClient {
	return NewClient(nil)
}

func NewClient(namespace *string) *NamespacedClient {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if namespace == nil {
		defaultNamespace := os.Getenv("NAMESPACE_DEFAULT")
		if defaultNamespace != "" {
			configOverrides.Context.Namespace = defaultNamespace
		}
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		log.Printf("NamespacedClient couldn't be created (config): %s", err)
		return nil
	}

	scheme := runtime.NewScheme()
	err = v1.AddToScheme(scheme)
	if err != nil {
		log.Fatalf("NamespacedClient couldn't be created (core): %s", err)
		return nil
	}
	err = alphav1.AddToScheme(scheme)
	if err != nil {
		log.Fatalf("NamespacedClient couldn't be created (package-r): %s", err)
		return nil
	}

	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("NamespacedClient couldn't be created (client): %s", err)
		return nil
	}

	ns, _, err := kubeConfig.Namespace()
	if err != nil {
		log.Fatalf("NamespacedClient couldn't be created (namespace): %s", err)
		return nil
	}

	return &NamespacedClient{client: cl, namespace: ns}
}

func (nsc *NamespacedClient) ListSources(ctx context.Context) (*alphav1.SourceList, error) {
	var list alphav1.SourceList
	err := nsc.client.List(ctx, &list, &client.ListOptions{Namespace: nsc.namespace})
	return &list, err
}

func (nsc *NamespacedClient) GetSource(ctx context.Context, name string) (*alphav1.Source, error) {
	var obj alphav1.Source
	err := nsc.client.Get(ctx, types.NamespacedName{Namespace: nsc.namespace, Name: name}, &obj)
	return &obj, err
}

func (nsc *NamespacedClient) GetSecret(ctx context.Context, name string) (*v1.Secret, error) {
	var secret v1.Secret
	err := nsc.client.Get(ctx, types.NamespacedName{Namespace: nsc.namespace, Name: name}, &secret)
	return &secret, err
}
