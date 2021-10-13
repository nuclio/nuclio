# Node Selector for Nuclio functions
Node affinity can be applied to Nuclio functions to determine on which nodes 
they can be placed. The rules are defined using custom labels on nodes and label selectors. 
Node affinity allows towards Spot or On Demand groups of nodes.

## Node Selector
Using the **Node Selector** you can assign Nuclio functions to specific nodes within the cluster. 
**Node Selector** is available for all modes of deployment in the platform including the platform UI, 
command line, and programmable interfaces.

To assign Nuclio functions to specific nodes you use a Kubernetes label.

You can use multiple labels to assign Nuclio functions to specific nodes. 
However, when you use multiple labels a logical `and` is performed on the labels.

**Note:**
* Do not use node specific labels as this may result in eliminating all possible nodes.
* When assigning Nuclio functions to Spot instances it is the user’s responsibility 
  to deal with preempting issues within the running application/function.

 