Service Mesh is a means to manage communications between microservices. In a common case, when microservice A makes a call to microservice B, you cannot influence such communication:

<ul style="text-align: left;">
<li>First of all, you cannot change the call destination, replacing microservice B with any other microservice (for example, split microservice B on two microservices and split API between them)</li>
<li>You cannot make any measurement/flow control &ndash; throttling, circuit breaker, security check etc.&nbsp;&nbsp;</li></ul>

![Communication Cases](/docs/images/image2020-2-11-18-22-18.png)

<p style="text-align: left;">Service Mesh introduces such a manageable smart mediator between microservices &ndash; a Gateway. By Gateway you can decouple microservices from their communication rules and control traffic between microservices.</p><ac:structured-macro ac:name="info" ac:schema-version="1" ac:macro-id="56769a29-bab6-4244-b4d3-f218cf77c1a7"><ac:parameter ac:name="title" /><ac:rich-text-body>
<p>Note that NC implementation of service mesh differs from canonical approach. In the industry, service mesh is implemented via Sidecars, that intercepts traffic between microservices on network layer and you cannot make a call between microservices bypassing the gateway. In Cloud Core we use a lite approach &ndash; Gateway is a common microservice and, technically, it does not prohibit microservice-to-microservice communication.</p></ac:rich-text-body></ac:structured-macro><ac:structured-macro ac:name="info" ac:schema-version="1" ac:macro-id="581ad46c-f6a3-427a-b04c-a9a6fa4a6ab7"><ac:rich-text-body>
<p>Gateways have existed in Cloud Core from the very beginning. Service Mesh is an evolution of the former functions.</p></ac:rich-text-body></ac:structured-macro>
<p style="text-align: left;">Gateway roles in the system:</p>
<ul style="text-align: left;">
<li><strong>Aggregation</strong>. Microservice publishes APIs on a gateway &ndash; these only APIs that are published on a gateway could be called by another microservice. Other APIs will be private.&nbsp;</li>
<li><strong> <span style="color: rgb(0,51,102);">Routing</span></strong>. Gateway can decide which microservice should be called to process an incoming request, taking into account the request parameters and headers.&nbsp;</li>
<li><strong>Access points.</strong> <span>&nbsp;</span>Gateway (together with Kubernetes ingress/Openshift route ) is the only recommended approach to expose APIs outside of the cloud.&nbsp;</li>
<li><strong>Traffic control.</strong> <span>&nbsp;</span>Gateway collects traffic metrics, can act as a circuit breaker (with the request retry), and can perform throttling.<span>&nbsp;</span> <u> <span style="color: rgb(0,51,102);">This functionality is available in gateway framework, but not yet implemented in Cloud Core.</span></u></li></ul>
<p style="text-align: left;">In general, the solution will contain several Gateways at a time, and communication will be performed as shown on a picture below.</p>

![Solution Gateways](/docs/images/gateway-roles.png)

<p style="text-align: left;"><span>Despite of all gateways using the same source code (docker image), several roles of a gateway exist:</span></p>
<ul style="text-align: left;">
<li><strong>Public/Private</strong> <strong>gateways </strong>&ndash; are Ingress gateways, i.e. they route the incoming traffic from outside of the cloud to the pods of all families that published their routes to the Internet. By default, the only way to publish API from cloud is to do it through public or private gateway.
<ul>
<li>Public gateway should be used for APIs targeted on end user and that will be exposed to the Internet.</li>
<li>Private gateway should be used for APIs targeted on internal (back-office) staff &ndash; troubleshooters, administrators etc. And usually it is <strong>not </strong>exposed to the Internet.&nbsp;</li></ul></li>
<li><strong>Fa&ccedil;ade</strong><span>&nbsp;</span> <strong>gateways</strong> <span>&nbsp;</span>&ndash; serve a particular family (group of different versions of the same microservices).&nbsp;</li>
<li><strong>Internal</strong>&nbsp;<strong>gateway</strong> &ndash; is the gateway &ldquo;by default&rdquo; &ndash; it is mostly used by Cloud Core, but also could be used by a microservice that does not &quot;want&quot; to have its own fa&ccedil;ade gateway.</li>
<li><strong>Composite gateway &ndash;</strong>&nbsp;<span> the </span>concept between the internal and fa&ccedil;ade gateways, when one gateway serves a set of microservices and acts as API aggregator.&nbsp;</li>
<li><strong>Some dedicated gateway &ndash;</strong><span>&nbsp;</span>not present on the picture, but an application could create additional gateways for a specific role. <ac:inline-comment-marker ac:ref="317d9baa-9dba-400e-8a6e-4b303afe069c">For integration purposes we can create a dedicated gateway and publish the only APIs that are required for the integration. Moreover, if this integration resides in some untrusted network (for example on the Internet), we can configure firewall rules to make this gateway available to this particular only integration system, rather than for the entire Internet.</li>
<li><strong>Egress gateway&nbsp;</strong>&ndash; a dedicated gateway that is used to send requests outside of a cloud for integration purposes. It acts as a mediation between the solution and the external systems, and it deals with authentication, authorization, header routing etc.&nbsp;</li></ul>
<p style="text-align: left;">Note that gateway always communicates with a pod through a <a href="https://kubernetes.io/docs/concepts/services-networking/service/">service&nbsp;</a>(k8s, Openshift), not with a pod directly.&nbsp;&nbsp;</p>
<p style="text-align: left;">Despite of the gateway role, communication is always performed in a sequence:</p>
<p style="text-align: left;"><em>client microservice / external client&nbsp;&rarr; gateway service &rarr;&nbsp;gateway pod &rarr; target microservice service&nbsp;&rarr; target microservice pod</em></p>
<p style="text-align: left;">A service in this sequence acts as a load balancer, rolling upgrade orchestrate, service discovery etc. Thus such an action as pod scaling, pod failover (migration from node to node) is not handled by gateway, i.e. gateway dictates a &quot;static&quot; traffic routing model between the services, while the routing on a pod level is fully covered by Openshift/k8s services and a network layer.&nbsp;</p>
<p style="text-align: left;">Communication patterns:</p>
<ul style="text-align: left;">
<li>Microservice-to-microservice communication is performed via fa&ccedil;ade / internal gateways.</li>
<li>External communication is performed via public/private gateways.</li>
<li>Direct microservice-to-microservice calls (through services, across gateways) are prohibited. But we cannot control and block such attempts, as they cannot be controlled themselves.&nbsp;</li></ul>
<p style="text-align: left;">Traffic routing rules are stored in the gateway Routing Table. A simplified view of routing table:</p>
<table class="wrapped" style="text-align: left;"><colgroup> <col /> <col /> <col /> </colgroup>
<tbody>
<tr>
<th style="text-align: left;">Upstream URL</th>
<th style="text-align: left;">Header parameter</th>
<th style="text-align: left;">Downstream URL</th></tr>
<tr>
<td style="text-align: left;">/getOrder</td>
<td style="text-align: left;">v1</td>
<td style="text-align: left;">http:// order-manager:8080/api/ <strong>v1</strong>/getOrder</td></tr>
<tr>
<td style="text-align: left;"><span>/getOrder</span></td>
<td style="text-align: left;">v2</td>
<td style="text-align: left;"><span>http:// order-manager:8080/api/ <strong>v2</strong>/getOrder</span></td></tr>
<tr>
<td style="text-align: left;"><span>/getOrder</span></td>
<td style="text-align: left;">v3</td>
<td style="text-align: left;">http:// order-manager <strong>-new</strong>:8080/api/<strong>v3</strong>/getOrder</td></tr></tbody></table>
<p style="text-align: left;">where:</p>
<ul style="text-align: left;">
<li>Upstream URL is a URL on gateway itself, thus client microservice will call 'http://gateway-name:8080/&lt;upstreamURL&gt;'.</li>
<li>Header parameter is a parameter of HTTP header of the request.</li>
<li>Downstream URL is a URL of the target REST API to be called.&nbsp;</li></ul>
<p style="text-align: left;">Every microservice should be registered in a gateway, but it does not mean that every microservice should own a gateway. Several microservices could<span>&nbsp;</span> <strong>share a gateway</strong>, when one microservice has a dedicated (fa&ccedil;ade) gateway, while another microservice reuses it and registers its routes in this &quot;foreign&quot; gateway.&nbsp;</p>
<p><br /></p>
