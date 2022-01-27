package main

import (
	"github.com/jaxxstorm/pulumi-scaleway/sdk/go/scaleway"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	v1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type ProviderDependency struct {
	ctx      *pulumi.Context
	provider pulumi.ProviderResource
}

func (p ProviderDependency) createArgoCD() error {
	argocdNS, err := v1.NewNamespace(p.ctx, "argocd", &v1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("argocd"),
		},
	}, pulumi.Provider(p.provider))
	if err != nil {
		return err
	}
	_, err = helm.NewRelease(p.ctx, "argocd-helm", &helm.ReleaseArgs{
		Name:      pulumi.String("argocd"),
		Chart:     pulumi.String("argo-cd"),
		Version:   pulumi.String("3.29.5"),
		Namespace: argocdNS.Metadata.Name(),
		RepositoryOpts: helm.RepositoryOptsArgs{
			Repo: pulumi.String("https://argoproj.github.io/argo-helm"),
		},
		Values: pulumi.Map{
			"controller": pulumi.Map{
				"serviceAccount": pulumi.Map{
					"automountServiceAccountToken": pulumi.Bool(true),
				},
			},
			"dex": pulumi.Map{
				"enabled": pulumi.Bool(false),
				"serviceAccount": pulumi.Map{
					"automountServiceAccountToken": pulumi.Bool(true),
				},
			},
			"redis": pulumi.Map{
				"serviceAccount": pulumi.Map{
					"automountServiceAccountToken": pulumi.Bool(true),
				},
			},
			"server": pulumi.Map{
				"config": pulumi.Map{
					"url": pulumi.String("https://argocd.ediri.cloud"),
				},
				"extraArgs": pulumi.Array{
					pulumi.String("--insecure"),
				},
				"ingress": pulumi.Map{
					"ingressClassName": pulumi.String("nginx"),
					"hosts": pulumi.Array{
						pulumi.String("argocd.ediri.cloud"),
					},
					"enabled": pulumi.Bool(true),
					"annotations": pulumi.Map{
						"external-dns.alpha.kubernetes.io/hostname": pulumi.String("argocd.ediri.cloud"),
						"external-dns.alpha.kubernetes.io/ttl":      pulumi.String("60"),
					},
				},
			},
			"repoServer": pulumi.Map{
				"serviceAccount": pulumi.Map{
					"automountServiceAccountToken": pulumi.Bool(true),
				},
			},
		},
	}, pulumi.Provider(p.provider), pulumi.Parent(argocdNS))
	if err != nil {
		return err
	}
	return nil
}

func (p *ProviderDependency) createExternalDns() error {
	externalDNSNS, err := v1.NewNamespace(p.ctx, "external-dns", &v1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("external-dns"),
		},
	}, pulumi.Provider(p.provider))
	if err != nil {
		return err
	}

	scw := config.New(p.ctx, "scaleway")

	_, err = v1.NewSecret(p.ctx, "external-dns-credentials", &v1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("external-dns-credentials"),
			Namespace: externalDNSNS.Metadata.Name(),
		},
		StringData: pulumi.StringMap{
			"access_key": pulumi.String(scw.Get("access_key")),
			"secret_key": pulumi.String(scw.Get("secret_key")),
		},
		Type: pulumi.String("Opaque"),
	}, pulumi.Provider(p.provider), pulumi.Parent(externalDNSNS))
	if err != nil {
		return err
	}
	return nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cluster, err := scaleway.NewKubernetesCluster(ctx, "vcluster-webinar", &scaleway.KubernetesClusterArgs{
			Name:    pulumi.String("vcluster-webinar"),
			Version: pulumi.String("1.23"),
			Region:  pulumi.String("fr-par"),
			Cni:     pulumi.String("cilium"),
			FeatureGates: pulumi.StringArray{
				pulumi.String("HPAScaleToZero"),
			},
			Tags: pulumi.StringArray{
				pulumi.String("pulumi"),
			},
			AutoUpgrade: &scaleway.KubernetesClusterAutoUpgradeArgs{
				Enable:                     pulumi.Bool(true),
				MaintenanceWindowStartHour: pulumi.Int(3),
				MaintenanceWindowDay:       pulumi.String("sunday"),
			},
			AdmissionPlugins: pulumi.StringArray{
				pulumi.String("AlwaysPullImages"),
			},
		})
		if err != nil {
			return err
		}
		pool, err := scaleway.NewKubernetesNodePool(ctx, "vcluster-webinar-pool", &scaleway.KubernetesNodePoolArgs{
			Zone:        pulumi.String("fr-par-1"),
			Name:        pulumi.String("vcluster-webinar-pool"),
			NodeType:    pulumi.String("DEV1-L"),
			Size:        pulumi.Int(1),
			Autoscaling: pulumi.Bool(true),
			MinSize:     pulumi.Int(1),
			MaxSize:     pulumi.Int(3),
			Autohealing: pulumi.Bool(true),
			ClusterId:   cluster.ID(),
		})
		if err != nil {
			return err
		}
		ctx.Export("cluster_id", cluster.ID())
		ctx.Export("kubeconfig", pulumi.ToSecret(cluster.Kubeconfigs.Index(pulumi.Int(0)).ConfigFile()))

		provider, err := kubernetes.NewProvider(ctx, "kubernetes", &kubernetes.ProviderArgs{
			Kubeconfig: cluster.Kubeconfigs.Index(pulumi.Int(0)).ConfigFile(),
		}, pulumi.Parent(pool))

		dep := &ProviderDependency{
			ctx:      ctx,
			provider: provider,
		}
		if err != nil {
			return err
		}
		err = dep.createExternalDns()
		if err != nil {
			return err
		}
		err = dep.createArgoCD()
		if err != nil {
			return err
		}
		return nil
	})
}
