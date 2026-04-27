# 🛡️ SDD DESIGN: Scouter v7.0 (The Context OS)

## 🎯 Technical Approach
Integrate Scouter with the Engram persistence layer to realize the 'Context OS' paradigm. This demands stable memory anchoring via predictable `topic_key` schemas, optimizing LLM context bounds through Progressive Disclosure (`Verbose` flag in `handleImpact`), and embedding a background passive learning mechanism in `handleSelfHeal` to autonomously extract and store engineering wisdom.

## 🏛️ Architecture Decisions

### 1. Stable Topic Keys Convention
- **Choice**: Define a strict hierarchical taxonomy for `topic_key`: `{domain}/{context}/{entity}` (e.g., `scouter/risk/{symbol}`, `scouter/fix/{symbol}`). Update the Engram integration to accept and index by this key.
- **Alternatives**: UUID-based topics or unstructured tags.
- **Rationale**: Hierarchical string paths allow `mem_search` to filter context by prefixes efficiently and map entity relationships programmatically without semantic confusion.

### 2. Progressive Disclosure for `handleImpact`
- **Choice**: Introduce a `Verbose` boolean in `ImpactParams`. If `false` (Compact), return only immediate first-level dependents and an aggregated `RiskScore`. If `true` (Full), generate the complete transitive chain with AST previews.
- **Alternatives**: Returning paginated graph responses or compressed JSON blobs.
- **Rationale**: A default `Verbose=false` ensures zero-slop context reduction for routine tasks, aggressively shielding the context window, while preserving on-demand depth for verification (`sdd-verify`).

### 3. Passive Learning in `handleSelfHeal`
- **Choice**: Intercept successful resolutions within `handleSelfHeal` to automatically invoke Engram's `mem_capture_passive` (or `mem_save`). The payload synthesizes the root cause (RCA) and the applied structural fix.
- **Alternatives**: Relying on the agent to manually call `mem_save` after a heal.
- **Rationale**: Automates organizational learning and continuously enriches the Hakaishin Vault. Direct alignment with the Wave 9 Autonomous Sovereignty mandate.

## 🔀 Data Flow
```text
[Client MCP / Agent]
       │
       ▼
 ┌─────────────────┐                ┌──────────────────────┐
 │ handleImpact    ├─(Verbose=F)───►│ Return Compact (AST) │
 │ (ImpactParams)  ├─(Verbose=T)───►│ Return Full Graph    │
 └───────┬─────────┘                └──────────────────────┘
         │
[Diagnostic Event]
         │
         ▼
 ┌─────────────────┐   success      ┌──────────────────────┐
 │ handleSelfHeal  ├───────────────►│ Extract RCA & Fix    │
 └─────────────────┘                └──────────┬───────────┘
                                               │
                                               ▼
                                    ┌──────────────────────┐
                                    │ Engram Storage Layer │
                                    │ topic_key:           │
                                    │ scouter/fix/{symbol} │
                                    └──────────────────────┘
```

## 📂 File Changes (Impact-Verified)
| File | Action | Rationale |
|---|---|---|
| `internal/mcp/handlers.go` | Modify | Update `handleImpact` logic to branch payload density based on `params.Verbose`. Hook `handleSelfHeal` success to passive Engram capture. |
| `internal/engine/executor.go` | Modify | Update `ImpactParams` struct with `Verbose bool` and propagate it to the underlying dependency resolution engine. |
| `internal/store/store.go` | Modify | Implement the topic key taxonomy and routing logic to the Engram persistence API. |

## 🧩 Interfaces / Contracts

```go
// internal/engine/executor.go
type ImpactParams struct {
    SymbolName string `json:"symbolName"`
    FilePath   string `json:"filePath"`
    MaxDepth   int    `json:"maxDepth"`
    Verbose    bool   `json:"verbose"` // New: Toggles Full vs Compact payload
}

// Compact payload definition
type CompactImpactPayload struct {
    Symbol     string   `json:"symbol"`
    RiskScore  float64  `json:"riskScore"`
    Dependents []string `json:"dependents,omitempty"` // Strictly 1st-level
}

// Engram Passive Capture Payload
type HealWisdomPayload struct {
    TopicKey   string `json:"topic_key"` // e.g., "scouter/fix/IndexGraph"
    RootCause  string `json:"root_cause"`
    Resolution string `json:"resolution"`
}
```

## ⚡ Token Efficiency Analysis
- **Full Mode (`Verbose=true`)**: Exposes complete transitive dependency graphs, deep AST node structural snapshots, and caller traces. Estimated cost: **~2,000 to ~4,500 tokens**.
- **Compact Mode (`Verbose=false`)**: Provides a high-level `RiskScore` and immediate dependents only. Estimated cost: **~300 to ~500 tokens**.
- **Net Gain**: Progressive Disclosure yields approximately **80% to 88% token savings** per impact query, satisfying the Pure Signal operational mandate.
