package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// AnonymizeNode traverses the tree from the given node and returns an anonymized S-Expression.
// It masks identifiers and literals to preserve only the structural logic.
func AnonymizeNode(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}

	kind := node.Type(lang)

	// Normalize method and function declarations to match specs
	if kind == "method_declaration" || kind == "method_spec" || kind == "method_signature" || kind == "method_definition" || strings.HasPrefix(kind, "method_") {
		kind = "method"
	}
	if kind == "function_declaration" || kind == "function_definition" {
		kind = "function"
	}

	// Masking Identifiers
	if strings.Contains(kind, "identifier") {
		return "[IDENT]"
	}

	// Masking Literals
	if strings.Contains(kind, "literal") || kind == "string" || kind == "number" || kind == "integer" || kind == "float" {
		return "[LIT]"
	}

	// If it's a leaf node (no children), return its kind
	childCount := int(node.ChildCount())
	if childCount == 0 {
		return kind
	}

	// Recursive traversal for all children
	var children []string
	var seenName bool
	for i := 0; i < childCount; i++ {
		child := node.Child(int(i))
		if child != nil {
			childKind := child.Type(lang)

			// Normalize method and function definitions by stripping receiver and body.
			// This makes struct methods structurally match interface methods.
			if kind == "method" || kind == "function" {
				if childKind == "block" || childKind == "func" {
					continue // Ignore function body
				}
				if childKind == "field_identifier" || childKind == "identifier" {
					seenName = true
				}
				// Skip parameter_list if we haven't seen the name yet (this is the receiver)
				if childKind == "parameter_list" && !seenName {
					continue
				}
			}

			anonymized := AnonymizeNode(child, source, lang)
			if anonymized != "" {
				children = append(children, anonymized)
			}
		}
	}

	if len(children) == 0 {
		return kind
	}

	return fmt.Sprintf("(%s %s)", kind, strings.Join(children, " "))
}

// GetStructuralHash generates a SHA-256 hash of the anonymized structural representation of a node.
func GetStructuralHash(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	sExpr := AnonymizeNode(node, source, lang)
	hash := sha256.Sum256([]byte(sExpr))
	return hex.EncodeToString(hash[:])
}
