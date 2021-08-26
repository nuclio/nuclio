# Node affinity for Nuclio functions
Node affinity can be applied to Nuclio functions to determine on which nodes 
they can be placed. The rules are defined using custom labels on nodes and label selectors. 
Node affinity allows towards Spot or On Demand groups of nodes.

## On Demand vs Spot 

Amazon Elastic Compute Cloud (Amazon EC2) provides scalable computing capacity in the Amazon Web Services (AWS) Cloud. 
Using Amazon EC2 eliminates your need to invest in hardware up front, so you can develop and deploy applications faster. 

Using the Iguazio platform you can deploy two different kinds of EC2 instances, on-demand and spot. 
On-Demand Instances provide full control over the EC2 instance lifecycle. You decide when to launch, stop, hibernate, start, 
reboot, or terminate it. With spot instances you request EC2 capacity from specific availability zones and is 
susceptible to spot capacity availability. This is a good choice if you can be flexible about when your applications run 
and if your applications can be interrupted.

## Stateless and Stateful Applications 
When deploying your Nuclio functions to specific nodes, please take into consideration that on demand 
nodes are best designed to run stateful applications while spot nodes are best designed to stateless applications. 
Nuclio functions which are stateful, and are assigned to run on spot nodes, may be subject to interruption 
and will to be designed so that the job/function state will be saved when scaling to zero.

## Node Selector
Using the **Node Selector** you can assign Nuclio functions to specific nodes within the cluster. 
**Node Selector** is available for all modes of deployment in the platform including the platform UI, 
command line, and programmable interfaces.

To assign Nuclio functions to specific nodes you use the Kubernetes node label 
`app.iguazio.com/lifecycle` with the values of:

* preemptible – assign to EC2 Spot instances

* non-preemptible – assign to EC2 On Demand instances

**Note:**
By default Iguazio uses the key:value pair

```app.iguazio.com/lifecycle = preemptible```

or

```app.iguazio.com/lifecycle = non-preemptible```

to determine spot or on demand nodes.

You can use multiple labels to assign Nuclio functions to specific nodes. 
However, when you use multiple labels a logical `and` is performed on the labels.

**Note:**
* Do not use node specific labels as this may result in eliminating all possible nodes.
* When assigning Nuclio functions to Spot instances it is the user’s responsibility 
  to deal with preempting issues within the running application/function.

**To assign a Nuclio function to a node:**
1. From the platform dashboard, press projects in the left menu pane. 
2. Press on a project, and then press Real time functions (Nuclio).
3. Select a function from the list or press New function to create a new function.
4. Press the Configuration tab.
5. In the Resources pane, in the Node selector section, press Create new entry.

![node selector](/docs/assets/images/nuclio_function_rsources_node_selector.png)

Enter a key value pair.

**Note:** The same key value pairs cannot be entered more than once.

![non-preemtible](/docs/assets/images/nuclio_key_non-preemtible.png)

Or

![preemtible](/docs/assets/images/nuclio_key_preemtible.png)

The key value pair is saved automatically. 