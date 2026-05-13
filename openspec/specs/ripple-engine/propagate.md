# Propagate Specification (Deepened)

## Purpose
Traces the blast radius of a symbol and applies transformations across the project.

## Requirements
### Requirement: Streamed Execution
The system MUST use streaming iterators (Go 1.25 `iter.Seq`) to propagate changes, ensuring constant memory usage regardless of the blast radius size.
#### Scenario: Large Scale Ripple
- GIVEN a symbol with 500+ callers across 100 files
- WHEN the propagate command is executed
- THEN the system MUST process the transformations in a streamed fashion
- AND the peak memory usage SHOULD NOT exceed a defined threshold (e.g., 50MB for metadata).

### Requirement: Deterministic Caller Resolution
The system MUST resolve callers using the LSP-aware `ImpactEngine` to ensure all symbolic references are identified across all file boundaries.
#### Scenario: Cross-Package Rename
- GIVEN a public function `Sum` in package A used in package B
- WHEN `Sum` is renamed to `Add` via RippleEngine
- THEN the system MUST correctly identify and transform the calls in package B.

### Requirement: Cross-Package Interface Resolution
The system SHALL resolve interface implementations across the entire workspace (`./...`) regardless of package boundaries using semantic analysis (`go/types.Implements`).

#### Scenario: Interface Satisfaction Across Packages
- GIVEN an interface `Logger` in package `pkg/log`
- AND a struct `ConsoleLogger` in package `pkg/console` that implements `Logger`
- WHEN Ripple analysis is run for `Logger`
- THEN the system MUST identify `ConsoleLogger` as an implementation even though it is in a different package.

### Requirement: Hierarchical Link Enrichment
The `Linker` MUST detect interface-to-interface embedding and establish bi-directional links in the symbol graph.
- **Bi-directional**: Links must exist for both "Embeds" and "EmbeddedBy".
- **Propagation**: Method changes in an embedded interface MUST propagate to all implementations of any interface that embeds it.

#### Scenario: Deep Inheritance Propagation
- GIVEN Interface `A` with `M1()`.
- AND Interface `B` embeds `A`.
- AND Struct `C` implements `B`.
- WHEN `M1()` in `A` is renamed to `M2()`.
- THEN Impact Analysis MUST identify `Struct C` as a "Broken Implementation".

### Requirement: Method Set Compliance
The system MUST correctly identify interface satisfaction based on Go method set rules.

#### Scenario: Pointer Receiver Validation
- GIVEN an interface `Writer` with method `Write()`
- AND a struct `File` with method `func (f *File) Write()` (pointer receiver)
- WHEN checking if `File` (value) implements `Writer`
- THEN the system MUST report that it does NOT satisfy the interface
- AND WHEN checking if `*File` (pointer) implements `Writer`
- THEN the system MUST report that it DOES satisfy the interface.

### Requirement: Structural Interface Satisfaction
The system MUST support interface satisfaction via embedding and composition.

#### Scenario: Embedded Interface Satisfaction
- GIVEN an interface `ReadWriter` composed of `Reader` and `Writer`
- AND a struct `Buffer` that embeds a struct implementing `Reader` and implements `Writer` directly
- WHEN checking if `Buffer` implements `ReadWriter`
- THEN the system MUST correctly resolve the implementation through the embedded field.
