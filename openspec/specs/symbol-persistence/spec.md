# Symbol Persistence Specification

## Purpose
Define the metadata and storage requirements for symbols in the Scouter graph to enable accurate cross-package analysis.

## Requirements

### Requirement: Fully Qualified Symbol Identity

The system MUST store the fully qualified package path (e.g., `github.com/user/repo/pkg`) for every symbol in the SQLite store to distinguish between identical names in different packages.

#### Scenario: Cross-Package Indexing

- GIVEN a project with packages `pkg/a` and `pkg/b`
- WHEN the parser indexes symbols in `pkg/a`
- THEN the `package_path` column in the database MUST contain `github.com/user/repo/pkg/a` for those symbols.

### Requirement: Receiver Metadata

The system MUST store the receiver type (Pointer vs Value) for method symbols to enable accurate method set comparison during interface satisfaction analysis.

#### Scenario: Method Receiver Storage

- GIVEN a method `func (t T) ValueMethod()` and `func (t *T) PointerMethod()`
- WHEN the parser indexes these methods
- THEN the database MUST record that `ValueMethod` has a value receiver and `PointerMethod` has a pointer receiver.
