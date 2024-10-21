# Remove svclb daemonset

Date: 2024-10-21

## Status

Approved

## Context

Services of type LoadBalancer have, among others, one extra configuration parameter called externalTrafficPolicy. It can have two different values: cluster or local. The default is cluster and we support it. The problem is with local.

When using externalTrafficPolicy=local, three things must happen:
1 - The client source IP must be preserved
2 - The traffic must stay local, i.e. service must be served by a pod running in the node where the request came
3 - If the request ingresses via a node where no pod server is running, request should fail

Our current klipper-lb solution is unfortunately not honoring any of those requirements. I will demonstrate this.

To make use of klipper-lb pipelines, we must deploy k3s with a configured node-external-ip. That IP must be a natted IP by the cloud provider, so that traffic reaching the node does not include the IP. This is common in public clouds like AWS or Azure, where node-external-ip is the public IP of the instance.

Imagine a deployment of K3s with three nodes:

Node1:
    * internal-ip: 10.1.1.12
    * exernal-ip: 20.224.75.242

Node2:
    * internal-ip: 10.1.1.16
    * exernal-ip: 20.224.75.227

Node3:
    * internal-ip: 10.1.1.17
    * exernal-ip: 20.13.19.83

Now we deploy an application that includes a deployment with 2 replicas. These replicas run on Node1 and Node2. That application includes a Kubernetes service of type LoadBalancer and externalTrafficPolicy=local. This is the service:

```
NAME         TYPE           CLUSTER-IP     EXTERNAL-IP                   PORT(S)          AGE   SELECTOR
httpbin      LoadBalancer   10.43.43.218   20.224.75.227,20.224.75.242   8000:31355/TCP   25m   app=httpbin
```
The external-ips are correct since servers are running in Node1 and Node2.

The app is a networking toolset and if we query the url `/ip`, we will get the source IP. If we query the three public IPs:

```
$> curl 20.224.75.227:8000/ip
{
  "origin": "10.42.1.6"
}

$> curl 20.224.75.242:8000/ip
{
  "origin": "10.42.0.10"
}

$> curl 20.13.19.83:8000/ip
{
  "origin": "10.42.3.3"
}
```
Here we can already observe how it does not preserve the source IP. That source IP is the IP of the local svclb pod. The problem is that klipper-lb is applying [MASQUERADE](https://github.com/k3s-io/klipper-lb/blob/master/entry#L72) on the traffic and thus replacing the sourceIP.

Moreover, we can observe how curl is working on Node3, that should not happen because there is no local httpbin pod. The reason for this is because our implementation of svclb deploys a daemonset, i.e. one pod in each node (unless the node has the service port unavailable). Our implementation is correctly showing only 2 external-ips for the service but fails to remove the networking pipelines in the other node, so traffic can reach the service. And it can reach the service because we are not honoring the 2nd requirement: traffic must stay local.

To demonstrate that traffic does not stay local, we must deep dive into iptables. When traffic leaves the svclb, remember the traffic has been masqueraded, so the sourceIP now is the IP of the svclb pod, i.e. it is within the clusterCIDR range, this is important for an upcoming explanation. Apart from that, the destinationIP and the destinationPort are [DNAT using the nodeIP and the nodePort port](https://github.com/k3s-io/k3s/blob/master/pkg/cloudprovider/servicelb.go#L558-L569). We can see this if we do tcpdump on the svclb pod:

```
13:29:38.546244 IP 92.186.228.99.46846 > 10.42.3.3.8000: Flags [S], seq 2377127606, win 64240, options [mss 1460,sackOK,TS val 1653522051 ecr 0,nop,wscale 7], length 0
13:29:38.546268 IP 10.42.3.3.46846 > 10.1.1.17.31355: Flags [S], seq 2377127606, win 64240, options [mss 1460,sackOK,TS val 1653522051 ecr 0,nop,wscale 7], length 0
13:29:38.547651 IP 10.1.1.17.31355 > 10.42.3.3.46846: Flags [S.], seq 4157657780, ack 2377127607, win 64308, options [mss 1410,sackOK,TS val 4233562759 ecr 1653522051,nop,wscale 7], length 0
13:29:38.547659 IP 10.42.3.3.8000 > 92.186.228.99.46846: Flags [S.], seq 4157657780, ack 2377127607, win 64308, options [mss 1410,sackOK,TS val 4233562759 ecr 1653522051,nop,wscale 7], length 0
```

`92.186.228.99` ==> client IP. It's a public IP
`10.42.3.3` ==> It's the IP of the svclb pod running in Node3

In the first packet, we see the request. In the second packet we can see the same packet but after the DNAT and the MASQ. This is what happened:

`92.186.228.99` became `10.42.3.3` because of MASQ. Note that `10.42.3.3` is the IP of svclb interface
`10.42.3.3:8000` became `10.1.1.17:31355` because of the DNAT. `10.1.1.17` is the local nodeIP and `31355` is the nodePort to reach the service, as you can observe above, right before the curl commands.

Because the destination IP is now the local nodeIP, we go back to the root network namespace, and there we are hitting the chain `KUBE-NODEPORTS`:
```
Chain KUBE-NODEPORTS (1 references)
target     prot opt source               destination         
KUBE-EXT-UQMCRMJZLI3FTLDP  tcp  --  anywhere             anywhere             /* kube-system/traefik:web */ tcp dpt:32538
KUBE-EXT-CVG3OEGEH7H5P3HQ  tcp  --  anywhere             anywhere             /* kube-system/traefik:websecure */ tcp dpt:31718
KUBE-EXT-FREKB6WNWYJLKTHC  tcp  --  anywhere             anywhere             /* default/httpbin:http */ tcp dpt:31355
```

Since we are interested in `31355` our chain is the last one:
```
Chain KUBE-EXT-FREKB6WNWYJLKTHC (3 references)
target     prot opt source               destination         
KUBE-SVC-FREKB6WNWYJLKTHC  all  --  10.42.0.0/16         anywhere             /* pod traffic for default/httpbin:http external destinations */
KUBE-MARK-MASQ  all  --  anywhere             anywhere             /* masquerade LOCAL traffic for default/httpbin:http external destinations */ ADDRTYPE match src-type LOCAL
KUBE-SVC-FREKB6WNWYJLKTHC  all  --  anywhere             anywhere             /* route LOCAL traffic for default/httpbin:http external destinations */ ADDRTYPE match src-type LOCAL
```

Remember, when we reach this rules, our source IP is in the clusterCIDR range because it is the IP of the local svclb pod. That means we are hitting the first rule `KUBE-SVC-FREKB6WNWYJLKTHC`, which basically load balances, with 0.5 probability, the traffic across the existing two httpbin pods:
```
Chain KUBE-SVC-FREKB6WNWYJLKTHC (3 references)
target     prot opt source               destination         
KUBE-MARK-MASQ  tcp  -- !10.42.0.0/16         10.43.43.218         /* default/httpbin:http cluster IP */ tcp dpt:8000
KUBE-SEP-RUCF7MA7CMMKMMX3  all  --  anywhere             anywhere             /* default/httpbin:http -> 10.42.0.8:80 */ statistic mode random probability 0.50000000000
KUBE-SEP-LZUMEJMLNJBPTY4D  all  --  anywhere             anywhere             /* default/httpbin:http -> 10.42.1.4:80 */
```

In nodes where httpbin pod runs, for example Node2, in KUBE-EXT chain of port `31355`, we see an extra KUBE-SVL chain coming last, which points direclty at the local pod:
```
Chain KUBE-EXT-FREKB6WNWYJLKTHC (3 references)
target     prot opt source               destination         
KUBE-SVC-FREKB6WNWYJLKTHC  all  --  10.42.0.0/16         anywhere             /* pod traffic for default/httpbin:http external destinations */
KUBE-MARK-MASQ  all  --  anywhere             anywhere             /* masquerade LOCAL traffic for default/httpbin:http external destinations */ ADDRTYPE match src-type LOCAL
KUBE-SVC-FREKB6WNWYJLKTHC  all  --  anywhere             anywhere             /* route LOCAL traffic for default/httpbin:http external destinations */ ADDRTYPE match src-type LOCAL
KUBE-SVL-FREKB6WNWYJLKTHC  all  --  anywhere             anywhere       
```

However, since our source IP is in the clusterCIDR range, we are still going through `KUBE-SVC-FREKB6WNWYJLKTHC` and thus the traffic is distributed across all pods and not honoring the rule of traffic having to stay local.


## Solution 1

First of all, we pass an extra env variable to the svclb pods so that we specify if traffic must be MASQ or not. Klipper-lb will read that variable and based on that it will add the MASQ rule in iptables or not. This way, we will honor the rule that the client source IP must be preserved.

Second of all, we will stop using the nodeIP and the nodePort port when externalTrafficPolicy=local. We will still use the regular ClusterIP and the regular Service port. As we are not masquerading anymore, we will hit the `KUBE-SVL` chain that will apply DNAT pointing at the local pod, thus honoring the traffic must stay local rule. Moreover, in nodes where the pod is not running, as there is no `KUBE-SVL`, traffic will get dropped, hence honoring the last rule.

The problem with this solution is that because we are not masquerading, the traffic never goes back to svclb. Note that in svclb is where we are doing the DNAT nodeIP ==> serviceIP. As a consequence, the DNAT is never reversed, so we see traffic leaving the node with the service IP as source IP:

```
13:53:24.325084 IP 92.186.228.99.40512 > 10.1.1.17.8000: Flags [S], seq 864918624, win 64240, options [mss 1460,sackOK,TS val 1654947829 ecr 0,nop,wscale 7], length 0
13:53:24.325224 IP 10.43.7.136.8000 > 92.186.228.99.40512: Flags [S.], seq 3049898895, ack 864918625, win 64308, options [mss 1410,sackOK,TS val 2852268053 ecr 1654947829,nop,wscale 7], length 0
```

`92.186.228.99` is the client IP
`10.1.1.17` is the node IP
`10.43.7.136` is the service IP

Packets are dropped by public clouds as they are expecting `10.1.1.17.8000` to reply, not `10.43.7.136.8000`.

## Solution 2

First of all, we pass an extra env variable to the svclb pods so that we specify if traffic must be MASQ or not. Klipper-lb will read that variable and based on that it will add the MASQ rule in iptables or not. This way, we will honor the rule that the client source IP must be preserved.

Second of all, we will stop using the nodeIP and the nodePort port when externalTrafficPolicy=local. We will direclty point at the local podIP and the port implementing the service. Flannel would MASQ the traffic when it comes back from the pod using the nodeIP.

## Hack to preserve the client IP

Do not define the externalIPs as node-external-IP but still use it to curl the service. As a consequence, kube-proxy pipelines will intercept the packet and handle it correctly 

## Decision

