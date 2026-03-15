package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func meshContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul Mesh, a distributed compute mesh for coordinating AI workloads across nodes. Mesh manages cluster state, node discovery, and workload distribution.

Key capabilities:
- View cluster status including health, leader, and resource utilization.
- List all nodes in the mesh with their roles and capabilities.
- Get detailed information about a specific node.
- Link new nodes to the mesh using invite codes.

Help users manage their compute mesh, troubleshoot node issues, and understand cluster health.`,
		Tools: []stream.Tool{
			{
				Name:        "cluster_status",
				Description: "Get the current cluster status including health, leader node, and aggregate resource utilization.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "list_nodes",
				Description: "List all nodes in the mesh with their roles, status, and capabilities.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "node_info",
				Description: "Get detailed information about a specific node including hardware, workloads, and metrics.",
				InputSchema: mustJSON(`{"type":"object","properties":{"node_id":{"type":"string","description":"ID of the node to inspect"}},"required":["node_id"]}`),
			},
			{
				Name:        "link_node",
				Description: "Link a new node to the mesh using an invite code.",
				InputSchema: mustJSON(`{"type":"object","properties":{"code":{"type":"string","description":"Invite code for linking the node"}},"required":["code"]}`),
			},
		},
	}
}
