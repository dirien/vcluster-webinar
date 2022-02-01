> This blog entry,was part of my webinar for the Cloud Native Islamabad meetup (27.01.2022)

## TL;DR Code

https://github.com/dirien/vcluster-webinar

## Introduction

In this blog entry, I will show you how to create build a cloud native Kubernetes cluster vending machine with vcluster.

It's a wrapup of my webinar for the Cloud Native Islamabad meetup (27.01.2022). If you did not see it. You can watch it here:

%[https://www.youtube.com/watch?v=GkaqN8urStg]

## Prerequisites

- You need to have a Scaleway account.
- `kubectl` and `helm` cli must be installed.
- `pulumi` cli must be installed, and you need to have a pulumi account to save the state there.
- `task` cli must be installed (for using Taskfile.yaml as Makefile alternative).

I will use Scaleway as the cloud provider for our managed Kubernetes cluster (called Kosmos)

## Taskfile

Task is a task runner / build tool that aims to be simpler and easier to use than, for example, GNU Make.

Our taskfile looks like this:

````yaml
# https://taskfile.dev

version: '3'

tasks:
  workaround:
    dir: workaround/kube-prometheus-stack-crds
    env:
      KUBECONFIG: ../../infrastructure/controlplane-scaleway/kubeconfig.yaml
    cmds:
      - cmd: kubectl delete -k .
        ignore_error: true
      - kubectl create -k .

  applications:
    dir: applications/
    env:
      KUBECONFIG: ../infrastructure/controlplane-scaleway/kubeconfig.yaml
    cmds:
      - kubectl apply -f applications.yaml

  infrastructure:
    dir: infrastructure/controlplane-scaleway
    env:
      KUBECONFIG: kubeconfig.yaml
    cmds:
      - pulumi up -y
      - pulumi stack output kubeconfig --show-secrets > kubeconfig.yaml

  get-service-cidr:
    env:
      KUBECONFIG: infrastructure/controlplane-scaleway/kubeconfig.yaml
    cmds:
      - kubectl create service clusterip test --clusterip 1.1.1.1 --tcp=80:80
````


## Scaleway

Scaleway is a cloud provider with a variety of services. Scaleway Elements is their Cloud Platform offering, with
everything you need to run your workload in the Cloud. From virtual machines to serverless functions, you will find a
huge range of services.

You can quickly register here to get an [account](https://console.scaleway.com/register).

### Access And Secret Key

Generate a new access and secret key (https://console.scaleway.com/project/credentials)

And export the keys as ENV variables:

```
export ACCESS_KEY=xxx
export SECRET_KEY=yyy
export ORGANISATION_ID=zzz
```

See https://www.scaleway.com/en/docs/generate-api-keys/ for even more details on how Scaleway Credentials work.

## Pulumi

Pulumi is an open source infrastructure as code tool for creating, deploying, and managing cloud infrastructure. Pulumi
works with traditional infrastructure like VMs, networks, and databases, in addition to modern architectures, including
containers, Kubernetes clusters, and serverless functions. Pulumi supports dozens of public, private, and hybrid cloud
service providers.

In this tutorial, we will use for example golang as our programming language.

### Install Pulumi

Installing Pulumi is easy, just head over to the [get-stated](https://www.pulumi.com/docs/get-started/install/) website
and chose the appropriate version and way to download the cli. To store your state files, you can use their
free [SaaS](https://app.pulumi.com/signin?reason=401) offering

## Create The Pulumi Infrastructure Program

Let us start with the main Pulumi program. Just create run following commands in your terminal inside your project
folder (I use `infrastructure` as the folder):

```bash
pulumi new go
```

Follow the instructions to create a new project with golang as programing language and fill in the required information.

Pulumi offers plenty of templates, if you are unsure of what to use, just use type following command and choose from the
huge collection of templates:

```bash
pulumi new
Please choose a template:  [Use arrows to move, enter to select, type to filter]
aws-csharp                   A minimal AWS C# Pulumi program
aws-go                       A minimal AWS Go Pulumi program
aws-javascript               A minimal AWS JavaScript Pulumi program
aws-python                   A minimal AWS Python Pulumi program
aws-typescript               A minimal AWS TypeScript Pulumi program
azure-csharp                 A minimal Azure Native C# Pulumi program
azure-go                     A minimal Azure Native Go Pulumi program
azure-javascript             A minimal JavaScript Pulumi program with the native Azure provider
azure-python                 A minimal Azure Native Python Pulumi program
....
```

## Add The Scaleway Provider

The Scaleway provider binary is a third party binary. It can be installed using the pulumi plugin command.

```bash
pulumi plugin install resource scaleway v0.1.8 --server https://dl.briggs.work/pulumi/releases/plugins
```

Then you can add the go module via:

```bash
go get github.com/jaxxstorm/pulumi-scaleway/sdk/go/scaleway
```

And finally we add the Scaleway Credentials via following commands to the Pulumi config:

```bash
pulumi config set scaleway:access_key YYYY --secret
pulumi config set scaleway:secret_key ZZZZ --secret
```

More details -> https://www.pulumi.com/registry/packages/scaleway/

## The Scaleway Infrastructure Deployment

The deployment of the Scaleway infrastructure is really the bare minimum. We want to focus more on the deployment of the `vcluster`.

```go
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
```

Nothing special here, we create the cluster in the `fr-par` zone, with the `DEV1-L` node type. To be on the safe side, I activate the Autoscaling feature, and set the minimum and maximum size to 1 and 3. You can adjust this for your needs.

You can deploy the cluster using the following command:

```bash
task infrastructure
```

## The Minimum ArgoCD Installation

Now comes the funny part, we're going to install ArgoCD on the cluster. But just the bare minimum we need to. We want to deploy everything else via GitOps. Including the ArgoCD.

```go
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
```

The function `createExternalDns` is to install the Scaleway secrets as Kubernetes secret via the Pulumi way. So no need for an extra
vault solution.

The next function `createArgoCD` is to deploy the ArgoCD on the cluster. But really the bare minimum we need to.

```go
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
```

## Deploy The Rest Of The Application, Using GitOps

Now you can call:

```bash
task applications
```

this will deploy the missing applications, via GitOps through ArgoCD. We define the ArgoCD application like this:

````yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: applications
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "99"
spec:
  destination:
    name: in-cluster
    namespace: argocd
  project: default
  source:
    path: applications
    repoURL: https://github.com/dirien/vcluster-webinar.git
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
````

Pointing to our `kustomization.yaml` file in the `applications` directory.

There we composite the applications, we want to deploy. You could have a separate GItOps repository for this too.

We install following applications:

- argocd
- cert-manager
- external-dns
- ingress-nginx
- kube_prometheus_stack
- vcluster

> Note: To use Ingress for the `vcluster`, we need to enable passthrough-mode in the `ingress-nginx` Helm chart.

````yaml
...
helm:
  values: |-
    ...
    controller:
      extraArgs:
        enable-ssl-passthrough: ""
      ...
chart: ingress-nginx
...
````

We follow here the `App of Apps` approach. You can find more about this interesting pattern here -> https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern

![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643747987777/Lkw3CtCu5j.png)

### ArgoCD waves

It's important here to mention, that we are using the `argocd.argoproj.io/sync-wave` annotation to define the different waves.

This is important, because we want to deploy the applications in the right order. This important, if we have some dependencies between the applications.
For example Prometheus with the ServiceMonitor.

To define the waves, we use the `argocd.argoproj.io/sync-wave` annotation and chose the wave number. The wave number is the order in which the applications will be deployed.

```yaml
metadata:
  name: kube-prometheus-stack
  annotations:
    argocd.argoproj.io/sync-wave: "1"
```

## vcluster

Virtual clusters are fully working Kubernetes clusters that run on top of other Kubernetes clusters. Compared to fully separate "real" clusters, virtual clusters do not have their own node pools. Instead, they are scheduling workloads inside the underlying cluster while having their own separate control plane.

If you get Inception vibes, welcome to the party!
![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643749164726/GUnnylnMH.png)

If you want to know more, I recomend to watch this latest session of `Containers from the Couch` with Justin, Rich and Lukas

%[https://www.youtube.com/watch?v=a8fIyUd9438]

I created a folder called `vcluster`. And here we are going to use the `App of apps` approach again. You probably spotted the
`vcluster` application in the `applications` folder. Here we are pointing to the `kustomization.yaml` file, this time in the `vcluster` folder.

````yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: vcluster
  annotations:
    argocd.argoproj.io/sync-wave: "99"
spec:
  destination:
    name: in-cluster
    namespace: argocd
  project: default
  source:
    path: vcluster
    repoURL: https://github.com/dirien/vcluster-webinar.git
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
````

![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643748035435/YmUHobM2s.png)

Inside the `vcluster` folder we define different ArgoCD applications. Depending on the type of `vcluster` we want to deploy.
We use `kusomization.yaml` again to glue the ArgoCD applications together. The structure of the folders is completely up to you.


Currently `vcluster` needs, when installed via `helm` the CIDR range provided by the `serviceCIDR` flag.

To get the CIDR range, we use following target in our `taskfile`:

```bash
task get-service-cidr

error: failed to create ClusterIP service: Service "test" is invalid: spec.clusterIPs: Invalid value: []string{"1.1.1.1"}: 
failed to allocate IP 1.1.1.1: the provided IP (1.1.1.1) is not in the valid range. The range of valid IPs is 10.32.0.0/12
```

The valid IP range will be displayed in the `taskfile` output. Here it is `10.32.0.0/12`

Here an example of a `vcluster` application, using `k0s`:

````yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: team-4
  labels:
    team: team-4
    department: development
    project: backend
    distro: k3s
    version: v1.21.4-k3s1
  annotations:
    argocd.argoproj.io/sync-wave: "99"
spec:
  destination:
    name: in-cluster
    namespace: team-4
  project: default
  source:
    repoURL: 'https://charts.loft.sh'
    targetRevision: 0.6.0-alpha.7
    helm:
      values: |-
        rbac:
          clusterRole:
            create: true
          role:
            create: true
            extended: true
        syncer:
          extraArgs:
            - --tls-san=team-4.ediri.cloud
            - --out-kube-config-server=https://team-4.ediri.cloud
        serviceCIDR: 10.32.0.0/12
        ingress:
          enabled: true
          host: team-4.ediri.cloud
          annotations:
            external-dns.alpha.kubernetes.io/hostname: team-4.ediri.cloud
            external-dns.alpha.kubernetes.io/ttl: "60"
    chart: vcluster
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
````

![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643748121500/XtoGzSoAv.png)

As we deployed an ingress controller and external DNS, we can use this to access the `vcluster`.

This is absolut brilliant, as we can now order via git pull requests new cluster.

### Accessing the vcluster

I am going to use the `vcluster` cli here. But you could also get the `kubeconfig` from your Ops Team

```bash
vcluster connect team-4 -n team-4 --server=https://team-4.ediri.cloud
[done] √ Virtual cluster kube config written to: ./kubeconfig.yaml. You can access the cluster via `kubectl --kubeconfig ./kubeconfig.yaml get namespaces`
```

![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643749033377/8WqkxazQX.png)

Now we can access the cluster as usual, with the `kubectl` command.

```bash
kubectl --kubeconfig ./kubeconfig.yaml get namespaces
NAME              STATUS   AGE
default           Active   5d1h
kube-system       Active   5d1h
kube-public       Active   5d1h
kube-node-lease   Active   5d1h
```

Or schedule our workload:

```bash
kubectl run nginx --image=nginx
pod/nginx created

kubectl port-forward pod/nginx 8080:80
Forwarding from 127.0.0.1:8080 -> 80
Forwarding from [::1]:8080 -> 80
Handling connection for 8080


curl localhost:8080
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

## Monitoring

With our monitoring stack, we can monitor our `vcluster`, very comfortably. Just head over to the Grafana and browse the dashboard you need.

![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643748267309/mCJEsGKTT.png)

## Some links

- https://github.com/dirien/vcluster-webinar
- https://www.vcluster.com/
- https://taskfile.dev/
- https://argoproj.github.io/argo-cd/
- https://www.scaleway.com/

![image.png](https://cdn.hashnode.com/res/hashnode/image/upload/v1643749091015/wzybDIS81.png)
