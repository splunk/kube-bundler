# kube-bundler
kube-bundler creates and manages self-contained deployment units called application bundles.

Application bundles are files ending in `.kb` that contain all the bits and bytes your service needs to run - from yaml manifests, deployment tools (like helm or an operator), maintenance scripts, docker images, or other code.

_As a bundle creator_, you define the specification for your application bundle, such as configuration values, their defaults, your bundle's cluster resources, and dependencies on other bundles. You choose the preferred deployment toolchain that works best for your service.

_As a bundle user_, you install application bundles on a kubernetes cluster and use the standardized kube-bundler CLI to manage bundles. You don't need any knowledge of the tools or scripts used to perform the deployment.

Once bundles are installed, you are free from the burden of managing specific application components directly with kubectl. You manage applications like cattle, not pets, using the `kb` command.

Sample operations:

* Want to see a summary of current configuration? `kb config list mybundle`

* Need more configuration details, including descriptions, default values, and what's changed? `kb describe install mybundle`

* Need more replicas? `kb config set mybundle replicas=3 && kb deploy bundle mybundle`

* Want to see the differences between the current and running configuration? `kb diff mybundle`

* Want to smoketest the service? `kb smoketest mybundle`

Learn more by [Getting Started](01_getting-started.md)