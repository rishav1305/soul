# soul-mesh

## Overview
Distributed mesh networking library — hub election, WebSocket sync, NAT relay.

## Status
**Ready to Extract** | Source exists in soul-os

## Description
soul-mesh is a peer-to-peer mesh networking layer extracted from soul-os. It enables distributed AI nodes to discover each other, elect hub nodes, synchronize state, and communicate through NAT traversal.

## Key Components
- **Node**: Base mesh node with identity and capabilities
- **Discovery**: Peer discovery via multicast/broadcast + registry
- **Election**: Hub election algorithm for coordinator selection
- **Transport**: WebSocket-based communication layer
- **Sync**: State synchronization across mesh nodes
- **Linking**: NAT relay for nodes behind firewalls

## Source Files (from soul-os)
- `soul-os/brain/mesh/node.py`
- `soul-os/brain/mesh/discovery.py`
- `soul-os/brain/mesh/election.py`
- `soul-os/brain/mesh/transport.py`
- `soul-os/brain/mesh/sync.py`
- `soul-os/brain/mesh/linking.py`

## Extraction Plan
1. Copy mesh module files
2. Replace all `brain.*` imports
3. Create standalone configuration
4. Write clean README with architecture diagram
5. Working example: 2-node mesh demo
6. GitHub Actions CI
7. Publish to PyPI

## Portfolio Signal
Demonstrates distributed systems competence — hub election, NAT relay, WebSocket sync.

## Timeline
Sprint 1 (Week 1-2), parallel with soul-outreach Phase 1.
