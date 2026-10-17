# Delta for Symbol Persistence

## ADDED Requirements

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

# Delta for Ripple Engine (Type Resolution)

## ADDED Requirements

### Requirement: Cross-Package Interface Resolution

The system MUST resolve interface implementations across package boundaries using semantic analysis (`go/types.Implements`).

#### Scenario: Interface Satisfaction Across Packages

- GIVEN an interface `Logger` in package `pkg/log`
- AND a struct `ConsoleLogger` in package `pkg/console` that implements `Logger`
- WHEN Ripple analysis is run for `Logger`
- THEN the system MUST identify `ConsoleLogger` as an implementation even though it is in a different package.

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
