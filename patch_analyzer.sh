sed -i 's/store.Repository/AnalysisStore/g' internal/engine/analyzer.go
sed -i '/import (/a \
\n// AnalysisStore defines the data requirements for the AnalysisEngine.\ntype AnalysisStore interface {\n\tstore.SymbolRegistry\n\tstore.StructuralGraph\n\tstore.DiagnosticStore\n\tstore.TransactionManager\n}\n' internal/engine/analyzer.go
